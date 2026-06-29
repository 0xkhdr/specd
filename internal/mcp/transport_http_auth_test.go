package mcp

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// recordingHandler reports whether it was reached, so a test can prove a 401
// short-circuits before dispatch (R3.2).
func recordingHandler(reached *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*reached = true
		w.WriteHeader(http.StatusOK)
	})
}

// An unset token leaves the handler untouched: the loopback-default path stays
// byte-for-byte unchanged (R3.4).
func TestTokenAuthUnsetIsPassThrough(t *testing.T) {
	var reached bool
	h := tokenAuth("", recordingHandler(&reached))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/rpc", nil))
	if !reached || rec.Code != http.StatusOK {
		t.Fatalf("pass-through failed: reached=%v code=%d", reached, rec.Code)
	}
}

// With a token set, only the exact bearer header is admitted; everything else is
// a 401 that never reaches dispatch (R3.1–R3.3).
func TestTokenAuthEnforcesBearer(t *testing.T) {
	const token = "s3cret-token"
	cases := []struct {
		name       string
		authHeader string
		wantCode   int
		wantReach  bool
	}{
		{"missing", "", http.StatusUnauthorized, false},
		{"wrong-token", "Bearer nope", http.StatusUnauthorized, false},
		{"no-scheme", token, http.StatusUnauthorized, false},
		{"correct", "Bearer " + token, http.StatusOK, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var reached bool
			h := tokenAuth(token, recordingHandler(&reached))
			req := httptest.NewRequest(http.MethodPost, "/rpc", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.wantCode {
				t.Errorf("code = %d, want %d", rec.Code, tc.wantCode)
			}
			if reached != tc.wantReach {
				t.Errorf("dispatch reached = %v, want %v", reached, tc.wantReach)
			}
		})
	}
}

func TestIsLoopbackBind(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:8765", true},
		{"localhost:8765", true},
		{"[::1]:8765", true},
		{"0.0.0.0:8765", false},
		{"192.168.1.10:8765", false},
		{"example.com:8765", false},
	}
	for _, tc := range cases {
		if got := isLoopbackBind(tc.addr); got != tc.want {
			t.Errorf("isLoopbackBind(%q) = %v, want %v", tc.addr, got, tc.want)
		}
	}
}

// warnExposure must fire only on an unauthenticated non-loopback bind, name the
// mitigation env var, and never echo the token value (token-leakage risk).
func TestWarnExposure(t *testing.T) {
	const token = "do-not-leak-me"

	var loud bytes.Buffer
	warnExposure(&loud, "0.0.0.0:8765", "")
	if !strings.Contains(loud.String(), mcpTokenEnv) || !strings.Contains(loud.String(), "0.0.0.0:8765") {
		t.Errorf("non-loopback warning missing risk/mitigation: %q", loud.String())
	}

	var quiet bytes.Buffer
	warnExposure(&quiet, "127.0.0.1:8765", "")
	if quiet.Len() != 0 {
		t.Errorf("loopback bind warned: %q", quiet.String())
	}

	var withToken bytes.Buffer
	warnExposure(&withToken, "0.0.0.0:8765", token)
	if withToken.Len() != 0 {
		t.Errorf("token-protected bind warned: %q", withToken.String())
	}
	if strings.Contains(loud.String()+quiet.String()+withToken.String(), token) {
		t.Error("warning output leaked the token value")
	}
}
