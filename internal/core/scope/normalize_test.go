package scope

import "testing"

func TestNormalize(t *testing.T) {
	if got, err := Normalize("./a/b.go"); err != nil || got != "a/b.go" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	for _, p := range []string{"../x", "/x", "C:\\x", "a/../../x"} {
		if _, err := Normalize(p); err == nil {
			t.Fatalf("%q accepted", p)
		}
	}
}
