package adapter

import (
	"strings"
	"testing"
)

func TestClassifyTaxonomy(t *testing.T) {
	for _, c := range AllClasses() {
		if !c.Valid() {
			t.Errorf("%s should be valid", c)
		}
	}
	if len(AllClasses()) != 9 {
		t.Fatalf("taxonomy has %d classes, want 9", len(AllClasses()))
	}
	if Class("mystery").Valid() {
		t.Error("unknown class validated")
	}
}

func TestClassifyRestricted(t *testing.T) {
	restricted := map[Class]bool{ClassSecret: true, ClassSourceContent: true, ClassPrompt: true}
	for _, c := range AllClasses() {
		if c.Restricted() != restricted[c] {
			t.Errorf("%s.Restricted() = %v, want %v", c, c.Restricted(), restricted[c])
		}
	}
}

func TestRedactForExport(t *testing.T) {
	refs := []Ref{
		{Name: "meta", Digest: "m", Class: ClassPublicMetadata, Inline: "ok-to-share"},
		{Name: "secret", Digest: "s", Class: ClassSecret, Inline: "AKIA-TOPSECRET"},
		{Name: "code", Digest: "c", Class: ClassSourceContent, Inline: "func main(){}"},
		{Name: "prompt", Digest: "p", Class: ClassPrompt, Inline: "you are a helpful..."},
	}
	kept, redactions := RedactForExport(refs, ExportPolicy{})
	if len(kept) != len(refs) {
		t.Fatalf("refs count changed: %d", len(kept))
	}
	// Default policy: restricted inline content must be absent after export.
	for _, r := range kept {
		if r.Class.Restricted() && r.Inline != "" {
			t.Errorf("restricted class %s leaked inline content", r.Class)
		}
	}
	// Public metadata inline survives; digests always survive.
	for _, r := range kept {
		if r.Name == "meta" && r.Inline == "" {
			t.Error("public metadata inline wrongly stripped")
		}
		if r.Digest == "" {
			t.Error("digest dropped during redaction")
		}
	}
	if len(redactions) != 3 {
		t.Fatalf("expected 3 redaction records, got %d", len(redactions))
	}
	// The export must prove absence: no restricted secret text anywhere.
	blob := ""
	for _, r := range kept {
		blob += r.Inline
	}
	if strings.Contains(blob, "TOPSECRET") {
		t.Fatal("secret content present in exported refs")
	}
}

func TestRedactOptInInline(t *testing.T) {
	refs := []Ref{{Name: "code", Digest: "c", Class: ClassSourceContent, Inline: "src"}}
	kept, redactions := RedactForExport(refs, ExportPolicy{AllowInline: []Class{ClassSourceContent}})
	if kept[0].Inline != "src" {
		t.Fatal("opted-in inline was stripped")
	}
	if len(redactions) != 0 {
		t.Fatalf("opted-in class recorded a redaction: %d", len(redactions))
	}
}
