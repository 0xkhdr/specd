package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestObservabilityConformanceOfflineAndBounded(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, name := range []string{"internal/core/attestation.go", "internal/core/program.go"} {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		for _, forbidden := range []string{"net/http", "google.golang.org", "go.opentelemetry.io"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s imports or names forbidden runtime transport %q", name, forbidden)
			}
		}
	}
	doc, err := os.ReadFile(filepath.Join(root, "docs", "adapters", "telemetry.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, contract := range []string{"attestation/v1", "HMAC-SHA256", "Core never resolves model availability", "no mapping can bypass verify evidence"} {
		if !strings.Contains(string(doc), contract) {
			t.Fatalf("adapter contract missing %q", contract)
		}
	}
}
