package core

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Deterministic, IO-free slicers that extract the minimal relevant block from a
// source artifact instead of handing a model the whole file. Each takes raw
// content plus a selector and returns (slice, found). When the selector matches
// nothing, found is false and slice is empty so the caller can decide whether to
// fall back to the whole artifact — the slicer never silently returns everything.
//
// All slicers preserve source order, normalize line endings to "\n", and trim
// trailing blank lines from each block so output is byte-stable across runs.

var (
	// sliceH2Re matches a level-2 Markdown heading ("## ...").
	sliceH2Re = regexp.MustCompile(`^##\s`)
	// sliceHeadingRe captures a Markdown ATX heading's level (run of #) and text.
	sliceHeadingRe = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*$`)
)

// trimTrailingBlank drops trailing all-whitespace lines from a block.
func trimTrailingBlank(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// TaskSlice returns the Markdown block for task taskID in tasksMd: the
// "- [ ] Tn — …" checkbox line plus its indented metadata lines, up to (but not
// including) the next task line, the next "## Wave N" header, or end of input.
// found is false when no task with that id exists; slice is then empty.
func TaskSlice(tasksMd, taskID string) (slice string, found bool) {
	lines := splitLines(tasksMd)
	start := -1
	for i, line := range lines {
		if m := taskRE.FindStringSubmatch(line); m != nil && m[2] == taskID {
			start = i
			break
		}
	}
	if start == -1 {
		return "", false
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if taskRE.MatchString(lines[i]) || waveRE.MatchString(lines[i]) {
			end = i
			break
		}
	}
	block := trimTrailingBlank(lines[start:end])
	return strings.Join(block, "\n"), true
}

// CoveredRequirements returns the "## Requirement N" blocks of reqMd whose
// numbers appear in ids, each running from its header to the next "## " heading
// or end of input. Blocks are emitted in ascending requirement-number order
// regardless of the order of ids, separated by a blank line. found is false when
// none of the ids match a requirement; slice is then empty.
func CoveredRequirements(reqMd string, ids []int) (slice string, found bool) {
	want := make(map[int]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	lines := splitLines(reqMd)
	blocks := map[int][]string{}
	cur := 0
	for _, line := range lines {
		if m := reqHeaderNumRe.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[1])
			if want[n] {
				cur = n
				blocks[n] = []string{line}
			} else {
				cur = 0
			}
			continue
		}
		// Any other level-2 heading closes the current requirement block.
		if sliceH2Re.MatchString(line) {
			cur = 0
			continue
		}
		if cur != 0 {
			blocks[cur] = append(blocks[cur], line)
		}
	}
	if len(blocks) == 0 {
		return "", false
	}
	nums := make([]int, 0, len(blocks))
	for n := range blocks {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	out := make([]string, 0, len(nums))
	for _, n := range nums {
		out = append(out, strings.Join(trimTrailingBlank(blocks[n]), "\n"))
	}
	return strings.Join(out, "\n\n"), true
}

// DesignSection returns the named section blocks of designMd. A heading matches
// when its text (after the leading #'s, trimmed) equals a requested heading,
// case-insensitively. A matched section runs from its heading to the next
// heading of the same or higher level (fewer or equal #'s), or end of input.
// Sections are emitted in document order, separated by a blank line. found is
// false when no requested heading matches; slice is then empty.
func DesignSection(designMd string, headings []string) (slice string, found bool) {
	want := make(map[string]bool, len(headings))
	for _, h := range headings {
		want[strings.ToLower(strings.TrimSpace(h))] = true
	}
	lines := splitLines(designMd)
	var out []string
	capturing := false
	capLevel := 0
	found = false
	for _, line := range lines {
		if m := sliceHeadingRe.FindStringSubmatch(line); m != nil {
			level := len(m[1])
			title := strings.ToLower(strings.TrimSpace(m[2]))
			if capturing && level <= capLevel {
				capturing = false
			}
			if !capturing && want[title] {
				capturing = true
				capLevel = level
				found = true
				if len(out) > 0 {
					out = append(trimTrailingBlank(out), "")
				}
				out = append(out, line)
				continue
			}
		}
		if capturing {
			out = append(out, line)
		}
	}
	if !found {
		return "", false
	}
	return strings.Join(trimTrailingBlank(out), "\n"), true
}

// RecentMemory returns the most recent n entries of memoryMd. Memory is
// append-only, so entries later in the file are more recent; this returns the
// last n "## <key>" blocks in document order. HTML comments (the template
// scaffolding) are stripped first so doc-only memory files yield no entries.
// found is false when memoryMd has no entries or n <= 0; slice is then empty.
func RecentMemory(memoryMd string, n int) (slice string, found bool) {
	if n <= 0 {
		return "", false
	}
	lines := splitLines(StripHTMLComments(memoryMd))
	var starts []int
	for i, line := range lines {
		if sliceH2Re.MatchString(line) {
			starts = append(starts, i)
		}
	}
	if len(starts) == 0 {
		return "", false
	}
	first := starts[0]
	if len(starts) > n {
		first = starts[len(starts)-n]
	}
	block := trimTrailingBlank(lines[first:])
	return strings.Join(block, "\n"), true
}
