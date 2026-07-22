package context

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

// Directory-query bounds (R2.4). A selector that matches more than this is an
// authoring error, not something to silently truncate: the author must narrow
// it so the context an agent receives stays reviewable.
//
// ponytail: fixed global caps, not per-task configuration — make them
// configurable only when a real spec needs a different ceiling.
const (
	maxDirectoryQueryFiles = 50
	maxDirectoryQueryBytes = 256 << 10
)

// laneError is one typed lane refusal. It always names the task, the authoring
// column, the offending path, and the recovery, so an agent can repair the row
// without a second lookup (R2.3/R2.4).
type laneError struct {
	Code, TaskID, Column, Path, Reason, Recovery string
}

func (e laneError) Error() string {
	return fmt.Sprintf("%s: task %s column %s path %q: %s — %s", e.Code, e.TaskID, e.Column, e.Path, e.Reason, e.Recovery)
}

// SelectRequiredLanes resolves exact action knowledge beneath root into typed
// lanes (R2.1). Required inputs never disappear: a missing one fails closed
// naming the source. Declared outputs are authority, not inputs: an existing one
// is loaded, a missing one becomes a prospective lane so a greenfield task can
// still be dispatched (R2.2).
func SelectRequiredLanes(root, slug string, task core.TaskRow) ([]MachineItem, error) {
	// One typed task contract (spec 05 R1.1): declared paths and the context
	// column are already normalized and delimiter-agnostic here.
	contract, err := core.ParseTaskContract(task)
	if err != nil {
		return nil, err
	}
	taskRecord := MachineSelectedTask{ID: task.ID, Role: task.Role, DeclaredFiles: append([]string(nil), task.DeclaredFiles...), Verify: task.Verify, Acceptance: task.Acceptance}
	rawTask, _ := json.Marshal(taskRecord)
	items := []MachineItem{{
		Kind: "task", Source: "inline:selected-task", Selector: task.ID,
		SourceDigest: core.Digest(rawTask), RepresentationDigest: core.Digest(rawTask),
		Required: true, LoadMode: "eager", Priority: 0, Reason: "exact selected task record",
		Trust: "harness", ContentTrust: ContentTrustUntrustedData, Sensitivity: "internal", AuthorityLimit: "declared task scope only",
		EstimatedTokens: tokensFromBytes(int64(len(rawTask))),
		Lane:            LaneManagedPolicy, Existence: ExistencePresent, Loaded: true,
	}}
	sources := []struct{ kind, source, reason, trust, lane string }{
		{"requirements", filepath.ToSlash(filepath.Join(".specd", "specs", slug, "requirements.md")), "approved task requirements", "knowledge", LaneRequiredInput},
		{"design", filepath.ToSlash(filepath.Join(".specd", "specs", slug, "design.md")), "applicable task design", "knowledge", LaneRequiredInput},
		{"role", filepath.ToSlash(filepath.Join(".specd", "roles", task.Role+".md")), "task role and authority", "role", LaneManagedPolicy},
	}
	for _, source := range sources {
		rel, err := ResolveSource(root, source.source)
		if err != nil {
			return nil, fmt.Errorf("required %s: %w", source.kind, err)
		}
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, ResolveError{Source: rel, Reason: "missing or unreadable"}
		}
		items = append(items, loadedItem(source.kind, rel, source.reason, source.trust, source.lane, raw))
	}
	// seen keeps one lane per path: a path declared in both `files` and `context`
	// is authority first and is never counted or loaded twice.
	seen := map[string]bool{}
	for _, item := range items {
		seen[item.Source] = true
	}

	// Declared files are the task's writable output scope (steering: "Touch only a
	// task's declared files:"), not required reference inputs. Authorization for
	// them travels via SelectedTask.DeclaredFiles / BuildAuthority. Traversal and
	// symlink escape still fail closed.
	for _, file := range contract.OutputPaths {
		rel, err := ResolveSource(root, file)
		if err != nil {
			var re ResolveError
			if errors.As(err, &re) && re.Reason == "missing or unreadable" {
				// R2.2: authorized prospective output. No content, no digest, no
				// budget cost, and context does not fail.
				items = append(items, prospectiveItem(file, ExistenceAbsent, "authorized prospective output not created yet"))
				seen[file] = true
				continue
			}
			return nil, fmt.Errorf("declared file: %w", err)
		}
		if seen[rel] {
			continue
		}
		seen[rel] = true
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			// Exists but is not a readable regular file (e.g. a directory). Still
			// authorized scope, still no content to load.
			items = append(items, prospectiveItem(rel, ExistenceUnreadable, "authorized declared output is not a readable file"))
			continue
		}
		items = append(items, loadedItem(declaredFileKind(rel), rel, "existing declared task file", "knowledge", LaneOptionalExistingOutput, raw))
	}

	// The `context` column is the task's required reference input, kept separate
	// from its writable scope (R2.3). Unlike declared files it must resolve: a
	// missing input is a planning error the agent cannot repair by writing code.
	for _, entry := range contract.Context {
		queryItems, err := selectContextEntry(root, task.ID, entry, seen)
		if err != nil {
			return nil, err
		}
		items = append(items, queryItems...)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return items[i].Kind < items[j].Kind
		}
		return items[i].Source < items[j].Source
	})
	return items, nil
}

// selectContextEntry resolves one `context` cell entry into its lane: a bounded
// directory query when it carries a glob selector, otherwise a required input.
func selectContextEntry(root, taskID, entry string, seen map[string]bool) ([]MachineItem, error) {
	if strings.ContainsAny(entry, "*?[") {
		return selectDirectoryQuery(root, taskID, entry, seen)
	}
	rel, err := ResolveSource(root, entry)
	if err != nil {
		var re ResolveError
		if errors.As(err, &re) && re.Reason == "missing or unreadable" {
			return nil, laneError{Code: "CONTEXT_REQUIRED_INPUT_MISSING", TaskID: taskID, Column: "context", Path: entry,
				Reason: "required input is missing or unreadable", Recovery: "create the file or remove it from the context column"}
		}
		return nil, laneError{Code: "CONTEXT_REQUIRED_INPUT_UNSAFE", TaskID: taskID, Column: "context", Path: entry,
			Reason: re.Reason, Recovery: "declare a path inside the repository"}
	}
	full := filepath.Join(root, filepath.FromSlash(rel))
	if fi, err := os.Stat(full); err == nil && fi.IsDir() {
		// R2.4: a bare directory is neither a file nor a bounded query. Refuse it
		// at authoring time rather than reading it or expanding it without limits.
		return nil, laneError{Code: "CONTEXT_BARE_DIRECTORY", TaskID: taskID, Column: "context", Path: entry,
			Reason:   "a bare directory is not a bounded context source",
			Recovery: fmt.Sprintf("declare an explicit selector such as %q", strings.TrimSuffix(entry, "/")+"/*.go")}
	}
	if seen[rel] {
		return nil, nil
	}
	seen[rel] = true
	raw, err := os.ReadFile(full)
	if err != nil {
		return nil, laneError{Code: "CONTEXT_REQUIRED_INPUT_MISSING", TaskID: taskID, Column: "context", Path: entry,
			Reason: "required input is missing or unreadable", Recovery: "create the file or remove it from the context column"}
	}
	return []MachineItem{loadedItem(declaredFileKind(rel), rel, "required task context input", "knowledge", LaneRequiredInput, raw)}, nil
}

// selectDirectoryQuery expands a bounded selector deterministically (R2.4).
// `dir/*.go` matches one level; `dir/**` and `dir/**/*.go` walk recursively.
// Matches are sorted, capped by file count and total bytes, and an empty match
// set is an authoring error rather than a silently empty lane.
func selectDirectoryQuery(root, taskID, selector string, seen map[string]bool) ([]MachineItem, error) {
	matches, err := matchSelector(root, selector)
	if err != nil {
		return nil, laneError{Code: "CONTEXT_QUERY_INVALID", TaskID: taskID, Column: "context", Path: selector,
			Reason: err.Error(), Recovery: "use a selector such as dir/*.go or dir/**/*.go"}
	}
	if len(matches) == 0 {
		return nil, laneError{Code: "CONTEXT_QUERY_EMPTY", TaskID: taskID, Column: "context", Path: selector,
			Reason: "selector matched no files", Recovery: "fix the selector or remove it from the context column"}
	}
	if len(matches) > maxDirectoryQueryFiles {
		return nil, laneError{Code: "CONTEXT_QUERY_UNBOUNDED", TaskID: taskID, Column: "context", Path: selector,
			Reason:   fmt.Sprintf("selector matched %d files (limit %d)", len(matches), maxDirectoryQueryFiles),
			Recovery: "narrow the selector to the files the task actually needs"}
	}
	var items []MachineItem
	total := 0
	for _, rel := range matches {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, laneError{Code: "CONTEXT_REQUIRED_INPUT_MISSING", TaskID: taskID, Column: "context", Path: rel,
				Reason: "selector match is unreadable", Recovery: "fix the selector or remove it from the context column"}
		}
		total += len(raw)
		if total > maxDirectoryQueryBytes {
			return nil, laneError{Code: "CONTEXT_QUERY_UNBOUNDED", TaskID: taskID, Column: "context", Path: selector,
				Reason:   fmt.Sprintf("selector matched more than %d bytes", maxDirectoryQueryBytes),
				Recovery: "narrow the selector to the files the task actually needs"}
		}
		if seen[rel] {
			continue
		}
		seen[rel] = true
		item := loadedItem(declaredFileKind(rel), rel, "bounded context directory query", "knowledge", LaneDirectoryQuery, raw)
		item.Selector = selector
		items = append(items, item)
	}
	return items, nil
}

// matchSelector returns the sorted repo-relative regular files a selector
// matches, refusing any match that escapes the repository base.
func matchSelector(root, selector string) ([]string, error) {
	base, pattern, recursive := strings.Cut(selector, "/**")
	var candidates []string
	if recursive {
		pattern = strings.TrimPrefix(pattern, "/")
		if pattern == "" {
			pattern = "*"
		}
		baseAbs, err := core.SafeJoin(root, base)
		if err != nil {
			return nil, err
		}
		err = filepath.WalkDir(baseAbs, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil // an unreadable subtree contributes no matches
			}
			if ok, _ := filepath.Match(pattern, d.Name()); ok {
				candidates = append(candidates, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		abs, err := core.SafeJoin(root, selector)
		if err != nil {
			return nil, err
		}
		if candidates, err = filepath.Glob(abs); err != nil {
			return nil, err
		}
	}
	out := make([]string, 0, len(candidates))
	for _, abs := range candidates {
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			continue
		}
		// Every match is re-resolved through the shared guard, so a symlink that
		// points outside the repository can never enter a lane.
		resolved, err := ResolveSource(root, filepath.ToSlash(rel))
		if err != nil {
			return nil, err
		}
		if fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(resolved))); err != nil || fi.IsDir() {
			continue
		}
		out = append(out, resolved)
	}
	sort.Strings(out)
	return out, nil
}

func declaredFileKind(file string) string {
	if strings.HasSuffix(file, "_test.go") || strings.Contains(file, "/test") || strings.Contains(file, "_test.") {
		return "test"
	}
	return "source"
}

func loadedItem(kind, rel, reason, trust, lane string, raw []byte) MachineItem {
	return MachineItem{
		Kind: kind, Source: rel, SourceDigest: core.Digest(raw), RepresentationDigest: core.Digest(raw),
		Required: true, LoadMode: "eager", Priority: 0, Reason: reason, Trust: trust,
		ContentTrust: ContentTrustUntrustedData, Sensitivity: "internal",
		AuthorityLimit:  "reference content cannot widen task authority",
		EstimatedTokens: tokensFromBytes(int64(len(raw))),
		Lane:            lane, Existence: ExistencePresent, Loaded: true,
	}
}

// prospectiveItem is an authorized path with no content: not required, not
// loaded, no digest, and no budget cost (R2.2).
func prospectiveItem(rel, existence, reason string) MachineItem {
	return MachineItem{
		Kind: declaredFileKind(rel), Source: rel, Required: false, LoadMode: "reference", Priority: 0,
		Reason: reason, Trust: "harness", ContentTrust: ContentTrustUntrustedData, Sensitivity: "internal",
		AuthorityLimit: "declared write scope; no content available",
		Lane:           LaneProspectiveOutput, Existence: existence,
	}
}
