package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/security"
)

// maxSecurityFileBytes caps the per-file content the scanners read, so a giant
// generated/binary blob in the changed set cannot blow up memory or time. Files
// over the cap are skipped (they are not source under review).
const maxSecurityFileBytes = 2 << 20 // 2 MiB

// runSecurityCheck implements `specd check <slug> --security`: it gathers the
// working-tree changed files, runs the deterministic security suite over their
// contents, records a summary in state.json, and renders the findings. Exit is
// gate (1) only when a blocking (error-severity) finding is present — advisory
// scanners never fail the command (plan risk 2). The suite is opt-in per
// config.security.*; with no scanner enabled it is a no-op reporting "off".
func runSecurityCheck(root, slug string, args cli.Args) int {
	if err := core.RequireSpec(root, slug); err != nil {
		return specdExit(err)
	}
	cfg := core.LoadConfig(root)
	scfg := security.Config{
		Secrets:   cfg.Security.Secrets,
		Injection: cfg.Security.Injection,
		Slopsquat: cfg.Security.Slopsquat,
	}

	allow, err := loadSecurityAllowlist(root)
	if err != nil {
		return specdExit(err)
	}

	files := readChangedForSecurity(root)
	findings := security.Scan(scfg, files, allow)

	blocking := 0
	byScanner := map[string]int{}
	for _, f := range findings {
		byScanner[f.Scanner]++
		if f.Severity == security.SeverityBlock {
			blocking++
		}
	}

	// Record a deterministic summary in state (dual-write discipline).
	if err := recordSecurityScan(root, slug, len(findings), blocking, byScanner); err != nil {
		return specdExit(err)
	}

	if args.Bool("json") {
		out := map[string]interface{}{
			"ok": blocking == 0, "findings": findings, "blocking": blocking, "byScanner": byScanner,
		}
		if err := core.PrintJSON(out); err != nil {
			return specdExit(err)
		}
		if blocking > 0 {
			return core.ExitGate
		}
		return core.ExitOK
	}

	if scfg.Secrets == "" && scfg.Injection == "" && scfg.Slopsquat == "" {
		fmt.Printf("security: suite off (enable via config.security.{secrets,injection,slopsquat}) — no scan for '%s'\n", slug)
		return core.ExitOK
	}
	for _, f := range findings {
		errLine("%-5s %s:%d [%s/%s] %s", string(f.Severity), f.File, f.Line, f.Scanner, f.Rule, f.Message)
	}
	if blocking > 0 {
		errLine("\n✗ security: %d blocking finding(s) across %d total.", blocking, len(findings))
		return core.ExitGate
	}
	fmt.Printf("✓ security: %d advisory finding(s), 0 blocking for '%s'\n", len(findings), slug)
	return core.ExitOK
}

// loadSecurityAllowlist reads and validates .specd/security/allow.json. An absent
// file is an empty allowlist. A malformed file (or a reasonless entry) is a hard
// error — a broken allowlist must fail loudly, never silently allow.
func loadSecurityAllowlist(root string) (security.Allowlist, error) {
	path := filepath.Join(root, ".specd", "security", "allow.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return security.Allowlist{}, nil
		}
		return security.Allowlist{}, core.GateError(fmt.Sprintf("cannot read %s: %v", path, err))
	}
	allow, perr := security.ParseAllowlist(data)
	if perr != nil {
		return security.Allowlist{}, core.GateError(perr.Error())
	}
	return allow, nil
}

// readChangedForSecurity returns the working-tree changed files' contents, capped
// per file and skipping unreadable/oversize entries. Deterministic order.
func readChangedForSecurity(root string) []security.ChangedFile {
	names := changedFiles(root)
	sort.Strings(names)
	out := make([]security.ChangedFile, 0, len(names))
	for _, name := range names {
		p := filepath.Join(root, name)
		info, err := os.Stat(p)
		if err != nil || info.IsDir() || info.Size() > maxSecurityFileBytes {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		out = append(out, security.ChangedFile{Path: name, Content: string(b)})
	}
	return out
}

// recordSecurityScan writes the scan summary into state under the spec lock.
func recordSecurityScan(root, slug string, total, blocking int, byScanner map[string]int) error {
	_, err := core.WithSpecLock[struct{}](root, slug, func() (struct{}, error) {
		state, err := core.LoadState(root, slug)
		if err != nil {
			return struct{}{}, err
		}
		if state == nil {
			return struct{}{}, core.NotFoundError(fmt.Sprintf("spec '%s' not found", slug))
		}
		state.Security = &core.SecurityScan{
			Findings:  total,
			Blocking:  blocking,
			ByScanner: byScanner,
			Time:      core.NowISO(),
		}
		return struct{}{}, core.SaveState(root, slug, state)
	})
	return err
}
