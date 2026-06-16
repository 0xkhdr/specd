package core

import (
	"strings"
	"testing"
)

func TestPackManifest(t *testing.T) {
	valid := `{
      "name": "demo",
      "version": "1.0.0",
      "description": "x",
      "files": [{"path": ".specd/steering/a.md", "content": "hi"}]
    }`
	p, err := ParsePack([]byte(valid))
	if err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	if p.Name != "demo" || p.Version != "1.0.0" || len(p.Files) != 1 {
		t.Fatalf("parsed pack wrong: %+v", p)
	}

	bad := []struct {
		name, json, wantErr string
	}{
		{"executable hook", `{"name":"x","version":"1","files":[{"path":"a","content":"b"}],"postInstall":"rm -rf /"}`, "declarative-only"},
		{"hooks field", `{"name":"x","version":"1","files":[{"path":"a","content":"b"}],"hooks":["x"]}`, "declarative-only"},
		{"unknown field", `{"name":"x","version":"1","files":[{"path":"a","content":"b"}],"mystery":1}`, "unknown field"},
		{"missing name", `{"version":"1","files":[{"path":"a","content":"b"}]}`, "name"},
		{"missing version", `{"name":"x","files":[{"path":"a","content":"b"}]}`, "version"},
		{"no files", `{"name":"x","version":"1","files":[]}`, "no files"},
		{"abs path", `{"name":"x","version":"1","files":[{"path":"/etc/passwd","content":"b"}]}`, "must be relative"},
		{"traversal", `{"name":"x","version":"1","files":[{"path":"../../etc/passwd","content":"b"}]}`, "escapes"},
		{"non-canonical", `{"name":"x","version":"1","files":[{"path":"a/./b","content":"b"}]}`, "canonical"},
		{"duplicate path", `{"name":"x","version":"1","files":[{"path":"a","content":"1"},{"path":"a","content":"2"}]}`, "duplicate"},
		{"not json", `{not json`, "valid JSON"},
	}
	for _, c := range bad {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParsePack([]byte(c.json))
			if err == nil {
				t.Fatalf("expected rejection, got nil")
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("error %q does not mention %q", err.Error(), c.wantErr)
			}
		})
	}
}

func TestPackManifestBuiltins(t *testing.T) {
	packs, err := BuiltinPacks()
	if err != nil {
		t.Fatalf("BuiltinPacks: %v", err)
	}
	if len(packs) < 2 {
		t.Fatalf("want ≥2 built-in packs, got %d", len(packs))
	}
	names := map[string]bool{}
	for _, p := range packs {
		names[p.Name] = true
	}
	for _, want := range []string{"minimal", "go-service"} {
		if !names[want] {
			t.Errorf("missing built-in pack %q", want)
		}
	}
	// Sorted by name.
	for i := 1; i < len(packs); i++ {
		if packs[i-1].Name > packs[i].Name {
			t.Errorf("packs not sorted: %q before %q", packs[i-1].Name, packs[i].Name)
		}
	}
	if _, err := BuiltinPack("go-service"); err != nil {
		t.Errorf("BuiltinPack(go-service): %v", err)
	}
	if _, err := BuiltinPack("nope"); err == nil {
		t.Error("BuiltinPack(nope) should error")
	}
}
