package security

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
)

// Gate is the opt-in security gate registered by `check --security`. It scans
// tracked files and resolves per-scanner severity from config.
type Gate struct {
	cfg core.SecurityConfig
}

// New builds the gate with the resolved per-scanner severity config.
func New(cfg core.SecurityConfig) Gate { return Gate{cfg: cfg} }

func (Gate) Name() string { return "security" }

// Run enumerates tracked files under ctx.Root and returns gate findings. Error
// findings fail the gate; warn findings print but pass; off scanners are skipped
// (R5). Allowlisted findings are suppressed from the gate result (recorded
// separately by the caller for reports, R6).
func (g Gate) Run(ctx gates.CheckCtx) []gates.Finding {
	return GateFindings(Analyze(ctx.Root, g.cfg))
}

// GateFindings projects an analysis Result to gate findings: allowlisted and
// off findings drop out; error/warn map to gate severities.
func GateFindings(result Result) []gates.Finding {
	var out []gates.Finding
	for _, f := range result.Findings {
		if f.Allowlisted {
			continue
		}
		var sev gates.Severity
		switch f.Severity {
		case "error":
			sev = gates.Error
		case "warn":
			sev = gates.Warn
		default:
			continue
		}
		out = append(out, gates.Finding{
			Gate:     "security",
			Severity: sev,
			Message:  formatMessage(f),
		})
	}
	return out
}

func formatMessage(f Finding) string {
	loc := f.File
	if f.Line > 0 {
		loc = f.File + ":" + itoa(f.Line)
	}
	return f.Scanner + "/" + f.Rule + " " + loc + " — " + f.Excerpt
}

// Result is the full analysis: every scanner finding with severity resolved and
// allowlist status stamped. Consumed by the gate (for pass/fail) and by the
// caller (for recording under state.security).
type Result struct {
	Findings []Finding
}

// scanners returns the active scanner set. Severity "off" disables a scanner.
func (g Gate) enabled() map[string]bool {
	return map[string]bool{
		"secrets":   g.cfg.Secrets != "off",
		"injection": g.cfg.Injection != "off",
		"slopsquat": g.cfg.Slopsquat != "off",
	}
}

// Analyze runs every non-off scanner over the tracked working tree and returns
// findings with severity + allowlist status resolved. A load error in the
// allowlist fails closed (its error finding is included at error severity).
func Analyze(root string, cfg core.SecurityConfig) Result {
	g := Gate{cfg: cfg}
	enabled := g.enabled()
	files, inputFindings := scanInputs(root)

	allow, allowFindings := loadGovernedAllowlist(root, cfg.Profile, time.Now().UTC())

	var raw []Finding
	raw = append(raw, policyFindings(root, cfg)...)
	scanners := []Scanner{secretsScanner{}, injectionScanner{}, slopsquatScanner{}}
	for _, sc := range scanners {
		if !enabled[sc.Name()] {
			continue
		}
		raw = append(raw, sc.Scan(files)...)
	}

	severity := map[string]string{
		"secrets":   cfg.Secrets,
		"injection": cfg.Injection,
		"slopsquat": cfg.Slopsquat,
	}
	for i := range raw {
		raw[i].Severity = severity[raw[i].Scanner]
		raw[i].Allowlisted = allow.allows(raw[i].Fingerprint)
	}
	raw = append(raw, inputFindings...)
	// Allowlist load failures surface as error findings regardless of scanners.
	raw = append(raw, allowFindings...)

	sortFindings(raw)
	return Result{Findings: raw}
}

func policyFindings(root string, cfg core.SecurityConfig) []Finding {
	var findings []Finding
	if cfg.CleanWorktree != "" && cfg.CleanWorktree != "off" && !cleanWorktree(root) {
		findings = append(findings, Finding{
			Scanner:  "policy",
			Rule:     "clean-worktree",
			Severity: cfg.CleanWorktree,
			Excerpt:  "git worktree has uncommitted changes",
		})
	}
	if cfg.Sandbox != "" && cfg.Sandbox != "off" && !sandboxActive() {
		findings = append(findings, Finding{
			Scanner:  "policy",
			Rule:     "sandbox",
			Severity: cfg.Sandbox,
			Excerpt:  "sandbox policy enabled but no sandbox marker detected",
		})
	}
	return findings
}

func cleanWorktree(root string) bool {
	out, err := exec.Command("git", "-C", root, "status", "--porcelain").Output()
	return err == nil && strings.TrimSpace(string(out)) == ""
}

func sandboxActive() bool {
	for _, key := range []string{"SPECD_SANDBOX_ACTIVE", "CODEX_SANDBOX", "SANDBOX"} {
		if os.Getenv(key) != "" {
			return true
		}
	}
	return false
}

func sortFindings(f []Finding) {
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
		return f[i].Fingerprint < f[j].Fingerprint
	})
}

type scanRef struct {
	Path string
	Kind ScanKind
}

// scanInputs enumerates the repository boundary. Any enumeration/read failure
// is itself an error finding, never an empty successful scan.
func scanInputs(root string) ([]ScanInputV1, []Finding) {
	var refs []scanRef
	var findings []Finding
	for _, query := range []struct {
		kind ScanKind
		args []string
	}{
		{ScanKindSource, []string{"ls-files", "-z", "--cached"}},
		{ScanKindUntracked, []string{"ls-files", "-z", "--others", "--exclude-standard"}},
	} {
		out, err := exec.Command("git", append([]string{"-C", root}, query.args...)...).Output()
		if err != nil {
			findings = append(findings, inputErrorFinding("enumeration-failed", ".", "unable to enumerate repository inputs"))
			continue
		}
		for _, rel := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
			if rel != "" {
				refs = append(refs, scanRef{Path: filepath.ToSlash(rel), Kind: query.kind})
			}
		}
	}
	for _, runtime := range []struct {
		dir  string
		kind ScanKind
	}{
		{filepath.Join(".specd", "specs"), ScanKindSpec},
		{filepath.Join(".specd", "steering"), ScanKindSteering},
		{filepath.Join(".specd", "roles"), ScanKindRole},
		{filepath.Join(".specd", "memory"), ScanKindMemory},
	} {
		base := filepath.Join(root, runtime.dir)
		err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Type().IsRegular() {
				rel, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				refs = append(refs, scanRef{Path: filepath.ToSlash(rel), Kind: runtime.kind})
			}
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			findings = append(findings, inputErrorFinding("enumeration-failed", filepath.ToSlash(runtime.dir), "unable to enumerate runtime inputs"))
		}
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Path < refs[j].Path })
	refs = uniqueScanRefs(refs)
	inputs, readFindings := readScanInputs(root, refs)
	return inputs, append(findings, readFindings...)
}

func uniqueScanRefs(refs []scanRef) []scanRef {
	out := refs[:0]
	for _, ref := range refs {
		if len(out) > 0 && out[len(out)-1].Path == ref.Path {
			// Runtime classifications replace generic tracked/untracked kinds.
			if ref.Kind != ScanKindSource && ref.Kind != ScanKindUntracked {
				out[len(out)-1] = ref
			}
			continue
		}
		out = append(out, ref)
	}
	return out
}

func readScanInputs(root string, refs []scanRef) ([]ScanInputV1, []Finding) {
	inputs := make([]ScanInputV1, 0, len(refs))
	var findings []Finding
	for _, ref := range refs {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ref.Path)))
		if err != nil {
			findings = append(findings, inputErrorFinding("read-failed", ref.Path, "unable to read scanner input"))
			continue
		}
		input := NewScanInput(ref.Path, ref.Kind, TrustUntrustedData, content)
		input.Root = root
		inputs = append(inputs, input)
	}
	return inputs, findings
}

func inputErrorFinding(rule, file, excerpt string) Finding {
	return Finding{Scanner: "input", Rule: rule, File: file, Severity: "error", Fingerprint: fingerprint(rule, file, excerpt), Excerpt: excerpt}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
