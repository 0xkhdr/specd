package adapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSchemaVersionNegotiationPolicyPublished(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "adapter-contract.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	for _, required := range []string{
		"Adapter schema compatibility and negotiation",
		"independent of CLI and on-disk state schemas",
		"exact offered version",
		"no implicit downgrade",
		"breaking change",
		"additive change",
	} {
		if !strings.Contains(doc, required) {
			t.Errorf("version policy missing %q", required)
		}
	}
}

func TestSchemaVersionFailsClosedWithoutExactOffer(t *testing.T) {
	req := sampleRequest()
	req.SchemaVersion = "adapter/v2"
	if err := req.Validate(); err == nil {
		t.Fatal("unsupported adapter version accepted")
	}
}
