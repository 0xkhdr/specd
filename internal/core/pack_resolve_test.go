package core

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func sha256Hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

const sampleRemotePack = `{"name":"remote-demo","version":"0.1.0","description":"x","files":[{"path":".specd/steering/extra.md","content":"hi"}]}`

func TestPackResolve(t *testing.T) {
	// Built-in resolves by bare name.
	if _, err := ResolvePack("minimal", ""); err != nil {
		t.Fatalf("ResolvePack(minimal): %v", err)
	}
	if _, err := ResolvePack("go-service", ""); err != nil {
		t.Fatalf("ResolvePack(go-service): %v", err)
	}
	// An unknown built-in fails.
	if _, err := ResolvePack("does-not-exist", ""); err == nil {
		t.Error("unknown built-in pack should fail")
	}
	// A pin on a bare name is rejected (pin only means remote).
	if _, err := ResolvePack("minimal", "deadbeef"); err == nil {
		t.Error("pin on a built-in name should be rejected")
	}

	// Remote resolves over loopback with the correct pin.
	body := []byte(sampleRemotePack)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	p, err := ResolvePack(srv.URL, sha256Hex(body))
	if err != nil {
		t.Fatalf("ResolvePack(remote, correct pin): %v", err)
	}
	if p.Name != "remote-demo" {
		t.Errorf("resolved pack name = %q", p.Name)
	}
}

// TestPackFailClosed asserts the resolver refuses on a missing or mismatched
// pin and that VerifyAndParsePack never returns a pack on a digest mismatch.
func TestPackFailClosed(t *testing.T) {
	body := []byte(sampleRemotePack)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	// Missing pin on a remote ref.
	if _, err := ResolvePack(srv.URL, ""); err == nil {
		t.Error("remote pack with no pin must fail closed")
	} else if !strings.Contains(err.Error(), "pinned") {
		t.Errorf("error = %q, want pin-required message", err)
	}

	// Wrong pin.
	if _, err := ResolvePack(srv.URL, sha256Hex([]byte("something else"))); err == nil {
		t.Error("remote pack with wrong pin must fail closed")
	}

	// Direct: a mismatch returns no pack.
	if p, err := VerifyAndParsePack(body, "00", "test"); err == nil || p != nil {
		t.Errorf("VerifyAndParsePack mismatch returned pack=%v err=%v", p, err)
	}
}
