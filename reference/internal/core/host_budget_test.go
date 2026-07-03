package core

import (
	"os"
	"testing"
)

func TestHostContextBudgetFromEnv(t *testing.T) {
	cases := []struct {
		name string
		set  bool
		val  string
		want int
	}{
		{"unset", false, "", 0},
		{"empty", true, "", 0},
		{"non-numeric", true, "abc", 0},
		{"negative", true, "-5", 0},
		{"zero", true, "0", 0},
		{"positive", true, "12000", 12000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.set {
				t.Setenv("SPECD_MAX_CONTEXT_TOKENS", tc.val)
			} else {
				// t.Setenv first so the original value is restored on cleanup,
				// then unset to exercise the absent-variable branch.
				t.Setenv("SPECD_MAX_CONTEXT_TOKENS", "")
				os.Unsetenv("SPECD_MAX_CONTEXT_TOKENS")
			}
			if got := HostContextBudgetFromEnv(); got != tc.want {
				t.Errorf("HostContextBudgetFromEnv() = %d, want %d", got, tc.want)
			}
		})
	}
}
