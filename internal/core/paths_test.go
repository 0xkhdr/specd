package core

import (
	"errors"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSafeJoin (R2.2): repo-relative paths resolve under root; empty, absolute,
// and traversal-escaping inputs are refused.
func TestSafeJoinPath(t *testing.T) {
	root := t.TempDir()
	for _, bad := range []string{"", "/etc/passwd", "../escape", "a/../../b"} {
		if _, err := SafeJoin(root, bad); err == nil {
			t.Fatalf("SafeJoin accepted unsafe %q", bad)
		}
	}
	abs, err := SafeJoin(root, ".specd/specs/demo/tasks.md")
	if err != nil {
		t.Fatalf("SafeJoin rejected safe path: %v", err)
	}
	if !strings.HasPrefix(abs, root+string(filepath.Separator)) {
		t.Fatalf("SafeJoin escaped root: %s", abs)
	}
}

func TestRefusalSpecDirSink(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(SpecdDir(root), "specs", "demo")
	if got := SpecDir(root, "demo"); got != want {
		t.Fatalf("SpecDir = %q, want %q", got, want)
	}
	for _, slug := range []string{"", "..", "../escape", "/absolute", "a/b"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("SpecDir accepted invalid slug %q", slug)
				}
			}()
			_ = SpecDir(root, slug)
		}()
	}
}

// Every direct join below .specd/specs/<slug> must stay inside SpecDir, where
// slug validation cannot be skipped by a caller.
func TestRefusalPerSpecJoinsUseSink(t *testing.T) {
	walkProductionGo(t, func(path string, fset *token.FileSet, file *ast.File) {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil || function.Name.Name == "SpecDir" {
				continue
			}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if ok && isDirectSpecsJoin(call) {
					position := fset.Position(call.Pos())
					t.Errorf("%s:%d: %s bypasses SpecDir", path, position.Line, function.Name.Name)
				}
				return true
			})
		}
	})
}

func isDirectSpecsJoin(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Join" || len(call.Args) < 3 {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok || pkg.Name != "filepath" {
		return false
	}
	base, ok := call.Args[0].(*ast.CallExpr)
	if !ok {
		return false
	}
	specdDir, ok := base.Fun.(*ast.Ident)
	if !ok || specdDir.Name != "SpecdDir" {
		return false
	}
	specs, ok := call.Args[1].(*ast.BasicLit)
	return ok && specs.Kind == token.STRING && specs.Value == `"specs"`
}

func TestFindRoot(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(filepath.Join(root, specdDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := FindRoot(nested)
	if err != nil {
		t.Fatalf("FindRoot: %v", err)
	}
	if got != root {
		t.Fatalf("FindRoot=%q want %q", got, root)
	}
	var nf NotFoundError
	if _, err := FindRoot(t.TempDir()); !errors.As(err, &nf) || nf.ExitCode() != 3 {
		t.Fatalf("missing root err=%T %[1]v", err)
	}
}

func TestFindRootResolvesSymlink(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, specdDirName, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	got, err := FindRoot(filepath.Join(link, specdDirName, "nested"))
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(root)
	if got != want {
		t.Fatalf("FindRoot=%q want canonical %q", got, want)
	}
}

func TestSlug(t *testing.T) {
	valid := []string{"a", "a1", "spec-01", "0-a"}
	for _, slug := range valid {
		if !ValidSlug(slug) || ValidateSlug(slug) != nil {
			t.Fatalf("valid slug rejected: %q", slug)
		}
	}
	invalid := []string{"", "-", "-a", "a-", "a--b", "A", "a_b", "a.b"}
	// Path-traversal escapes must be rejected: a slug is a path component under
	// .specd/specs/<slug>/, so any `..`, absolute path, or separator that could
	// escape that directory is invalid (T-04-03, security trust boundary).
	traversal := []string{
		"..", "../x", "../../etc", "a/../b", "..\\x",
		"/etc/passwd", "/abs", "a/b", "spec/../..",
		".specd", "spec..d",
	}
	for _, slug := range append(invalid, traversal...) {
		if ValidSlug(slug) || ValidateSlug(slug) == nil {
			t.Fatalf("invalid slug accepted: %q", slug)
		}
	}
}
