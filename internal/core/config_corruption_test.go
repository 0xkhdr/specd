package core

import (
	"path/filepath"
	"strings"
	"testing"
)

// Config corruption & secret-diagnostic matrix (spec A6).
//
// migrate-config and SPECD_CONFIG_FORMAT cover the happy path. These tests pin
// the negative edges: truncated/corrupt config must fail loud (no partial
// apply), a dual config.json+config.yml must resolve deterministically and be
// announced, and the diagnostic paths must never echo a secret value.

func projectConfig(root, name string) string {
	return filepath.Join(root, ".specd", name)
}

// Req 1.1 — truncated YAML mid-document is rejected with a clear error and never
// partial-applied. Each case must surface an error diagnostic for the file.
func TestConfigTruncatedYAMLRejected(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"missing-colon", "gates:\n  maxContextTokens 5000\n"},
		{"unterminated-quote", "mode: \"automatic\ngates:\n  maxContextTokens: 5000\n"},
		{"unterminated-sequence", "namespaces: [read, write\n"},
		{"odd-indent", "gates:\n   maxContextTokens: 5000\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolateGlobalConfig(t)
			root := t.TempDir()
			writeConfigFile(t, projectConfig(root, "config.yml"), tc.body)
			_, res := LoadConfigWithDiagnostics(root)
			if !hasDiagError(res.Diagnostics) {
				t.Fatalf("truncated YAML %q should produce an error diagnostic; got %+v", tc.name, res.Diagnostics)
			}
			// No partial apply: a file with an error diagnostic is skipped, so it
			// must not be recorded as the effective project config.
			if res.ProjectPath != "" {
				t.Fatalf("corrupt config must not be applied, but ProjectPath=%q", res.ProjectPath)
			}
		})
	}
}

// Req 1.2 — JSON with duplicate keys. Go's decoder takes the last value; we pin
// that documented resolution rather than inventing new behavior.
func TestConfigDuplicateJSONKeysLastWins(t *testing.T) {
	isolateGlobalConfig(t)
	root := t.TempDir()
	writeConfigFile(t, projectConfig(root, "config.json"), `{"gates":{"maxContextTokens":1000},"gates":{"maxContextTokens":7000}}`)
	cfg, res := LoadConfigWithDiagnostics(root)
	if hasDiagError(res.Diagnostics) {
		t.Fatalf("duplicate-key JSON should parse (last wins), got error: %+v", res.Diagnostics)
	}
	if cfg.Gates.MaxContextTokens != 7000 {
		t.Fatalf("duplicate JSON key resolution = %d, want 7000 (last value wins)", cfg.Gates.MaxContextTokens)
	}
}

// Req 2 — dual config.yml + config.json present. Precedence is deterministic
// (YAML wins, per ConfigPaths order) and the ignored lower-priority file is
// announced via a diagnostic rather than silently dropped.
func TestConfigDualFileDeterministicAndAnnounced(t *testing.T) {
	isolateGlobalConfig(t)
	root := t.TempDir()
	writeConfigFile(t, projectConfig(root, "config.yml"), "gates:\n  maxContextTokens: 7000\n")
	writeConfigFile(t, projectConfig(root, "config.json"), `{"gates":{"maxContextTokens":1000}}`)

	cfg, res := LoadConfigWithDiagnostics(root)
	if cfg.Gates.MaxContextTokens != 7000 {
		t.Fatalf("dual-file precedence: maxContextTokens=%d, want 7000 (config.yml wins)", cfg.Gates.MaxContextTokens)
	}
	if !strings.HasSuffix(res.ProjectPath, "config.yml") {
		t.Fatalf("ProjectPath=%q, want config.yml", res.ProjectPath)
	}
	announced := false
	for _, d := range res.Diagnostics {
		if strings.Contains(d.Message, "ignored lower-priority") && strings.HasSuffix(d.Source, "config.json") {
			announced = true
		}
	}
	if !announced {
		t.Fatalf("dual-file conflict not announced; diagnostics=%+v", res.Diagnostics)
	}
}

// Req 3.1 — the env-override diagnostic path must never echo the value of an
// override, so a secret-valued override cannot leak into logs/diagnostics.
func TestConfigEnvOverrideDiagnosticNeverEchoesValue(t *testing.T) {
	const secret = "s3cr3t-do-not-log-me"
	t.Setenv("SPECD_MAX_CONTEXT_TOKENS", "1234") // recognized numeric override
	t.Setenv("SPECD_DEFAULT_VERIFY", secret)     // recognized string override, secret-looking value

	isolateGlobalConfig(t)
	root := t.TempDir()
	_, res := LoadConfigWithDiagnostics(root)
	for _, d := range res.Diagnostics {
		if strings.Contains(d.Message, secret) {
			t.Fatalf("env-override diagnostic leaked the value: %q", d.Message)
		}
		if strings.Contains(d.Message, "1234") {
			t.Fatalf("env-override diagnostic leaked the value: %q", d.Message)
		}
	}
}

// Req 3.2 — a secret-named orchestration policy field is rejected, and the
// rejection names the offending key but never its value.
func TestRejectSecretBearingNamesKeyNotValue(t *testing.T) {
	const value = "AKIA-super-secret-value"
	raw := []byte(`{"orchestration":{"apiKey":"` + value + `"}}`)
	err := rejectSecretBearingOrchestration(raw)
	if err == nil {
		t.Fatal("secret-bearing orchestration key must be rejected")
	}
	if !strings.Contains(err.Error(), "apiKey") {
		t.Fatalf("rejection should name the key; got %q", err.Error())
	}
	if strings.Contains(err.Error(), value) {
		t.Fatalf("rejection must NOT echo the secret value; got %q", err.Error())
	}
}
