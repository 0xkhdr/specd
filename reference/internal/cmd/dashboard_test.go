package cmd_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"os"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/cmd"
	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// TestDashboardBadMode confirms `specd dashboard --mode <bogus>` fails closed
// with a usage error before binding any socket.
func TestDashboardBadMode(t *testing.T) {
	h := th.New(t)
	validSpec(h, "auth", core.StatusExecuting)
	res := h.RunExpect(core.ExitUsage, "dashboard", "--mode", "bogus")
	if res.Code != core.ExitUsage {
		t.Fatalf("exit = %d, want %d", res.Code, core.ExitUsage)
	}
}

// TestDashboardNoRoot confirms `specd dashboard` outside a specd project returns
// a non-zero exit (never binds a socket) when no .specd root is found.
func TestDashboardNoRoot(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if code := cmd.RunDashboard(cli.ParseArgs(nil)); code == core.ExitOK {
		t.Fatalf("dashboard outside a project returned ExitOK, want non-zero")
	}
}

// TestDashboardBadAddr drives RunDashboard's full setup path (root, mode, addr,
// handler) and confirms an unbindable --addr surfaces as a non-zero exit rather
// than a panic — without leaving a server listening.
func TestDashboardBadAddr(t *testing.T) {
	h := th.New(t)
	validSpec(h, "auth", core.StatusExecuting)
	res := h.Run("dashboard", "--mode", "cost", "--addr", "256.256.256.256:1", "auth")
	if res.Code == core.ExitOK {
		t.Fatalf("unbindable addr returned ExitOK, want non-zero")
	}
}

// TestDashboardHome confirms the unified dashboard home page renders project-wide
// panels aggregated from local state.
func TestDashboardHome(t *testing.T) {
	h := th.New(t)
	validSpec(h, "auth", core.StatusExecuting)
	validSpec(h, "billing", core.StatusExecuting)

	srv := httptest.NewServer(cmd.NewDashboardHandler(h.Root, "auth", "all"))
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
	for _, want := range []string{"unified dashboard", "Specs &amp; waves", `href="/s/auth"`, `href="/s/billing"`} {
		if !strings.Contains(html, want) {
			t.Errorf("dashboard home missing %q", want)
		}
	}
}

// TestDashboardJSONAndModeFilter confirms /api/dashboard emits the JSON
// projection and honours a per-request ?mode= override.
func TestDashboardJSONAndModeFilter(t *testing.T) {
	h := th.New(t)
	validSpec(h, "auth", core.StatusExecuting)

	srv := httptest.NewServer(cmd.NewDashboardHandler(h.Root, "auth", "all"))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/dashboard?mode=cost")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var d core.DashboardData
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if d.Mode != "cost" {
		t.Fatalf("mode = %q, want cost", d.Mode)
	}
	if len(d.Specs) != 1 || d.Specs[0].Slug != "auth" {
		t.Fatalf("specs projection wrong: %+v", d.Specs)
	}
}

// TestDashboardReadOnly confirms the dashboard exposes no mutating routes and
// returns 404 (not a panic) for an unknown spec.
func TestDashboardReadOnly(t *testing.T) {
	h := th.New(t)
	validSpec(h, "auth", core.StatusExecuting)
	srv := httptest.NewServer(cmd.NewDashboardHandler(h.Root, "auth", "all"))
	defer srv.Close()

	for _, path := range []string{"/", "/s/auth", "/api/dashboard"} {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+path, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("POST %s = %d, want 405", path, resp.StatusCode)
		}
	}

	resp, err := http.Get(srv.URL + "/s/nope")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown spec = %d, want 404", resp.StatusCode)
	}
}
