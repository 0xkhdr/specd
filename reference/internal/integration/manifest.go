package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

const ManifestVersion = 1

type Manifest struct {
	Version int             `json:"version"`
	Entries []ManifestEntry `json:"entries"`
}

type ManifestEntry struct {
	Host         string `json:"host"`
	Scope        Scope  `json:"scope"`
	ServerName   string `json:"serverName"`
	Root         string `json:"root"`
	RootStrategy string `json:"rootStrategy"`
	Method       string `json:"method"`
	Target       string `json:"target"`
	Fingerprint  string `json:"fingerprint"`
	InstalledAt  string `json:"installedAt,omitempty"`
}

func NewManifest() Manifest {
	return Manifest{Version: ManifestVersion, Entries: []ManifestEntry{}}
}

func Fingerprint(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NewManifest(), nil
	}
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse integration manifest: %w", err)
	}
	if manifest.Version != ManifestVersion {
		return Manifest{}, fmt.Errorf("unsupported integration manifest version %d", manifest.Version)
	}
	if manifest.Entries == nil {
		manifest.Entries = []ManifestEntry{}
	}
	if err := validateManifest(manifest); err != nil {
		return Manifest{}, err
	}
	normalizeManifest(&manifest)
	return manifest, nil
}

func SaveManifest(path string, manifest Manifest) error {
	if manifest.Version == 0 {
		manifest.Version = ManifestVersion
	}
	if manifest.Version != ManifestVersion {
		return fmt.Errorf("unsupported integration manifest version %d", manifest.Version)
	}
	if manifest.Entries == nil {
		manifest.Entries = []ManifestEntry{}
	}
	if err := validateManifest(manifest); err != nil {
		return err
	}
	normalizeManifest(&manifest)
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return core.AtomicWrite(path, string(data)+"\n")
}

func VerifyOwnership(entry ManifestEntry, content []byte) error {
	if entry.Fingerprint == "" {
		return fmt.Errorf("integration %s/%s has no fingerprint", entry.Host, entry.ServerName)
	}
	actual := Fingerprint(content)
	if actual != entry.Fingerprint {
		return fmt.Errorf("integration ownership mismatch: fingerprint %s, want %s", actual, entry.Fingerprint)
	}
	return nil
}

func validateManifest(manifest Manifest) error {
	seen := map[string]bool{}
	for _, entry := range manifest.Entries {
		if entry.Host == "" || entry.ServerName == "" || entry.Target == "" || entry.Method == "" {
			return fmt.Errorf("integration manifest entry requires host, serverName, method, and target")
		}
		if entry.Scope != ScopeProject && entry.Scope != ScopeGlobal {
			return fmt.Errorf("integration %s/%s has invalid scope %q", entry.Host, entry.ServerName, entry.Scope)
		}
		if strings.ContainsRune(entry.Root, '\x00') || strings.ContainsRune(entry.Target, '\x00') {
			return fmt.Errorf("integration %s/%s contains NUL in path", entry.Host, entry.ServerName)
		}
		if entry.Fingerprint != "" {
			raw := strings.TrimPrefix(entry.Fingerprint, "sha256:")
			if !strings.HasPrefix(entry.Fingerprint, "sha256:") || len(raw) != sha256.Size*2 {
				return fmt.Errorf("integration %s/%s has invalid fingerprint", entry.Host, entry.ServerName)
			}
			if _, err := hex.DecodeString(raw); err != nil {
				return fmt.Errorf("integration %s/%s has invalid fingerprint", entry.Host, entry.ServerName)
			}
		}
		key := string(entry.Scope) + "\x00" + entry.Host + "\x00" + entry.ServerName
		if seen[key] {
			return fmt.Errorf("duplicate integration manifest entry for %s/%s", entry.Host, entry.ServerName)
		}
		seen[key] = true
	}
	return nil
}

func normalizeManifest(manifest *Manifest) {
	sort.Slice(manifest.Entries, func(i, j int) bool {
		a, b := manifest.Entries[i], manifest.Entries[j]
		if a.Host != b.Host {
			return a.Host < b.Host
		}
		if a.Scope != b.Scope {
			return a.Scope < b.Scope
		}
		if a.ServerName != b.ServerName {
			return a.ServerName < b.ServerName
		}
		return a.Target < b.Target
	})
}
