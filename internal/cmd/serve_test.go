package cmd_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xkhdr/specd/internal/cmd"
	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// TestServeParity asserts the live dashboard's GET / renders byte-identical
// HTML to `specd report --format html`, so the served view can never silently
// drift from the canonical static report.
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
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if string(body) != static {
		t.Errorf("served HTML differs from static report\nserved len=%d static len=%d", len(body), len(static))
	}
}

// TestServeReadOnly confirms the handler exposes no mutating routes and fails
// cleanly (no panic) on a missing spec.
func TestServeReadOnly(t *testing.T) {
	h := th.New(t)
	validSpec(h, "auth", core.StatusExecuting)
	srv := httptest.NewServer(cmd.NewServeHandler(h.Root, "auth"))
	defer srv.Close()

	// Non-GET is rejected.
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req, _ := http.NewRequest(method, srv.URL+"/", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("%s / = %d, want 405", method, resp.StatusCode)
		}
	}

	// Missing spec → 404, not a panic.
	missing := httptest.NewServer(cmd.NewServeHandler(h.Root, "nope"))
	defer missing.Close()
	resp, err := http.Get(missing.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("missing spec GET / = %d, want 404", resp.StatusCode)
	}
}
