package mcp

import (
	"bytes"
	"strings"
	"testing"
)

// TestTransport covers framed read/write in both newline and Content-Length
// modes, plus auto-detection on the first byte.
func TestTransport(t *testing.T) {
	t.Run("newline framed round trip", func(t *testing.T) {
		in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n")
		var out bytes.Buffer
		c := newConn(in, &out)

		raw, err := c.readMessage()
		if err != nil {
			t.Fatalf("readMessage: %v", err)
		}
		if c.mode != framingNewline {
			t.Fatalf("mode = %d, want newline", c.mode)
		}
		if !strings.Contains(string(raw), `"method":"ping"`) {
			t.Fatalf("raw = %s", raw)
		}

		if err := c.writeMessage(map[string]any{"ok": true}); err != nil {
			t.Fatalf("writeMessage: %v", err)
		}
		if got := out.String(); got != `{"ok":true}`+"\n" {
			t.Fatalf("written = %q", got)
		}
	})

	t.Run("content length framed round trip", func(t *testing.T) {
		body := `{"jsonrpc":"2.0","id":2,"method":"ping"}`
		framed := "Content-Length: " + itoa(len(body)) + "\r\n\r\n" + body
		var out bytes.Buffer
		c := newConn(strings.NewReader(framed), &out)

		raw, err := c.readMessage()
		if err != nil {
			t.Fatalf("readMessage: %v", err)
		}
		if c.mode != framingHeader {
			t.Fatalf("mode = %d, want header", c.mode)
		}
		if string(raw) != body {
			t.Fatalf("raw = %s, want %s", raw, body)
		}

		if err := c.writeMessage(map[string]any{"ok": true}); err != nil {
			t.Fatalf("writeMessage: %v", err)
		}
		if got := out.String(); !strings.HasPrefix(got, "Content-Length: 11\r\n\r\n") || !strings.HasSuffix(got, `{"ok":true}`) {
			t.Fatalf("written = %q", got)
		}
	})

	t.Run("blank lines skipped before message", func(t *testing.T) {
		c := newConn(strings.NewReader("\n\n"+`{"method":"x"}`+"\n"), &bytes.Buffer{})
		raw, err := c.readMessage()
		if err != nil {
			t.Fatalf("readMessage: %v", err)
		}
		if string(raw) != `{"method":"x"}` {
			t.Fatalf("raw = %s", raw)
		}
	})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
