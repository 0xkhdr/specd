package security

import "testing"

// FuzzParseAllowlist proves the allowlist parser never panics on hostile input
// (constitution invariant 7: fuzz every new parser). A malformed document must
// return an error, never crash.
func FuzzParseAllowlist(f *testing.F) {
	f.Add(`{"allow":[{"value":"x","reason":"y"}]}`)
	f.Add(`{"allow":[{"value":"x"}]}`)
	f.Add(``)
	f.Add(`{`)
	f.Add(`{"allow":null}`)
	f.Add(`{"unknown":1}`)
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = ParseAllowlist([]byte(s))
	})
}

// FuzzScanners proves every scanner is panic-safe over arbitrary file contents
// and manifest bytes.
func FuzzScanners(f *testing.F) {
	f.Add("package.json", "{\"dependencies\":{\"react\":\"1\"}}")
	f.Add("requirements.txt", "requests==2.0\n-r other.txt")
	f.Add("go.mod", "require (\ngithub.com/x/y v1.0.0\n)")
	f.Add("app.py", `password = "abc"`)
	f.Add("q.go", `db.Query("SELECT * FROM t WHERE x=" + v)`)
	f.Fuzz(func(t *testing.T, path, content string) {
		files := []ChangedFile{{Path: path, Content: content}}
		_ = Scan(Config{Secrets: "error", Injection: "error", Slopsquat: "error"}, files, Allowlist{})
		_ = parsePackageJSON(content)
		_ = parseRequirements(content)
		_ = parseGoMod(content)
	})
}
