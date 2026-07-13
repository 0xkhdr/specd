package core

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

type MemoryLintFinding struct{ Key, Message string }

// AnalyzeMemoryConflicts finds only deterministic, explicit conflicts. Unknown
// confidence is never treated as evidence; stale/superseded records are ignored.
func AnalyzeMemoryConflicts(blocks []MemBlock, asOf time.Time) []MemoryLintFinding {
	blocks = append([]MemBlock(nil), blocks...)
	sort.SliceStable(blocks, func(i, j int) bool {
		a, b := normalizeMemoryText(blocks[i].Key), normalizeMemoryText(blocks[j].Key)
		if a != b {
			return a < b
		}
		return blocks[i].Source < blocks[j].Source
	})
	var out []MemoryLintFinding
	seen := map[string]MemBlock{}
	type constraint struct {
		negative bool
		block    MemBlock
	}
	constraints := map[string]constraint{}
	for _, b := range blocks {
		if !activeMemoryAt(b, asOf) {
			continue
		}
		key := normalizeMemoryText(b.Key)
		if prior, ok := seen[key]; ok {
			out = append(out, MemoryLintFinding{"duplicate:" + key, fmt.Sprintf("duplicate normalized memory key %q: %q (%s) and %q (%s)", key, prior.Key, memoryProvenance(prior), b.Key, memoryProvenance(b))})
		} else {
			seen[key] = b
		}
		if strings.Contains(strings.ToLower(b.Raw), "mode=forced") && (strings.TrimSpace(b.Owner) == "" || !strings.Contains(b.Raw, "authority=") || !strings.Contains(b.Raw, "provenance=")) {
			out = append(out, MemoryLintFinding{"forced:" + key, fmt.Sprintf("forced memory promotion %q requires owner, authority, and provenance", b.Key)})
		}
		if !strings.EqualFold(strings.TrimSpace(b.Criticality), "critical") {
			continue
		}
		subject, negative, explicit := memoryConstraint(b.Pattern)
		if !explicit {
			continue
		}
		if prior, ok := constraints[subject]; ok && prior.negative != negative {
			out = append(out, MemoryLintFinding{"conflict:" + subject, fmt.Sprintf("contradictory active critical memory %q: %q (%s) conflicts with %q (%s)", subject, prior.block.Key, memoryProvenance(prior.block), b.Key, memoryProvenance(b))})
		} else if !ok {
			constraints[subject] = constraint{negative, b}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func normalizeMemoryText(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}
func activeMemoryAt(b MemBlock, asOf time.Time) bool {
	if b.Status != "" && !strings.EqualFold(b.Status, "active") {
		return false
	}
	if strings.TrimSpace(b.SupersededBy) != "" {
		return false
	}
	if b.ExpiresAt == "" || asOf.IsZero() {
		return true
	}
	expires, err := time.Parse("2006-01-02", b.ExpiresAt)
	return err == nil && asOf.UTC().Before(expires)
}
func memoryConstraint(pattern string) (string, bool, bool) {
	p := normalizeMemoryText(pattern)
	for _, prefix := range []string{"must not ", "never "} {
		if strings.HasPrefix(p, prefix) {
			return strings.TrimPrefix(p, prefix), true, true
		}
	}
	for _, prefix := range []string{"must ", "always "} {
		if strings.HasPrefix(p, prefix) {
			return strings.TrimPrefix(p, prefix), false, true
		}
	}
	return "", false, false
}
func memoryProvenance(b MemBlock) string {
	if p := strings.TrimSpace(b.Provenance); p != "" {
		return p
	}
	if p := strings.TrimSpace(b.Source); p != "" {
		return p
	}
	return "provenance=unknown"
}

// memBlockRE matches a markdown H2 heading, the boundary between memory blocks.
var memBlockRE = regexp.MustCompile(`(?m)^##\s+`)

// MemFields is one steering-memory entry. Related is the raw comma-separated
// value as supplied on the CLI; RenderMemBlock formats it into wikilinks.
type MemFields struct {
	Key                                                                   string
	Pattern                                                               string
	Detail                                                                string
	Source                                                                string
	Criticality                                                           string
	Related                                                               string
	Owner, LastValidatedAt, Provenance, Confidence, ExpiresAt, Supersedes string
}

// MemBlock is one indexed H2 memory record. Raw preserves selected bytes;
// Digest pins that representation without exposing it in a manifest.
type MemBlock struct {
	Key, Pattern, Detail, Source, Criticality, Related, Status, SupersededBy, AppliesTo, Raw, Digest string
	Owner, LastValidatedAt, Provenance, Confidence, ExpiresAt, Supersedes                            string
}

// IndexMemBlocks parses memory into a stable key-sorted block index. Duplicate
// keys fail closed because a selector must identify exactly one representation.
func IndexMemBlocks(text string) ([]MemBlock, error) {
	lines := strings.Split(text, "\n")
	var blocks []MemBlock
	seen := map[string]bool{}
	for i := 0; i < len(lines); {
		if !memBlockRE.MatchString(lines[i]) {
			i++
			continue
		}
		start := i
		key := strings.TrimSpace(strings.TrimPrefix(lines[i], "##"))
		i++
		for i < len(lines) && !memBlockRE.MatchString(lines[i]) {
			i++
		}
		if key == "" || seen[key] {
			return nil, fmt.Errorf("memory block key %q is empty or duplicated", key)
		}
		seen[key] = true
		raw := strings.TrimRight(strings.Join(lines[start:i], "\n"), " \t\n")
		b := MemBlock{Key: key, Raw: raw, Digest: Digest([]byte(raw))}
		for _, line := range strings.Split(raw, "\n") {
			field := func(prefix string) string { return strings.TrimSpace(strings.TrimPrefix(line, prefix)) }
			switch {
			case strings.HasPrefix(line, "**Pattern:**"):
				b.Pattern = field("**Pattern:**")
			case strings.HasPrefix(line, "**Detail:**"):
				b.Detail = field("**Detail:**")
			case strings.HasPrefix(line, "**Source:**"):
				b.Source = field("**Source:**")
			case strings.HasPrefix(line, "**Criticality:**"):
				b.Criticality = field("**Criticality:**")
			case strings.HasPrefix(line, "**Related:**"):
				b.Related = field("**Related:**")
			case strings.HasPrefix(line, "**Status:**"):
				b.Status = field("**Status:**")
			case strings.HasPrefix(line, "**Superseded-By:**"):
				b.SupersededBy = field("**Superseded-By:**")
			case strings.HasPrefix(line, "**Applies-To:**"):
				b.AppliesTo = field("**Applies-To:**")
			case strings.HasPrefix(line, "**Owner:**"):
				b.Owner = field("**Owner:**")
			case strings.HasPrefix(line, "**Last-Validated-At:**"):
				b.LastValidatedAt = field("**Last-Validated-At:**")
			case strings.HasPrefix(line, "**Provenance:**"):
				b.Provenance = field("**Provenance:**")
			case strings.HasPrefix(line, "**Confidence:**"):
				b.Confidence = field("**Confidence:**")
			case strings.HasPrefix(line, "**Expires-At:**"):
				b.ExpiresAt = field("**Expires-At:**")
			case strings.HasPrefix(line, "**Supersedes:**"):
				b.Supersedes = field("**Supersedes:**")
			}
		}
		blocks = append(blocks, b)
	}
	sort.Slice(blocks, func(i, j int) bool { return blocks[i].Key < blocks[j].Key })
	return blocks, nil
}

// RenderMemBlock renders a byte-stable `## <key>` block. Output starts at the
// heading and ends with a trailing newline; callers prepend a blank line to
// separate appended blocks. Pure function of its input.
func RenderMemBlock(f MemFields) string {
	out := fmt.Sprintf("## %s\n**Pattern:** %s\n**Detail:** %s\n**Source:** %s\n**Criticality:** %s\n**Related:** %s\n",
		f.Key, f.Pattern, f.Detail, f.Source, f.Criticality, renderRelated(f.Related))
	for _, field := range []struct{ name, value string }{
		{"Owner", f.Owner}, {"Last-Validated-At", f.LastValidatedAt}, {"Provenance", f.Provenance},
		{"Confidence", f.Confidence}, {"Expires-At", f.ExpiresAt}, {"Supersedes", f.Supersedes},
	} {
		if field.value != "" {
			out += fmt.Sprintf("**%s:** %s\n", field.name, field.value)
		}
	}
	return out + "**Status:** active\n"
}

// ValidateMemoryProvenance admits only durable local evidence, review, or
// governed-exception references. Free-form notes cannot become selected memory.
func ValidateMemoryProvenance(source string) error {
	parts := strings.SplitN(source, ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return errors.New("memory source must use evidence:, review:, or exception: provenance")
	}
	kind, ref := parts[0], strings.TrimSpace(parts[1])
	switch kind {
	case "evidence", "review":
		if err := validateEvidenceRef(ref); err != nil {
			return fmt.Errorf("memory %s provenance: %w", kind, err)
		}
	case "exception":
		if strings.ContainsAny(ref, " /\\\t\r\n") {
			return errors.New("memory exception provenance must be a stable identifier")
		}
	default:
		return errors.New("memory source must use evidence:, review:, or exception: provenance")
	}
	return nil
}

// renderRelated turns "a, b" into "[[a]], [[b]]"; empty input renders as "—".
func renderRelated(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "—"
	}
	var linked []string
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			linked = append(linked, "[["+p+"]]")
		}
	}
	if len(linked) == 0 {
		return "—"
	}
	return strings.Join(linked, ", ")
}

// ExtractMemBlock returns the `## <key>` block from text, from the heading up to
// (excluding) the next H2 heading, trailing whitespace trimmed. Returns "" when
// the key is absent. Pure function.
func ExtractMemBlock(text, key string) string {
	lines := strings.Split(text, "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "## "+key {
			start = i
			break
		}
	}
	if start == -1 {
		return ""
	}
	body := []string{lines[start]}
	for i := start + 1; i < len(lines); i++ {
		if memBlockRE.MatchString(lines[i]) {
			break
		}
		body = append(body, lines[i])
	}
	return strings.TrimRight(strings.Join(body, "\n"), " \t\n")
}

// CountSpecsWithBlock counts specs whose memory.md contains a `## <key>` block.
// The promotion threshold is a pure count of on-disk state — no LLM (RM.4).
func CountSpecsWithBlock(root, key string) int {
	count := 0
	for _, slug := range ListSpecs(root) {
		raw := ReadOrNull(SpecMemoryPath(root, slug))
		if raw != nil && ExtractMemBlock(*raw, key) != "" {
			count++
		}
	}
	return count
}

// RenderPromotion renders the block plus a deterministic provenance line for the
// steering store. date is pre-formatted (UTC) by the caller so output is
// byte-deterministic under an injected clock (RM.7). Pure function.
type PromotionAudit struct {
	Forced                bool
	Authority, Provenance string
}

func RenderPromotion(block, slug string, count int, date string, audits ...PromotionAudit) string {
	line := fmt.Sprintf("**Promoted:** from spec '%s' on %s (seen in %d spec(s))", slug, date, count)
	if len(audits) > 0 && audits[0].Forced {
		line += fmt.Sprintf(" [mode=forced authority=%s provenance=%s]", audits[0].Authority, audits[0].Provenance)
	}
	return fmt.Sprintf("\n%s\n%s\n", block, line)
}
