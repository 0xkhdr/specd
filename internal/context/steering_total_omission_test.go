package context

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
		want  bool
	}{
		{"absent directory", nil, false},
		{"all metadata-less", map[string]string{"a.md": metadataLess, "b.md": metadataLess}, true},
		{"one metadata-less among valid", map[string]string{"a.md": metadataLess, "b.md": withBlock}, false},
		{"all valid", map[string]string{"a.md": withBlock}, false},
		{"memory.md exempt", map[string]string{"memory.md": metadataLess, "a.md": withBlock}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.files != nil {
				dir := filepath.Join(root, ".specd", "steering")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				for name, body := range tc.files {
					if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
						t.Fatal(err)
					}
				}
			}
			got, err := SteeringTotalOmission(root)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("SteeringTotalOmission = %v, want %v", got, tc.want)
			}
		})
	}
}
