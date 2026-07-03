package pack

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func sha256HexReg(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func TestParseRegistryIndexValidation(t *testing.T) {
	good := `{"packs":[{"name":"web","url":"https://x/p.json","sha256":"` + strings.Repeat("a", 64) + `"}]}`
	if _, err := ParseRegistryIndex([]byte(good)); err != nil {
		t.Fatalf("valid index rejected: %v", err)
	}
	bad := []string{
		`{"packs":[{"name":"","url":"https://x","sha256":"` + strings.Repeat("a", 64) + `"}]}`,                                                            // empty name
		`{"packs":[{"name":"x","url":"","sha256":"` + strings.Repeat("a", 64) + `"}]}`,                                                                    // empty url
		`{"packs":[{"name":"x","url":"https://x","sha256":"short"}]}`,                                                                                     // bad sha
		`{"packs":[{"name":"x","url":"a","sha256":"` + strings.Repeat("a", 64) + `"},{"name":"x","url":"b","sha256":"` + strings.Repeat("b", 64) + `"}]}`, // dup
		`{"packs":[],"evil":1}`, // unknown field
	}
	for _, raw := range bad {
		if _, err := ParseRegistryIndex([]byte(raw)); err == nil {
			t.Errorf("expected rejection: %s", raw)
		}
	}
}

func TestPackLockDetectsMismatch(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specd"), 0o755); err != nil {
		t.Fatal(err)
	}
	l, err := LoadPackLock(root)
	if err != nil {
		t.Fatal(err)
	}
	shaA := strings.Repeat("a", 64)
	shaB := strings.Repeat("b", 64)
	if err := l.CheckAndPin("web", shaA); err != nil {
		t.Fatalf("first pin: %v", err)
	}
	if err := l.Save(root); err != nil {
		t.Fatal(err)
	}
	// Re-load and re-pin the same digest: fine.
	l2, _ := LoadPackLock(root)
	if err := l2.CheckAndPin("web", shaA); err != nil {
		t.Fatalf("re-pin same digest: %v", err)
	}
	// A different digest under the same name is a hard failure.
	if err := l2.CheckAndPin("web", shaB); err == nil {
		t.Fatal("expected lock mismatch failure")
	}
}

// gitRepoWithIndex builds a non-bare git repo containing registry.json and
// returns its path (usable as a registry git URL over the file transport).
func gitRepoWithIndex(t *testing.T, index string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, registryIndexName), []byte(index), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "t@test.local")
	run("config", "user.name", "t")
	run("config", "commit.gpgsign", "false")
	run("add", "-A")
	run("commit", "-q", "-m", "index")
	return dir
}

func TestResolveFromRegistryE2E(t *testing.T) {
	// A local pack manifest referenced by file:// with its true digest.
	manifest := `{"name":"web","version":"1.0.0","description":"x","files":[{"path":".specd/steering/a.md","content":"hi"}]}`
	packDir := t.TempDir()
	packPath := filepath.Join(packDir, "web.json")
	if err := os.WriteFile(packPath, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	index := `{"packs":[{"name":"web","url":"file://` + packPath + `","sha256":"` + sha256HexReg([]byte(manifest)) + `"}]}`
	registry := gitRepoWithIndex(t, index)

	pk, entry, err := ResolveFromRegistry("web", registry)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if pk.Name != "web" || entry.Name != "web" {
		t.Fatalf("resolved wrong pack: %+v", pk)
	}

	// Unknown name → not found.
	if _, _, err := ResolveFromRegistry("missing", registry); err == nil {
		t.Fatal("expected not-found for unknown pack name")
	}
}

// resolveRegistryPack is exercised directly to cover its transport branches
// without requiring a git registry clone (ResolveFromRegistry needs git).
func TestResolveRegistryPackHTTP(t *testing.T) {
	manifest := `{"name":"web","version":"1.0.0","description":"x","files":[{"path":".specd/steering/a.md","content":"hi"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(manifest))
	}))
	defer srv.Close()

	pk, err := resolveRegistryPack(RegistryEntry{Name: "web", URL: srv.URL, SHA256: sha256HexReg([]byte(manifest))})
	if err != nil {
		t.Fatalf("http resolve: %v", err)
	}
	if pk.Name != "web" {
		t.Fatalf("resolved wrong pack: %+v", pk)
	}
}

func TestResolveRegistryPackHTTPNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()
	if _, err := resolveRegistryPack(RegistryEntry{Name: "web", URL: srv.URL, SHA256: strings.Repeat("a", 64)}); err == nil {
		t.Fatal("expected HTTP non-200 failure")
	}
}

func TestResolveRegistryPackOversize(t *testing.T) {
	big := strings.Repeat("x", (1<<20)+1) // > maxPackBytes
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(big))
	}))
	defer srv.Close()
	_, err := resolveRegistryPack(RegistryEntry{Name: "web", URL: srv.URL, SHA256: strings.Repeat("a", 64)})
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("expected size-limit failure, got %v", err)
	}
}

func TestResolveRegistryPackFileMissing(t *testing.T) {
	_, err := resolveRegistryPack(RegistryEntry{Name: "web", URL: "file:///no/such/pack.json", SHA256: strings.Repeat("a", 64)})
	if err == nil {
		t.Fatal("expected read failure for missing file")
	}
}

func TestResolveRegistryPackBadScheme(t *testing.T) {
	_, err := resolveRegistryPack(RegistryEntry{Name: "web", URL: "ftp://x/p.json", SHA256: strings.Repeat("a", 64)})
	if err == nil || !strings.Contains(err.Error(), "unsupported url scheme") {
		t.Fatalf("expected unsupported-scheme failure, got %v", err)
	}
}

func TestResolveFromRegistryChecksumMismatchHardFails(t *testing.T) {
	manifest := `{"name":"web","version":"1.0.0","description":"x","files":[{"path":".specd/steering/a.md","content":"hi"}]}`
	packDir := t.TempDir()
	packPath := filepath.Join(packDir, "web.json")
	if err := os.WriteFile(packPath, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	// Registry pins a WRONG digest for the manifest.
	index := `{"packs":[{"name":"web","url":"file://` + packPath + `","sha256":"` + strings.Repeat("f", 64) + `"}]}`
	registry := gitRepoWithIndex(t, index)

	if _, _, err := ResolveFromRegistry("web", registry); err == nil {
		t.Fatal("expected SHA256 mismatch hard failure")
	} else if !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("error %q does not mention mismatch", err)
	}
}
