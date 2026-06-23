package contextpkg

import (
	"strings"
	"testing"
)

func TestEstimateTokensBasics(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want int
	}{
		{"empty_is_zero", "", 0},
		{"single_char", "a", 1},               // ceil(1/4)=1
		{"four_bytes_one_token", "abcd", 1},   // ceil(4/4)=1
		{"five_bytes_two_tokens", "abcde", 2}, // ceil(5/4)=2
		{"backtick_surcharge", "`", 2},        // ceil(1/4)=1 + ceil(1/2)=1
		{"pipe_table_surcharge", "a|b|c", 3},  // ceil(5/4)=2 + ceil(2/2)=1
		{"code_fence_denser", "```go", 4},     // ceil(5/4)=2 + ceil(3/2)=2
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := EstimateTokensString(tt.in); got != tt.want {
				t.Errorf("EstimateTokensString(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestEstimateTokensDeterministic(t *testing.T) {
	t.Parallel()
	in := []byte("# Heading\n\nSome prose with `code` and a | table | row.\n")
	first := EstimateTokens(in)
	for i := 0; i < 5; i++ {
		if got := EstimateTokens(in); got != first {
			t.Fatalf("EstimateTokens not deterministic: got %d, want %d", got, first)
		}
	}
}

func TestEstimateTokensStringMatchesBytes(t *testing.T) {
	t.Parallel()
	s := "mixed `fences` and |pipes| plus prose"
	if EstimateTokensString(s) != EstimateTokens([]byte(s)) {
		t.Errorf("EstimateTokensString and EstimateTokens disagree for %q", s)
	}
}

func TestEstimateTokensMonotonic(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	prev := EstimateTokens([]byte(b.String()))
	for i := 0; i < 500; i++ {
		// Mix prose, fences, and table chars so the surcharge path is exercised.
		switch i % 5 {
		case 0:
			b.WriteString("word ")
		case 1:
			b.WriteString("`x` ")
		case 2:
			b.WriteString("| c |")
		default:
			b.WriteString("aa")
		}
		cur := EstimateTokens([]byte(b.String()))
		if cur < prev {
			t.Fatalf("estimate decreased at step %d: %d < %d", i, cur, prev)
		}
		prev = cur
	}
}
