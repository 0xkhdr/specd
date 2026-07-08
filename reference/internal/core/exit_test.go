package core

import (
	"errors"
	"fmt"
	"testing"
)

// TestExitCodeTaxonomyGolden locks the documented exit-code values (R4.1, R4.3).
// These integers are the public contract a script branches on; renumbering one
// is a breaking regression, so they are pinned as a golden table and asserted
// distinct. Adding a NEW code is allowed (append a row); changing an existing
// value must fail this test.
func TestExitCodeTaxonomyGolden(t *testing.T) {
	golden := []struct {
		name string
		got  int
		want int
	}{
		{"ExitOK", ExitOK, 0},
		{"ExitGate", ExitGate, 1},
		{"ExitUsage", ExitUsage, 2},
		{"ExitNotFound", ExitNotFound, 3},
	}
	seen := map[int]string{}
	for _, c := range golden {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d (exit-code contract changed)", c.name, c.got, c.want)
		}
		if prev, dup := seen[c.got]; dup {
			t.Errorf("exit code %d shared by %s and %s; codes must be distinct", c.got, prev, c.name)
		}
		seen[c.got] = c.name
	}
}

func TestErrorConstructorsCarryCode(t *testing.T) {
	tests := []struct {
		name string
		err  *SpecdError
		want int
	}{
		{"gate_error_is_exit_1", GateError("x"), ExitGate},
		{"usage_error_is_exit_2", UsageError("x"), ExitUsage},
		{"not_found_error_is_exit_3", NotFoundError("x"), ExitNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.want {
				t.Errorf("code = %d, want %d", tt.err.Code, tt.want)
			}
			if tt.err.Error() != "x" {
				t.Errorf("Error() = %q, want %q", tt.err.Error(), "x")
			}
		})
	}
}

func TestIsSpecdError(t *testing.T) {
	t.Run("unwraps_through_fmt_errorf", func(t *testing.T) {
		// Arrange: a SpecdError wrapped with %w deep in the stack.
		base := NotFoundError("missing")
		wrapped := fmt.Errorf("load failed: %w", base)

		// Act
		se, ok := IsSpecdError(wrapped)

		// Assert
		if !ok {
			t.Fatal("IsSpecdError = false, want true through wrap")
		}
		if se.Code != ExitNotFound {
			t.Errorf("code = %d, want %d", se.Code, ExitNotFound)
		}
	})

	t.Run("returns_false_for_plain_error", func(t *testing.T) {
		_, ok := IsSpecdError(errors.New("plain"))
		if ok {
			t.Error("IsSpecdError = true for plain error, want false")
		}
	})
}
