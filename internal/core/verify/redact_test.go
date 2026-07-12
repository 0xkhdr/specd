package verify

import (
	"strings"
	"testing"
)

func TestRedactSecretValues(t *testing.T) {
	secret := "super-secret-fixture-value"
	r := NewRedactor([]string{secret})
	got := r.String("token=" + secret + " Authorization: Bearer abcdefghijklmnop")
	if strings.Contains(got, secret) || strings.Contains(got, "abcdefghijklmnop") {
		t.Fatalf("redacted output leaked secret: %q", got)
	}
	if !strings.Contains(got, Redacted) {
		t.Fatalf("redacted output = %q, want marker", got)
	}
}

func TestRedactAcrossWriteBoundaries(t *testing.T) {
	secret := "boundary-secret-value"
	var dst strings.Builder
	w := NewRedactingWriter(&dst, []string{secret})
	for _, part := range []string{"before boundary-", "secret-", "value after"} {
		if _, err := w.Write([]byte(part)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(dst.String(), secret) {
		t.Fatalf("stream leaked secret: %q", dst.String())
	}
}
