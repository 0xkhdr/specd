package gates

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSteeringTotalOmission(t *testing.T) {
	metadataLess := "# Steering\nno block here\n"
	withBlock := "<!-- specd-context\nid: ok\nversion: 1\npriority: 10\n-->\n# Steering\n"

	cases := []struct {
		name  string
		files map[string]string
		warns int
	}{
		{"empty directory", map[string]string{}, 0},
		{"all metadata-less", map[string]string{"a.md": metadataLess, "b.md": metadataLess}, 1},
		{"one metadata-less among valid", map[string]string{"a.md": metadataLess, "b.md": withBlock}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, ".specd", "steering")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			for name, body := range tc.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			findings := steeringApplicability(CheckCtx{Root: root})
			if len(findings) != tc.warns {
				t.Fatalf("findings = %+v, want %d", findings, tc.warns)
			}
			for _, f := range findings {
				if f.Severity != Warn {
					t.Fatalf("severity = %s, want warn (never a completion gate)", f.Severity)
				}
			}
		})
	}
}
