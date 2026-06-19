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
	return wsServer(t, func(conn *ws.Conn) {
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err := conn.WriteMessage(mt, msg); err != nil {
				return
			}
		}
	})
}

// wsServer starts a server whose /ws route upgrades and hands the Conn to fn.
func wsServer(t *testing.T, fn func(*ws.Conn)) string {
	t.Helper()
	app := goxpress.New()
	app.Get("/ws", func(c *goxpress.Context) error {
		conn, err := ws.Upgrade(c)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()
		fn(conn)
		return nil
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

// writeClientOp writes a masked client frame with an explicit opcode and FIN
// bit, as a conforming client must (all client frames are masked).
func writeClientOp(t *testing.T, conn net.Conn, opcode byte, fin bool, payload []byte) {
	t.Helper()
	var mask [4]byte
	if _, err := rand.Read(mask[:]); err != nil {
		t.Fatalf("rand: %v", err)
	}

	b0 := opcode
	if fin {
		b0 |= 0x80
	}
	n := len(payload)
	frame := make([]byte, 0, 2+4+n)
	frame = append(frame, b0, byte(0x80|n)) // payloads here stay < 126
	frame = append(frame, mask[:]...)
	for i := range payload {
		frame = append(frame, payload[i]^mask[i%4])
	}
	if _, err := conn.Write(frame); err != nil {
		t.Fatalf("write op frame: %v", err)
	}
}

// readServerOp reads a single server frame, returning its opcode and payload.
func readServerOp(t *testing.T, br *bufio.Reader) (byte, []byte) {
	t.Helper()
	var h [2]byte
	if _, err := io.ReadFull(br, h[:]); err != nil {
		t.Fatalf("read frame header: %v", err)
	}
	opcode := h[0] & 0x0f
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
	return opcode, payload
}

func TestWebSocketFragmented(t *testing.T) {
	host := echoServer(t)
	conn, br := dialWS(t, host)
	defer func() { _ = conn.Close() }()

	// Text message split across two frames: opcode text (FIN=0) + continuation
	// (FIN=1).
	writeClientOp(t, conn, 0x1, false, []byte("Hel"))
	writeClientOp(t, conn, 0x0, true, []byte("lo"))

	if _, got := readServerOp(t, br); string(got) != "Hello" {
		t.Errorf("reassembled = %q, want Hello", got)
	}
}

func TestWebSocketPing(t *testing.T) {
	host := echoServer(t)
	conn, br := dialWS(t, host)
	defer func() { _ = conn.Close() }()

	writeClientOp(t, conn, 0x9, true, []byte("pp")) // ping
	op, payload := readServerOp(t, br)
	if op != 0xA { // pong
		t.Errorf("opcode = 0x%x, want pong 0xA", op)
	}
	if string(payload) != "pp" {
		t.Errorf("pong payload = %q, want pp", payload)
	}
}

func TestWebSocketClose(t *testing.T) {
	host := echoServer(t)
	conn, br := dialWS(t, host)
	defer func() { _ = conn.Close() }()

	writeClientOp(t, conn, 0x8, true, nil) // close
	if op, _ := readServerOp(t, br); op != 0x8 {
		t.Errorf("opcode = 0x%x, want close 0x8", op)
	}
}

func TestWebSocketWriteText(t *testing.T) {
	host := wsServer(t, func(conn *ws.Conn) {
		_ = conn.WriteText("greetings")
	})
	conn, br := dialWS(t, host)
	defer func() { _ = conn.Close() }()

	op, payload := readServerOp(t, br)
	if op != 0x1 || string(payload) != "greetings" {
		t.Errorf("WriteText frame = 0x%x %q", op, payload)
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

func TestUpgradeHijackUnsupported(t *testing.T) {
	var upErr error
	app := goxpress.New()
	app.Get("/ws", func(c *goxpress.Context) error {
		_, upErr = ws.Upgrade(c)
		return nil
	})

	// A valid handshake request, but httptest.ResponseRecorder cannot be
	// hijacked, so Upgrade must surface the hijack error.
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	app.ServeHTTP(httptest.NewRecorder(), req)

	if upErr == nil || !strings.Contains(upErr.Error(), "hijack") {
		t.Errorf("err = %v, want hijack error", upErr)
	}
}

func TestUpgradeHeaderValidation(t *testing.T) {
	cases := []struct {
		name    string
		headers map[string]string
		wantSub string
	}{
		{
			name: "bad version",
			headers: map[string]string{
				"Upgrade": "websocket", "Connection": "Upgrade",
				"Sec-WebSocket-Version": "8", "Sec-WebSocket-Key": "x",
			},
			wantSub: "unsupported websocket version",
		},
		{
			name: "missing key",
			headers: map[string]string{
				"Upgrade": "websocket", "Connection": "Upgrade",
				"Sec-WebSocket-Version": "13",
			},
			wantSub: "missing Sec-WebSocket-Key",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var upErr error
			app := goxpress.New()
			app.Get("/ws", func(c *goxpress.Context) error {
				_, upErr = ws.Upgrade(c)
				return nil
			})
			req := httptest.NewRequest(http.MethodGet, "/ws", nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			app.ServeHTTP(httptest.NewRecorder(), req)
			if upErr == nil || !strings.Contains(upErr.Error(), tc.wantSub) {
				t.Errorf("err = %v, want substring %q", upErr, tc.wantSub)
			}
		})
	}
}
