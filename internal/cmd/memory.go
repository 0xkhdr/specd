package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

var validCriticalities = map[string]bool{"minor": true, "important": true, "critical": true}

// memoryNow is the injectable clock for promotion provenance (RM.7). Tests
// override it so promotion output is byte-deterministic.
var memoryNow = func() time.Time { return time.Now().UTC() }

// runMemory implements `specd memory <slug> <add|promote>`. add appends a
// pattern block to the spec's memory.md; promote lifts an existing block into
// the shared steering store once it clears the promotion threshold (RM.1–RM.9).
func runMemory(root string, args []string, flags map[string]string) error {
	if len(args) < 2 {
		return errors.New("usage: specd memory <slug> <add|promote> [flags]")
	}
	slug, sub := args[0], args[1]
	if err := core.ValidateSlug(slug); err != nil {
		return err
	}
	specDir := filepath.Join(core.SpecdDir(root), "specs", slug)
	if info, err := os.Stat(specDir); err != nil || !info.IsDir() {
		return fmt.Errorf("spec %q not found", slug)
	}
	switch sub {
	case "add":
		return memoryAdd(root, slug, flags)
	case "promote":
		return memoryPromote(root, slug, flags)
	default:
		return fmt.Errorf("unknown memory subcommand %q (expected add|promote)", sub)
	}
}

func memoryAdd(root, slug string, flags map[string]string) error {
	f := core.MemFields{
		Key:         flags["key"],
		Pattern:     flags["pattern"],
		Detail:      flags["body"],
		Source:      flags["source"],
		Criticality: flags["criticality"],
		Related:     flags["related"],
	}
	if f.Key == "" || f.Pattern == "" || f.Detail == "" || f.Source == "" {
		return errors.New(`memory add requires --key --pattern "..." --body "..." --source "..." --criticality <c>`)
	}
	if !validCriticalities[f.Criticality] {
		return errors.New("--criticality must be one of: minor, important, critical")
	}
	memPath := core.SpecMemoryPath(root, slug)
	_, err := core.WithSpecLock(root, func() (struct{}, error) {
		return struct{}{}, core.AppendFile(memPath, "\n"+core.RenderMemBlock(f))
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "memory: added '%s' to %s/memory.md\n", f.Key, slug)
	return nil
}

func memoryPromote(root, slug string, flags map[string]string) error {
	key := flags["key"]
	if key == "" {
		return errors.New("memory promote requires --key <key>")
	}
	force := flagEnabled(flags, "force")
	_, err := core.WithSpecLock(root, func() (struct{}, error) {
		raw := ""
		if p := core.ReadOrNull(core.SpecMemoryPath(root, slug)); p != nil {
			raw = *p
		}
		block := core.ExtractMemBlock(raw, key)
		if block == "" {
			return struct{}{}, fmt.Errorf("memory: key '%s' not found in %s/memory.md", key, slug)
		}
		threshold := promotionThreshold(root)
		count := core.CountSpecsWithBlock(root, key)
		if count < threshold && !force {
			return struct{}{}, fmt.Errorf("memory: pattern '%s' seen in %d spec(s); promotion threshold is %d. Re-run with --force to promote anyway", key, count, threshold)
		}
		date := memoryNow().Format("2006-01-02")
		promoted := core.RenderPromotion(block, slug, count, date)
		if err := core.AppendFile(core.SteeringMemoryPath(root), promoted); err != nil {
			return struct{}{}, err
		}
		fmt.Fprintf(os.Stdout, "memory: promoted '%s' from %s to steering/memory.md (seen in %d spec(s), threshold %d)\n", key, slug, count, threshold)
		return struct{}{}, nil
	})
	return err
}

func promotionThreshold(root string) int {
	cfg, _ := core.LoadConfig(configPaths(root), getenv())
	return cfg.PromotionThreshold
}
