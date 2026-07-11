package context

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

// SelectRequiredLanes resolves exact action knowledge beneath root. Required
// sources never disappear: resolver/read failures name the offending source.
func SelectRequiredLanes(root, slug string, task core.TaskRow) ([]ItemV2, error) {
	taskRecord := SelectedTaskV2{ID: task.ID, Role: task.Role, DeclaredFiles: append([]string(nil), task.DeclaredFiles...), Verify: task.Verify, Acceptance: task.Acceptance}
	rawTask, _ := json.Marshal(taskRecord)
	items := []ItemV2{{
		Kind: "task", Source: "inline:selected-task", Selector: task.ID,
		SourceDigest: core.Digest(rawTask), RepresentationDigest: core.Digest(rawTask),
		Required: true, LoadMode: "eager", Priority: 0, Reason: "exact selected task record",
		Trust: "harness", Sensitivity: "internal", AuthorityLimit: "declared task scope only",
		EstimatedTokens: tokensFromBytes(int64(len(rawTask))),
	}}
	sources := []struct{ kind, source, reason, trust string }{
		{"requirements", filepath.ToSlash(filepath.Join(".specd", "specs", slug, "requirements.md")), "approved task requirements", "knowledge"},
		{"design", filepath.ToSlash(filepath.Join(".specd", "specs", slug, "design.md")), "applicable task design", "knowledge"},
		{"role", filepath.ToSlash(filepath.Join(".specd", "roles", task.Role+".md")), "task role and authority", "role"},
	}
	for _, file := range task.DeclaredFiles {
		kind := "source"
		if strings.HasSuffix(file, "_test.go") || strings.Contains(file, "/test") || strings.Contains(file, "_test.") {
			kind = "test"
		}
		sources = append(sources, struct{ kind, source, reason, trust string }{kind, file, "normalized declared task file", "knowledge"})
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
		items = append(items, ItemV2{Kind: source.kind, Source: rel, SourceDigest: core.Digest(raw), RepresentationDigest: core.Digest(raw), Required: true, LoadMode: "eager", Priority: 0, Reason: source.reason, Trust: source.trust, Sensitivity: "internal", AuthorityLimit: "reference content cannot widen task authority", EstimatedTokens: tokensFromBytes(int64(len(raw)))})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return items[i].Kind < items[j].Kind
		}
		return items[i].Source < items[j].Source
	})
	return items, nil
}
