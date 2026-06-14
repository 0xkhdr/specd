package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAnalyzeBoot_Python(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"pyproject.toml": `[project]
name = "myapp"
dependencies = ["fastapi", "sqlalchemy"]
[tool.pytest.ini_options]
testpaths = ["tests"]
`,
	})
	res := AnalyzeBoot(root)

	if got := res.Stacks; !reflect.DeepEqual(got, []string{"python"}) {
		t.Fatalf("stacks = %v, want [python]", got)
	}
	if res.ProjectName != "myapp" {
		t.Fatalf("projectName = %q, want myapp", res.ProjectName)
	}
	if res.Verify != "pytest" || res.VerifyFrom != "pyproject.toml [tool.pytest.ini_options]" {
		t.Fatalf("verify = %q from %q", res.Verify, res.VerifyFrom)
	}
	if !reflect.DeepEqual(res.Frameworks["python"], []string{"fastapi", "pytest", "sqlalchemy"}) {
		t.Fatalf("frameworks = %v", res.Frameworks["python"])
	}
}

func TestAnalyzeBoot_PythonUnittestFallback(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"requirements.txt": "requests==2.31\n"})
	res := AnalyzeBoot(root)
	if res.Verify != "python -m unittest discover" {
		t.Fatalf("verify = %q, want unittest fallback", res.Verify)
	}
	if res.VerifyFrom != "requirements.txt" {
		t.Fatalf("verifyFrom = %q", res.VerifyFrom)
	}
}

func TestAnalyzeBoot_Node(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"package.json": `{"name":"web","scripts":{"test":"jest"},"dependencies":{"react":"^18"},"devDependencies":{"jest":"^29"}}`,
	})
	res := AnalyzeBoot(root)
	if !reflect.DeepEqual(res.Stacks, []string{"nodejs"}) {
		t.Fatalf("stacks = %v", res.Stacks)
	}
	if res.Verify != "npm test" || res.VerifyFrom != "package.json (scripts.test)" {
		t.Fatalf("verify = %q from %q", res.Verify, res.VerifyFrom)
	}
	if !reflect.DeepEqual(res.Frameworks["nodejs"], []string{"jest", "react"}) {
		t.Fatalf("frameworks = %v", res.Frameworks["nodejs"])
	}
}

func TestAnalyzeBoot_NodeVitestNoScript(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"package.json": `{"name":"web","devDependencies":{"vitest":"^1"}}`,
	})
	res := AnalyzeBoot(root)
	if res.Verify != "vitest run" {
		t.Fatalf("verify = %q, want vitest run", res.Verify)
	}
}

func TestAnalyzeBoot_RustAndGo(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"Cargo.toml": "[package]\nname = \"crab\"\n[dependencies]\naxum = \"0.7\"\ntokio = \"1\"\n",
		"go.mod":     "module github.com/me/svc\n\ngo 1.22\n\nrequire github.com/gin-gonic/gin v1.9.1\n",
	})
	res := AnalyzeBoot(root)
	if !reflect.DeepEqual(res.Stacks, []string{"go", "rust"}) {
		t.Fatalf("stacks = %v, want [go rust]", res.Stacks)
	}
	if !reflect.DeepEqual(res.Frameworks["rust"], []string{"axum", "tokio"}) {
		t.Fatalf("rust frameworks = %v", res.Frameworks["rust"])
	}
	if !reflect.DeepEqual(res.Frameworks["go"], []string{"gin"}) {
		t.Fatalf("go frameworks = %v", res.Frameworks["go"])
	}
	// Go and Rust share priority 30; Python absent, so the alphabetically-first
	// of the tie (go) supplies the primary verify command.
	if res.Verify != "go test ./..." {
		t.Fatalf("verify = %q", res.Verify)
	}
}

func TestAnalyzeBoot_PolyglotPriority(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"pyproject.toml": "[tool.pytest.ini_options]\n",
		"go.mod":         "module x\ngo 1.22\n",
	})
	res := AnalyzeBoot(root)
	// Python (priority 40) wins the primary verify slot over Go (30).
	if res.Verify != "pytest" {
		t.Fatalf("verify = %q, want pytest (higher priority)", res.Verify)
	}
	if len(res.VerifyAlts) != 1 || res.VerifyAlts[0] != "go test ./..." {
		t.Fatalf("alts = %v", res.VerifyAlts)
	}
}

func TestAnalyzeBoot_GoNodeMonorepo(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                    "module x\ngo 1.22\n",
		"services/web/package.json": `{"name":"web","scripts":{"test":"jest"}}`,
	})
	a := AnalyzeBoot(root)
	// Nested manifest => monorepo layout.
	if a.Layout != "monorepo" {
		t.Fatalf("layout = %q, want monorepo", a.Layout)
	}
	// Only the root go.mod is a stack source; the nested package.json drives
	// layout but node detection only scans the root, so stacks = [go].
	if !reflect.DeepEqual(a.Stacks, []string{"go"}) {
		t.Fatalf("stacks = %v, want [go]", a.Stacks)
	}
	// Determinism: identical output across runs.
	if b := AnalyzeBoot(root); !reflect.DeepEqual(a, b) {
		t.Fatalf("non-deterministic monorepo output:\n%+v\n%+v", a, b)
	}
}

func TestAnalyzeBoot_UnknownRepo(t *testing.T) {
	root := t.TempDir()
	// Files present, but none is a recognized manifest.
	writeFiles(t, root, map[string]string{
		"README.md": "# hi\n",
		"main.c":    "int main(){return 0;}\n",
		"notes.txt": "todo\n",
	})
	a := AnalyzeBoot(root)
	if len(a.Stacks) != 0 {
		t.Fatalf("stacks = %v, want none for unknown repo", a.Stacks)
	}
	if a.Verify != "" {
		t.Fatalf("verify = %q, want empty for unknown repo", a.Verify)
	}
	if a.ProjectName != baseName(root) {
		t.Fatalf("projectName = %q, want dir base", a.ProjectName)
	}
	if b := AnalyzeBoot(root); !reflect.DeepEqual(a, b) {
		t.Fatalf("non-deterministic unknown output:\n%+v\n%+v", a, b)
	}
}

func TestAnalyzeBoot_Deterministic(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"pyproject.toml": "[project]\nname=\"a\"\ndependencies=[\"flask\"]\n",
		"package.json":   `{"name":"b","dependencies":{"express":"^4"}}`,
	})
	a := AnalyzeBoot(root)
	b := AnalyzeBoot(root)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("non-deterministic output:\n%+v\n%+v", a, b)
	}
}

func TestAnalyzeBoot_Conflict(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"package.json": `{"name":"w","devDependencies":{"jest":"^29","vitest":"^1"}}`,
	})
	res := AnalyzeBoot(root)
	if len(res.Conflicts) != 1 {
		t.Fatalf("conflicts = %v, want one", res.Conflicts)
	}
}

func TestAnalyzeBoot_EmptyRepo(t *testing.T) {
	root := t.TempDir()
	res := AnalyzeBoot(root)
	if len(res.Stacks) != 0 {
		t.Fatalf("stacks = %v, want none", res.Stacks)
	}
	if res.ProjectName != baseName(root) {
		t.Fatalf("projectName = %q, want dir base %q", res.ProjectName, baseName(root))
	}
}

func TestAnalyzeBoot_JavaRubyPhpElixir(t *testing.T) {
	cases := []struct {
		name       string
		files      map[string]string
		wantStack  string
		wantVerify string
	}{
		{"maven", map[string]string{"pom.xml": "<project><dependencies>spring</dependencies></project>"}, "java", "mvn test"},
		{"gradle", map[string]string{"build.gradle": "dependencies { junit }"}, "java", "gradle test"},
		{"ruby-rspec", map[string]string{"Gemfile": "gem 'rails'\ngem 'rspec'\n"}, "ruby", "bundle exec rspec"},
		{"ruby-rake", map[string]string{"Gemfile": "gem 'sinatra'\n"}, "ruby", "rake test"},
		{"php", map[string]string{"composer.json": `{"name":"v/p","require":{"laravel/framework":"^10"},"require-dev":{"phpunit/phpunit":"^10"}}`}, "php", "vendor/bin/phpunit"},
		{"elixir", map[string]string{"mix.exs": "defmodule X do\n  def project, do: [app: :myapp]\n  # phoenix\nend"}, "elixir", "mix test"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, tc.files)
			res := AnalyzeBoot(root)
			if !contains(res.Stacks, tc.wantStack) {
				t.Fatalf("stacks = %v, want %s", res.Stacks, tc.wantStack)
			}
			if res.Verify != tc.wantVerify {
				t.Fatalf("verify = %q, want %q", res.Verify, tc.wantVerify)
			}
		})
	}
}

func TestAnalyzeBoot_ElixirName(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"mix.exs": "[app: :cool_app, version: \"0.1.0\"]"})
	if got := AnalyzeBoot(root).ProjectName; got != "cool_app" {
		t.Fatalf("projectName = %q, want cool_app", got)
	}
}

func TestAnalyzeBoot_BuildAndCI(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                   "module x\ngo 1.22\n",
		"Makefile":                 "all:\n",
		"Dockerfile":               "FROM scratch\n",
		".github/workflows/ci.yml": "on: push\n",
		"Jenkinsfile":              "pipeline {}\n",
	})
	res := AnalyzeBoot(root)
	if !reflect.DeepEqual(res.Build, []string{"make", "docker"}) {
		t.Fatalf("build = %v, want [make docker]", res.Build)
	}
	if !reflect.DeepEqual(res.CI, []string{"github-actions", "jenkins"}) {
		t.Fatalf("ci = %v", res.CI)
	}
	// Build/CI source files folded into sources.
	if !contains(res.Sources, "Makefile") || !contains(res.Sources, ".github/workflows") {
		t.Fatalf("sources missing build/ci files: %v", res.Sources)
	}
}

func TestDetectLayout(t *testing.T) {
	t.Run("src", func(t *testing.T) {
		root := t.TempDir()
		writeFiles(t, root, map[string]string{"go.mod": "module x\n", "src/main.go": "package main\n"})
		if got := detectLayout(root); got != "src" {
			t.Fatalf("layout = %q, want src", got)
		}
	})
	t.Run("flat", func(t *testing.T) {
		root := t.TempDir()
		writeFiles(t, root, map[string]string{"go.mod": "module x\n"})
		if got := detectLayout(root); got != "flat" {
			t.Fatalf("layout = %q, want flat", got)
		}
	})
	t.Run("monorepo", func(t *testing.T) {
		root := t.TempDir()
		writeFiles(t, root, map[string]string{
			"go.mod":              "module x\n",
			"services/api/go.mod": "module api\n",
		})
		if got := detectLayout(root); got != "monorepo" {
			t.Fatalf("layout = %q, want monorepo", got)
		}
	})
	t.Run("skips vendored manifests", func(t *testing.T) {
		root := t.TempDir()
		writeFiles(t, root, map[string]string{
			"go.mod":                        "module x\n",
			"node_modules/dep/package.json": "{}",
		})
		if got := detectLayout(root); got != "flat" {
			t.Fatalf("layout = %q, want flat (node_modules ignored)", got)
		}
	})
}

func TestCheckBootFreshness(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specd"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFiles(t, root, map[string]string{"go.mod": "module x\ngo 1.22\n"})

	// Write a fresh boot.json by hand from a real analysis.
	res := AnalyzeBoot(root)
	b, _ := json.Marshal(res)
	if err := os.WriteFile(filepath.Join(root, ".specd", "boot.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}

	fresh, err := CheckBootFreshness(root)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Stale {
		t.Fatalf("expected fresh, got issues: %v", fresh.Issues)
	}

	// Add a new stack → drift.
	writeFiles(t, root, map[string]string{"package.json": `{"name":"w"}`})
	stale, err := CheckBootFreshness(root)
	if err != nil {
		t.Fatal(err)
	}
	if !stale.Stale {
		t.Fatal("expected stale after adding package.json")
	}

	// Missing boot.json → NotFoundError.
	root2 := t.TempDir()
	os.MkdirAll(filepath.Join(root2, ".specd"), 0o755)
	if _, err := CheckBootFreshness(root2); err == nil {
		t.Fatal("expected error when boot.json absent")
	}
}

func TestCheckBootFreshness_SourceRemoved(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".specd"), 0o755)
	writeFiles(t, root, map[string]string{"go.mod": "module x\ngo 1.22\n"})
	res := AnalyzeBoot(root)
	b, _ := json.Marshal(res)
	os.WriteFile(filepath.Join(root, ".specd", "boot.json"), b, 0o644)

	// Remove the recorded source.
	os.Remove(filepath.Join(root, "go.mod"))
	out, err := CheckBootFreshness(root)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Stale {
		t.Fatal("expected stale after removing go.mod")
	}
}

func TestScanFrameworks_WordBoundary(t *testing.T) {
	// "express" must not match "expressive"; "next" must match standalone.
	got := scanFrameworks("expressive next-thing\nnext\n", []string{"express", "next"})
	if !reflect.DeepEqual(got, []string{"next"}) {
		t.Fatalf("got %v, want [next]", got)
	}
}

func BenchmarkBootDetect(b *testing.B) {
	root := b.TempDir()
	files := map[string]string{
		"go.mod":         "module github.com/example/app\n\nrequire github.com/gin-gonic/gin v1.9.0\n",
		"package.json":   `{"name":"web","dependencies":{"react":"18","express":"4"},"devDependencies":{"jest":"29"},"scripts":{"test":"jest"}}`,
		"pyproject.toml": "[project]\nname = \"svc\"\ndependencies = [\"fastapi\", \"sqlalchemy\"]\n[tool.pytest.ini_options]\ntestpaths = [\"tests\"]\n",
		"Cargo.toml":     "[package]\nname = \"engine\"\n[dependencies]\naxum = \"0.7\"\ntokio = \"1\"\n",
		"Makefile":       "build:\n\tgo build ./...\n",
		"Dockerfile":     "FROM scratch\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = AnalyzeBoot(root)
	}
}
