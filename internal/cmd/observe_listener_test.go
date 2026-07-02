package cmd

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// seedCorrelatableSpec writes a minimal spec with a task files contract so a
// payload frame can correlate, without the full test harness (this is a
// white-box package-cmd test).
func seedCorrelatableSpec(t *testing.T, root, slug string) {
	t.Helper()
	dir := core.SpecDir(root, slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	tasks := "# Tasks\n\n- [ ] T1 Charge\n  - files: internal/svc/*.go\n  - requirements: 1\n"
	if err := core.AtomicWrite(filepath.Join(dir, "tasks.md"), tasks); err != nil {
		t.Fatal(err)
	}
	st := core.InitialState(slug, "Svc")
	st.Status = core.StatusExecuting
	if err := core.SaveState(root, slug, &st); err != nil {
		t.Fatal(err)
	}
}

// TestObserveHandleAuthAndApply drives the inbound handler directly: bad method,
// missing/wrong token, oversize body, malformed payload, and the happy path that
// writes a midreq entry.
func TestObserveHandleAuthAndApply(t *testing.T) {
	root := t.TempDir()
	seedCorrelatableSpec(t, root, "billing")
	const token = "s3cret"
	body := `{"service":"billing","environment":"prod","severity":"error","message":"boom","frames":[{"file":"internal/svc/charge.go","line":1}]}`

	// Wrong method → 405.
	rec := httptest.NewRecorder()
	observeHandle(rec, httptest.NewRequest(http.MethodGet, "/errors", nil), root, token, 1<<20, "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET code = %d, want 405", rec.Code)
	}

	// Missing token → 401.
	rec = httptest.NewRecorder()
	observeHandle(rec, httptest.NewRequest(http.MethodPost, "/errors", strings.NewReader(body)), root, token, 1<<20, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no-token code = %d, want 401", rec.Code)
	}

	// Oversize → 413.
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/errors", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	observeHandle(rec, req, root, token, 4, "")
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversize code = %d, want 413", rec.Code)
	}

	// Malformed → 400.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/errors", strings.NewReader("not json"))
	req.Header.Set("Authorization", "Bearer "+token)
	observeHandle(rec, req, root, token, 1<<20, "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("malformed code = %d, want 400", rec.Code)
	}

	// Happy path → 200 + midreq written.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/errors", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	observeHandle(rec, req, root, token, 1<<20, "billing")
	if rec.Code != http.StatusOK {
		t.Fatalf("happy code = %d body=%s", rec.Code, rec.Body.String())
	}
	if b, _ := os.ReadFile(core.ArtifactPath(root, "billing", "mid-requirements.md")); !strings.Contains(string(b), "boom") {
		t.Errorf("midreq not written: %s", b)
	}
}

// TestObserveServerServe drives the real HTTP server end-to-end against a
// loopback listener, then shuts it down — covering the serve path.
func TestObserveServerServe(t *testing.T) {
	root := t.TempDir()
	seedCorrelatableSpec(t, root, "billing")
	const token = "tok"

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := newObserveServer(root, token, 1<<20, "billing")
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	url := "http://" + ln.Addr().String() + "/errors"
	body := `{"service":"billing","environment":"prod","severity":"critical","message":"served","frames":[{"file":"internal/svc/x.go"}]}`
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if b, _ := os.ReadFile(core.ArtifactPath(root, "billing", "mid-requirements.md")); !strings.Contains(string(b), "served") {
		t.Errorf("served payload not applied: %s", b)
	}
}

// TestPrepareObserveListener covers the config/bind path end-to-end: refusals
// with no token, and a bound loopback listener + live server when configured.
func TestPrepareObserveListener(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".specd"), 0o755)
	seedCorrelatableSpec(t, root, "billing")

	// No token → error, no listener.
	if _, _, err := prepareObserveListener(root, ""); err == nil {
		t.Error("no-token prepare should error")
	}

	// Configure a token and bind loopback:0.
	_ = os.WriteFile(filepath.Join(root, ".specd", "config.yml"),
		[]byte("version: 1\nobserve:\n  token: tok\n  addr: 127.0.0.1:0\n"), 0o644)
	ln, srv, err := prepareObserveListener(root, "billing")
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer ln.Close()
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	body := `{"service":"billing","severity":"error","message":"prepared","frames":[{"file":"internal/svc/x.go"}]}`
	req, _ := http.NewRequest(http.MethodPost, "http://"+ln.Addr().String()+"/errors", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestBearerOK(t *testing.T) {
	if !bearerOK("Bearer abc", "abc") {
		t.Error("valid bearer rejected")
	}
	for _, bad := range []string{"", "abc", "Bearer wrong", "Basic abc"} {
		if bearerOK(bad, "abc") {
			t.Errorf("bearerOK(%q) = true, want false", bad)
		}
	}
}

func TestIsLoopbackAddr(t *testing.T) {
	for _, ok := range []string{"127.0.0.1:0", "localhost:8080", "[::1]:9000"} {
		if !isLoopbackAddr(ok) {
			t.Errorf("isLoopbackAddr(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"0.0.0.0:80", "192.168.1.5:80", "example.com:80", "garbage"} {
		if isLoopbackAddr(bad) {
			t.Errorf("isLoopbackAddr(%q) = true, want false", bad)
		}
	}
}
