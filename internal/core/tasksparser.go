package core

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/0xkhdr/specd/internal/spec"
)

var (
	MandatoryKeys = []string{"why", "role", "files", "contract", "acceptance", "verify", "depends"}
	KeyOrder      = []string{"why", "role", "files", "contract", "acceptance", "verify", "depends", "requirements"}
	ValidRoles    = []string{"investigator", "builder", "reviewer", "verifier"}

	validRoleSet = sliceToSet(ValidRoles)
)

// ReadonlyRoles and IsReadonlyRole are re-exported from internal/spec so both
// core and the context engine share a single source of truth for the role set.
var ReadonlyRoles = spec.ReadonlyRoles

// IsValidRole reports whether r is a recognized task role.
func IsValidRole(r string) bool { return validRoleSet[r] }

// IsReadonlyRole reports whether r is a read-only role (no runnable verify
// command required to complete).
func IsReadonlyRole(r string) bool { return spec.IsReadonlyRole(r) }

func sliceToSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

type AnnotationKind string

const (
	AnnotComplete AnnotationKind = "complete"
	AnnotBlocked  AnnotationKind = "blocked"
)

type Annotation struct {
	Kind     AnnotationKind
	Evidence string // for complete
	Ts       string // for complete
	Reason   string // for blocked
}

type ParsedTask struct {
	ID         string
	Title      string
	Wave       int
	Checked    bool
	Meta       map[string]string
	Annotation *Annotation
	Line       int
}

type ParsedTasks struct {
	Title string
	Tasks []ParsedTask
}

var (
	taskRE          = regexp.MustCompile(`^- \[( |x)\] (T\d+) — (.*)$`)
	waveRE          = regexp.MustCompile(`^## Wave (\d+)\s*$`)
	titleRE         = regexp.MustCompile(`^# Tasks — (.*)$`)
	metaRE          = regexp.MustCompile(`^  - ([a-z]+): (.*)$`)
	annotCompleteRE = regexp.MustCompile(` ✓ complete · evidence: (.*?) · ([^·]*)$`)
	annotBlockedRE  = regexp.MustCompile(` ⚠ blocked · reason: (.*)$`)
)

func splitAnnotation(rawTitle string) (title string, ann *Annotation) {
	if m := annotCompleteRE.FindStringSubmatchIndex(rawTitle); m != nil {
		subs := annotCompleteRE.FindStringSubmatch(rawTitle)
		return strings.TrimRight(rawTitle[:m[0]], " "), &Annotation{
			Kind:     AnnotComplete,
			Evidence: decodeAnnotationField(subs[1]),
			Ts:       decodeAnnotationField(strings.TrimSpace(subs[2])),
		}
	}
	if m := annotBlockedRE.FindStringSubmatchIndex(rawTitle); m != nil {
		subs := annotBlockedRE.FindStringSubmatch(rawTitle)
		return strings.TrimRight(rawTitle[:m[0]], " "), &Annotation{
			Kind:   AnnotBlocked,
			Reason: decodeAnnotationField(subs[1]),
		}
	}
	return rawTitle, nil
}

// Annotation fields (evidence, reason, ts) are agent-authored free text written
// onto a single tasks.md line whose fields are delimited by " · ". A newline
// would split the line and a literal "·" could collide with the delimiter, so
// fields are encoded on write and decoded on read. The transform is lossless:
// decodeAnnotationField(encodeAnnotationField(s)) == s for any s.
//
//	\  -> \\        (escape introducer)
//	\n -> \n  \r -> \r   (newline normalization keeps fields single-line)
//	·  -> \m        (protects the field delimiter)
//
// Readers tolerate legacy unescaped fields: a string with no "\" escape
// sequences decodes to itself.
func encodeAnnotationField(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '·':
			b.WriteString(`\m`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func decodeAnnotationField(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		if rs[i] == '\\' && i+1 < len(rs) {
			switch rs[i+1] {
			case '\\':
				b.WriteRune('\\')
				i++
				continue
			case 'n':
				b.WriteRune('\n')
				i++
				continue
			case 'r':
				b.WriteRune('\r')
				i++
				continue
			case 'm':
				b.WriteRune('·')
				i++
				continue
			}
		}
		b.WriteRune(rs[i])
	}
	return b.String()
}

func ParseDepends(value string) []string {
	v := strings.TrimSpace(value)
	if v == "" || v == "—" || v == "-" || strings.ToLower(v) == "none" {
		return []string{}
	}
	parts := strings.Split(v, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// acceptancePairRe matches a single "<req>.<crit>=<test-name>" mapping token
// inside a task's `acceptance:` value. The criterion id is the stable
// ExtractCriteria/verify key space ("1.2"); the test name is a contiguous
// non-space token (a test function name or `pkg -run Name` selector with no
// spaces). Tokens may be separated by commas, semicolons, or whitespace.
var acceptancePairRe = regexp.MustCompile(`(\d+\.\d+)\s*=\s*([^\s,;]+)`)

// ParseAcceptanceMap reads criterion-id → test-name mappings from a task's
// `acceptance:` metadata value. It is intentionally lenient: free-form prose
// with no "id=test" tokens yields an empty (non-nil) map, so existing specs
// whose acceptance lines are descriptive remain valid and the acceptance gate
// stays a no-op for them. Later tokens win on duplicate ids (last write).
func ParseAcceptanceMap(value string) map[string]string {
	out := map[string]string{}
	for _, m := range acceptancePairRe.FindAllStringSubmatch(value, -1) {
		out[m[1]] = m[2]
	}
	return out
}

func ParseRequirements(value string) []int {
	parts := strings.Split(value, ",")
	var out []int
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err == nil {
			out = append(out, n)
		}
	}
	return out
}

func ParseTasks(text string) (ParsedTasks, error) {
	lines := splitLines(StripHTMLComments(text))
	var title string
	currentWave := 0
	var tasks []ParsedTask
	var current *ParsedTask
	seen := map[string]int{} // task id -> line where first defined

	flush := func() error {
		if current == nil {
			return nil
		}
		for _, k := range MandatoryKeys {
			if _, ok := current.Meta[k]; !ok {
				return GateError(fmt.Sprintf("tasks.md:%d: task %s missing key(s): %s", current.Line, current.ID, k))
			}
		}
		tasks = append(tasks, *current)
		current = nil
		return nil
	}

	for i, line := range lines {
		lineNo := i + 1

		if m := titleRE.FindStringSubmatch(line); m != nil {
			title = strings.TrimSpace(m[1])
			continue
		}
		if m := waveRE.FindStringSubmatch(line); m != nil {
			if err := flush(); err != nil {
				return ParsedTasks{}, err
			}
			n, _ := strconv.Atoi(m[1])
			currentWave = n
			continue
		}
		if m := taskRE.FindStringSubmatch(line); m != nil {
			if err := flush(); err != nil {
				return ParsedTasks{}, err
			}
			if currentWave == 0 {
				return ParsedTasks{}, GateError(fmt.Sprintf("tasks.md:%d: task %s appears before any '## Wave N' header", lineNo, m[2]))
			}
			if first, dup := seen[m[2]]; dup {
				return ParsedTasks{}, GateError(fmt.Sprintf("tasks.md:%d: duplicate task id %s (first defined at line %d)", lineNo, m[2], first))
			}
			seen[m[2]] = lineNo
			bare, ann := splitAnnotation(m[3])
			current = &ParsedTask{
				ID:         m[2],
				Title:      bare,
				Wave:       currentWave,
				Checked:    m[1] == "x",
				Meta:       make(map[string]string),
				Annotation: ann,
				Line:       lineNo,
			}
			continue
		}
		if m := metaRE.FindStringSubmatch(line); m != nil {
			if current == nil {
				return ParsedTasks{}, GateError(fmt.Sprintf("tasks.md:%d: metadata '%s' outside of a task", lineNo, m[1]))
			}
			key := m[1]
			valid := false
			for _, k := range KeyOrder {
				if k == key {
					valid = true
					break
				}
			}
			if !valid {
				return ParsedTasks{}, GateError(fmt.Sprintf("tasks.md:%d: unknown metadata key '%s'", lineNo, key))
			}
			current.Meta[key] = strings.TrimSpace(m[2])
			continue
		}
	}
	if err := flush(); err != nil {
		return ParsedTasks{}, err
	}
	if title == "" {
		return ParsedTasks{}, GateError("tasks.md:1: missing '# Tasks — <Title>' header")
	}
	return ParsedTasks{Title: title, Tasks: tasks}, nil
}

func serializeTask(t ParsedTask) string {
	checked := " "
	if t.Checked {
		checked = "x"
	}
	titleLine := fmt.Sprintf("- [%s] %s — %s", checked, t.ID, t.Title) + annotationSuffix(t.Annotation)
	var metaLines []string
	for _, k := range KeyOrder {
		if v, ok := t.Meta[k]; ok {
			metaLines = append(metaLines, fmt.Sprintf("  - %s: %s", k, v))
		}
	}
	parts := append([]string{titleLine}, metaLines...)
	return strings.Join(parts, "\n")
}

func SerializeTasks(doc ParsedTasks) string {
	out := []string{fmt.Sprintf("# Tasks — %s", doc.Title), ""}
	waveSet := make(map[int]bool)
	for _, t := range doc.Tasks {
		waveSet[t.Wave] = true
	}
	waves := make([]int, 0, len(waveSet))
	for w := range waveSet {
		waves = append(waves, w)
	}
	sort.Ints(waves)
	for _, w := range waves {
		out = append(out, fmt.Sprintf("## Wave %d", w))
		for _, t := range doc.Tasks {
			if t.Wave == w {
				out = append(out, serializeTask(t), "")
			}
		}
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n") + "\n"
}

func FindTask(doc ParsedTasks, id string) *ParsedTask {
	for i, t := range doc.Tasks {
		if t.ID == id {
			return &doc.Tasks[i]
		}
	}
	return nil
}

func RenderTaskLine(id, bareTitle string, checked bool, ann *Annotation) string {
	ch := " "
	if checked {
		ch = "x"
	}
	return fmt.Sprintf("- [%s] %s — %s", ch, id, bareTitle) + annotationSuffix(ann)
}

// annotationSuffix renders the trailing " ✓ complete · …" / " ⚠ blocked · …"
// fragment appended to a task line. Shared by serializeTask and RenderTaskLine
// so the on-disk annotation format has a single source of truth.
func annotationSuffix(ann *Annotation) string {
	if ann == nil {
		return ""
	}
	switch ann.Kind {
	case AnnotComplete:
		return fmt.Sprintf(" ✓ complete · evidence: %s · %s", encodeAnnotationField(ann.Evidence), encodeAnnotationField(ann.Ts))
	case AnnotBlocked:
		return fmt.Sprintf(" ⚠ blocked · reason: %s", encodeAnnotationField(ann.Reason))
	}
	return ""
}

func ApplyTaskAnnotation(text, id string, checked bool, ann *Annotation) (string, error) {
	lines := splitLines(text)
	scan := splitLines(StripHTMLComments(text))
	for i, sl := range scan {
		m := taskRE.FindStringSubmatch(sl)
		if m != nil && m[2] == id {
			bare, _ := splitAnnotation(m[3])
			lines[i] = RenderTaskLine(id, bare, checked, ann)
			return strings.Join(lines, "\n"), nil
		}
	}
	return "", GateError(fmt.Sprintf("tasks.md: task line for '%s' not found", id))
}
