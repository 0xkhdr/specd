package cmd

import (
	"strings"
	"testing"
)

func TestScrubbedEnv(t *testing.T) {
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SPECD_VERIFY_TIMEOUT_MS", "1000")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "leak-me")

	env := scrubbedEnv()
	joined := strings.Join(env, "\n")

	if !strings.Contains(joined, "PATH=/usr/bin") {
		t.Error("scrubbedEnv dropped allowlisted PATH")
	}
	if !strings.Contains(joined, "SPECD_VERIFY_TIMEOUT_MS=1000") {
		t.Error("scrubbedEnv dropped SPECD_* var")
	}
	for _, kv := range env {
		if strings.HasPrefix(kv, "AWS_SECRET_ACCESS_KEY") {
			t.Error("scrubbedEnv leaked inherited secret AWS_SECRET_ACCESS_KEY")
		}
	}
}
