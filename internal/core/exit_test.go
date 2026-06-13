package core

import (
	"errors"
	"fmt"
	"testing"
)

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
