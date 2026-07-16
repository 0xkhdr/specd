package core

import (
	"reflect"
	"testing"
)

func TestDriverCanonicalDigest(t *testing.T) {
	a := BootstrapV1{ProtocolVersion: DriverProtocolVersion, Root: "/repo", Specs: []string{"z", "a"}, PaletteDigest: "p", ConfigDigest: "c", GuidanceDigest: "g", ContextSchemaDigest: "x", Findings: []DriverFinding{{Code: "Z", Severity: "error", RecoveryAction: "fix z"}, {Code: "A", Severity: "warning", RecoveryAction: "fix a"}}}
	b := a
	b.Specs = []string{"a", "z"}
	b.Findings = []DriverFinding{a.Findings[1], a.Findings[0]}
	if DriverDigest(a) != DriverDigest(b) {
		t.Fatal("driver digest depends on input ordering")
	}
}

func TestDriverValidateFailsClosed(t *testing.T) {
	valid := BootstrapV1{ProtocolVersion: DriverProtocolVersion, Root: "/repo", PaletteDigest: "p", ConfigDigest: "c", GuidanceDigest: "g", ContextSchemaDigest: "x"}
	if err := ValidateBootstrapV1(valid); err != nil {
		t.Fatal(err)
	}
	valid.ProtocolVersion = "2"
	if err := ValidateBootstrapV1(valid); err == nil {
		t.Fatal("unknown major version accepted")
	}
	valid.ProtocolVersion = DriverProtocolVersion
	valid.PaletteDigest = ""
	if err := ValidateBootstrapV1(valid); err == nil {
		t.Fatal("missing required digest accepted")
	}
}

func TestDispatchDigestPinsSemanticFields(t *testing.T) {
	d := DispatchV1{ProtocolVersion: DriverProtocolVersion, Root: "/repo", SpecSlug: "demo", TaskID: "T1", Role: "craftsman", DeclaredFiles: []string{"b.go", "a.go"}, Acceptance: []string{"R2", "R1"}, Verify: "go test ./...", ContextRef: "ctx", ContextDigest: "ctx-d", ConfigDigest: "cfg", PaletteDigest: "pal", AuthorityRef: "auth", SubjectHead: "head"}
	d.EnvelopeDigest = DispatchDigest(d)
	if err := ValidateDispatchV1(d); err != nil {
		t.Fatal(err)
	}
	d.Verify = "printf ok"
	if err := ValidateDispatchV1(d); err == nil {
		t.Fatal("changed verify accepted")
	}
	d.Verify = "go test ./..."
	d.EnvelopeDigest = DispatchDigest(d)
	d.DeclaredFiles = []string{"a.go", "b.go"}
	d.Acceptance = []string{"R1", "R2"}
	if err := ValidateDispatchV1(d); err != nil {
		t.Fatal(err)
	}
}

func TestDriverGuideCanonicalActions(t *testing.T) {
	g := DriverGuideV1{ProtocolVersion: DriverProtocolVersion, Root: "/repo", SpecSlug: "demo", Phase: PhaseExecute, Status: StatusTasks, NextActions: []NextAction{{ID: "z", Command: "verify", Actor: "agent", SideEffect: "write", SourceRef: "palette"}, {ID: "a", Command: "status", Actor: "agent", SideEffect: "read", SourceRef: "palette"}}}
	CanonicalizeDriverGuide(&g)
	if g.NextActions[0].ID != "z" {
		t.Fatalf("workflow action order changed: %+v", g.NextActions)
	}
}

func TestDriverProjectsPaletteAndHumanAuthority(t *testing.T) {
	g := ProjectDriverGuide("/repo", "demo", StatusTasks, []string{"requirements", "design"}, []string{"T2"}, []DriverFinding{{Code: "BLOCKED", Severity: "error", RecoveryAction: "fix task"}})
	if g.ProtocolVersion != DriverProtocolVersion || len(g.NextActions) == 0 {
		t.Fatalf("guide = %+v", g)
	}
	for _, action := range g.NextActions {
		cmd, ok := CommandByName(action.Command)
		if !ok || !cmd.AllowsPhase(g.Phase) {
			t.Fatalf("invalid projected action: %+v", action)
		}
		if cmd.HumanOnly && (action.Actor != "human" || !action.AuthorityRequired) {
			t.Fatalf("human action agent-authorized: %+v", action)
		}
	}
}

func TestDriverApproveUsesSimpleHumanHandoff(t *testing.T) {
	g := ProjectDriverGuide("/repo", "demo", StatusRequirements, nil, nil, nil)
	for _, action := range g.NextActions {
		if action.Command != "approve" {
			continue
		}
		if action.Actor != "human" || !action.AuthorityRequired || action.SideEffect != string(EffectStateWrite) {
			t.Fatalf("approve action authority = %+v", action)
		}
		if want := []string{"demo"}; !reflect.DeepEqual(action.Args, want) {
			t.Fatalf("approve args = %v, want %v", action.Args, want)
		}
		return
	}
	t.Fatal("driver omitted human approval handoff")
}

func TestDriverCompleteLoopUsesNarrowOperation(t *testing.T) {
	g := ProjectDriverGuide("/repo", "demo", StatusTasks, []string{"requirements", "design"}, []string{"T1"}, nil)
	want := []string{"status", "context", "verify", "complete-task", "check", "approve"}
	got := make([]string, 0, len(g.NextActions))
	for _, action := range g.NextActions {
		got = append(got, action.Command)
		if action.Command == "complete-task" {
			if action.Actor != "agent" || action.SideEffect != string(EffectStateWrite) || !action.AuthorityRequired {
				t.Fatalf("complete-task action = %+v", action)
			}
			op, ok := OperationByID("complete-task")
			if !ok || op.Command != action.Command || !op.TaskRequired || op.ScopeSource != "task" {
				t.Fatalf("complete-task operation = %+v, found %v", op, ok)
			}
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("completion loop = %v, want %v", got, want)
	}
}

func TestDriverActionsProjectCanonicalOperations(t *testing.T) {
	g := ProjectDriverGuide("/repo", "demo", StatusTasks, nil, []string{"T1"}, nil)
	for _, action := range g.NextActions {
		op, ok := ResolveOperation(action.Command, action.Args, nil)
		if !ok {
			t.Fatalf("action has no canonical operation: %+v", action)
		}
		if action.ID != op.ID || action.Command != op.Command || action.Actor != string(op.Actor) ||
			action.SideEffect != string(op.Effect) || action.AuthorityRequired != op.AuthorityRequired ||
			!reflect.DeepEqual(action.AllowedPhases, op.AllowedPhases) || action.SourceRef != "core.Operations/"+op.ID {
			t.Errorf("action drifted from operation\naction=%+v\noperation=%+v", action, op)
		}
	}
}
