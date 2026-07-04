package context

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestHUDRender(t *testing.T) {
	m, err := BuildManifest("demo", []core.TaskRow{{ID: "T1", Role: "craftsman"}}, "T1")
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}
	out := RenderHUD(m)
	for _, want := range []string{"mode: craftsman", "demo", "T1", "LOAD", "BYTES", "TOKENS", "TOTAL"} {
		if !strings.Contains(out, want) {
			t.Fatalf("HUD missing %q:\n%s", want, out)
		}
	}
	// Every load item's label appears.
	for _, item := range m.Items {
		if !strings.Contains(out, itemLabel(item)) {
			t.Fatalf("HUD missing item %q:\n%s", itemLabel(item), out)
		}
	}
}

// TestHUDMatchesJSON asserts the token total shown by the HUD equals the value
// the --json surface serializes — one engine, two renders (RH.3).
func TestHUDMatchesJSON(t *testing.T) {
	m, err := BuildManifest("demo", []core.TaskRow{{ID: "T1", Role: "validator"}}, "T1")
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var jsonView Manifest
	if err := json.Unmarshal(raw, &jsonView); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	hud := RenderHUD(m)
	totalLine := ""
	for _, line := range strings.Split(hud, "\n") {
		if strings.HasPrefix(line, "TOTAL") {
			totalLine = line
		}
	}
	if totalLine == "" {
		t.Fatalf("no TOTAL line:\n%s", hud)
	}
	fields := strings.Fields(totalLine) // TOTAL <bytes> <tokens>
	hudTokens, err := strconv.Atoi(fields[len(fields)-1])
	if err != nil {
		t.Fatalf("parse HUD token total %q: %v", totalLine, err)
	}
	if hudTokens != jsonView.EstimatedTokens {
		t.Fatalf("HUD token total %d != --json %d", hudTokens, jsonView.EstimatedTokens)
	}
}
