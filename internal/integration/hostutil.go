package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

const specdServerName = "specd"

type CommandRunner func(ctx context.Context, dir, command string, args []string) ([]byte, error)

type AdapterDeps struct {
	Detector Detector
	Run      CommandRunner
	Now      func() time.Time
}

func defaultAdapterDeps() AdapterDeps {
	return AdapterDeps{
		Detector: DefaultDetector(),
		Run: func(ctx context.Context, dir, command string, args []string) ([]byte, error) {
			cmd := exec.CommandContext(ctx, command, args...)
			cmd.Dir = dir
			return cmd.CombinedOutput()
		},
		Now: time.Now,
	}
}

func normalizeAdapterDeps(deps AdapterDeps) AdapterDeps {
	defaults := defaultAdapterDeps()
	if deps.Detector.LookPath == nil {
		deps.Detector.LookPath = defaults.Detector.LookPath
	}
	if deps.Detector.Stat == nil {
		deps.Detector.Stat = defaults.Detector.Stat
	}
	if deps.Run == nil {
		deps.Run = defaults.Run
	}
	if deps.Now == nil {
		deps.Now = defaults.Now
	}
	return deps
}

func specdServer(root string) map[string]any {
	return map[string]any{
		"command": "specd",
		"args":    []string{"mcp", "--root", root},
	}
}

func inspectJSONServer(root, host, target string, scope Scope) (HostState, []byte, error) {
	return inspectJSONServerAtPath(root, host, target, scope, []string{"mcpServers"})
}

func inspectJSONServerAtPath(root, host, target string, scope Scope, keyPath []string) (HostState, []byte, error) {
	state := HostState{Host: host, Scope: scope, Target: target}
	data, err := os.ReadFile(target)
	if os.IsNotExist(err) {
		state.Reason = "project configuration does not exist"
		return state, nil, nil
	}
	if err != nil {
		return state, nil, err
	}

	var document map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return state, nil, fmt.Errorf("parse %s: %w", target, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return state, nil, fmt.Errorf("parse %s: trailing JSON content", target)
	} else if err != io.EOF {
		return state, nil, fmt.Errorf("parse %s: %w", target, err)
	}
	parent := document
	for _, key := range keyPath {
		value, ok := parent[key]
		if !ok {
			state.Reason = strings.Join(keyPath, ".") + " object is absent"
			return state, nil, nil
		}
		next, ok := value.(map[string]any)
		if !ok {
			return state, nil, fmt.Errorf("JSON key %q is not an object", key)
		}
		parent = next
	}
	server, ok := parent[specdServerName]
	if !ok {
		state.Reason = "specd server is not registered"
		return state, nil, nil
	}
	canonical, err := json.Marshal(server)
	if err != nil {
		return state, nil, fmt.Errorf("encode %s server entry: %w", host, err)
	}
	state.Fingerprint = Fingerprint(canonical)
	state.Registered = matchesSpecdServer(server, root)
	if !state.Registered {
		state.Reason = "specd server entry does not match the project root"
		return state, canonical, nil
	}
	state.Reason = "specd server is registered for this project"

	manifest, err := LoadManifest(core.IntegrationsPath(root))
	if err != nil {
		return state, canonical, err
	}
	for _, entry := range manifest.Entries {
		if entry.Host == host && entry.Scope == scope && entry.ServerName == specdServerName {
			state.Owned = entry.Fingerprint == state.Fingerprint
			if !state.Owned {
				state.Reason = "specd server registration differs from the owned manifest entry"
			}
			break
		}
	}
	return state, canonical, nil
}

func installWorkspaceJSON(
	plan HostPlan,
	deps AdapterDeps,
	target string,
	keyPath []string,
	server map[string]any,
	nextAction string,
) (HostResult, error) {
	result := HostResult{
		Host:       plan.Host,
		Status:     "configured",
		Targets:    []string{target},
		Backups:    []string{},
		Warnings:   []string{},
		NextAction: nextAction,
	}
	if err := validateConfigTarget(plan.Root, target); err != nil {
		return result, err
	}
	before, _, err := inspectJSONServerAtPath(plan.Root, plan.Host, target, plan.Scope, keyPath)
	if err != nil {
		result.Status = "manual"
		result.Warnings = []string{err.Error()}
		result.NextAction = "repair the host configuration or merge the generated specd snippet manually"
		return result, nil
	}
	if before.Registered {
		if before.Owned {
			return result, nil
		}
		if before.Reason == "specd server registration differs from the owned manifest entry" {
			return result, fmt.Errorf("%s registration ownership mismatch at %s", plan.Host, target)
		}
		result.Status = "existing"
		result.Warnings = append(result.Warnings, "matching unowned specd registration left unchanged")
		return result, nil
	}
	if before.Fingerprint != "" {
		return result, fmt.Errorf("%s has an existing unowned specd registration at %s", plan.Host, target)
	}

	merged, err := MergeJSONServer(JSONMergeOptions{
		Root:       plan.Root,
		Target:     target,
		KeyPath:    keyPath,
		ServerName: specdServerName,
		Server:     server,
		Now:        deps.Now,
	})
	if err != nil {
		result.Status = "manual"
		result.Warnings = []string{err.Error()}
		result.NextAction = "repair the host configuration or merge the generated specd snippet manually"
		return result, nil
	}
	if merged.Backup != "" {
		result.Backups = append(result.Backups, merged.Backup)
	}
	after, canonical, err := inspectJSONServerAtPath(plan.Root, plan.Host, target, plan.Scope, keyPath)
	if err != nil {
		return result, err
	}
	if !after.Registered {
		return result, fmt.Errorf("%s workspace merge did not install the expected project registration", plan.Host)
	}
	if err := recordIntegration(plan.Root, plan.Host, plan.Scope, target, "project-json", canonical, deps.Now()); err != nil {
		return result, fmt.Errorf("record %s integration: %w", plan.Host, err)
	}
	result.Changed = merged.Changed
	return result, nil
}

func matchesSpecdServer(server any, root string) bool {
	object, ok := server.(map[string]any)
	if !ok || object["command"] != "specd" {
		return false
	}
	rawArgs, ok := object["args"].([]any)
	if !ok || len(rawArgs) != 3 {
		return false
	}
	want := []string{"mcp", "--root", root}
	for i, value := range rawArgs {
		if value != want[i] {
			return false
		}
	}
	return true
}

func installNativeJSON(
	ctx context.Context,
	deps AdapterDeps,
	plan HostPlan,
	target string,
) (HostResult, error) {
	result := HostResult{
		Host:       plan.Host,
		Status:     "configured",
		Targets:    []string{target},
		Backups:    []string{},
		Warnings:   []string{},
		NextAction: "reload the coding agent and confirm the specd tools are available",
	}
	if err := validateConfigTarget(plan.Root, target); err != nil {
		return result, err
	}
	before, _, err := inspectJSONServer(plan.Root, plan.Host, target, plan.Scope)
	if err != nil {
		return result, err
	}
	if before.Registered {
		if before.Owned {
			return result, nil
		}
		if before.Reason == "specd server registration differs from the owned manifest entry" {
			return result, fmt.Errorf("%s registration ownership mismatch at %s", plan.Host, target)
		}
		result.Status = "existing"
		result.Warnings = append(result.Warnings, "matching unowned specd registration left unchanged")
		return result, nil
	}
	if before.Fingerprint != "" {
		return result, fmt.Errorf("%s has an existing unowned specd registration at %s", plan.Host, target)
	}
	if len(plan.Actions) != 1 || plan.Actions[0].Command == "" {
		return result, fmt.Errorf("%s native install plan has no command", plan.Host)
	}

	action := plan.Actions[0]
	output, err := deps.Run(ctx, plan.Root, action.Command, action.Args)
	if err != nil {
		return result, fmt.Errorf("%s registration failed: %w: %s", plan.Host, err, bytes.TrimSpace(output))
	}
	after, canonical, err := inspectJSONServer(plan.Root, plan.Host, target, plan.Scope)
	if err != nil {
		return result, err
	}
	if !after.Registered {
		return result, fmt.Errorf("%s command completed without installing the expected project registration", plan.Host)
	}
	if err := recordIntegration(plan.Root, plan.Host, plan.Scope, target, "native-cli", canonical, deps.Now()); err != nil {
		return result, fmt.Errorf("record %s integration: %w", plan.Host, err)
	}
	result.Changed = true
	return result, nil
}

func recordIntegration(root, host string, scope Scope, target, method string, content []byte, now time.Time) error {
	manifestPath := core.IntegrationsPath(root)
	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		return err
	}
	relativeTarget, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	entry := ManifestEntry{
		Host:         host,
		Scope:        scope,
		ServerName:   specdServerName,
		Root:         ".",
		RootStrategy: "project",
		Method:       method,
		Target:       filepath.ToSlash(relativeTarget),
		Fingerprint:  Fingerprint(content),
		InstalledAt:  now.UTC().Format(time.RFC3339Nano),
	}
	replaced := false
	for i := range manifest.Entries {
		current := manifest.Entries[i]
		if current.Host == host && current.Scope == scope && current.ServerName == specdServerName {
			manifest.Entries[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		manifest.Entries = append(manifest.Entries, entry)
	}
	sort.Slice(manifest.Entries, func(i, j int) bool {
		return manifest.Entries[i].Host < manifest.Entries[j].Host
	})
	return SaveManifest(manifestPath, manifest)
}
