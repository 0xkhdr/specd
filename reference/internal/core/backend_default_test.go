package core

import "testing"

// TestDefaultLinksNoDriver is the integrity guarantee for the build-tag
// adapters: in a default build no optional (driver-backed) backend is compiled
// in, so the binary links no redis/postgres driver. The registry being empty is
// the in-code proxy for `go list -deps` showing no driver module; go.mod itself
// declares zero external dependencies, so a driver could only enter via a build
// tag — which the default build does not set.
func TestDefaultLinksNoDriver(t *testing.T) {
	if len(optionalBackends) != 0 {
		t.Errorf("default build registered optional backends %v — a driver leaked into the default binary", optionalBackends)
	}
	// file and git are always available without any driver.
	for _, name := range []string{"", "file", "git"} {
		if _, err := SelectBackend(name); err != nil {
			t.Errorf("SelectBackend(%q): %v", name, err)
		}
	}
	// redis/postgres are not compiled in by default → fail closed.
	for _, name := range []string{"redis", "postgres"} {
		if _, err := SelectBackend(name); err == nil {
			t.Errorf("SelectBackend(%q) should fail closed in a default build", name)
		}
	}
}
