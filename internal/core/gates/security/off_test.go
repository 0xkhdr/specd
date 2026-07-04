package security

import (
	"testing"

	"github.com/0xkhdr/specd/internal/core/gates"
)

func TestSecurityOffByDefault(t *testing.T) {
	registry := gates.CoreRegistry()
	for _, finding := range registry.Run(gates.CheckCtx{}) {
		if finding.Gate == "security" {
			t.Fatalf("security gate ran by default: %#v", finding)
		}
	}
}
