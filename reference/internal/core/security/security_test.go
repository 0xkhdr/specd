package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// positiveCorpus are inputs a competent security gate MUST catch. The >90%
// catch-rate acceptance target (spec §6) is asserted against this set.
var positiveCorpus = []struct {
	name string
	cfg  Config
	file ChangedFile
}{
	{"aws key", Config{Secrets: "error"}, ChangedFile{"app.py", `aws_key = "AKIA` + `IOSFODNN7EXAMPLE"`}},
	{"github token", Config{Secrets: "error"}, ChangedFile{"cfg.env", `TOKEN=ghp_` + `012345678901234567890123456789012345`}},
	{"private key pem", Config{Secrets: "error"}, ChangedFile{"id_rsa", "-----BEGIN RSA PRIVATE KEY-----\nMIIabc\n-----END RSA PRIVATE KEY-----"}},
	{"jwt", Config{Secrets: "error"}, ChangedFile{"t.txt", `auth: eyJ` + `hbGciOiJI.eyJzdWIiOiIxMjM0.SflKxwRJSMeKKF2QT4`}},
	{"slack token", Config{Secrets: "error"}, ChangedFile{"s.txt", `hook = xox` + `b-1234567890-abcdefghijklmno`}},
	{"high entropy password", Config{Secrets: "error"}, ChangedFile{"c.yml", `password: 9f8K` + `q2Lm7Xz4Vb1Np6Rw3Ty5`}},
	{"sql concat go", Config{Injection: "error"}, ChangedFile{"q.go", "db.Query(\"SELECT * FROM users WHERE id = \" + id)"}},
	{"sql concat python fstring", Config{Injection: "error"}, ChangedFile{"q.py", `cur.execute(f"SELECT * FROM t WHERE x={v}")`}},
	{"exec interpolation", Config{Injection: "error"}, ChangedFile{"r.py", `os.system("rm -rf " + path)`}},
	{"npm typosquat", Config{Slopsquat: "error"}, ChangedFile{"package.json", "{\n\"dependencies\": {\n\"reactt\": \"1.0.0\"\n}\n}"}},
	{"pypi typosquat", Config{Slopsquat: "error"}, ChangedFile{"requirements.txt", "reqeusts==2.0.0"}},
	{"go typosquat", Config{Slopsquat: "error"}, ChangedFile{"go.mod", "require (\ngithub.com/stretchr/testfiy v1.0.0\n)"}},
}

func TestSecurityCatchRate(t *testing.T) {
	caught := 0
	for _, tc := range positiveCorpus {
		findings := Scan(tc.cfg, []ChangedFile{tc.file}, Allowlist{})
		if len(findings) > 0 {
			caught++
		} else {
			t.Logf("MISS: %s", tc.name)
		}
	}
	rate := float64(caught) / float64(len(positiveCorpus))
	if rate < 0.90 {
		t.Fatalf("catch rate %.2f (%d/%d) below 0.90 target", rate, caught, len(positiveCorpus))
	}
}

// negativeCorpus are benign inputs that MUST NOT fire (false-positive control).
var negativeCorpus = []struct {
	name string
	cfg  Config
	file ChangedFile
}{
	{"prose", Config{Secrets: "error", Injection: "error", Slopsquat: "error"}, ChangedFile{"README.md", "This project selects data from the users table."}},
	{"env placeholder", Config{Secrets: "error"}, ChangedFile{"cfg", `password: ${DB_PASSWORD}`}},
	{"example password", Config{Secrets: "error"}, ChangedFile{"cfg", `password: your-password-changeme-example`}},
	{"parameterized sql", Config{Injection: "error"}, ChangedFile{"q.go", `db.Query("SELECT * FROM users WHERE id = ?", id)`}},
	{"exact popular dep", Config{Slopsquat: "error"}, ChangedFile{"requirements.txt", "requests==2.31.0"}},
	{"low entropy assignment", Config{Secrets: "error"}, ChangedFile{"cfg", `token = aaaaaaaaaaaaaaaaaa`}},
}

func TestSecurityNoFalsePositives(t *testing.T) {
	for _, tc := range negativeCorpus {
		if f := Scan(tc.cfg, []ChangedFile{tc.file}, Allowlist{}); len(f) > 0 {
			t.Errorf("%s: unexpected findings %+v", tc.name, f)
		}
	}
}

// TestAllowlistSuppressesAndRequiresReason proves an allowlisted value is
// suppressed and a reasonless entry is rejected.
func TestAllowlistSuppressesAndRequiresReason(t *testing.T) {
	file := ChangedFile{"app.py", `aws_key = "AKIAIOSFODNN7EXAMPLE"`}
	if f := Scan(Config{Secrets: "error"}, []ChangedFile{file}, Allowlist{}); len(f) == 0 {
		t.Fatal("expected a finding without allowlist")
	}
	allow, err := ParseAllowlist([]byte(`{"allow":[{"value":"AKIAIOSFODNN7EXAMPLE","reason":"documented example key in fixtures"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if f := Scan(Config{Secrets: "error"}, []ChangedFile{file}, allow); len(f) != 0 {
		t.Fatalf("allowlisted value still flagged: %+v", f)
	}
	if _, err := ParseAllowlist([]byte(`{"allow":[{"value":"x"}]}`)); err == nil {
		t.Fatal("expected error for reasonless allowlist entry")
	}
}

// TestZeroFindingsOnOwnSource proves the suite is quiet on this repo's real Go
// source — the false-positive control at scale (spec §6). The security package's
// own pattern-definition files are excluded: they intentionally embed the very
// patterns the scanners look for.
func TestZeroFindingsOnOwnSource(t *testing.T) {
	root := repoRoot(t)
	var files []ChangedFile
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "testdata" || base == "fixtures" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		// Exclude the scanner definition files (they embed the patterns by design).
		if strings.Contains(filepath.ToSlash(path), "internal/core/security/") {
			return nil
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, _ := filepath.Rel(root, path)
		files = append(files, ChangedFile{Path: rel, Content: string(b)})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no source files scanned")
	}
	findings := Scan(Config{Secrets: "error", Injection: "warn", Slopsquat: "warn"}, files, Allowlist{})
	if len(findings) > 0 {
		for _, f := range findings {
			t.Errorf("false positive: %s:%d [%s] %s", f.File, f.Line, f.Rule, f.Message)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("repo root (go.mod) not found")
	return ""
}
