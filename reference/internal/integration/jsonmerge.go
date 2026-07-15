package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

type JSONMergeOptions struct {
	Root       string
	Target     string
	KeyPath    []string
	ServerName string
	Server     any
	Now        func() time.Time
	WriteFile  func(path, content string) error
}

type JSONMergeResult struct {
	Changed bool   `json:"changed"`
	Target  string `json:"target"`
	Backup  string `json:"backup"`
}

// MergeJSONServer mutates one named nested server entry, preserving all other
// semantic JSON values. Existing files are backed up before atomic replacement.
//
//nolint:gocyclo // pre-existing complexity debt, out of scope for spec S3 — tracked for a future cleanup pass
func MergeJSONServer(options JSONMergeOptions) (JSONMergeResult, error) {
	result := JSONMergeResult{Target: options.Target}
	if options.Root == "" || options.Target == "" || options.ServerName == "" || len(options.KeyPath) == 0 {
		return result, fmt.Errorf("root, target, key path, and server name are required")
	}
	if strings.ContainsRune(options.Target, '\x00') || strings.ContainsRune(options.Root, '\x00') {
		return result, fmt.Errorf("JSON config path contains NUL")
	}
	if err := validateConfigTarget(options.Root, options.Target); err != nil {
		return result, err
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.WriteFile == nil {
		options.WriteFile = core.AtomicWrite
	}

	document := map[string]any{}
	existing, err := os.ReadFile(options.Target)
	existed := err == nil
	if err != nil && !os.IsNotExist(err) {
		return result, err
	}
	if existed {
		decoder := json.NewDecoder(bytes.NewReader(existing))
		decoder.UseNumber()
		if err := decoder.Decode(&document); err != nil {
			return result, fmt.Errorf("parse %s: %w", options.Target, err)
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			if err == nil {
				return result, fmt.Errorf("parse %s: trailing JSON content", options.Target)
			}
			return result, fmt.Errorf("parse %s: %w", options.Target, err)
		}
	}

	parent := document
	for _, key := range options.KeyPath {
		value, ok := parent[key]
		if !ok {
			next := map[string]any{}
			parent[key] = next
			parent = next
			continue
		}
		next, ok := value.(map[string]any)
		if !ok {
			return result, fmt.Errorf("JSON key %q is not an object", key)
		}
		parent = next
	}
	parent[options.ServerName] = options.Server

	updated, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return result, fmt.Errorf("encode JSON config: %w", err)
	}
	updated = append(updated, '\n')
	if existed && bytes.Equal(existing, updated) {
		return result, nil
	}
	if existed {
		result.Backup = options.Target + ".specd-backup-" + options.Now().UTC().Format("20060102T150405.000000000Z")
		if err := core.AtomicWrite(result.Backup, string(existing)); err != nil {
			return result, fmt.Errorf("backup JSON config: %w", err)
		}
	}
	if err := options.WriteFile(options.Target, string(updated)); err != nil {
		return result, fmt.Errorf("write JSON config: %w", err)
	}
	result.Changed = true
	return result, nil
}

func validateConfigTarget(root, target string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("JSON config target %s escapes project scope %s", target, root)
	}
	if info, err := os.Lstat(targetAbs); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("JSON config target %s is a symlink", target)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	existing := filepath.Dir(targetAbs)
	for {
		if _, err := os.Lstat(existing); err == nil {
			break
		} else if !os.IsNotExist(err) {
			return err
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return fmt.Errorf("cannot resolve JSON config parent for %s", target)
		}
		existing = parent
	}
	resolvedParent, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return err
	}
	resolvedRoot, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return err
	}
	rel, err = filepath.Rel(resolvedRoot, resolvedParent)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("JSON config target %s resolves outside project scope %s", target, root)
	}
	return nil
}
