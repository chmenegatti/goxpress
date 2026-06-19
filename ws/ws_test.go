package ws_test

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chmenegatti/goxpress"
	"github.com/chmenegatti/goxpress/ws"
)

// echoServer starts a goXpress server whose /ws route echoes every message back
// to the client, and returns its host:port.
func echoServer(t *testing.T) string {
	t.Helper()
	app := goxpress.New()
	app.Get("/ws", func(c *goxpress.Context) error {
		conn, err := ws.Upgrade(c)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				return nil
			}
			if err := conn.WriteMessage(mt, msg); err != nil {
				return nil
			}
		}
	})

	srv := httptest.NewServer(app)
	t.Cleanup(srv.Close)
	return strings.TrimPrefix(srv.URL, "http://")
}

// dialWS opens a TCP connection and performs the client-side WebSocket
// handshake against host, returning the connection and a buffered reader
// positioned just after the 101 response.
func dialWS(t *testing.T, host string) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, err := net.Dial("tcp", host)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	req := "GET /ws HTTP/1.1\r\n" +
		"Host: " + host + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	if _, err := io.WriteString(conn, req); err != nil {
		t.Fatalf("write handshake: %v", err)
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatalf("read handshake response: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("handshake status = %d, want 101", resp.StatusCode)
	}
	if got := resp.Header.Get("Sec-WebSocket-Accept"); got != "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=" {
		t.Fatalf("Sec-WebSocket-Accept = %q, want s3pPLMBiTxaQ9kYGzzhZRbK+xOo=", got)
	}
	return conn, br
}

// writeClientFrame writes a masked single-frame text message, as a real client
// must.
func writeClientFrame(t *testing.T, conn net.Conn, payload []byte) {
	t.Helper()
	var mask [4]byte
	if _, err := rand.Read(mask[:]); err != nil {
		t.Fatalf("rand: %v", err)
	}

	b0 := byte(0x80 | 0x1) // FIN + text opcode.
	n := len(payload)
	hdr := make([]byte, 0, 8)
	if n < 126 {
		hdr = append(hdr, b0, byte(0x80|n))
	} else {
		hdr = append(hdr, b0, 0x80|126, byte(n>>8), byte(n))
	}
	hdr = append(hdr, mask[:]...)

	frame := make([]byte, 0, len(hdr)+n)
	frame = append(frame, hdr...)
	for i := range payload {
		frame = append(frame, payload[i]^mask[i%4])
	}

	if _, err := conn.Write(frame); err != nil {
		t.Fatalf("write frame: %v", err)
	}
}

// readServerFrame reads a single unmasked server frame and returns its payload.
func readServerFrame(t *testing.T, br *bufio.Reader) []byte {
	t.Helper()
	var h [2]byte
	if _, err := io.ReadFull(br, h[:]); err != nil {
		t.Fatalf("read frame header: %v", err)
	}
	length := uint64(h[1] & 0x7f)
	switch length {
	case 126:
		var ext [2]byte
		_, _ = io.ReadFull(br, ext[:])
		length = uint64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		_, _ = io.ReadFull(br, ext[:])
		length = binary.BigEndian.Uint64(ext[:])
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(br, payload); err != nil {
		t.Fatalf("read payload: %v", err)
	}
	return payload
}

func TestWebSocketEcho(t *testing.T) {
	host := echoServer(t)
	conn, br := dialWS(t, host)
	defer func() { _ = conn.Close() }()

	for _, msg := range []string{"hello", "world", strings.Repeat("x", 200)} {
		writeClientFrame(t, conn, []byte(msg))
		if got := string(readServerFrame(t, br)); got != msg {
			t.Errorf("echo = %q, want %q", got, msg)
		}
	}
}

func TestUpgradeRejectsNonWebSocket(t *testing.T) {
	app := goxpress.New()
	var upErr error
	app.Get("/ws", func(c *goxpress.Context) error {
		_, upErr = ws.Upgrade(c)
		if upErr != nil {
			return c.String(http.StatusBadRequest, "%s", upErr.Error())
		}
		return nil
	})

	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ws", nil))
	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
	if upErr == nil || !strings.Contains(upErr.Error(), "not a websocket") {
		t.Errorf("err = %v, want not-a-websocket", upErr)
	}
}
