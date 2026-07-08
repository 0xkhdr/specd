package context

import "testing"

func TestEstimateNoLLM(t *testing.T) {
	a := EstimateNoLLM("abcdefghijkl")
	b := EstimateNoLLM("abcdefghijkl")
	if a != 3 || b != a {
		t.Fatalf("EstimateNoLLM = %d, %d; want stable 3", a, b)
	}
}
