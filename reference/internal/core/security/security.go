// Package security is the deterministic security-gate suite (V8/P4.2). Every
// scanner here is a pure function over file contents: stdlib only, no network, no
// LLM, no embedded CVE database. The binary never judges code quality — it makes
// skipping the deterministic checks impossible. Heuristic scanners default to
// advisory severity (plan risk 2: a noisy gate that gets disabled is worse than a
// modest one); allowlists carry mandatory reasons.
package security

import "sort"

// Severity ranks a finding. Advisory findings are reported but do not fail a
// gate unless the operator raises the scanner to blocking.
type Severity string

const (
	SeverityInfo  Severity = "info"
	SeverityWarn  Severity = "warn"
	SeverityBlock Severity = "error"
)

// Finding is one deterministic security observation. Line is 1-indexed; 0 means
// the finding is file-scoped (no single line).
type Finding struct {
	Scanner  string   `json:"scanner"`
	File     string   `json:"file"`
	Line     int      `json:"line"`
	Rule     string   `json:"rule"`
	Message  string   `json:"message"`
	Severity Severity `json:"severity"`
}

// ChangedFile is one file's content presented to the scanners. Path is
// repo-relative; Content is the full file text. Scanners never read the disk —
// the caller supplies contents so scanning is pure and testable.
type ChangedFile struct {
	Path    string
	Content string
}

// SortFindings orders findings deterministically (invariant 6/7): by file, then
// line, then scanner, then rule. Callers render in this order so output is
// byte-stable across runs.
func SortFindings(f []Finding) {
	sort.SliceStable(f, func(i, j int) bool {
		if f[i].File != f[j].File {
			return f[i].File < f[j].File
		}
		if f[i].Line != f[j].Line {
			return f[i].Line < f[j].Line
		}
		if f[i].Scanner != f[j].Scanner {
			return f[i].Scanner < f[j].Scanner
		}
		return f[i].Rule < f[j].Rule
	})
}

// Config selects which scanners run and at what severity. An empty string / "off"
// disables a scanner; "warn" is advisory; "error" blocks. The zero value runs
// nothing, so a repo that never opts in is unaffected (invariant 9).
type Config struct {
	Secrets   string
	Injection string
	Slopsquat string
}

func severityFor(mode string, def Severity) (Severity, bool) {
	switch mode {
	case "", "off":
		return "", false
	case "warn":
		return SeverityWarn, true
	case "error":
		return SeverityBlock, true
	default:
		return def, true
	}
}

// Scan runs every enabled scanner over the changed files and returns the merged,
// deterministically-sorted findings. allow is the secrets allowlist (entries with
// mandatory reasons); pass nil for none.
func Scan(cfg Config, files []ChangedFile, allow Allowlist) []Finding {
	var out []Finding
	if sev, on := severityFor(cfg.Secrets, SeverityBlock); on {
		out = append(out, withSeverity(ScanSecrets(files, allow), sev)...)
	}
	if sev, on := severityFor(cfg.Injection, SeverityWarn); on {
		out = append(out, withSeverity(ScanInjection(files), sev)...)
	}
	if sev, on := severityFor(cfg.Slopsquat, SeverityWarn); on {
		out = append(out, withSeverity(ScanSlopsquat(files), sev)...)
	}
	SortFindings(out)
	return out
}

// withSeverity stamps the operator-selected severity onto a scanner's raw
// findings, letting the same detection be advisory in one repo and blocking in
// another without changing the scanner.
func withSeverity(f []Finding, sev Severity) []Finding {
	for i := range f {
		f[i].Severity = sev
	}
	return f
}
