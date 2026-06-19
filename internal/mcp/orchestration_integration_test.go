package mcp_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/cmd"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/mcp"
	th "github.com/0xkhdr/specd/internal/testharness"
)

type orchestrationMCPClient struct {
	t    *testing.T
	base string
	next int
}

func newOrchestrationMCPClient(t *testing.T, httpSSE bool) orchestrationMCPClient {
	t.Helper()
	client := orchestrationMCPClient{t: t}
	if httpSSE {
		addr := freePort(t)
		go func() { _ = mcp.ServeHTTP(addr, cmd.Dispatch) }()
		waitReady(t, addr)
		client.base = "http://" + addr
	}
	return client
}

func (c *orchestrationMCPClient) call(tool string, arguments map[string]any) map[string]any {
	c.t.Helper()
	var result map[string]any
	if c.base == "" {
		result = mcpToolResult(c.t, tool, arguments)
	} else {
		result = httpResult(c.t, c.base, c.nextPath(), toolCallRequest(c.t, tool, arguments))
	}
	if result["isError"] == true {
		c.t.Fatalf("%s returned MCP error: %v", tool, result)
	}
	return result
}

func (c *orchestrationMCPClient) callError(tool string, arguments map[string]any) map[string]any {
	c.t.Helper()
	var result map[string]any
	if c.base == "" {
		result = mcpToolResult(c.t, tool, arguments)
	} else {
		result = httpResult(c.t, c.base, c.nextPath(), toolCallRequest(c.t, tool, arguments))
	}
	if result["isError"] != true {
		c.t.Fatalf("%s succeeded, want tool-level error: %v", tool, result)
	}
	return result
}

func (c *orchestrationMCPClient) structured(tool string, arguments map[string]any) map[string]any {
	c.t.Helper()
	result := c.call(tool, arguments)
	structured, ok := result["structuredContent"].(map[string]any)
	if !ok {
		c.t.Fatalf("%s missing structuredContent: %v", tool, result)
	}
	return structured
}

func (c *orchestrationMCPClient) nextPath() string {
	c.next++
	if c.next%2 == 0 {
		return "/sse"
	}
	return "/rpc"
}

func toolCallRequest(t *testing.T, tool string, arguments map[string]any) string {
	t.Helper()
	rawArgs, err := json.Marshal(arguments)
	if err != nil {
		t.Fatalf("marshal tool args: %v", err)
	}
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":%q,"arguments":%s}}`, tool, rawArgs)
}

type mcpLifecycleSummary struct {
	SuccessSpecStatus  core.SpecStatus                 `json:"successSpecStatus"`
	SuccessTaskStatus  core.TaskStatus                 `json:"successTaskStatus"`
	SuccessSession     core.OrchestrationSessionStatus `json:"successSession"`
	SuccessEvents      []string                        `json:"successEvents"`
	CancelSpecStatus   core.SpecStatus                 `json:"cancelSpecStatus"`
	CancelTaskStatus   core.TaskStatus                 `json:"cancelTaskStatus"`
	CancelSession      core.OrchestrationSessionStatus `json:"cancelSession"`
	CancelEvents       []string                        `json:"cancelEvents"`
	CancelDirectiveCnt int                             `json:"cancelDirectiveCount"`
}

func TestMCPOrchestrationLifecycleStdioAndHTTPSSE(t *testing.T) {
	stdio := runMCPOrchestrationLifecycle(t, false)
	httpSSE := runMCPOrchestrationLifecycle(t, true)
	if !reflect.DeepEqual(httpSSE, stdio) {
		t.Fatalf("HTTP/SSE lifecycle summary != stdio\n http/sse: %#v\n stdio:    %#v", httpSSE, stdio)
	}
}

func runMCPOrchestrationLifecycle(t *testing.T, httpSSE bool) mcpLifecycleSummary {
	t.Helper()
	h := th.New(t)
	seedLifecycleSpec(h, "mcp-life", "test -f pass.flag")
	seedLifecycleSpec(h, "mcp-cancel", "true")
	client := newOrchestrationMCPClient(t, httpSSE)

	const lifeSession = "18181818181818181818181818181818"
	lifeStart := decodeMCPStructured[core.OrchestrationStepResult](t, client.structured("specd_brain", brainArgs("start", "mcp-life", lifeSession)))
	if lifeStart.Decision.Action != core.OrchestrationDispatch || lifeStart.Decision.Attempt != 1 || lifeStart.Event == nil {
		t.Fatalf("life start decision=%#v event=%#v, want dispatch attempt 1", lifeStart.Decision, lifeStart.Event)
	}
	client.structured("specd_brain", map[string]any{"args": []string{"status"}, "session": lifeSession})
	paused := decodeMCPStructured[core.OrchestrationSession](t, client.structured("specd_brain", map[string]any{"args": []string{"pause"}, "session": lifeSession}))
	if paused.Status != core.OrchestrationSessionPaused {
		t.Fatalf("pause status=%s, want paused", paused.Status)
	}
	pausedStep := decodeMCPStructured[core.OrchestrationStepResult](t, client.structured("specd_brain", brainArgs("step", "mcp-life", lifeSession)))
	if pausedStep.Decision.Action != core.OrchestrationWait || pausedStep.Event != nil {
		t.Fatalf("paused step=%#v event=%#v, want wait without event", pausedStep.Decision, pausedStep.Event)
	}
	resumed := decodeMCPStructured[core.OrchestrationSession](t, client.structured("specd_brain", map[string]any{"args": []string{"resume"}, "session": lifeSession}))
	if resumed.Status != core.OrchestrationSessionRunning {
		t.Fatalf("resume status=%s, want running", resumed.Status)
	}

	lifeMission := missionFromStep(t, h.Root, lifeStart)
	claim := decodeMCPStructured[core.PinkyClaim](t, client.structured("specd_pinky", map[string]any{"args": []string{"claim"}, "mission": writeMCPMission(t, h, lifeMission)}))
	if claim.Mission.WorkerID != lifeMission.WorkerID || claim.Mission.TaskID != lifeMission.TaskID {
		t.Fatalf("claim=%#v mission=%#v", claim, lifeMission)
	}
	client.structured("specd_pinky", leaseArgs("heartbeat", lifeMission))
	client.structured("specd_pinky", map[string]any{"args": []string{"progress"}, "session": lifeMission.SessionID, "worker": lifeMission.WorkerID, "spec": lifeMission.Spec, "task": lifeMission.TaskID, "attempt": lifeMission.Attempt, "percent": 50, "message": "halfway"})
	client.callError("specd_verify", map[string]any{"args": []string{"mcp-life", "T1"}})
	client.structured("specd_pinky", map[string]any{"args": []string{"block"}, "session": lifeMission.SessionID, "worker": lifeMission.WorkerID, "spec": lifeMission.Spec, "task": lifeMission.TaskID, "attempt": lifeMission.Attempt, "reason": "pass flag missing"})
	client.call("specd_pinky", leaseArgs("release", lifeMission))

	if err := os.WriteFile(h.Path("pass.flag"), []byte("ok\n"), 0o644); err != nil {
		t.Fatalf("write pass.flag: %v", err)
	}
	retryStep := decodeMCPStructured[core.OrchestrationStepResult](t, client.structured("specd_brain", brainArgs("step", "mcp-life", lifeSession)))
	if retryStep.Decision.Action != core.OrchestrationDispatch || retryStep.Decision.Attempt != 2 || retryStep.Event == nil {
		t.Fatalf("retry decision=%#v event=%#v, want dispatch attempt 2", retryStep.Decision, retryStep.Event)
	}
	retryMission := missionFromStep(t, h.Root, retryStep)
	client.structured("specd_pinky", map[string]any{"args": []string{"claim"}, "mission": writeMCPMission(t, h, retryMission)})
	client.call("specd_verify", map[string]any{"args": []string{"mcp-life", "T1"}})
	rec := verificationRecord(t, h.Root, "mcp-life", "T1")
	client.structured("specd_pinky", reportArgs(retryMission, rec, "retry passed"))
	if _, err := core.ReconcilePinkyEvidence(h.Root, terminalReport(retryMission, rec, "retry passed"), core.LoadConfig(h.Root).Orchestration); err != nil {
		t.Fatalf("reconcile MCP evidence: %v", err)
	}
	client.call("specd_pinky", leaseArgs("release", retryMission))
	completeStep := decodeMCPStructured[core.OrchestrationStepResult](t, client.structured("specd_brain", brainArgs("step", "mcp-life", lifeSession)))
	if completeStep.Decision.Action != core.OrchestrationCompleteSession {
		t.Fatalf("complete decision=%#v, want complete-session", completeStep.Decision)
	}

	const cancelSession = "19191919191919191919191919191919"
	cancelStart := decodeMCPStructured[core.OrchestrationStepResult](t, client.structured("specd_brain", brainArgs("start", "mcp-cancel", cancelSession)))
	if cancelStart.Decision.Action != core.OrchestrationDispatch || cancelStart.Event == nil {
		t.Fatalf("cancel start decision=%#v event=%#v, want dispatch", cancelStart.Decision, cancelStart.Event)
	}
	cancelMission := missionFromStep(t, h.Root, cancelStart)
	client.structured("specd_pinky", map[string]any{"args": []string{"claim"}, "mission": writeMCPMission(t, h, cancelMission)})
	cancelled := decodeMCPStructured[core.OrchestrationSession](t, client.structured("specd_brain", map[string]any{"args": []string{"cancel"}, "session": cancelSession}))
	if cancelled.Status != core.OrchestrationSessionCancelling {
		t.Fatalf("cancel status=%s, want cancelling", cancelled.Status)
	}
	directiveStep := decodeMCPStructured[core.OrchestrationStepResult](t, client.structured("specd_brain", brainArgs("step", "mcp-cancel", cancelSession)))
	if directiveStep.Decision.Action != core.OrchestrationCancel || directiveStep.Event == nil || directiveStep.Event.Type != core.ACPMessageDirective {
		t.Fatalf("cancel step=%#v event=%#v, want cancel directive", directiveStep.Decision, directiveStep.Event)
	}
	client.call("specd_pinky", leaseArgs("release", cancelMission))
	cancelComplete := decodeMCPStructured[core.OrchestrationStepResult](t, client.structured("specd_brain", brainArgs("step", "mcp-cancel", cancelSession)))
	if cancelComplete.Decision.Action != core.OrchestrationCompleteSession {
		t.Fatalf("cancel complete decision=%#v, want complete-session", cancelComplete.Decision)
	}

	return summarizeMCPLifecycle(t, h.Root, lifeSession, cancelSession)
}

func seedLifecycleSpec(h *th.Harness, slug, verify string) {
	h.T.Helper()
	h.Spec(slug).
		Req("demo", "As a user, I want demo.", "THE SYSTEM SHALL satisfy demo.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Title: "do demo", Files: "pass.flag", Verify: verify, Requirements: []int{1}}).
		Status(core.StatusExecuting).
		Build()
}

func brainArgs(subcommand, slug, sessionID string) map[string]any {
	return map[string]any{
		"args":            []string{subcommand, slug},
		"session":         sessionID,
		"approval-policy": "manual",
		"max-workers":     1,
		"max-retries":     2,
		"timeout-seconds": 3600,
	}
}

func leaseArgs(subcommand string, mission core.PinkyMission) map[string]any {
	return map[string]any{
		"args":    []string{subcommand},
		"session": mission.SessionID,
		"worker":  mission.WorkerID,
		"attempt": mission.Attempt,
	}
}

func reportArgs(mission core.PinkyMission, rec *core.VerificationRecord, summary string) map[string]any {
	return map[string]any{
		"args":             []string{"report"},
		"session":          mission.SessionID,
		"worker":           mission.WorkerID,
		"spec":             mission.Spec,
		"task":             mission.TaskID,
		"attempt":          mission.Attempt,
		"verification-ref": core.VerificationRef(rec),
		"summary":          summary,
		"changed-files":    strings.Join(rec.ChangedFiles, ","),
		"git-head":         verificationGitHead(rec),
		"duration-ms":      100,
		"host-tokens":      10,
		"host-cost":        "0.00",
	}
}

func terminalReport(mission core.PinkyMission, rec *core.VerificationRecord, summary string) core.PinkyTerminalReport {
	return core.PinkyTerminalReport{
		SessionID:       mission.SessionID,
		WorkerID:        mission.WorkerID,
		Spec:            mission.Spec,
		TaskID:          mission.TaskID,
		Attempt:         mission.Attempt,
		VerificationRef: core.VerificationRef(rec),
		Summary:         summary,
		ChangedFiles:    append([]string{}, rec.ChangedFiles...),
		GitHead:         verificationGitHead(rec),
		DurationMs:      100,
		HostTokens:      10,
		HostCost:        "0.00",
	}
}

func missionFromStep(t *testing.T, root string, step core.OrchestrationStepResult) core.PinkyMission {
	t.Helper()
	workerID := fmt.Sprintf("%s-a%d", strings.ToLower(step.Decision.TaskID), step.Decision.Attempt)
	mission, err := core.BuildPinkyMission(root, step.Decision.Spec, step.Snapshot.SessionID, workerID, step.Decision.TaskID, step.Decision.Attempt, core.LoadConfig(root).Orchestration)
	if err != nil {
		t.Fatalf("BuildPinkyMission: %v", err)
	}
	return mission
}

func writeMCPMission(t *testing.T, h *th.Harness, mission core.PinkyMission) string {
	t.Helper()
	raw, err := json.MarshalIndent(mission, "", "  ")
	if err != nil {
		t.Fatalf("marshal mission: %v", err)
	}
	dir := h.Path(".specd/tmp")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir mission tmp: %v", err)
	}
	path := filepath.Join(dir, "mission-"+mission.WorkerID+".json")
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		t.Fatalf("write mission: %v", err)
	}
	return path
}

func decodeMCPStructured[T any](t *testing.T, structured map[string]any) T {
	t.Helper()
	raw, err := json.Marshal(structured)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode structured content: %v\n%s", err, raw)
	}
	return out
}

func verificationRecord(t *testing.T, root, slug, taskID string) *core.VerificationRecord {
	t.Helper()
	loaded, err := core.LoadSpec(root, slug)
	if err != nil {
		t.Fatalf("LoadSpec(%s): %v", slug, err)
	}
	rec := loaded.State.Tasks[taskID].Verification
	if rec == nil || !rec.Verified {
		t.Fatalf("verification record for %s/%s = %#v, want passing", slug, taskID, rec)
	}
	return rec
}

func verificationGitHead(rec *core.VerificationRecord) string {
	if rec == nil || rec.GitHead == nil {
		return ""
	}
	return *rec.GitHead
}

func summarizeMCPLifecycle(t *testing.T, root, lifeSession, cancelSession string) mcpLifecycleSummary {
	t.Helper()
	lifeSpec, err := core.LoadSpec(root, "mcp-life")
	if err != nil {
		t.Fatalf("load life spec: %v", err)
	}
	cancelSpec, err := core.LoadSpec(root, "mcp-cancel")
	if err != nil {
		t.Fatalf("load cancel spec: %v", err)
	}
	lifeSessionState, err := core.LoadOrchestrationSession(root, lifeSession)
	if err != nil {
		t.Fatalf("load life session: %v", err)
	}
	cancelSessionState, err := core.LoadOrchestrationSession(root, cancelSession)
	if err != nil {
		t.Fatalf("load cancel session: %v", err)
	}
	cancelEvents := eventSummary(t, root, cancelSession)
	return mcpLifecycleSummary{
		SuccessSpecStatus:  lifeSpec.State.Status,
		SuccessTaskStatus:  lifeSpec.State.Tasks["T1"].Status,
		SuccessSession:     lifeSessionState.Status,
		SuccessEvents:      eventSummary(t, root, lifeSession),
		CancelSpecStatus:   cancelSpec.State.Status,
		CancelTaskStatus:   cancelSpec.State.Tasks["T1"].Status,
		CancelSession:      cancelSessionState.Status,
		CancelEvents:       cancelEvents,
		CancelDirectiveCnt: countPrefix(cancelEvents, "directive/T1/1"),
	}
}

func eventSummary(t *testing.T, root, sessionID string) []string {
	t.Helper()
	store, err := core.NewACPStore(root)
	if err != nil {
		t.Fatalf("NewACPStore: %v", err)
	}
	events, err := store.ReadEvents(sessionID, "summary")
	if err != nil {
		t.Fatalf("ReadEvents(%s): %v", sessionID, err)
	}
	out := make([]string, 0, len(events))
	for _, event := range events {
		out = append(out, fmt.Sprintf("%s/%s/%d", event.Type, event.Task, event.Attempt))
	}
	return out
}

func countPrefix(items []string, prefix string) int {
	count := 0
	for _, item := range items {
		if strings.HasPrefix(item, prefix) {
			count++
		}
	}
	return count
}
