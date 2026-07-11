package adapter

import (
	"go/build"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// modulePath is this repository's module path; it is the prefix of every
// first-party import. Anything outside it that carries a dotted domain segment
// is a third-party dependency.
const modulePath = "github.com/0xkhdr/specd"

// trustedCoreRoots are the directory subtrees that make up the trusted core:
// deterministic domain logic, gates, the DAG/report paths (all inside
// internal/core) and the bounded context builder. No package under these roots
// may import a provider/model, eval-service, deployment, telemetry-backend,
// protocol, or network package, nor the adapter package itself (R1.1, R1.3).
var trustedCoreRoots = []string{
	"internal/core",
	"internal/context",
}

// adapterPkg is the package that trusted core must never reach (R1.3).
var adapterPkg = modulePath + "/internal/adapter"

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller path")
	}
	// internal/adapter/import_guard_test.go -> repo root
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// prohibited reports whether a first-party package may import p, and why not.
// It rejects the adapter package (isolation), the network/protocol stdlib
// packages, and any third-party module.
func prohibited(p string) (bool, string) {
	if p == adapterPkg || strings.HasPrefix(p, adapterPkg+"/") {
		return true, "trusted core must not import internal/adapter"
	}
	if isNetwork(p) {
		return true, "trusted core must not import a network/protocol package"
	}
	if isExternalModule(p) {
		return true, "trusted core must not import a third-party dependency"
	}
	return false, ""
}

// isNetwork matches the net package and its transport/protocol children while
// allowing the pure-parsing helpers that touch no socket.
func isNetwork(p string) bool {
	switch p {
	case "net/url", "net/netip", "net/textproto":
		return false
	}
	return p == "net" || strings.HasPrefix(p, "net/")
}

// isExternalModule matches import paths whose first segment is a dotted domain
// (e.g. a real third-party module) and that are not part of this module.
func isExternalModule(p string) bool {
	if p == modulePath || strings.HasPrefix(p, modulePath+"/") {
		return false
	}
	first := p
	if i := strings.IndexByte(p, '/'); i >= 0 {
		first = p[:i]
	}
	return strings.Contains(first, ".")
}

// enumerateCorePackages walks the trusted-core roots and returns the import
// path of every buildable package found. The enumeration is asserted in the
// test itself so a broken walk fails loudly instead of vacuously passing.
func enumerateCorePackages(t *testing.T, root string) []string {
	t.Helper()
	var pkgs []string
	for _, sub := range trustedCoreRoots {
		start := filepath.Join(root, sub)
		err := filepath.WalkDir(start, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				return nil
			}
			bp, err := build.ImportDir(path, 0)
			if err != nil {
				if _, ok := err.(*build.NoGoError); ok {
					return nil // directory has no buildable Go files
				}
				return err
			}
			rel, err := filepath.Rel(root, bp.Dir)
			if err != nil {
				return err
			}
			pkgs = append(pkgs, modulePath+"/"+filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", start, err)
		}
	}
	return pkgs
}

// packageImports returns the non-test imports of a first-party package.
func packageImports(root, importPath string) ([]string, error) {
	rel := strings.TrimPrefix(importPath, modulePath+"/")
	bp, err := build.ImportDir(filepath.Join(root, filepath.FromSlash(rel)), 0)
	if err != nil {
		return nil, err
	}
	return bp.Imports, nil
}

// TestImportGuard enforces the boundary invariant (R1.1, R1.3): no package in
// the trusted core, nor any first-party package it transitively reaches, may
// import a prohibited package class. Transitivity is followed only through
// first-party edges so a gate cannot reach an adapter or the network via a
// core helper (R1.3).
func TestImportGuard(t *testing.T) {
	root := repoRoot(t)

	seeds := enumerateCorePackages(t, root)
	if len(seeds) == 0 {
		t.Fatal("no trusted-core packages enumerated — the walk is broken")
	}
	for _, want := range []string{
		modulePath + "/internal/core",
		modulePath + "/internal/core/gates",
		modulePath + "/internal/context",
	} {
		if !contains(seeds, want) {
			t.Fatalf("enumeration missing keystone package %s", want)
		}
	}

	visited := map[string]bool{}
	queue := append([]string(nil), seeds...)
	for len(queue) > 0 {
		pkg := queue[0]
		queue = queue[1:]
		if visited[pkg] {
			continue
		}
		visited[pkg] = true

		imports, err := packageImports(root, pkg)
		if err != nil {
			t.Fatalf("imports of %s: %v", pkg, err)
		}
		for _, imp := range imports {
			if bad, why := prohibited(imp); bad {
				t.Errorf("boundary violation: %s imports %s — %s", pkg, imp, why)
			}
			// Follow only first-party edges to keep the closure inside the
			// module; stdlib packages that legitimately touch net internally
			// are not part of the trusted-core reachability graph.
			if imp == modulePath || strings.HasPrefix(imp, modulePath+"/") {
				if !visited[imp] {
					queue = append(queue, imp)
				}
			}
		}
	}

	// Positive control: the classifier must actually reject the classes it
	// claims to, so a passing run above cannot be a vacuous no-op.
	for _, sample := range []string{
		adapterPkg,
		adapterPkg + "/runner",
		"net",
		"net/http",
		"net/rpc",
		"google.golang.org/grpc",
		"github.com/prometheus/client_golang/prometheus",
	} {
		if bad, _ := prohibited(sample); !bad {
			t.Errorf("classifier failed to reject prohibited import %q", sample)
		}
	}
	for _, sample := range []string{
		"fmt",
		"os/exec",
		"encoding/json",
		"net/url",
		modulePath + "/internal/core",
	} {
		if bad, why := prohibited(sample); bad {
			t.Errorf("classifier wrongly rejected allowed import %q: %s", sample, why)
		}
	}
}

// TestZeroDependency enforces R1.2: the binary retains zero runtime
// dependencies. go.mod must carry no require directive outside test tooling —
// today it carries none at all — so a new dependency fails this test. `go mod
// verify` (run alongside in the task's verify line) covers the module cache.
func TestZeroDependency(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	inRequireBlock := false
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.TrimSpace(line)
		switch {
		case f == "require (":
			inRequireBlock = true
			t.Errorf("go.mod declares a require block — zero-dependency invariant violated")
		case inRequireBlock && f == ")":
			inRequireBlock = false
		case inRequireBlock:
			t.Errorf("go.mod requires a runtime dependency: %q", f)
		case strings.HasPrefix(f, "require "):
			t.Errorf("go.mod requires a runtime dependency: %q", f)
		}
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
