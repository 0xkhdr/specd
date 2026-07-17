package security

import (
	"path"
	"strings"
)

// DeclaredDep is the operator's approved reason/source for one dependency,
// declared out of band (e.g. .specd/security/dependencies.json). Both fields
// are required — a dependency without a stated reason and a stated source is
// treated as unapproved (R6.1).
type DeclaredDep struct {
	Reason string `json:"reason"`
	Source string `json:"source"`
}

// DependencyPolicy governs the manifest-diff dependency gate. Profile selects
// finding severity (production → error, otherwise warn). RegistryAllowlist lists
// approved module path prefixes; empty means the host check is not enforced.
// Declared is the approved baseline — a dependency absent here is "new" and must
// carry a reason/source before it is allowed.
type DependencyPolicy struct {
	Profile           string
	RegistryAllowlist []string
	Declared          map[string]DeclaredDep
}

// manifestScanner inspects dependency manifests (go.mod) and lockfiles (go.sum)
// against the declared baseline. Unlike slopsquat it does NOT exclude lockfiles:
// a go.sum-only change is still inspected (R6.1).
type manifestScanner struct{ policy DependencyPolicy }

func (manifestScanner) Name() string { return "manifest" }

func (manifestScanner) Exclude(input ScanInputV1) bool {
	base := path.Base(strings.ReplaceAll(input.Path, "\\", "/"))
	return base != "go.mod" && base != "go.sum"
}

// ScanManifest runs the dependency policy over the given manifests/lockfiles.
func ScanManifest(files []TrackedFile, policy DependencyPolicy) []Finding {
	return manifestScanner{policy: policy}.Scan(files)
}

func (s manifestScanner) Scan(files []ScanInputV1) []Finding {
	sev := profileSeverity(s.policy.Profile)
	var findings []Finding
	for _, file := range files {
		if s.Exclude(file) {
			continue
		}
		if strings.HasSuffix(file.Path, "go.sum") {
			findings = append(findings, s.scanLockfile(file, sev)...)
			continue
		}
		findings = append(findings, s.scanModule(file, sev)...)
	}
	return findings
}

// scanModule requires a declared reason/source for every dependency and an
// allowlisted registry host when the allowlist is set.
func (s manifestScanner) scanModule(file ScanInputV1, sev string) []Finding {
	var findings []Finding
	for _, dep := range parseGoMod(string(file.Content)) {
		decl, ok := s.policy.Declared[dep.path]
		if !ok || strings.TrimSpace(decl.Reason) == "" || strings.TrimSpace(decl.Source) == "" {
			findings = append(findings, Finding{
				Scanner:     "manifest",
				Rule:        "require-reason",
				File:        file.Path,
				Line:        dep.line,
				Severity:    sev,
				Fingerprint: fingerprint("require-reason", file.Path, dep.path),
				Excerpt:     dep.path + " added without declared reason/source",
			})
		}
		if !registryAllowed(dep.path, s.policy.RegistryAllowlist) {
			findings = append(findings, Finding{
				Scanner:     "manifest",
				Rule:        "unknown-registry",
				File:        file.Path,
				Line:        dep.line,
				Severity:    sev,
				Fingerprint: fingerprint("unknown-registry", file.Path, dep.path),
				Excerpt:     dep.path + " not in registry allowlist",
			})
		}
	}
	return findings
}

// scanLockfile validates that every go.sum checksum uses the pinned h1:
// algorithm; an unknown algorithm is an unverifiable provenance claim (R6.1).
func (s manifestScanner) scanLockfile(file ScanInputV1, sev string) []Finding {
	var findings []Finding
	for i, raw := range strings.Split(string(file.Content), "\n") {
		fields := strings.Fields(raw)
		if len(fields) != 3 {
			continue // blank / malformed spacing — not a checksum claim
		}
		if !strings.HasPrefix(fields[2], "h1:") {
			findings = append(findings, Finding{
				Scanner:     "manifest",
				Rule:        "unknown-checksum",
				File:        file.Path,
				Line:        i + 1,
				Severity:    sev,
				Fingerprint: fingerprint("unknown-checksum", file.Path, fields[0]+" "+fields[1]),
				Excerpt:     fields[0] + " " + fields[1] + " uses unknown checksum algorithm",
			})
		}
	}
	return findings
}

// registryAllowed reports whether dep matches any allowed module path prefix.
// An empty allowlist disables the host check (opt-in default).
func registryAllowed(dep string, allowlist []string) bool {
	if len(allowlist) == 0 {
		return true
	}
	for _, a := range allowlist {
		if strings.HasPrefix(dep, a) {
			return true
		}
	}
	return false
}

// profileSeverity maps a security profile to the severity new governance
// findings carry: production fails closed at error, everything else warns.
func profileSeverity(profile string) string {
	if profile == "production" {
		return "error"
	}
	return "warn"
}
