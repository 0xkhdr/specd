package security

import (
	"strings"
	"testing"
)

func TestSecrets(t *testing.T) {
	t.Run("flags_synthetic_aws_and_github_keys_redacted", func(t *testing.T) {
		findings := secretsScanner{}.Scan([]TrackedFile{readFixture(t, "secrets/leak.txt")})
		rules := map[string]Finding{}
		for _, f := range findings {
			rules[f.Rule] = f
		}
		aws, ok := rules["aws-access-key"]
		if !ok {
			t.Fatalf("aws key not flagged: %+v", findings)
		}
		if aws.Line != 2 {
			t.Errorf("aws line = %d, want 2", aws.Line)
		}
		if strings.Contains(aws.Excerpt, "ABCDEFGHIJKLMNOP") {
			t.Errorf("excerpt leaked the key: %q", aws.Excerpt)
		}
		if _, ok := rules["github-pat"]; !ok {
			t.Errorf("github pat not flagged: %+v", findings)
		}
	})

	t.Run("clean_file_is_true_negative", func(t *testing.T) {
		if f := (secretsScanner{}).Scan([]TrackedFile{readFixture(t, "secrets/clean.txt")}); len(f) != 0 {
			t.Fatalf("clean file produced findings: %+v", f)
		}
	})

	t.Run("high_entropy_base64_flagged", func(t *testing.T) {
		file := TrackedFile{Path: "cfg.txt", Content: []byte("token=Xf9Kq2wLpZ7bV3nR8sT1yU4cE6dA0gH5jM2oNqPrStU\n")}
		found := false
		for _, f := range (secretsScanner{}).Scan([]TrackedFile{file}) {
			if f.Rule == "high-entropy-string" {
				found = true
			}
		}
		if !found {
			t.Fatal("expected high-entropy finding")
		}
	})

	t.Run("low_entropy_repetitive_token_not_flagged", func(t *testing.T) {
		file := TrackedFile{Path: "cfg.txt", Content: []byte("value=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n")}
		for _, f := range (secretsScanner{}).Scan([]TrackedFile{file}) {
			if f.Rule == "high-entropy-string" {
				t.Fatalf("repetitive token should not be flagged: %+v", f)
			}
		}
	})
}
