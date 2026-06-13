package testharness

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

// SpecBuilder is a fluent constructor for gate-valid specs. It writes the
// artifacts (requirements.md / design.md / tasks.md) and a consistent state.json
// directly to disk, bypassing the CLI gates so tests can start from any desired
// state. Defaults are chosen so the minimal spec still passes `specd check`.
type SpecBuilder struct {
	h      *Harness
	slug   string
	title  string
	reqs   []reqBlock
	design []designSection
	tasks  []TaskSpec
	status core.SpecStatus
	phase  core.Phase
	gate   core.Gate
	turn   int
}

type reqBlock struct {
	name     string
	story    string
	criteria []string
}

type designSection struct {
	name string
	body string
}

// TaskSpec describes one task to seed. Only ID is required; the rest default to
// values that satisfy the task-schema, DAG, evidence and sync gates.
type TaskSpec struct {
	ID           string
	Title        string
	Role         string // default "builder"
	Wave         int    // default 1
	Depends      []string
	Requirements []int
	Verify       string // default "true" (a runnable, exit-0 command)
	Why          string
	Files        string
	Contract     string
	Acceptance   string
	Status       core.TaskStatus // default pending
}

// Spec begins building a spec with the given slug. Title defaults to a
// title-cased slug and status to "requirements".
func (h *Harness) Spec(slug string) *SpecBuilder {
	return &SpecBuilder{
		h:      h,
		slug:   slug,
		title:  titleCase(slug),
		status: core.StatusRequirements,
		gate:   core.GateNone,
	}
}

// Title overrides the spec title.
func (b *SpecBuilder) Title(t string) *SpecBuilder { b.title = t; return b }

// Req appends a requirement block. Each criterion must be a valid EARS sentence
// (e.g. "THE SYSTEM SHALL ..."); pass none to get a single default criterion.
func (b *SpecBuilder) Req(name, story string, criteria ...string) *SpecBuilder {
	if len(criteria) == 0 {
		criteria = []string{"THE SYSTEM SHALL satisfy " + name + "."}
	}
	b.reqs = append(b.reqs, reqBlock{name: name, story: story, criteria: criteria})
	return b
}

// FullDesign fills all seven mandatory design sections with non-empty,
// TODO-free bodies so the design gate passes.
func (b *SpecBuilder) FullDesign() *SpecBuilder {
	for _, s := range core.DesignSections {
		b.design = append(b.design, designSection{name: s, body: "Seeded " + s + " content for tests."})
	}
	return b
}

// DesignSection sets or appends a single design section body.
func (b *SpecBuilder) DesignSection(name, body string) *SpecBuilder {
	b.design = append(b.design, designSection{name: name, body: body})
	return b
}

// AddTask appends a task to the spec.
func (b *SpecBuilder) AddTask(t TaskSpec) *SpecBuilder { b.tasks = append(b.tasks, t); return b }

// Status sets the spec status and derives a matching phase (override with Phase).
func (b *SpecBuilder) Status(s core.SpecStatus) *SpecBuilder {
	b.status = s
	b.phase = core.PhaseForStatus(s)
	return b
}

// Phase overrides the spec phase.
func (b *SpecBuilder) Phase(p core.Phase) *SpecBuilder { b.phase = p; return b }

// Gate sets the spec gate (e.g. core.GateAwaitingApproval).
func (b *SpecBuilder) Gate(g core.Gate) *SpecBuilder { b.gate = g; return b }

// Turn sets the turn counter.
func (b *SpecBuilder) Turn(n int) *SpecBuilder { b.turn = n; return b }

// Build writes the spec to disk and returns its slug, failing the test on any
// error.
func (b *SpecBuilder) Build() string {
	t := b.h.T
	t.Helper()

	dir := core.SpecDir(b.h.Root, b.slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("SpecBuilder.Build: mkdir: %v", err)
	}

	if len(b.reqs) > 0 {
		b.write("requirements.md", b.renderRequirements())
	}
	if len(b.design) > 0 {
		b.write("design.md", b.renderDesign())
	}

	doc := b.parsedTasks()
	if len(doc.Tasks) > 0 {
		b.write("tasks.md", core.SerializeTasks(doc))
	}

	st := core.InitialState(b.slug, b.title)
	if len(doc.Tasks) > 0 {
		core.Reconcile(&st, doc)
		b.applyTaskStates(&st)
	}
	if b.phase == "" {
		b.phase = core.PhaseForStatus(b.status)
	}
	st.Status = b.status
	st.Phase = b.phase
	st.Gate = b.gate
	st.Turn = b.turn

	if err := core.SaveState(b.h.Root, b.slug, &st); err != nil {
		t.Fatalf("SpecBuilder.Build: SaveState: %v", err)
	}
	return b.slug
}

func (b *SpecBuilder) write(name, content string) {
	if err := core.AtomicWrite(core.ArtifactPath(b.h.Root, b.slug, name), content); err != nil {
		b.h.T.Fatalf("SpecBuilder.Build: write %s: %v", name, err)
	}
}

func (b *SpecBuilder) renderRequirements() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Requirements — %s\n\n", b.title)
	for i, r := range b.reqs {
		fmt.Fprintf(&sb, "## Requirement %d: %s\n", i+1, r.name)
		fmt.Fprintf(&sb, "**User story:** %s\n\n", r.story)
		sb.WriteString("**Acceptance criteria:**\n")
		for j, c := range r.criteria {
			fmt.Fprintf(&sb, "%d. %s\n", j+1, c)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func (b *SpecBuilder) renderDesign() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Design — %s\n\n", b.title)
	for _, s := range b.design {
		fmt.Fprintf(&sb, "## %s\n%s\n\n", s.name, s.body)
	}
	return sb.String()
}

func (b *SpecBuilder) parsedTasks() core.ParsedTasks {
	out := core.ParsedTasks{Title: b.title}
	for _, t := range b.tasks {
		ts := t.withDefaults()
		meta := map[string]string{
			"why":        ts.Why,
			"role":       ts.Role,
			"files":      ts.Files,
			"contract":   ts.Contract,
			"acceptance": ts.Acceptance,
			"verify":     ts.Verify,
			"depends":    dependsMeta(ts.Depends),
		}
		if len(ts.Requirements) > 0 {
			meta["requirements"] = intsMeta(ts.Requirements)
		}
		pt := core.ParsedTask{
			ID:      ts.ID,
			Title:   ts.Title,
			Wave:    ts.Wave,
			Checked: ts.Status == core.TaskComplete,
			Meta:    meta,
		}
		switch ts.Status {
		case core.TaskComplete:
			pt.Annotation = &core.Annotation{Kind: core.AnnotComplete, Evidence: "seeded", Ts: core.NowISO()}
		case core.TaskBlocked:
			pt.Annotation = &core.Annotation{Kind: core.AnnotBlocked, Reason: "seeded blocker"}
		}
		out.Tasks = append(out.Tasks, pt)
	}
	return out
}

// applyTaskStates overlays the requested per-task status onto the reconciled
// state, seeding evidence + a verified record for completed tasks and a blocker
// entry for blocked ones, so the evidence and sync gates stay green.
func (b *SpecBuilder) applyTaskStates(st *core.State) {
	for _, t := range b.tasks {
		ts := t.withDefaults()
		entry := st.Tasks[ts.ID]
		entry.Status = ts.Status
		stamp := core.NowISO()
		switch ts.Status {
		case core.TaskComplete:
			ev := "seeded complete evidence"
			entry.Evidence = &ev
			entry.StartedAt = &stamp
			entry.FinishedAt = &stamp
			if !isReadonlyRole(ts.Role) {
				entry.Verification = &core.VerificationRecord{
					Command: ts.Verify, ExitCode: 0, Verified: true, RanAt: stamp,
				}
			}
		case core.TaskRunning:
			entry.StartedAt = &stamp
		case core.TaskBlocked:
			reason := "seeded blocker"
			entry.Blocker = &reason
			st.Blockers = append(st.Blockers, core.Blocker{Task: ts.ID, Reason: reason, Since: "Turn 0"})
		}
		st.Tasks[ts.ID] = entry
	}
}

func (t TaskSpec) withDefaults() TaskSpec {
	if t.Title == "" {
		t.Title = t.ID
	}
	if t.Role == "" {
		t.Role = "builder"
	}
	if t.Wave == 0 {
		t.Wave = 1
	}
	if t.Verify == "" {
		t.Verify = "true"
	}
	if t.Why == "" {
		t.Why = "seeded rationale"
	}
	if t.Files == "" {
		t.Files = "seeded.go"
	}
	if t.Contract == "" {
		t.Contract = "seeded contract"
	}
	if t.Acceptance == "" {
		t.Acceptance = "seeded acceptance"
	}
	if t.Status == "" {
		t.Status = core.TaskPending
	}
	return t
}

func isReadonlyRole(role string) bool {
	for _, r := range core.ReadonlyRoles {
		if r == role {
			return true
		}
	}
	return false
}

func dependsMeta(deps []string) string {
	if len(deps) == 0 {
		return "none"
	}
	return strings.Join(deps, ", ")
}

func intsMeta(ns []int) string {
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ", ")
}

func titleCase(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
