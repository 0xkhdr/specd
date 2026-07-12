package security

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/0xkhdr/specd/internal/core/scope"
)

// dangerPatterns are destructive shell fragments matched as substrings against
// added/modified file content. Each maps to a short, non-leaking label used as
// the finding excerpt (the raw matched line is never echoed — R5.3).
var dangerPatterns = []struct{ needle, label string }{
	{"rm -rf /", "recursive delete of a root path"},
	{"rm -rf ~", "recursive delete of home"},
	{"rm -rf $home", "recursive delete of home"},
	{":(){:|:&};:", "fork bomb"},
	{"mkfs", "filesystem format"},
	{"dd if=/dev/", "raw device write"},
	{"> /dev/sd", "raw device write"},
	{"git push --force", "force push"},
	{"git push -f", "force push"},
	{"chmod 777", "world-writable chmod"},
	{"chmod -r 777", "recursive world-writable chmod"},
	{"curl | sh", "pipe-to-shell install"},
	{"curl|sh", "pipe-to-shell install"},
	{"wget | sh", "pipe-to-shell install"},
	{"| sudo sh", "pipe-to-root-shell install"},
}

// scriptExts are files where gaining an exec bit is expected, so exec-mode is
// suppressed for them — a documented false-positive control (R6.3).
var scriptExts = map[string]bool{".sh": true, ".bash": true, ".zsh": true, ".py": true, ".rb": true, ".pl": true}

// secretFileSuffixes / secretFileNames flag newly generated credential material.
var secretFileSuffixes = []string{".pem", ".key", ".p12", ".pfx", ".keystore"}
var secretFileNames = map[string]bool{
	"id_rsa": true, "id_dsa": true, "id_ecdsa": true, "id_ed25519": true,
	"credentials": true, ".env": true, ".netrc": true, ".npmrc": true,
}

// ScanDangerous runs deterministic dangerous-change policies over a normalized
// diff (R6.3): destructive shell fragments, world-writable / unexpected-exec
// mode changes, auth/ownership policy changes, generated secret files, and
// symlink escapes. Profile selects severity (production → error); generated
// secrets and symlink escapes always fail closed at error. False positives are
// controlled by scanning only added/changed files, targeted patterns, script
// extension carve-outs, and the fingerprint allowlist reused by the caller.
func ScanDangerous(root string, diff scope.Diff, profile string) []Finding {
	sev := profileSeverity(profile)
	var findings []Finding
	for _, c := range diff.Changes {
		if c.Kind == "D" || c.Kind == "deleted" {
			continue // deletions carry no dangerous content
		}
		findings = append(findings, modeFindings(c, sev)...)
		if f, ok := authzFinding(c.Path, sev); ok {
			findings = append(findings, f)
		}
		if f, ok := secretFileFinding(c.Path); ok {
			findings = append(findings, f)
		}
		if f, ok := symlinkFinding(root, c.Path); ok {
			findings = append(findings, f)
		}
		findings = append(findings, contentFindings(root, c.Path, sev)...)
	}
	return findings
}

func modeFindings(c scope.Change, sev string) []Finding {
	if c.NewMode == "" {
		return nil
	}
	perm := octalPerm(c.NewMode)
	oldPerm := octalPerm(c.OldMode)
	var out []Finding
	if perm&0o002 != 0 {
		out = append(out, Finding{Scanner: "dangerous", Rule: "world-writable", File: c.Path, Severity: sev,
			Fingerprint: fingerprint("world-writable", c.Path, c.NewMode), Excerpt: c.Path + " became world-writable (" + c.NewMode + ")"})
	}
	ext := strings.ToLower(path.Ext(c.Path))
	if perm&0o111 != 0 && oldPerm&0o111 == 0 && !scriptExts[ext] {
		out = append(out, Finding{Scanner: "dangerous", Rule: "exec-mode", File: c.Path, Severity: sev,
			Fingerprint: fingerprint("exec-mode", c.Path, c.NewMode), Excerpt: c.Path + " gained an executable bit (" + c.NewMode + ")"})
	}
	return out
}

// octalPerm extracts the permission bits from a git mode string like "100755".
func octalPerm(mode string) int {
	if len(mode) < 3 {
		return 0
	}
	var perm int
	for _, r := range mode[len(mode)-3:] {
		if r < '0' || r > '7' {
			return 0
		}
		perm = perm*8 + int(r-'0')
	}
	return perm
}

func authzFinding(p, sev string) (Finding, bool) {
	base := path.Base(p)
	if base == "CODEOWNERS" || base == "sudoers" || base == ".htpasswd" || strings.HasSuffix(p, ".rego") {
		return Finding{Scanner: "dangerous", Rule: "authz-change", File: p, Severity: sev,
			Fingerprint: fingerprint("authz-change", p, base), Excerpt: p + " changes an authorization/ownership policy"}, true
	}
	return Finding{}, false
}

func secretFileFinding(p string) (Finding, bool) {
	base := path.Base(p)
	if secretFileNames[base] {
		return secretFinding(p), true
	}
	for _, suf := range secretFileSuffixes {
		if strings.HasSuffix(base, suf) {
			return secretFinding(p), true
		}
	}
	return Finding{}, false
}

func secretFinding(p string) Finding {
	// Generated credential material always fails closed at error severity.
	return Finding{Scanner: "dangerous", Rule: "generated-secret", File: p, Severity: "error",
		Fingerprint: fingerprint("generated-secret", p, path.Base(p)), Excerpt: p + " looks like generated secret material"}
}

func symlinkFinding(root, p string) (Finding, bool) {
	full := filepath.Join(root, filepath.FromSlash(p))
	fi, err := os.Lstat(full)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		return Finding{}, false
	}
	target, err := filepath.EvalSymlinks(full)
	if err != nil {
		// A dangling or unresolvable symlink is itself suspicious — fail closed.
		return Finding{Scanner: "dangerous", Rule: "symlink-escape", File: p, Severity: "error",
			Fingerprint: fingerprint("symlink-escape", p, "unresolvable"), Excerpt: p + " is an unresolvable symlink"}, true
	}
	absRoot, _ := filepath.Abs(root)
	absTarget, _ := filepath.Abs(target)
	if absTarget != absRoot && !strings.HasPrefix(absTarget, absRoot+string(os.PathSeparator)) {
		return Finding{Scanner: "dangerous", Rule: "symlink-escape", File: p, Severity: "error",
			Fingerprint: fingerprint("symlink-escape", p, "escape"), Excerpt: p + " symlinks outside the repository"}, true
	}
	return Finding{}, false
}

func contentFindings(root, p, sev string) []Finding {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
	if err != nil {
		return nil
	}
	lower := strings.ToLower(string(content))
	seen := map[string]bool{}
	var out []Finding
	for _, dp := range dangerPatterns {
		if seen[dp.label] || !strings.Contains(lower, dp.needle) {
			continue
		}
		seen[dp.label] = true
		out = append(out, Finding{Scanner: "dangerous", Rule: "destructive-shell", File: p, Severity: sev,
			Fingerprint: fingerprint("destructive-shell", p, dp.needle), Excerpt: p + ": " + dp.label})
	}
	return out
}
