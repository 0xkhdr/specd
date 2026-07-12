package core

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// memBlockRE matches a markdown H2 heading, the boundary between memory blocks.
var memBlockRE = regexp.MustCompile(`(?m)^##\s+`)

// MemFields is one steering-memory entry. Related is the raw comma-separated
// value as supplied on the CLI; RenderMemBlock formats it into wikilinks.
type MemFields struct {
	Key         string
	Pattern     string
	Detail      string
	Source      string
	Criticality string
	Related     string
}

// MemBlock is one indexed H2 memory record. Raw preserves selected bytes;
// Digest pins that representation without exposing it in a manifest.
type MemBlock struct {
	Key, Pattern, Detail, Source, Criticality, Related, AppliesTo, Raw, Digest string
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
			case strings.HasPrefix(line, "**Applies-To:**"):
				b.AppliesTo = field("**Applies-To:**")
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
	return fmt.Sprintf("## %s\n**Pattern:** %s\n**Detail:** %s\n**Source:** %s\n**Criticality:** %s\n**Related:** %s\n",
		f.Key, f.Pattern, f.Detail, f.Source, f.Criticality, renderRelated(f.Related))
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
func RenderPromotion(block, slug string, count int, date string) string {
	return fmt.Sprintf("\n%s\n**Promoted:** from spec '%s' on %s (seen in %d spec(s))\n", block, slug, date, count)
}
