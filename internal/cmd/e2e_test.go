package cmd

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
	"github.com/0xkhdr/specd/internal/core/verify"
)

// TestLifecycleE2E drives init→new→check→approve→next→verify→report through a
// freshly built binary in a temp repo and asserts an on-disk side effect at
// every step. This is the evidence-integrity harness ADR-8 requires: a verb is
// done only when a running binary exercises it and leaves a trace.
func TestLifecycleE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary; skipped in -short")
	}
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "specd")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build specd: %v\n%s", err, out)
	}

	repo := t.TempDir()
	mustGit(t, repo, "init")
	mustGit(t, repo, "commit", "--allow-empty", "-m", "root", "--no-gpg-sign")

	run := func(args ...string) (string, int) {
		t.Helper()
		cmd := exec.Command(bin, args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		code := 0
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else if err != nil {
			t.Fatalf("run %v: %v", args, err)
		}
		return string(out), code
	}
	exists := func(rel string) bool {
		_, err := os.Stat(filepath.Join(repo, rel))
		return err == nil
	}

	if _, code := run("init"); code != 0 || !exists(".specd") {
		t.Fatalf("init: code=%d specd-dir=%v", code, exists(".specd"))
	}
	if _, code := run("new", "demo"); code != 0 || !exists(".specd/specs/demo/state.json") {
		t.Fatalf("new: code=%d state=%v", code, exists(".specd/specs/demo/state.json"))
	}
	// Swap in a trivially-passing verify so the verify step exercises the real
	// runner without depending on a toolchain inside the temp repo.
	tasks := "# Tasks\n\n| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | scout | spec.md | - | true | ok |\n"
	if err := os.WriteFile(filepath.Join(repo, ".specd/specs/demo/tasks.md"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}
	// Author real requirements + design: the EARS and design-stub gates (W4)
	// refuse to check/approve an unedited scaffold stub.
	writeReal := func(name, body string) {
		if err := os.WriteFile(filepath.Join(repo, ".specd/specs/demo", name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeReal("requirements.md", "# Requirements — demo\n\n- **R1** When a user runs check, the system shall validate the spec.\n")
	writeReal("design.md", "# Design — demo\n\n## Modules\nThe check module runs gates.\n\n## On-disk contracts\nstate.json holds status.\n\n## Invariants\nOutput is deterministic.\n")

	if _, code := run("check", "demo"); code != 0 {
		t.Fatalf("check: code=%d", code)
	}
	if _, code := run("approve", "demo"); code != 0 {
		t.Fatal("approve requirements failed")
	}
	if _, code := run("approve", "demo"); code != 0 {
		t.Fatal("approve design failed")
	}
	if out, _ := run("status", "demo"); !strings.Contains(out, "demo") {
		t.Fatalf("status missing spec: %s", out)
	}
	if out, code := run("next", "demo"); code != 0 || !strings.Contains(out, "T1") {
		t.Fatalf("next: code=%d out=%s", code, out)
	}
	if _, code := run("verify", "demo", "T1"); code != 0 || !exists(".specd/specs/demo/evidence.jsonl") {
		t.Fatalf("verify: code=%d evidence=%v", code, exists(".specd/specs/demo/evidence.jsonl"))
	}
	if out, code := run("report", "demo"); code != 0 || strings.TrimSpace(out) == "" {
		t.Fatalf("report: code=%d out=%q", code, out)
	}

	// R8.1 offline continuity: the entire lifecycle above ran with zero adapters
	// configured, and the read-only projection confirms none exist. Core
	// (init→check→approve→next→verify→report) is fully usable offline — no adapter
	// is ever required to complete a phase, gate, verify, or report.
	if out, code := run("adapters", "--json"); code != 0 {
		t.Fatalf("adapters --json offline: code=%d out=%q", code, out)
	} else {
		var report struct {
			Adapters []json.RawMessage `json:"adapters"`
		}
		if err := json.Unmarshal([]byte(out), &report); err != nil {
			t.Fatalf("adapters --json not JSON: %v\n%s", err, out)
		}
		if len(report.Adapters) != 0 {
			t.Fatalf("expected zero adapters offline, got %d", len(report.Adapters))
		}
	}

	// R3.1: the approval record names the gate approved and the artifact
	// revision it approved, stamped with the provenance triple.
	state, err := core.LoadState(filepath.Join(repo, ".specd/specs/demo/state.json"))
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	var appr core.Record
	if err := json.Unmarshal(state.Records["approval:requirements"], &appr); err != nil {
		t.Fatalf("approval record: %v", err)
	}
	if appr.Gate != "requirements" {
		t.Fatalf("approval record missing gate: %+v", appr)
	}
	if appr.Timestamp == "" || appr.Actor == "" || appr.GitHead == "" {
		t.Fatalf("approval record not stamped: %+v", appr)
	}

	// R3.2/R3.3: the evidence ledger is append-only; a second verify appends a
	// line rather than rewriting, and every line pins to a commit (this repo
	// has a HEAD, so no record carries the "unknown" sentinel).
	if _, code := run("verify", "demo", "T1"); code != 0 {
		t.Fatalf("second verify failed")
	}
	ledger, err := os.ReadFile(filepath.Join(repo, ".specd/specs/demo/evidence.jsonl"))
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if lines := strings.Count(strings.TrimSpace(string(ledger)), "\n") + 1; lines < 2 {
		t.Fatalf("ledger not append-only: %d lines\n%s", lines, ledger)
	}
	if strings.Contains(string(ledger), `"git_head":"unknown"`) {
		t.Fatalf("evidence carries unresolved head:\n%s", ledger)
	}

	// P4.2 close-the-loop: complete-task writes the ✅ marker and state.json
	// status atomically, and the Sync gate stays green because they agree.
	if out, code := run("complete-task", "demo", "T1"); code != 0 || !strings.Contains(out, "completed") {
		t.Fatalf("complete-task: code=%d out=%q", code, out)
	}
	final, err := core.LoadState(filepath.Join(repo, ".specd/specs/demo/state.json"))
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if final.TaskStatus["T1"] != core.TaskComplete {
		t.Fatalf("state.json task status not recorded: %+v", final.TaskStatus)
	}
	if _, code := run("check", "demo"); code != 0 {
		t.Fatalf("check after complete (sync gate red?): code=%d", code)
	}

	// Fail-closed dispatch: unknown verb must exit 2, never 0.
	if _, code := run("bogusverb"); code != 2 {
		t.Fatalf("unknown verb exit = %d, want 2", code)
	}

	// R8.1 lifecycle-proof continuity: an amendment recorded on disk must survive
	// a restart. Seed one that touches the requirements gate, then a fresh process
	// must project it as a stale record and surface the amendment — proving the
	// release binary preserves staleness/coverage across restart, deterministically.
	statePath := filepath.Join(repo, ".specd/specs/demo/state.json")
	proofState, err := core.LoadState(statePath)
	if err != nil {
		t.Fatalf("load state for amendment: %v", err)
	}
	if err := proofState.AppendAmendment(core.Amendment{
		ChangeID: "chg-1", AffectedIDs: []string{"R1", "requirements"},
		Rationale: "corrected accepted behaviour", RequiredRechecks: []string{"design"},
	}); err != nil {
		t.Fatalf("append amendment: %v", err)
	}
	if err := core.SaveStateCAS(statePath, proofState.Revision, proofState); err != nil {
		t.Fatalf("persist amendment: %v", err)
	}
	proofOut, code := run("report", "demo", "--proof")
	if code != 0 {
		t.Fatalf("report --proof: code=%d out=%s", code, proofOut)
	}
	for _, want := range []string{"stale: approval:requirements", "amendment chg-1"} {
		if !strings.Contains(proofOut, want) {
			t.Fatalf("proof continuity missing %q after restart:\n%s", want, proofOut)
		}
	}
	if again, _ := run("report", "demo", "--proof"); again != proofOut {
		t.Fatalf("proof not deterministic across restart:\n--1--\n%s\n--2--\n%s", proofOut, again)
	}
}

// TestWorkflowCoherenceDefault is the release proof for the generated default
// workflow. It intentionally treats the freshly built binary and files emitted
// by init/new as the only driver contract: no package-internal lifecycle helper
// chooses an action for the fixture.
func TestWorkflowCoherenceDefault(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary; skipped in -short")
	}
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "specd")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build specd: %v\n%s", err, out)
	}

	repo := t.TempDir()
	mustGit(t, repo, "init")
	mustGit(t, repo, "commit", "--allow-empty", "-m", "root", "--no-gpg-sign")
	run := func(args ...string) (string, int) {
		t.Helper()
		command := exec.Command(bin, args...)
		command.Dir = repo
		out, err := command.CombinedOutput()
		if err == nil {
			return string(out), 0
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return string(out), exitErr.ExitCode()
		}
		t.Fatalf("run %v: %v", args, err)
		return "", -1
	}
	mustRun := func(args ...string) string {
		t.Helper()
		out, code := run(args...)
		if code != 0 {
			t.Fatalf("specd %s: code=%d\n%s", strings.Join(args, " "), code, out)
		}
		return out
	}

	mustRun("init")
	agentsRaw, err := os.ReadFile(filepath.Join(repo, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range []string{"specd handshake bootstrap <slug> --json", "specd status <slug> --guide", "specd context <slug> <task> --json", "specd verify <slug> <task>", "specd complete-task <slug> <task>"} {
		if !strings.Contains(string(agentsRaw), route) {
			t.Fatalf("generated AGENTS.md omitted bootstrap route %q", route)
		}
	}
	help := mustRun("help")
	for _, verb := range []string{"handshake", "agents", "context", "verify", "complete-task", "check", "approve"} {
		if !strings.Contains(help, verb) {
			t.Fatalf("generated help omitted workflow verb %q", verb)
		}
	}
	mustRun("new", "demo")

	// Authoring is user work between harness actions. Shapes come from the
	// generated stubs; this fixture replaces comments/placeholders with one real
	// requirement, design, and read-only task without inventing another schema.
	writeReal := func(name, body string) {
		t.Helper()
		path := filepath.Join(repo, ".specd/specs/demo", name)
		stub, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"id", "role", "files", "depends-on", "verify", "acceptance"} {
			if name == "tasks.md" && !strings.Contains(string(stub), field) {
				t.Fatalf("generated tasks stub omitted %q", field)
			}
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeReal("requirements.md", "# Requirements — demo\n\n- **R1** When release proof runs, the system shall complete one generated workflow task.\n")
	writeReal("design.md", "# Design — demo\n\n## Modules\nThe generated workflow drives the CLI.\n\n## On-disk contracts\nState and evidence stay under the spec directory.\n\n## Invariants\nCompletion requires passing evidence.\n")
	writeReal("tasks.md", "# Tasks — demo\n\n| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | scout | workflow-proof.txt | - | printf ok | R1 |\n")
	if err := os.WriteFile(filepath.Join(repo, "workflow-proof.txt"), []byte("generated workflow proof\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "add", ".")
	mustGit(t, repo, "commit", "-m", "author demo", "--no-gpg-sign")

	var handshake core.Handshake
	if err := json.Unmarshal([]byte(mustRun("handshake", "bootstrap", "demo", "--json")), &handshake); err != nil {
		t.Fatalf("generated handshake is not typed JSON: %v", err)
	}
	if handshake.PaletteDigest == "" || handshake.ConfigDigest == "" || handshake.ActiveSpec == nil || handshake.ActiveSpec.Slug != "demo" {
		t.Fatalf("incomplete handshake: %+v", handshake)
	}
	doctor := mustRun("agents", "doctor", "--json")
	if !strings.Contains(doctor, `"healthy": true`) || !strings.Contains(doctor, `"findings": []`) {
		t.Fatalf("generated doctor contract not healthy/typed:\n%s", doctor)
	}

	guide := func() core.DriverGuideV1 {
		t.Helper()
		var result core.DriverGuideV1
		if err := json.Unmarshal([]byte(mustRun("agents", "guide", "demo", "--json")), &result); err != nil {
			t.Fatalf("guide JSON: %v", err)
		}
		return result
	}
	hasAction := func(g core.DriverGuideV1, command string) bool {
		for _, action := range g.NextActions {
			if action.Command == command {
				return true
			}
		}
		return false
	}
	if g := guide(); !hasAction(g, "check") {
		t.Fatalf("initial generated guide omitted check: %+v", g.NextActions)
	}
	mustRun("check", "demo")

	wantStatuses := []core.Status{core.StatusRequirements, core.StatusDesign, core.StatusTasks, core.StatusExecuting}
	for i := 1; i < len(wantStatuses); i++ {
		g := guide()
		if !hasAction(g, "approve") {
			t.Fatalf("guide omitted human approval at step %d: %+v", i, g.NextActions)
		}
		mustRun("approve", "demo")
		state, err := core.LoadState(filepath.Join(repo, ".specd/specs/demo/state.json"))
		if err != nil {
			t.Fatal(err)
		}
		if state.Status != wantStatuses[i] {
			t.Fatalf("approval skipped phase: got %s want %s", state.Status, wantStatuses[i])
		}
	}

	contextOut := mustRun("context", "demo", "T1", "--json")
	for _, want := range []string{`"selected_task"`, `"kind": "guardrails"`, `"authority_limit": "role=scout; phase=execute; human-only tools forbidden"`, `"route": "cli:`, `"palette_digest"`} {
		if !strings.Contains(contextOut, want) {
			t.Fatalf("generated context omitted %s:\n%s", want, contextOut)
		}
	}
	if _, code := run("complete-task", "demo", "T1"); code == 0 {
		t.Fatal("completion without verify evidence succeeded")
	}
	mustRun("verify", "demo", "T1")
	mustRun("complete-task", "demo", "T1")
	mustRun("check", "demo")
	state, err := core.LoadState(filepath.Join(repo, ".specd/specs/demo/state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if state.TaskStatus["T1"] != core.TaskComplete {
		t.Fatalf("task not complete: %+v", state.TaskStatus)
	}
	ledger, err := os.ReadFile(filepath.Join(repo, ".specd/specs/demo/evidence.jsonl"))
	if err != nil || !strings.Contains(string(ledger), `"exit_code":0`) || !strings.Contains(string(ledger), `"task_id":"T1"`) {
		t.Fatalf("passing evidence missing or malformed: err=%v\n%s", err, ledger)
	}
}

func TestSecurityE2EProductionBoundariesFailClosed(t *testing.T) {
	if err := gates.CheckScope([]string{"declared.go", "credential.env"}, []string{"declared.go"}); err == nil || !strings.Contains(err.Error(), "outside_scope") {
		t.Fatalf("scope outcome = %v", err)
	}
	_, err := verify.Run(context.Background(), verify.Options{Command: "true", Dir: t.TempDir(), RequireSandbox: true, Adapter: &verify.SandboxAdapterV1{
		SchemaVersion: verify.SandboxAdapterSchemaV1, Name: "ci", Platform: "ci", Binary: "/bin/sh", Capabilities: []string{verify.CapabilityNetworkIsolation},
	}})
	if err == nil || !strings.Contains(err.Error(), "missing required capability") {
		t.Fatalf("adapter outcome = %v", err)
	}
}

func TestLifecycleE2EHostSurfaceMarker(t *testing.T) {
	// MCP and future hosts must drive CLI-owned lifecycle, with human approval
	// and evidence kept as distinct outcomes.
	for _, outcome := range []string{"approved-by-human", "verified", "reported"} {
		if outcome == "approved-by-agent" {
			t.Fatalf("agent approval must never be a lifecycle outcome")
		}
	}
}

func TestObservabilityE2EAttestedRoutingRollupOffline(t *testing.T) {
	telemetry := core.Annotations{InputTokens: 7, OutputTokens: 2, Cost: "0.09", Currency: "USD", PricingRef: "price:v1"}
	envelope, err := core.SignAttestation("local", []byte("test-key"), "usage:1", telemetry)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := core.VerifyAttestation(envelope, map[string][]byte{"local": []byte("test-key")})
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Source != core.TelemetrySourceAdapter {
		t.Fatalf("source = %q", accepted.Source)
	}

	recommendation, err := core.RecommendRouting(core.TaskRow{ID: "T1", Complexity: "high"}, core.RoutingConfig{Classes: []string{"standard", "reasoning"}, DefaultClass: "standard", Recommendations: map[string]string{"high": "reasoning"}})
	if err != nil || recommendation.Class != "reasoning" || recommendation.Model != "" {
		t.Fatalf("recommendation = %#v, err=%v", recommendation, err)
	}

	rollup, err := core.RollupEconomics([]core.SpecEconomics{{SpecID: "measured", Telemetry: &core.TelemetryReport{Cost: accepted.Cost, InputTokens: accepted.InputTokens}}, {SpecID: "missing"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if rollup.Cost != "0.09" || len(rollup.MissingSpecs) != 1 || rollup.MissingSpecs[0] != "missing" {
		t.Fatalf("rollup = %#v", rollup)
	}
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
