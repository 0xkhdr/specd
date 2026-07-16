package core

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestHandshakeToolContracts(t *testing.T) {
	hs := BootstrapHandshake(Config{})
	if hs.OperationSchemaVersion != OperationSchemaVersion {
		t.Fatalf("operation schema = %d, want %d", hs.OperationSchemaVersion, OperationSchemaVersion)
	}
	if len(hs.ToolContracts) == 0 {
		t.Fatal("tool contracts missing")
	}
	for _, tool := range hs.ToolContracts {
		if tool.OperationID == "" || tool.Route == "" || len(tool.Phases) == 0 || len(tool.ExitCodes) == 0 {
			t.Fatalf("incomplete tool contract: %+v", tool)
		}
		if tool.HumanOnly && tool.Mutable == false {
			t.Fatalf("human-only mutation not labeled mutable: %+v", tool)
		}
	}
	want := map[string]bool{"eval.import": true, "eval.status": true}
	for _, id := range hs.Tools {
		delete(want, id)
	}
	if len(want) != 0 {
		t.Fatalf("handshake missing operations: %v", want)
	}
}

func TestManagedDigest(t *testing.T) {
	root := t.TempDir()
	assets, err := ManagedAssets()
	if err != nil {
		t.Fatal(err)
	}
	for _, asset := range assets {
		path := filepath.Join(root, asset.RelPath)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(asset.Block()+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("user\n"+agentsBegin+"\nharness\n"+agentsEnd+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := ManagedDigest(root)
	if err != nil || before == "" {
		t.Fatalf("managed digest: %q, %v", before, err)
	}
	if err := os.WriteFile(filepath.Join(root, assets[0].RelPath), []byte(assets[0].Block()+"\nuser tail\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	afterUserEdit, _ := ManagedDigest(root)
	if before != afterUserEdit {
		t.Fatal("content outside managed region changed digest")
	}
	if err := os.WriteFile(filepath.Join(root, assets[0].RelPath), []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	afterManagedEdit, _ := ManagedDigest(root)
	if before == afterManagedEdit {
		t.Fatal("managed content drift did not change digest")
	}
}

func TestHandshakeBind(t *testing.T) {
	state := InitialState("demo")
	state.Status = StatusTasks
	state.Phase = PhaseForStatus(StatusTasks)
	state.Revision = 7
	hs, err := BootstrapHandshakeForRoot(t.TempDir(), Config{}, &state, []string{"specd status demo --guide --json"})
	if err != nil {
		t.Fatal(err)
	}
	if hs.Binary.Version == "" || hs.StateSchemaVersion != StateSchemaVersion || hs.ContextSchemaVersion == "" || hs.TemplateSchemaVersion != TemplateVersion {
		t.Fatalf("schema/binary identity missing: %+v", hs)
	}
	if hs.WorkspaceRoot == "" || hs.ActiveSpec == nil || hs.ActiveSpec.Slug != "demo" || hs.ActiveSpec.Status != StatusTasks || hs.ActiveSpec.Revision != 7 {
		t.Fatalf("workspace/spec identity missing: %+v", hs)
	}
	if hs.PaletteDigest == "" || hs.ConfigDigest == "" || hs.ManagedDigest == "" || hs.GuidanceDigest == "" || hs.ContextSchemaDigest == "" || len(hs.Tools) == 0 || len(hs.NextCommands) != 1 {
		t.Fatalf("operational identity missing: %+v", hs)
	}
}

func TestHandshakeDigestIsolation(t *testing.T) {
	root := t.TempDir()
	if err := WriteScaffold(root); err != nil {
		t.Fatal(err)
	}
	base, err := BootstrapHandshakeForRoot(root, Config{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if base.GuidanceDigest != base.ManagedDigest {
		t.Fatalf("guidance digest must preserve managed compatibility identity: %q != %q", base.GuidanceDigest, base.ManagedDigest)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("user-only\n"+agentsBegin+"\nharness\n"+agentsEnd+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := BootstrapHandshakeForRoot(root, Config{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if base.GuidanceDigest == changed.GuidanceDigest {
		t.Fatal("managed guidance drift not detected")
	}
	if base.PaletteDigest != changed.PaletteDigest || base.ConfigDigest != changed.ConfigDigest || base.ContextSchemaDigest != changed.ContextSchemaDigest {
		t.Fatal("guidance drift changed unrelated handshake digests")
	}
}

func TestContextSchemaDigestStableAndIndependent(t *testing.T) {
	first := ContextSchemaDigest()
	second := ContextSchemaDigest()
	if first == "" || first != second {
		t.Fatalf("context schema digest unstable: %q %q", first, second)
	}
	other, err := GuidanceDigest(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if first == other {
		t.Fatal("context schema digest must be isolated from guidance digest")
	}
}

func TestHandshakeTypedSources(t *testing.T) {
	hs, err := BootstrapHandshakeForRoot(t.TempDir(), Config{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(hs.Authority.HarnessInstructions) == 0 {
		t.Fatal("harness instruction authority missing")
	}
	want := []string{"requirements", "source", "test_output", "adapter_observation"}
	if !reflect.DeepEqual(hs.Authority.UntrustedInputs, want) {
		t.Fatalf("untrusted source classes = %v, want %v", hs.Authority.UntrustedInputs, want)
	}
}
