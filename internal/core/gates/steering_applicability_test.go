package gates

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSteeringApplicabilityWarning(t *testing.T) {
	metadataLess := "# Steering\nno block here\n"
	withBlock := "<!-- specd-context\nid: ok\nversion: 1\npriority: 10\n-->\n# Steering\n"
	const remedy = "every steering file is dropped from the machine manifest for missing `specd-context` metadata; add a `specd-context` block (id, version, priority) to each `.specd/steering/*.md` or drivers run with no project steering"

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
			var findings []Finding
			for _, finding := range CoreRegistry().Run(CheckCtx{Root: root}) {
				if finding.Gate == "steering-applicability" {
					findings = append(findings, finding)
				}
			}
			if len(findings) != tc.warns {
				t.Fatalf("findings = %+v, want %d", findings, tc.warns)
			}
			for _, f := range findings {
				if f.Severity != Warn {
					t.Fatalf("severity = %s, want warn (never a completion gate)", f.Severity)
				}
				if f.Message != remedy {
					t.Fatalf("message = %q, want actionable remedy %q", f.Message, remedy)
				}
			}
		})
	}
}
