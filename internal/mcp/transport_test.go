package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
)

// TestTransport covers framed read/write in both newline and Content-Length
// modes, plus auto-detection on the first byte.
func TestTransport(t *testing.T) {
	t.Run("newline_framed_round_trip", func(t *testing.T) {
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

	t.Run("content_length_framed_round_trip", func(t *testing.T) {
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

	t.Run("blank_lines_skipped_before_message", func(t *testing.T) {
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

func TestMCPMalformedOversizedStdioRequest(t *testing.T) {
	tooLargeMsg := `{"jsonrpc":"2.0","id":1,"method":"ping","pad":"` + strings.Repeat("x", maxRPCBody) + `"}`
	tooLarge := tooLargeMsg + "\n" + `{"jsonrpc":"2.0","id":2,"method":"ping"}` + "\n"
	var out bytes.Buffer
	err := Serve(strings.NewReader(tooLarge), &out, func(string, cli.Args) (int, bool) { return 0, true }, nil)
	if err != nil {
		t.Fatalf("Serve: %v", err)
	}
	var responses []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var resp map[string]any
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("response not JSON: %q: %v", line, err)
		}
		responses = append(responses, resp)
	}
	if len(responses) != 2 {
		t.Fatalf("got %d responses, want 2: %v", len(responses), responses)
	}
	e, ok := responses[0]["error"].(map[string]any)
	if !ok || e["code"].(float64) != -32600 {
		t.Fatalf("oversized response = %v, want -32600 error", responses[0])
	}
	if _, ok := responses[1]["result"]; !ok {
		t.Fatalf("server did not recover after oversized line: %v", responses[1])
	}
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
