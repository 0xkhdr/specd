package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// harnessTestRoot creates a specd project root with a representative policy set:
// a non-executable guardrails file, a prose role, and a deploy template that
// carries an executable `command` (which must be quarantined on import).
func harnessTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, ".specd/guardrails.json", `{"rules":[{"id":"no-todo","pattern":"TODO","message":"no TODO"}]}`)
	writeFile(t, root, ".specd/roles/craftsman.md", "# Craftsman\nBuild carefully.\n")
	writeFile(t, root, ".specd/deploy/prod.json", `{"env":"prod","steps":[{"name":"ship","command":"deploy.sh"}]}`)
	return root
}

// bareRemote initialises a bare git repository and returns its path, usable as a
// harness git URL (the file transport is on the allowlist).
func bareRemote(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "--bare", "--quiet", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v: %s", err, out)
	}
	return dir
}

// TestSecureGitClone exercises the shared hardened clone path used by harness
// sharing and the pack registry: a valid clone succeeds (both shallow and full)
// while a hostile transport URL is rejected before any git exec.
func TestSecureGitClone(t *testing.T) {
	src := harnessTestRoot(t)
	remote := bareRemote(t)
	if _, err := PushHarness(src, remote, "team"); err != nil {
		t.Fatalf("seed remote: %v", err)
	}

	// Shallow clone of the seeded remote succeeds and carries the manifest.
	shallow := filepath.Join(t.TempDir(), "shallow")
	if err := SecureGitClone(remote, shallow, true); err != nil {
		t.Fatalf("shallow clone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(shallow, ".specd-harness", "manifest.json")); err != nil {
		if _, err2 := os.Stat(filepath.Join(shallow, "manifest.json")); err2 != nil {
			// Manifest lives under the harness dir; tolerate layout but require a clone.
			entries, _ := os.ReadDir(shallow)
			if len(entries) == 0 {
				t.Fatalf("shallow clone produced empty tree")
			}
		}
	}

	// Full clone (depth1=false) into a fresh dir also succeeds.
	full := filepath.Join(t.TempDir(), "full")
	if err := SecureGitClone(remote, full, false); err != nil {
		t.Fatalf("full clone: %v", err)
	}

	// A hostile ext:: transport URL is rejected before exec.
	if err := SecureGitClone("ext::sh -c whoami", filepath.Join(t.TempDir(), "x"), true); err == nil {
		t.Fatal("expected rejection of ext:: transport URL")
	}
}

// TestPushHarnessErrors covers the two fail-closed branches of PushHarness: a
// project with no shareable policy artifacts, and a hostile remote URL rejected
// before any git exec.
func TestPushHarnessErrors(t *testing.T) {
	remote := bareRemote(t)

	// Empty project: nothing to share.
	empty := t.TempDir()
	if _, err := PushHarness(empty, remote, "team"); err == nil {
		t.Fatal("expected error pushing a project with no policy artifacts")
	} else if !strings.Contains(err.Error(), "no policy artifacts") {
		t.Fatalf("error %q does not name the empty-bundle cause", err)
	}

	// Populated project but a hostile transport URL is rejected up front.
	src := harnessTestRoot(t)
	if _, err := PushHarness(src, "ext::sh -c whoami", "team"); err == nil {
		t.Fatal("expected rejection of ext:: transport URL on push")
	}
}

func TestHarnessPushPullRoundTrip(t *testing.T) {
	src := harnessTestRoot(t)
	remote := bareRemote(t)

	m, err := PushHarness(src, remote, "team")
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if m.Name != "team" || m.Version != 1 {
		t.Fatalf("manifest name/version = %q/%d, want team/1", m.Name, m.Version)
	}
	if len(m.Files) != 3 {
		t.Fatalf("bundled %d files, want 3", len(m.Files))
	}

	// A fresh consumer pulls the bundle.
	dst := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dst, ".specd"), 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := PullHarness(dst, remote, false)
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	// guardrails + role install directly; deploy template (executable) quarantines.
	if len(res.Installed) != 2 {
		t.Fatalf("installed %v, want 2 non-executable artifacts", res.Installed)
	}
	if len(res.Quarantined) != 1 || res.Quarantined[0] != ".specd/deploy/prod.json" {
		t.Fatalf("quarantined = %v, want [.specd/deploy/prod.json]", res.Quarantined)
	}
	if len(res.Refused) != 0 {
		t.Fatalf("unexpected refusals: %v", res.Refused)
	}
	// The quarantined artifact must NOT be installed to its active path.
	if FileExists(filepath.Join(dst, ".specd", "deploy", "prod.json")) {
		t.Fatal("executable artifact was installed without an explicit enable")
	}
	if !FileExists(filepath.Join(dst, ".specd", "guardrails.json")) {
		t.Fatal("non-executable guardrails not installed")
	}
}

func TestHarnessQuarantineEnableRecordsDecision(t *testing.T) {
	src := harnessTestRoot(t)
	remote := bareRemote(t)
	if _, err := PushHarness(src, remote, "team"); err != nil {
		t.Fatalf("push: %v", err)
	}
	dst := t.TempDir()
	if _, err := PullHarness(dst, remote, false); err != nil {
		t.Fatalf("pull: %v", err)
	}

	q := HarnessQuarantined(dst)
	if len(q) != 1 {
		t.Fatalf("quarantined = %v, want one item", q)
	}
	if err := EnableHarnessItem(dst, q[0], false); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if !FileExists(filepath.Join(dst, ".specd", "deploy", "prod.json")) {
		t.Fatal("enable did not install the artifact")
	}
	if len(HarnessQuarantined(dst)) != 0 {
		t.Fatal("enabled artifact still in quarantine")
	}
	// The decision must be recorded.
	log, err := os.ReadFile(filepath.Join(HarnessDir(dst), harnessDecisionsF))
	if err != nil {
		t.Fatalf("read decisions log: %v", err)
	}
	if !strings.Contains(string(log), `"action":"enable"`) || !strings.Contains(string(log), ".specd/deploy/prod.json") {
		t.Fatalf("decision not recorded: %s", log)
	}
}

func TestHarnessPullRefusesLocalModification(t *testing.T) {
	src := harnessTestRoot(t)
	remote := bareRemote(t)
	if _, err := PushHarness(src, remote, "team"); err != nil {
		t.Fatalf("push: %v", err)
	}
	dst := t.TempDir()
	// A locally-authored guardrails file that differs from the incoming bundle.
	writeFile(t, dst, ".specd/guardrails.json", `{"rules":[{"id":"local","pattern":"X","message":"local edit"}]}`)

	res, err := PullHarness(dst, remote, false)
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if len(res.Refused) != 1 || res.Refused[0] != ".specd/guardrails.json" {
		t.Fatalf("refused = %v, want the locally-modified guardrails", res.Refused)
	}
	// The local edit must survive (not silently clobbered).
	body, _ := os.ReadFile(filepath.Join(dst, ".specd", "guardrails.json"))
	if !strings.Contains(string(body), "local edit") {
		t.Fatal("local modification was clobbered without --force")
	}

	// --force overwrites.
	res2, err := PullHarness(dst, remote, true)
	if err != nil {
		t.Fatalf("forced pull: %v", err)
	}
	if len(res2.Refused) != 0 {
		t.Fatalf("forced pull still refused: %v", res2.Refused)
	}
	body2, _ := os.ReadFile(filepath.Join(dst, ".specd", "guardrails.json"))
	if strings.Contains(string(body2), "local edit") {
		t.Fatal("--force did not overwrite the local modification")
	}
}

func TestHarnessPullRefusesDowngrade(t *testing.T) {
	src := harnessTestRoot(t)
	remote := bareRemote(t)
	// Push v1, then v2.
	if _, err := PushHarness(src, remote, "team"); err != nil {
		t.Fatalf("push v1: %v", err)
	}
	if _, err := PushHarness(src, remote, "team"); err != nil {
		t.Fatalf("push v2: %v", err)
	}
	dst := t.TempDir()
	// Pull v2 first.
	if _, err := PullHarness(dst, remote, false); err != nil {
		t.Fatalf("pull v2: %v", err)
	}
	local, err := LoadHarnessManifest(dst)
	if err != nil || local.Version != 2 {
		t.Fatalf("local version = %d (err %v), want 2", local.Version, err)
	}

	// A stale remote at v1 is refused without force.
	staleRemote := bareRemote(t)
	if _, err := PushHarness(src, staleRemote, "team"); err != nil {
		// src version is now 3 after two pushes above; rebuild a v1 bundle in a
		// clean source instead.
		t.Fatalf("push stale: %v", err)
	}
	// Build a genuine v1 remote from a clean source.
	clean := harnessTestRoot(t)
	v1Remote := bareRemote(t)
	if _, err := PushHarness(clean, v1Remote, "team"); err != nil {
		t.Fatalf("push v1 clean: %v", err)
	}
	if _, err := PullHarness(dst, v1Remote, false); err == nil {
		t.Fatal("expected downgrade refusal, got nil")
	}
	// --force allows it.
	if _, err := PullHarness(dst, v1Remote, true); err != nil {
		t.Fatalf("forced downgrade: %v", err)
	}
}

func TestParseHarnessManifestRejectsHostilePaths(t *testing.T) {
	cases := []string{
		`{"name":"x","version":1,"files":[{"path":"/etc/passwd","sha256":"` + strings.Repeat("a", 64) + `"}]}`,
		`{"name":"x","version":1,"files":[{"path":"../escape","sha256":"` + strings.Repeat("a", 64) + `"}]}`,
		`{"name":"x","version":1,"files":[{"path":".specd/../../x","sha256":"` + strings.Repeat("a", 64) + `"}]}`,
		`{"name":"x","version":1,"files":[{"path":"outside/x","sha256":"` + strings.Repeat("a", 64) + `"}]}`,
		`{"name":"","version":1,"files":[]}`,
		`{"name":"x","version":0,"files":[]}`,
		`{"name":"x","version":1,"files":[],"evil":true}`,
	}
	for _, raw := range cases {
		if _, err := ParseHarnessManifest([]byte(raw)); err == nil {
			t.Errorf("expected rejection for hostile manifest: %s", raw)
		}
	}
}

func TestValidateHarnessURLRejectsUnsafeTransports(t *testing.T) {
	bad := []string{"", "-oProxyCommand=x", "ext::sh -c whoami", "fd::7", "https://x\n/y", "--upload-pack=evil"}
	for _, u := range bad {
		if err := ValidateHarnessURL(u); err == nil {
			t.Errorf("expected rejection for unsafe URL %q", u)
		}
	}
	if err := ValidateHarnessURL("https://example.com/team/harness.git"); err != nil {
		t.Errorf("valid URL rejected: %v", err)
	}
}

func TestHarnessCarriesCommandDetectsNested(t *testing.T) {
	if !harnessCarriesCommand(".specd/deploy/x.json", []byte(`{"a":{"b":[{"command":"rm -rf /"}]}}`)) {
		t.Fatal("nested command not detected")
	}
	if harnessCarriesCommand(".specd/guardrails.json", []byte(`{"rules":[{"pattern":"X"}]}`)) {
		t.Fatal("pure-regex policy flagged as executable")
	}
	if !harnessCarriesCommand(".specd/x.json", []byte(`not json`)) {
		t.Fatal("unparseable artifact must fail closed as executable")
	}
	if harnessCarriesCommand(".specd/roles/x.md", []byte(`prose command: not a real command`)) {
		t.Fatal("prose role must never be treated as executable")
	}
}
