package core

import "testing"

func TestApplyVars(t *testing.T) {
	got := ApplyVars("hello {{name}}, wave {{n}} {{name}}", map[string]string{
		"name": "Pinky",
		"n":    "1",
	})
	if want := "hello Pinky, wave 1 Pinky"; got != want {
		t.Errorf("ApplyVars = %q, want %q", got, want)
	}
	// No vars and unknown placeholders pass through untouched.
	if got := ApplyVars("{{unset}} stays", nil); got != "{{unset}} stays" {
		t.Errorf("ApplyVars passthrough = %q", got)
	}
}
