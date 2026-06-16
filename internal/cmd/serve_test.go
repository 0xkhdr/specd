package cmd_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/cmd"
	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// TestServeParity asserts the live dashboard's GET /s/<slug> renders
// byte-identical HTML to `specd report --format html`, so the served report can
// never silently drift from the canonical static report.
func TestServeParity(t *testing.T) {
	h := th.New(t)
	validSpec(h, "auth", core.StatusExecuting)

	// Canonical static report.
	out := h.Path("report.html")
	h.RunExpect(core.ExitOK, "report", "auth", "--format", "html", "--out", out)
	static := h.ReadFile("report.html")

	// Served view.
	srv := httptest.NewServer(cmd.NewServeHandler(h.Root, "auth"))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/s/auth")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if string(body) != static {
		t.Errorf("served HTML differs from static report\nserved len=%d static len=%d", len(body), len(static))
	}
}

// TestServeIndex confirms GET / lists every spec under the project with a link
// to its report.
func TestServeIndex(t *testing.T) {
	h := th.New(t)
	validSpec(h, "auth", core.StatusExecuting)
	validSpec(h, "billing", core.StatusExecuting)

	srv := httptest.NewServer(cmd.NewServeHandler(h.Root, "auth"))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	html := string(body)
	for _, want := range []string{`href="/s/auth"`, `href="/s/billing"`} {
		if !strings.Contains(html, want) {
			t.Errorf("index missing %q", want)
		}
	}
}

// TestServeReadOnly confirms the handler exposes no mutating routes and fails
// cleanly (no panic) on a missing spec.
func TestServeReadOnly(t *testing.T) {
	h := th.New(t)
	validSpec(h, "auth", core.StatusExecuting)
	srv := httptest.NewServer(cmd.NewServeHandler(h.Root, "auth"))
	defer srv.Close()

	// Non-GET is rejected on every route.
	for _, path := range []string{"/", "/s/auth", "/api/report"} {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
			req, _ := http.NewRequest(method, srv.URL+path, nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("%s %s = %d, want 405", method, path, resp.StatusCode)
			}
		}
	}

	// Unknown spec → 404, not a panic.
	resp, err := http.Get(srv.URL + "/s/nope")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown spec GET /s/nope = %d, want 404", resp.StatusCode)
	}
}

// TestServeEventsMount confirms the dashboard mounts the reused frontier SSE
// stream at /events and delivers a well-formed initial snapshot frame.
func TestServeEventsMount(t *testing.T) {
	h := th.New(t)
	validSpec(h, "auth", core.StatusExecuting)

	srv := httptest.NewServer(cmd.NewServeHandler(h.Root, "auth"))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	sc := bufio.NewScanner(resp.Body)
	var data string
	for sc.Scan() {
		if line := sc.Text(); strings.HasPrefix(line, "data: ") {
			data = strings.TrimPrefix(line, "data: ")
			break
		}
	}
	cancel() // disconnect → handler returns

	var ev core.FrontierEvent
	if err := json.Unmarshal([]byte(data), &ev); err != nil {
		t.Fatalf("SSE frame not valid JSON: %q (%v)", data, err)
	}
}
