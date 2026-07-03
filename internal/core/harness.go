package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// The harness bundle makes a project's configured policy a shareable, versioned
// team asset (V11/P6.1). It carries the declarative policy artifacts — guardrails
// rules, deploy templates, roles, routing — under `.specd/harness/`, with a
// `harness.json` manifest recording each file's SHA256 for pinning. Sharing rides
// on `git` through the same stdlib-exec discipline as the git state backend: no
// git library dependency, scrubbed env, an allowlisted transport set, and remote
// URL validation.
//
// The load-bearing security property is quarantine: any imported artifact that
// carries an executable `command` check (a deploy step, an eval command, a custom
// gate) arrives disabled — copied to `.specd/harness/quarantine/` and listed, but
// never installed to its active path — until an operator runs `harness enable`,
// which is recorded in the harness decision log. Non-executable policy (regex
// guardrails, role prose) installs directly, but never silently overwrites a
// locally-modified file: that requires --force.
const (
	harnessDirName      = "harness"
	harnessManifestName = "harness.json"
	harnessFilesSubdir  = "files"
	harnessQuarantineD  = "quarantine"
	harnessDecisionsF   = "decisions.jsonl"
)

// HarnessFile is one policy artifact carried by a bundle. Path is relative to the
// project root (canonical, forward-slash, never escaping the tree). SHA256 pins
// the content; Executable marks an artifact that carries a `command` check and so
// must be quarantined on import.
type HarnessFile struct {
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	Executable bool   `json:"executable"`
}

// HarnessManifest is the parsed `.specd/harness/harness.json`. Version is a plain
// monotonically-increasing int compared across pulls to refuse downgrades;
// Provenance records where the bundle came from ("local" or a remote URL).
type HarnessManifest struct {
	Name       string        `json:"name"`
	Version    int           `json:"version"`
	Provenance string        `json:"provenance"`
	Files      []HarnessFile `json:"files"`
}

// HarnessDir returns the path to root's `.specd/harness` bundle directory.
func HarnessDir(root string) string { return filepath.Join(root, ".specd", harnessDirName) }

func harnessManifestPath(root string) string {
	return filepath.Join(HarnessDir(root), harnessManifestName)
}

func harnessQuarantineDir(root string) string {
	return filepath.Join(HarnessDir(root), harnessQuarantineD)
}

func harnessDecisionsPath(root string) string {
	return filepath.Join(HarnessDir(root), harnessDecisionsF)
}

// harnessPolicyGlobs are the project-level policy artifacts a bundle collects, in
// a deterministic, security-reviewable order. Only files that exist are included.
// Per-spec state (state.json, evals under specs/) is never bundled — the harness
// carries shareable team policy, not one repo's work.
func harnessPolicyPaths(root string) []string {
	specd := filepath.Join(root, ".specd")
	var rels []string
	add := func(rel string) {
		if FileExists(filepath.Join(root, rel)) {
			rels = append(rels, rel)
		}
	}
	add(filepath.ToSlash(filepath.Join(".specd", "guardrails.json")))
	add(filepath.ToSlash(filepath.Join(".specd", "routing.json")))
	// deploy/<env>.json templates and roles/* prose, sorted within each dir.
	for _, sub := range []string{"deploy", "roles"} {
		dir := filepath.Join(specd, sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		var names []string
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			names = append(names, e.Name())
		}
		sort.Strings(names)
		for _, n := range names {
			rels = append(rels, filepath.ToSlash(filepath.Join(".specd", sub, n)))
		}
	}
	return rels
}

// harnessCarriesCommand reports whether a policy artifact carries an executable
// `command` check and so must be quarantined on import. It scans the decoded JSON
// recursively for any non-empty string under a command-bearing key, so a hostile
// bundle cannot smuggle a step command past the quarantine by nesting it. A file
// that is not valid JSON is treated as executable (fail-closed) unless it is a
// known prose artifact (roles/*), which is never JSON.
func harnessCarriesCommand(rel string, raw []byte) bool {
	if strings.HasPrefix(rel, ".specd/roles/") {
		return false
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return true // fail closed: an artifact we cannot inspect is treated as dangerous
	}
	return jsonHasCommandKey(doc)
}

func jsonHasCommandKey(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			switch strings.ToLower(k) {
			case "command", "rollbackcommand", "run", "exec", "script":
				if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
					return true
				}
			}
			if jsonHasCommandKey(val) {
				return true
			}
		}
	case []any:
		for _, e := range x {
			if jsonHasCommandKey(e) {
				return true
			}
		}
	}
	return false
}

func sha256Hex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// BuildHarnessManifest assembles a manifest for the project's current policy
// artifacts. Version is one past the existing local manifest (1 for a first
// build) so each push advances the shared version monotonically.
func BuildHarnessManifest(root, name string) (HarnessManifest, error) {
	m := HarnessManifest{Name: name, Version: 1, Provenance: "local"}
	if prior, err := LoadHarnessManifest(root); err == nil {
		m.Version = prior.Version + 1
		if strings.TrimSpace(name) == "" {
			m.Name = prior.Name
		}
	}
	if strings.TrimSpace(m.Name) == "" {
		m.Name = filepath.Base(root)
	}
	for _, rel := range harnessPolicyPaths(root) {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return HarnessManifest{}, GateError(fmt.Sprintf("harness: read %s: %v", rel, err))
		}
		m.Files = append(m.Files, HarnessFile{
			Path:       rel,
			SHA256:     sha256Hex(raw),
			Executable: harnessCarriesCommand(rel, raw),
		})
	}
	sort.Slice(m.Files, func(i, j int) bool { return m.Files[i].Path < m.Files[j].Path })
	return m, nil
}

// LoadHarnessManifest reads and validates the local bundle manifest.
func LoadHarnessManifest(root string) (HarnessManifest, error) {
	raw, err := os.ReadFile(harnessManifestPath(root))
	if err != nil {
		return HarnessManifest{}, NotFoundError("no harness bundle: run `specd harness push` or `specd harness pull` first")
	}
	return ParseHarnessManifest(raw)
}

// ParseHarnessManifest decodes and validates a manifest. It fails closed on
// unknown fields, missing required fields, a non-positive version, and any file
// path that could escape the project root (same discipline as pack manifests).
func ParseHarnessManifest(raw []byte) (HarnessManifest, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var m HarnessManifest
	if err := dec.Decode(&m); err != nil {
		return HarnessManifest{}, GateError(fmt.Sprintf("invalid harness manifest: %v", err))
	}
	if strings.TrimSpace(m.Name) == "" {
		return HarnessManifest{}, GateError("harness manifest missing required field: name")
	}
	if m.Version < 1 {
		return HarnessManifest{}, GateError(fmt.Sprintf("harness manifest version must be >= 1, got %d", m.Version))
	}
	seen := map[string]bool{}
	for _, f := range m.Files {
		if err := validateHarnessPath(f.Path); err != nil {
			return HarnessManifest{}, err
		}
		if len(f.SHA256) != 64 || strings.ToLower(f.SHA256) != f.SHA256 {
			return HarnessManifest{}, GateError(fmt.Sprintf("harness file %q has a malformed sha256", f.Path))
		}
		if seen[f.Path] {
			return HarnessManifest{}, GateError(fmt.Sprintf("harness manifest declares duplicate path %q", f.Path))
		}
		seen[f.Path] = true
	}
	return m, nil
}

// validateHarnessPath restricts a manifest path to the `.specd/` policy tree and
// rejects any escape (absolute, "..", non-canonical) — hostile-manifest defense.
func validateHarnessPath(p string) error {
	if p == "" {
		return GateError("harness file has empty path")
	}
	if path.IsAbs(p) || strings.HasPrefix(p, "/") {
		return GateError(fmt.Sprintf("harness file path %q must be relative", p))
	}
	clean := path.Clean(p)
	if clean != p {
		return GateError(fmt.Sprintf("harness file path %q is not canonical (want %q)", p, clean))
	}
	if clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return GateError(fmt.Sprintf("harness file path %q escapes the project root", p))
	}
	if !strings.HasPrefix(clean, ".specd/") {
		return GateError(fmt.Sprintf("harness file path %q must be under .specd/", p))
	}
	return nil
}

// WriteHarnessBundle writes the manifest plus a self-contained copy of every
// artifact under `.specd/harness/files/`, so the bundle can be committed and
// pushed to a separate repository without depending on the live policy files.
func WriteHarnessBundle(root string, m HarnessManifest) error {
	if err := writeHarnessManifest(root, m); err != nil {
		return err
	}
	for _, f := range m.Files {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.Path)))
		if err != nil {
			return GateError(fmt.Sprintf("harness: read %s: %v", f.Path, err))
		}
		dst := filepath.Join(HarnessDir(root), harnessFilesSubdir, filepath.FromSlash(f.Path))
		if err := AtomicWrite(dst, string(raw)); err != nil {
			return err
		}
	}
	return nil
}

func writeHarnessManifest(root string, m HarnessManifest) error {
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWrite(harnessManifestPath(root), string(out)+"\n")
}

// HarnessURLError is returned when a remote reference fails validation.
func ValidateHarnessURL(url string) error {
	u := strings.TrimSpace(url)
	if u == "" {
		return UsageError("harness: empty git URL")
	}
	if strings.HasPrefix(u, "-") {
		return GateError(fmt.Sprintf("harness: refusing URL that begins with '-' (option injection): %q", url))
	}
	if strings.ContainsAny(u, "\n\r\x00") {
		return GateError("harness: URL contains a control character")
	}
	// Block git's arbitrary-command transports outright; the allowlist below is
	// enforced a second time via GIT_ALLOW_PROTOCOL when git actually runs.
	lower := strings.ToLower(u)
	for _, bad := range []string{"ext::", "fd::", "-o", "--upload-pack", "--receive-pack"} {
		if strings.Contains(lower, bad) {
			return GateError(fmt.Sprintf("harness: refusing unsafe transport in URL %q", url))
		}
	}
	return nil
}

// harnessGit runs git as a direct arg-vector exec (never through a shell, so a
// hostile URL can never be interpreted as a command), with a scrubbed env, no
// terminal prompts, and a protocol allowlist that excludes git's arbitrary-command
// ext/fd transports.
func harnessGit(dir string, args ...string) (string, error) {
	full := args
	if dir != "" {
		full = append([]string{"-C", dir}, args...)
	}
	cmd := exec.Command("git", full...) //nolint:gosec // git is a fixed binary; args are a specd-supplied vector, not a shell string (see SECURITY.md)
	cmd.Env = append(ScrubbedEnv(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ALLOW_PROTOCOL=file:git:ssh:https:http",
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// SecureGitClone clones url into dst through the hardened git-exec discipline:
// the URL is validated (no arbitrary ext/fd transports, no option injection),
// the env is scrubbed, terminal prompts are disabled, and the protocol allowlist
// is enforced. depth1 requests a shallow clone. Shared by harness sharing and the
// pack registry so both ride one reviewed git path.
func SecureGitClone(url, dst string, depth1 bool) error {
	if err := ValidateHarnessURL(url); err != nil {
		return err
	}
	args := []string{"clone", "--quiet"}
	if depth1 {
		args = append(args, "--depth", "1")
	}
	args = append(args, "--", url, dst)
	if out, err := harnessGit("", args...); err != nil {
		return GateError(fmt.Sprintf("git clone %q failed: %s", url, strings.TrimSpace(out)))
	}
	return nil
}

// PushHarness builds the current bundle and pushes it to a remote git URL. It
// clones the remote into a temp working tree, copies the manifest and files in,
// commits, and pushes — so an existing shared history is preserved.
func PushHarness(root, url, name string) (HarnessManifest, error) {
	if err := ValidateHarnessURL(url); err != nil {
		return HarnessManifest{}, err
	}
	m, err := BuildHarnessManifest(root, name)
	if err != nil {
		return HarnessManifest{}, err
	}
	if len(m.Files) == 0 {
		return HarnessManifest{}, GateError("harness: no policy artifacts to share (no guardrails/deploy/roles/routing found)")
	}
	if err := WriteHarnessBundle(root, m); err != nil {
		return HarnessManifest{}, err
	}
	work, err := os.MkdirTemp("", "specd-harness-push-*")
	if err != nil {
		return HarnessManifest{}, err
	}
	defer os.RemoveAll(work)
	if out, err := harnessGit("", "clone", "--quiet", "--", url, work); err != nil {
		return HarnessManifest{}, GateError(fmt.Sprintf("harness push: clone failed: %s", out))
	}
	// Copy manifest + files into the working tree.
	if err := copyTree(filepath.Join(HarnessDir(root)), work); err != nil {
		return HarnessManifest{}, err
	}
	if out, err := harnessGit(work, "add", "-A"); err != nil {
		return HarnessManifest{}, GateError(fmt.Sprintf("harness push: stage failed: %s", out))
	}
	msg := fmt.Sprintf("specd harness %s v%d", m.Name, m.Version)
	if out, err := harnessGit(work,
		"-c", "user.email=specd@localhost", "-c", "user.name=specd",
		"commit", "--allow-empty", "--quiet", "-m", msg); err != nil {
		return HarnessManifest{}, GateError(fmt.Sprintf("harness push: commit failed: %s", out))
	}
	if out, err := harnessGit(work, "push", "--quiet", "origin", "HEAD"); err != nil {
		return HarnessManifest{}, GateError(fmt.Sprintf("harness push: push failed: %s", out))
	}
	return m, nil
}

// HarnessPullResult reports what a pull did: files installed, files quarantined
// (executable artifacts awaiting explicit enable), and files refused for local
// modification (only when not forced).
type HarnessPullResult struct {
	Manifest    HarnessManifest `json:"manifest"`
	Installed   []string        `json:"installed"`
	Quarantined []string        `json:"quarantined"`
	Refused     []string        `json:"refused"`
}

// PullHarness clones a remote bundle, verifies every file's pinned SHA256, and
// installs it. Non-executable policy installs directly unless it would clobber a
// locally-modified file (refused without force). Executable artifacts are copied
// to quarantine and listed, never installed. A remote whose version is older than
// the local bundle is refused without force (no silent downgrade). Any checksum
// mismatch is a hard failure that writes nothing.
func PullHarness(root, url string, force bool) (HarnessPullResult, error) {
	if err := ValidateHarnessURL(url); err != nil {
		return HarnessPullResult{}, err
	}
	work, err := os.MkdirTemp("", "specd-harness-pull-*")
	if err != nil {
		return HarnessPullResult{}, err
	}
	defer os.RemoveAll(work)
	if out, err := harnessGit("", "clone", "--quiet", "--depth", "1", "--", url, work); err != nil {
		return HarnessPullResult{}, GateError(fmt.Sprintf("harness pull: clone failed: %s", out))
	}
	raw, err := os.ReadFile(filepath.Join(work, harnessManifestName))
	if err != nil {
		return HarnessPullResult{}, GateError("harness pull: remote has no harness.json manifest")
	}
	incoming, err := ParseHarnessManifest(raw)
	if err != nil {
		return HarnessPullResult{}, err
	}

	prior, priorErr := LoadHarnessManifest(root)
	if priorErr == nil && incoming.Version < prior.Version && !force {
		return HarnessPullResult{}, GateError(fmt.Sprintf(
			"harness pull: remote version %d is older than local %d — refusing downgrade (override: --force)",
			incoming.Version, prior.Version))
	}
	priorSHA := map[string]string{}
	if priorErr == nil {
		for _, f := range prior.Files {
			priorSHA[f.Path] = f.SHA256
		}
	}

	// Verify every file's pinned checksum before writing anything (fail-closed).
	contents := make(map[string][]byte, len(incoming.Files))
	for _, f := range incoming.Files {
		body, err := os.ReadFile(filepath.Join(work, harnessFilesSubdir, filepath.FromSlash(f.Path)))
		if err != nil {
			return HarnessPullResult{}, GateError(fmt.Sprintf("harness pull: bundle missing file %q", f.Path))
		}
		if got := sha256Hex(body); got != f.SHA256 {
			return HarnessPullResult{}, GateError(fmt.Sprintf(
				"harness pull: %s SHA256 mismatch (got %s, pinned %s) — refusing to apply", f.Path, got, f.SHA256))
		}
		contents[f.Path] = body
	}

	res := HarnessPullResult{Manifest: incoming}
	res.Manifest.Provenance = url
	for _, f := range incoming.Files {
		body := contents[f.Path]
		if f.Executable {
			// Quarantine: copy to holding area, do not install.
			dst := filepath.Join(harnessQuarantineDir(root), filepath.FromSlash(f.Path))
			if err := AtomicWrite(dst, string(body)); err != nil {
				return HarnessPullResult{}, err
			}
			res.Quarantined = append(res.Quarantined, f.Path)
			continue
		}
		target := filepath.Join(root, filepath.FromSlash(f.Path))
		if locallyModified(target, f.SHA256, priorSHA[f.Path]) && !force {
			res.Refused = append(res.Refused, f.Path)
			continue
		}
		if err := AtomicWrite(target, string(body)); err != nil {
			return HarnessPullResult{}, err
		}
		res.Installed = append(res.Installed, f.Path)
	}

	// Persist the imported manifest as the new local bundle record (with remote
	// provenance) only if nothing was refused, so a partial pull does not claim a
	// clean version. Refusals leave the prior manifest intact.
	if len(res.Refused) == 0 {
		if err := writeHarnessManifest(root, res.Manifest); err != nil {
			return HarnessPullResult{}, err
		}
	}
	return res, nil
}

// locallyModified reports whether the on-disk target carries edits the pull would
// clobber: it exists, differs from the incoming content, and differs from what we
// last delivered (priorSHA). A file that is byte-identical to the last pull is
// pristine-as-delivered and safe to update.
func locallyModified(target, incomingSHA, priorSHA string) bool {
	body, err := os.ReadFile(target)
	if err != nil {
		return false // absent: nothing to clobber
	}
	cur := sha256Hex(body)
	if cur == incomingSHA {
		return false // already up to date
	}
	if priorSHA != "" && cur == priorSHA {
		return false // untouched since last pull: safe to advance
	}
	return true
}

// HarnessQuarantined lists the paths currently held in quarantine (imported
// executable artifacts awaiting explicit enable), in deterministic order.
func HarnessQuarantined(root string) []string {
	base := harnessQuarantineDir(root)
	var out []string
	_ = filepath.WalkDir(base, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(base, p)
		if rerr != nil {
			return nil
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(out)
	return out
}

// EnableHarnessItem installs one quarantined artifact to its active path and
// records the decision in the harness decision log. It refuses a path that is not
// currently quarantined, and refuses to clobber a locally-modified target without
// force.
func EnableHarnessItem(root, relPath string, force bool) error {
	if err := validateHarnessPath(relPath); err != nil {
		return err
	}
	src := filepath.Join(harnessQuarantineDir(root), filepath.FromSlash(relPath))
	body, err := os.ReadFile(src)
	if err != nil {
		return NotFoundError(fmt.Sprintf("harness: %q is not quarantined", relPath))
	}
	target := filepath.Join(root, filepath.FromSlash(relPath))
	if !force {
		if existing, err := os.ReadFile(target); err == nil && sha256Hex(existing) != sha256Hex(body) {
			return GateError(fmt.Sprintf("harness enable: %q already exists with different content — override with --force", relPath))
		}
	}
	if err := AtomicWrite(target, string(body)); err != nil {
		return err
	}
	if err := os.Remove(src); err != nil && !os.IsNotExist(err) {
		return err
	}
	entry := map[string]string{
		"time":   NowISO(),
		"action": "enable",
		"path":   relPath,
		"sha256": sha256Hex(body),
	}
	line, _ := json.Marshal(entry)
	return AppendFile(harnessDecisionsPath(root), string(line)+"\n")
}

// copyTree copies every regular file under src into dst, preserving relative
// paths. Used to stage the bundle into a clone working tree for push.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return AtomicWrite(filepath.Join(dst, rel), string(body))
	})
}
