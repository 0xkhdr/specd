package cmd

import (
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// brain_bootstrap_cov_test.go drives brainRunBootstrap's spec-creation success
// branch directly. Via the CLI the spec is loaded up front and errors
// not-found before bootstrap fires, so this branch is otherwise dead; the unit
// test scaffolds an inited workspace, then bootstraps a missing spec.

func TestBrainRunBootstrapCreatesSpec(t *testing.T) {
	root := initTestRoot(t) // chdir's into an empty temp workspace
	if code := RunInit(cli.Args{Flags: map[string]string{}}); code != core.ExitOK {
		t.Fatalf("RunInit exit = %d, want OK", code)
	}

	slug := "payments"
	if core.SpecExists(root, slug) {
		t.Fatalf("spec %s should not exist before bootstrap", slug)
	}

	args := cli.Args{Flags: map[string]string{"bootstrap": "true", "title": "Payments"}}
	items := []core.PreflightItem{{Kind: "spec", Message: "spec not found", Remedy: "specd new " + slug}}

	if ok := brainRunBootstrap(root, slug, args, items); !ok {
		t.Fatal("bootstrap should succeed for a spec item with --bootstrap")
	}
	if !core.SpecExists(root, slug) {
		t.Fatal("bootstrap did not create the spec")
	}
}

func TestBrainRunBootstrapBlocksUnit(t *testing.T) {
	root := initTestRoot(t)

	// A spec item without --bootstrap is blocked (no scaffolding).
	specItem := []core.PreflightItem{{Kind: "spec", Message: "missing", Remedy: "specd new x"}}
	if ok := brainRunBootstrap(root, "x", cli.Args{Flags: map[string]string{}}, specItem); ok {
		t.Fatal("spec item without --bootstrap should block")
	}

	// A non-spec item (e.g. steering) is always blocked — never auto-remedied.
	steeringItem := []core.PreflightItem{{Kind: "steering", Message: "missing steering", Remedy: "specd init"}}
	if ok := brainRunBootstrap(root, "x", cli.Args{Flags: map[string]string{"bootstrap": "true"}}, steeringItem); ok {
		t.Fatal("non-spec item should block even with --bootstrap")
	}

	// No items → ready.
	if ok := brainRunBootstrap(root, "x", cli.Args{Flags: map[string]string{}}, nil); !ok {
		t.Fatal("empty preflight should be ready")
	}
}
