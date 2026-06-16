package core

import "testing"

// acceptanceReqMd is a requirements.md whose Requirement 1 has two acceptance
// criteria, giving stable ids "1.1" and "1.2".
const acceptanceReqMd = `## Requirement 1 — Login

**User story:** As a user, I want to log in.

**Acceptance criteria:**
1. When credentials are valid, the system shall grant access.
2. When credentials are invalid, the system shall deny access.
`

// acceptanceTasksMd maps T1 to criterion 1.1 via its acceptance metadata.
const acceptanceTasksMd = `# Tasks — Login

## Wave 1
- [ ] T1 — implement login
  - why: users need access
  - role: builder
  - files: internal/auth/login.go
  - contract: login works
  - acceptance: 1.1=TestLoginValid
  - verify: go test ./internal/auth/
  - depends: —
  - requirements: 1
`

func mustParse(t *testing.T, md string) *ParsedTasks {
	t.Helper()
	doc, err := ParseTasks(md)
	if err != nil {
		t.Fatalf("ParseTasks: %v", err)
	}
	return &doc
}

func TestGateAcceptanceOffIsNoop(t *testing.T) {
	doc := mustParse(t, acceptanceTasksMd)
	st := &State{Tasks: map[string]TaskState{"T1": {ID: "T1", Status: TaskComplete}}}
	for _, mode := range []string{"", "off"} {
		c := CheckCtx{ReqMd: strp(acceptanceReqMd), Doc: doc, State: st, Cfg: Config{Gates: GatesCfg{Acceptance: mode}}}
		v, w := GateAcceptance(c)
		if len(v) != 0 || len(w) != 0 {
			t.Fatalf("acceptance=%q: want no findings, got v=%v w=%v", mode, v, w)
		}
	}
}

func TestGateAcceptanceCompleteWithoutPass(t *testing.T) {
	doc := mustParse(t, acceptanceTasksMd)
	st := &State{Tasks: map[string]TaskState{"T1": {ID: "T1", Status: TaskComplete}}}

	// error mode: missing recorded pass is a violation.
	c := CheckCtx{Slug: "demo", ReqMd: strp(acceptanceReqMd), Doc: doc, State: st, Cfg: Config{Gates: GatesCfg{Acceptance: "error"}}}
	v, _ := GateAcceptance(c)
	if len(v) != 1 || v[0].Gate != "acceptance" {
		t.Fatalf("want 1 acceptance violation, got %v", v)
	}

	// warn mode: same finding demoted to a warning.
	c.Cfg.Gates.Acceptance = "warn"
	v, w := GateAcceptance(c)
	if len(v) != 0 || len(w) != 1 {
		t.Fatalf("warn: want 0 violations / 1 warning, got v=%v w=%v", v, w)
	}

	// Recording the pass clears the finding.
	st.Acceptance = map[string]CriterionRecord{"1.1": {Status: "pass"}}
	c.Cfg.Gates.Acceptance = "error"
	v, _ = GateAcceptance(c)
	if len(v) != 0 {
		t.Fatalf("with recorded pass: want 0 violations, got %v", v)
	}
}

func TestGateAcceptanceUndefinedCriterionAlwaysError(t *testing.T) {
	// acceptance maps to 9.9 which is not in requirements.md.
	md := "# Tasks — X\n\n## Wave 1\n- [ ] T1 — x\n  - why: w\n  - role: builder\n  - files: a.go\n  - contract: c\n  - acceptance: 9.9=TestX\n  - verify: go test ./\n  - depends: —\n  - requirements: 1\n"
	doc := mustParse(t, md)
	st := &State{Tasks: map[string]TaskState{"T1": {ID: "T1", Status: TaskPending}}}
	// Even in warn mode, a broken reference is a hard violation.
	c := CheckCtx{Slug: "demo", ReqMd: strp(acceptanceReqMd), Doc: doc, State: st, Cfg: Config{Gates: GatesCfg{Acceptance: "warn"}}}
	v, _ := GateAcceptance(c)
	if len(v) != 1 {
		t.Fatalf("want 1 hard violation for undefined criterion, got %v", v)
	}
}

func scopeTasksMd(files string) string {
	return "# Tasks — X\n\n## Wave 1\n- [ ] T1 — x\n  - why: w\n  - role: builder\n  - files: " + files +
		"\n  - contract: c\n  - acceptance: —\n  - verify: go test ./\n  - depends: —\n  - requirements: 1\n"
}

func TestGateScopeOffIsNoop(t *testing.T) {
	doc := mustParse(t, scopeTasksMd("internal/core/login.go"))
	st := &State{Tasks: map[string]TaskState{"T1": {ID: "T1", Verification: &VerificationRecord{ChangedFiles: []string{"totally/unrelated.go"}}}}}
	for _, mode := range []string{"", "off", "*"} {
		c := CheckCtx{Doc: doc, State: st, Cfg: Config{Gates: GatesCfg{Scope: mode}}}
		v, w := GateScope(c)
		if len(v) != 0 || len(w) != 0 {
			t.Fatalf("scope=%q: want no findings, got v=%v w=%v", mode, v, w)
		}
	}
}

func TestGateScopeFlagsOutOfContract(t *testing.T) {
	doc := mustParse(t, scopeTasksMd("internal/core/*.go"))
	st := &State{Tasks: map[string]TaskState{"T1": {ID: "T1", Verification: &VerificationRecord{
		ChangedFiles: []string{"internal/core/login.go", "cmd/main.go"},
	}}}}
	c := CheckCtx{Doc: doc, State: st, Cfg: Config{Gates: GatesCfg{Scope: "error"}}}
	v, _ := GateScope(c)
	if len(v) != 1 || v[0].Gate != "scope" {
		t.Fatalf("want 1 scope violation for cmd/main.go, got %v", v)
	}

	// A "*" contract opts the task out individually.
	doc2 := mustParse(t, scopeTasksMd("*"))
	c.Doc = doc2
	v, _ = GateScope(c)
	if len(v) != 0 {
		t.Fatalf("star contract: want 0 violations, got %v", v)
	}
}

func TestMatchesAnyGlob(t *testing.T) {
	cases := []struct {
		p    string
		pats []string
		want bool
	}{
		{"internal/core/x.go", []string{"internal/core/*.go"}, true},
		{"internal/core/sub/x.go", []string{"internal/core/*.go"}, false},
		{"internal/core/sub/x.go", []string{"internal/core/**"}, true},
		{"internal/core/x.go", []string{"internal/core"}, true},
		{"docs/x.md", []string{"internal/core"}, false},
		{"a.go", []string{"a.go"}, true},
	}
	for _, tc := range cases {
		if got := matchesAnyGlob(tc.p, tc.pats); got != tc.want {
			t.Errorf("matchesAnyGlob(%q,%v)=%v want %v", tc.p, tc.pats, got, tc.want)
		}
	}
}
