package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ---- shared helpers -------------------------------------------------------

func readFileStr(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func baseName(root string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	return filepath.Base(abs)
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func joinComma(xs []string) string { return strings.Join(xs, ", ") }

// scanFrameworks returns the candidates that appear as whole words in content.
// Word boundaries avoid matching "expressive" when looking for "express".
func scanFrameworks(content string, candidates []string) []string {
	var found []string
	for _, c := range candidates {
		// TODO(stage06): hoist — this compiles one regex per candidate per call.
		re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(c) + `\b`)
		if re.MatchString(content) {
			found = append(found, c)
		}
	}
	return found
}

var tomlNameRe = regexp.MustCompile(`(?m)^\s*name\s*=\s*"([^"]+)"`)
var goModuleRe = regexp.MustCompile(`(?m)^\s*module\s+(\S+)`)

func tomlName(content string) string {
	if m := tomlNameRe.FindStringSubmatch(content); len(m) == 2 {
		return m[1]
	}
	return ""
}

// ---- build / ci / layout --------------------------------------------------

// detectBuildTools reports build orchestrators present at the repo root and the
// files that signalled them.
func detectBuildTools(root string) (tools, sources []string) {
	checks := []struct{ file, tool string }{
		{"Makefile", "make"},
		{"justfile", "just"},
		{"Taskfile.yml", "task"},
		{"Taskfile.yaml", "task"},
		{"Dockerfile", "docker"},
		{"docker-compose.yml", "docker compose"},
		{"docker-compose.yaml", "docker compose"},
	}
	seen := map[string]bool{}
	for _, c := range checks {
		if FileExists(filepath.Join(root, c.file)) {
			if !seen[c.tool] {
				seen[c.tool] = true
				tools = append(tools, c.tool)
			}
			sources = append(sources, c.file)
		}
	}
	return tools, sources
}

// detectCI reports CI providers configured at the repo root.
func detectCI(root string) (providers, sources []string) {
	dir := func(p string) bool {
		fi, err := os.Stat(filepath.Join(root, p))
		return err == nil && fi.IsDir()
	}
	if dir(".github/workflows") {
		providers = append(providers, "github-actions")
		sources = append(sources, ".github/workflows")
	}
	if FileExists(filepath.Join(root, ".gitlab-ci.yml")) {
		providers = append(providers, "gitlab-ci")
		sources = append(sources, ".gitlab-ci.yml")
	}
	if FileExists(filepath.Join(root, "Jenkinsfile")) {
		providers = append(providers, "jenkins")
		sources = append(sources, "Jenkinsfile")
	}
	if dir(".circleci") {
		providers = append(providers, "circleci")
		sources = append(sources, ".circleci")
	}
	return providers, sources
}

var manifestFiles = map[string]bool{
	"package.json": true, "pyproject.toml": true, "Cargo.toml": true,
	"go.mod": true, "composer.json": true, "Gemfile": true,
	"pom.xml": true, "build.gradle": true, "mix.exs": true,
}

var skipDirs = map[string]bool{
	".git": true, ".specd": true, "node_modules": true,
	"vendor": true, "target": true, "dist": true, "build": true,
	".venv": true, "venv": true, "__pycache__": true,
}

// detectLayout classifies the repo as monorepo (manifests nested below the
// root), src (a src/ directory), or flat. monorepo wins when both apply.
func detectLayout(root string) string {
	nested := false
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || nested {
			return nil
		}
		if d.IsDir() {
			if path != root && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !manifestFiles[d.Name()] {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if strings.Contains(rel, string(filepath.Separator)) {
			nested = true
		}
		return nil
	})
	if nested {
		return "monorepo"
	}
	if fi, err := os.Stat(filepath.Join(root, "src")); err == nil && fi.IsDir() {
		return "src"
	}
	return "flat"
}

// ---- python ---------------------------------------------------------------

type pythonDetector struct{}

func (pythonDetector) Name() string  { return "python" }
func (pythonDetector) Priority() int { return 40 }

func (pythonDetector) Detect(root string) *Detection {
	candidates := []string{"pyproject.toml", "requirements.txt", "setup.py", "setup.cfg"}
	var sources []string
	var combined strings.Builder
	for _, f := range candidates {
		if c := readFileStr(filepath.Join(root, f)); c != "" || FileExists(filepath.Join(root, f)) {
			sources = append(sources, f)
			combined.WriteString(c)
			combined.WriteString("\n")
		}
	}
	if len(sources) == 0 {
		return nil
	}

	pyproject := readFileStr(filepath.Join(root, "pyproject.toml"))
	d := &Detection{
		Stack:       "python",
		Frameworks:  scanFrameworks(combined.String(), []string{"django", "flask", "fastapi", "pytest", "sqlalchemy"}),
		Sources:     sources,
		ProjectName: tomlName(pyproject),
	}

	switch {
	case strings.Contains(pyproject, "[tool.pytest"):
		d.Verify, d.VerifyFrom = "pytest", "pyproject.toml [tool.pytest.ini_options]"
	case contains(d.Frameworks, "pytest"):
		d.Verify, d.VerifyFrom = "pytest", "pytest in dependencies"
	default:
		d.Verify, d.VerifyFrom = "python -m unittest discover", sources[0]
	}
	return d
}

// ---- node -----------------------------------------------------------------

type nodeDetector struct{}

func (nodeDetector) Name() string  { return "nodejs" }
func (nodeDetector) Priority() int { return 20 }

func (nodeDetector) Detect(root string) *Detection {
	raw, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return nil
	}
	var pkg struct {
		Name            string            `json:"name"`
		Scripts         map[string]string `json:"scripts"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	_ = json.Unmarshal(raw, &pkg)

	deps := map[string]bool{}
	for k := range pkg.Dependencies {
		deps[k] = true
	}
	for k := range pkg.DevDependencies {
		deps[k] = true
	}

	var fw []string
	for _, c := range []string{"react", "express", "next", "jest", "vitest"} {
		if deps[c] {
			fw = append(fw, c)
		}
	}

	d := &Detection{
		Stack:       "nodejs",
		Frameworks:  fw,
		Sources:     []string{"package.json"},
		ProjectName: pkg.Name,
	}
	switch {
	case pkg.Scripts["test"] != "":
		d.Verify, d.VerifyFrom = "npm test", "package.json (scripts.test)"
	case deps["vitest"]:
		d.Verify, d.VerifyFrom = "vitest run", "package.json (vitest dependency)"
	case deps["jest"]:
		d.Verify, d.VerifyFrom = "jest", "package.json (jest dependency)"
	}
	return d
}

// ---- rust -----------------------------------------------------------------

type rustDetector struct{}

func (rustDetector) Name() string  { return "rust" }
func (rustDetector) Priority() int { return 30 }

func (rustDetector) Detect(root string) *Detection {
	path := filepath.Join(root, "Cargo.toml")
	if !FileExists(path) {
		return nil
	}
	c := readFileStr(path)
	return &Detection{
		Stack:       "rust",
		Frameworks:  scanFrameworks(c, []string{"actix-web", "axum", "tokio", "sqlx"}),
		Sources:     []string{"Cargo.toml"},
		ProjectName: tomlName(c),
		Verify:      "cargo test",
		VerifyFrom:  "Cargo.toml",
	}
}

// ---- go -------------------------------------------------------------------

type goDetector struct{}

func (goDetector) Name() string  { return "go" }
func (goDetector) Priority() int { return 30 }

func (goDetector) Detect(root string) *Detection {
	path := filepath.Join(root, "go.mod")
	if !FileExists(path) {
		return nil
	}
	c := readFileStr(path)
	name := ""
	if m := goModuleRe.FindStringSubmatch(c); len(m) == 2 {
		name = filepath.Base(m[1])
	}
	return &Detection{
		Stack:       "go",
		Frameworks:  scanFrameworks(c, []string{"gin", "echo", "fiber", "gorm"}),
		Sources:     []string{"go.mod"},
		ProjectName: name,
		Verify:      "go test ./...",
		VerifyFrom:  "go.mod",
	}
}

// ---- java -----------------------------------------------------------------

type javaDetector struct{}

func (javaDetector) Name() string  { return "java" }
func (javaDetector) Priority() int { return 35 }

func (javaDetector) Detect(root string) *Detection {
	maven := FileExists(filepath.Join(root, "pom.xml"))
	gradle := FileExists(filepath.Join(root, "build.gradle")) || FileExists(filepath.Join(root, "build.gradle.kts"))
	if !maven && !gradle {
		return nil
	}
	var sources []string
	var combined strings.Builder
	for _, f := range []string{"pom.xml", "build.gradle", "build.gradle.kts"} {
		if FileExists(filepath.Join(root, f)) {
			sources = append(sources, f)
			combined.WriteString(readFileStr(filepath.Join(root, f)))
			combined.WriteString("\n")
		}
	}
	d := &Detection{
		Stack:      "java",
		Frameworks: scanFrameworks(combined.String(), []string{"spring", "junit", "quarkus", "micronaut"}),
		Sources:    sources,
	}
	if maven {
		d.Verify, d.VerifyFrom = "mvn test", "pom.xml"
	} else {
		d.Verify, d.VerifyFrom = "gradle test", "build.gradle"
	}
	return d
}

// ---- ruby -----------------------------------------------------------------

type rubyDetector struct{}

func (rubyDetector) Name() string  { return "ruby" }
func (rubyDetector) Priority() int { return 25 }

func (rubyDetector) Detect(root string) *Detection {
	path := filepath.Join(root, "Gemfile")
	if !FileExists(path) {
		return nil
	}
	c := readFileStr(path)
	fw := scanFrameworks(c, []string{"rails", "sinatra", "rspec", "minitest"})
	d := &Detection{
		Stack:      "ruby",
		Frameworks: fw,
		Sources:    []string{"Gemfile"},
	}
	if contains(fw, "rspec") {
		d.Verify, d.VerifyFrom = "bundle exec rspec", "rspec in Gemfile"
	} else {
		d.Verify, d.VerifyFrom = "rake test", "Gemfile"
	}
	return d
}

// ---- php ------------------------------------------------------------------

type phpDetector struct{}

func (phpDetector) Name() string  { return "php" }
func (phpDetector) Priority() int { return 25 }

func (phpDetector) Detect(root string) *Detection {
	raw, err := os.ReadFile(filepath.Join(root, "composer.json"))
	if err != nil {
		return nil
	}
	var pkg struct {
		Name       string            `json:"name"`
		Scripts    map[string]any    `json:"scripts"`
		Require    map[string]string `json:"require"`
		RequireDev map[string]string `json:"require-dev"`
	}
	_ = json.Unmarshal(raw, &pkg)

	deps := map[string]bool{}
	for k := range pkg.Require {
		deps[k] = true
	}
	for k := range pkg.RequireDev {
		deps[k] = true
	}
	has := func(needle string) bool {
		for k := range deps {
			if strings.Contains(k, needle) {
				return true
			}
		}
		return false
	}

	var fw []string
	for _, c := range []string{"laravel", "symfony", "phpunit"} {
		if has(c) {
			fw = append(fw, c)
		}
	}
	d := &Detection{
		Stack:       "php",
		Frameworks:  fw,
		Sources:     []string{"composer.json"},
		ProjectName: pkg.Name,
	}
	switch {
	case has("phpunit"):
		d.Verify, d.VerifyFrom = "vendor/bin/phpunit", "phpunit in composer.json"
	case pkg.Scripts["test"] != nil:
		d.Verify, d.VerifyFrom = "composer test", "composer.json (scripts.test)"
	}
	return d
}

// ---- elixir ---------------------------------------------------------------

type elixirDetector struct{}

func (elixirDetector) Name() string  { return "elixir" }
func (elixirDetector) Priority() int { return 25 }

var mixAppRe = regexp.MustCompile(`app:\s*:(\w+)`)

func (elixirDetector) Detect(root string) *Detection {
	path := filepath.Join(root, "mix.exs")
	if !FileExists(path) {
		return nil
	}
	c := readFileStr(path)
	name := ""
	if m := mixAppRe.FindStringSubmatch(c); len(m) == 2 {
		name = m[1]
	}
	return &Detection{
		Stack:       "elixir",
		Frameworks:  scanFrameworks(c, []string{"phoenix", "ecto"}),
		Sources:     []string{"mix.exs"},
		ProjectName: name,
		Verify:      "mix test",
		VerifyFrom:  "mix.exs",
	}
}
