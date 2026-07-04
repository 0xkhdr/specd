package context

import (
	"encoding/json"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestBuildManifest(t *testing.T) {
	tasks := []core.TaskRow{{ID: "T1", Role: "craftsman"}}
	got, err := BuildManifest("demo", tasks, "T1")
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}
	if got.Version != ManifestVersion || got.Mode != "craftsman" || len(got.Items) != 4 {
		t.Fatalf("manifest = %+v", got)
	}
	if got.Items[0].Kind != "role" || got.Items[3].Kind != "tasks" {
		t.Fatalf("items not deterministic: %+v", got.Items)
	}
}

func TestManifestValidate(t *testing.T) {
	manifest, err := BuildManifest("demo", []core.TaskRow{{ID: "T1", Role: "validator"}}, "T1")
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := ValidateManifest(raw); err != nil {
		t.Fatalf("ValidateManifest: %v", err)
	}
	if err := ValidateManifest([]byte(`{"version":"1"}`)); err == nil {
		t.Fatalf("ValidateManifest accepted malformed manifest")
	}
}
