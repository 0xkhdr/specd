package core

import (
	"os"
	"strings"
	"testing"
)

// Fail-loud state validation on resume (spec A10, Req 1).
//
// cross-spec-recovery resumes the program DAG by loading each child spec's
// state.json. If a child status has been hand-edited to an impossible value,
// resume must reject it with a clear, actionable error naming the spec and the
// offending value — never silently coerce it to a default. Valid lifecycle
// statuses must still load, so legitimate in-progress/transitional specs are
// not falsely rejected.
func TestLoadStateRejectsImpossibleChildStatus(t *testing.T) {
	t.Run("impossible-status-rejected", func(t *testing.T) {
		root := specRoot(t, "child")
		if err := os.WriteFile(statePath(root, "child"), []byte(`{"spec":"child","status":"frobnicate"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := LoadState(root, "child")
		se, ok := IsSpecdError(err)
		if !ok || se.Code != ExitGate {
			t.Fatalf("err = %v, want gate error on impossible status", err)
		}
		// The error must name both the offending spec and the invalid value so the
		// operator can fix it, and must not have coerced silently.
		msg := err.Error()
		if !strings.Contains(msg, "child") {
			t.Errorf("error should name the spec; got %q", msg)
		}
		if !strings.Contains(msg, "frobnicate") {
			t.Errorf("error should name the invalid value; got %q", msg)
		}
	})

	t.Run("empty-status-rejected", func(t *testing.T) {
		root := specRoot(t, "child")
		if err := os.WriteFile(statePath(root, "child"), []byte(`{"spec":"child","status":""}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadState(root, "child"); err == nil {
			t.Fatal("empty status should be rejected, not coerced to a default")
		}
	})

	// Every real lifecycle status must still load cleanly.
	valid := []SpecStatus{
		StatusRequirements, StatusDesign, StatusTasks,
		StatusExecuting, StatusVerifying, StatusComplete, StatusBlocked,
	}
	for _, status := range valid {
		status := status
		t.Run("valid-"+string(status), func(t *testing.T) {
			root := specRoot(t, "child")
			json := `{"spec":"child","status":"` + string(status) + `"}`
			if err := os.WriteFile(statePath(root, "child"), []byte(json), 0o644); err != nil {
				t.Fatal(err)
			}
			st, err := LoadState(root, "child")
			if err != nil {
				t.Fatalf("valid status %q rejected: %v", status, err)
			}
			if st.Status != status {
				t.Fatalf("status = %q, want %q", st.Status, status)
			}
		})
	}
}
