// Package ws provides a thin, dependency-free WebSocket (RFC 6455) server
// helper for goXpress, built entirely on the Go standard library.
//
// Upgrade hijacks the connection from a goXpress handler and returns a Conn for
// reading and writing messages:
//
//	app.Get("/ws", func(c *goxpress.Context) error {
//		conn, err := ws.Upgrade(c)
//		if err != nil {
//			return err
//		}
//		defer conn.Close()
//		for {
//			mt, msg, err := conn.ReadMessage()
//			if err != nil {
//				return nil
//			}
//			if err := conn.WriteMessage(mt, msg); err != nil {
//				return nil
//			}
//		}
//	})
//
// The implementation covers the server side: it reads masked client frames,
// reassembles fragmented messages, answers pings, and honors close frames. It
// is intentionally minimal — no per-message compression or subprotocol
// negotiation.
package ws

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/chmenegatti/goxpress"
)

// magicGUID is the fixed value concatenated with Sec-WebSocket-Key to compute
// the Sec-WebSocket-Accept response, per RFC 6455.
const magicGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// maxPayload bounds a single frame's payload to guard against memory blowups
// from a hostile or buggy peer.
const maxPayload = 32 << 20 // 32 MiB

// WebSocket frame opcodes.
const (
	opContinuation = 0x0
	opText         = 0x1
	opBinary       = 0x2
	opClose        = 0x8
	opPing         = 0x9
	opPong         = 0xA
)

// MessageType identifies the kind of a WebSocket data message. Its values match
// the corresponding RFC 6455 opcodes.
type MessageType int

const (
	// TextMessage denotes a UTF-8 text message.
	TextMessage MessageType = opText
	// BinaryMessage denotes a binary message.
	BinaryMessage MessageType = opBinary
)

// ErrClosed is returned by ReadMessage when the peer sends a close frame.
var ErrClosed = errors.New("ws: connection closed by peer")

// Conn is a server-side WebSocket connection. A Conn is safe for one concurrent
// reader and one concurrent writer; writes are serialized internally.
type Conn struct {
	conn net.Conn
	br   *bufio.Reader

	wmu sync.Mutex
}

// Upgrade completes the WebSocket handshake for the request behind c and
// returns a Conn. It hijacks the underlying connection, so after a successful
// Upgrade the handler owns the connection and the router writes nothing further
// for the request.
func Upgrade(c *goxpress.Context) (*Conn, error) {
	r := c.Request
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") ||
		!strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") {
		return nil, errors.New("ws: not a websocket upgrade request")
	}
	if r.Header.Get("Sec-WebSocket-Version") != "13" {
		return nil, errors.New("ws: unsupported websocket version")
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, errors.New("ws: missing Sec-WebSocket-Key")
	}

	conn, brw, err := http.NewResponseController(c.Writer).Hijack()
	if err != nil {
		return nil, fmt.Errorf("ws: hijack: %w", err)
	}

	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + acceptKey(key) + "\r\n\r\n"
	if _, err := brw.WriteString(resp); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := brw.Flush(); err != nil {
		_ = conn.Close()
		return nil, err
	}

	// The connection is hijacked; stop the router from touching the response.
	c.Abort()
	return &Conn{conn: conn, br: brw.Reader}, nil
}

// acceptKey computes the Sec-WebSocket-Accept value for a client key.
func acceptKey(key string) string {
	h := sha1.New() //nolint:gosec // mandated by RFC 6455, not used for security
	_, _ = io.WriteString(h, key+magicGUID)
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// ReadMessage reads the next text or binary message, reassembling fragments. It
// transparently answers ping frames with pongs and discards pongs. When the
// peer sends a close frame it echoes the close and returns ErrClosed.
func (c *Conn) ReadMessage() (MessageType, []byte, error) {
	for {
		fin, opcode, payload, err := c.readFrame()
		if err != nil {
			return 0, nil, err
		}

		switch opcode {
		case opText, opBinary:
			msg := payload
			for !fin {
				var fopcode byte
				fin, fopcode, payload, err = c.readFrame()
				if err != nil {
					return 0, nil, err
				}
				if fopcode != opContinuation {
					return 0, nil, errors.New("ws: expected continuation frame")
				}
				msg = append(msg, payload...)
			}
			return MessageType(opcode), msg, nil
		case opPing:
			if err := c.writeFrame(opPong, payload); err != nil {
				return 0, nil, err
			}
		case opPong:
			// Ignore unsolicited pongs.
		case opClose:
			_ = c.writeFrame(opClose, payload)
			return 0, nil, ErrClosed
		default:
			return 0, nil, fmt.Errorf("ws: unknown opcode 0x%x", opcode)
		}
	}
}

// readFrame reads and unmasks a single WebSocket frame.
func (c *Conn) readFrame() (fin bool, opcode byte, payload []byte, err error) {
	var h [2]byte
	if _, err = io.ReadFull(c.br, h[:]); err != nil {
		return false, 0, nil, err
	}
	fin = h[0]&0x80 != 0
	opcode = h[0] & 0x0f
	masked := h[1]&0x80 != 0
	length := uint64(h[1] & 0x7f)

	switch length {
	case 126:
		var ext [2]byte
		if _, err = io.ReadFull(c.br, ext[:]); err != nil {
			return false, 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err = io.ReadFull(c.br, ext[:]); err != nil {
			return false, 0, nil, err
		}
		length = binary.BigEndian.Uint64(ext[:])
	}

	if length > maxPayload {
		return false, 0, nil, fmt.Errorf("ws: payload %d exceeds limit", length)
	}

	var maskKey [4]byte
	if masked {
		if _, err = io.ReadFull(c.br, maskKey[:]); err != nil {
			return false, 0, nil, err
		}
	}

	payload = make([]byte, length)
	if _, err = io.ReadFull(c.br, payload); err != nil {
		return false, 0, nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}
	return fin, opcode, payload, nil
}

// WriteMessage writes data as a single, unfragmented message of the given type.
func (c *Conn) WriteMessage(mt MessageType, data []byte) error {
	return c.writeFrame(byte(mt), data)
}

// WriteText is a convenience wrapper that sends s as a text message.
func (c *Conn) WriteText(s string) error {
	return c.WriteMessage(TextMessage, []byte(s))
}

// writeFrame writes a single final (FIN) server frame with the given opcode.
// Server-to-client frames are never masked.
func (c *Conn) writeFrame(opcode byte, payload []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()

	var hdr []byte
	b0 := byte(0x80) | opcode // FIN set.
	n := len(payload)
	switch {
	case n < 126:
		hdr = []byte{b0, byte(n)}
	case n < 1<<16:
		hdr = []byte{b0, 126, byte(n >> 8), byte(n)}
	default:
		hdr = make([]byte, 10)
		hdr[0] = b0
		hdr[1] = 127
		binary.BigEndian.PutUint64(hdr[2:], uint64(n))
	}

	if _, err := c.conn.Write(hdr); err != nil {
		return err
	}
	if len(payload) > 0 {
		if _, err := c.conn.Write(payload); err != nil {
			return err
		}
	}
	return nil
}

// Close sends a close frame and closes the underlying connection.
func (c *Conn) Close() error {
	_ = c.writeFrame(opClose, nil)
	return c.conn.Close()
}
