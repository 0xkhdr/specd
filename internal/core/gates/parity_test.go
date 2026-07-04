package gates

import "testing"

func TestByteIdenticalWhenOptInsOff(t *testing.T) {
	ctx := CheckCtx{}
	a := CoreRegistry().Run(ctx)
	b := CoreRegistry().Run(ctx)
	if len(a) != len(b) {
		t.Fatalf("finding count differs: %d != %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("finding %d differs: %+v != %+v", i, a[i], b[i])
		}
	}
}
