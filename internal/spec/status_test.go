package spec

import "testing"

// TestSpecStatusIsValid locks the fail-loud allowlist: IsValid is the single
// source of truth for "a status specd ever writes", so a resume that reads a
// hand-edited or corrupt on-disk status can reject it instead of coercing it.
func TestSpecStatusIsValid(t *testing.T) {
	valid := []SpecStatus{
		StatusRequirements, StatusDesign, StatusTasks,
		StatusExecuting, StatusVerifying, StatusComplete, StatusBlocked,
	}
	for _, s := range valid {
		if !s.IsValid() {
			t.Errorf("IsValid(%q) = false, want true", s)
		}
	}

	invalid := []SpecStatus{"", "bogus", "Requirements", "done", "executing ", "complete\n"}
	for _, s := range invalid {
		if s.IsValid() {
			t.Errorf("IsValid(%q) = true, want false", s)
		}
	}
}
