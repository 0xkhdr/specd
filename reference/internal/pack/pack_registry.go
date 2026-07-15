package pack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// The pack registry (V11/P6.3) lets a named pack resolve without a hosted
// service: the registry index is itself a git repository holding a single
// registry.json that maps a pack name to a pinned {url, sha256}. `specd init
// --pack <name> --registry <git-url>` clones the index over the hardened
// git-exec path (core.SecureGitClone: scrubbed env, transport allowlist, URL
// validation), looks up the name, and resolves the referenced pack with the same
// fail-closed SHA256 verification as a direct remote --pack. Every resolution is
// pinned into .specd/pack.lock; a later resolution whose content hash disagrees
// with the lock is a hard failure — the supply-chain guard against a mutated
// registry silently swapping a pack's bytes.

const (
	registryIndexName = "registry.json"
	packLockName      = "pack.lock"
)

// RegistryEntry is one pack's pinned coordinates in the registry index.
type RegistryEntry struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// RegistryIndex is the parsed registry.json carried by the registry git repo.
type RegistryIndex struct {
	Packs []RegistryEntry `json:"packs"`
}

// ParseRegistryIndex decodes and validates a registry index. It fails closed on
// unknown fields, empty names/URLs, malformed digests, and duplicate names.
func ParseRegistryIndex(raw []byte) (RegistryIndex, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var idx RegistryIndex
	if err := dec.Decode(&idx); err != nil {
		return RegistryIndex{}, core.GateError(fmt.Sprintf("invalid pack registry index: %v", err))
	}
	seen := map[string]bool{}
	for _, e := range idx.Packs {
		if strings.TrimSpace(e.Name) == "" {
			return RegistryIndex{}, core.GateError("pack registry entry missing name")
		}
		if strings.TrimSpace(e.URL) == "" {
			return RegistryIndex{}, core.GateError(fmt.Sprintf("pack registry entry %q missing url", e.Name))
		}
		if len(e.SHA256) != 64 || strings.ToLower(e.SHA256) != e.SHA256 {
			return RegistryIndex{}, core.GateError(fmt.Sprintf("pack registry entry %q has a malformed sha256", e.Name))
		}
		if seen[e.Name] {
			return RegistryIndex{}, core.GateError(fmt.Sprintf("pack registry declares duplicate name %q", e.Name))
		}
		seen[e.Name] = true
	}
	return idx, nil
}

// LookupRegistryEntry finds a pack by name in the index.
func (idx RegistryIndex) LookupRegistryEntry(name string) (RegistryEntry, error) {
	for _, e := range idx.Packs {
		if e.Name == name {
			return e, nil
		}
	}
	var names []string
	for _, e := range idx.Packs {
		names = append(names, e.Name)
	}
	sort.Strings(names)
	return RegistryEntry{}, core.NotFoundError(fmt.Sprintf("pack %q not found in registry (available: %s)", name, strings.Join(names, ", ")))
}

// ResolveFromRegistry clones the registry git repo, looks up name, and resolves
// the referenced pack with fail-closed SHA256 verification. It returns the parsed
// pack and the resolved entry (whose SHA256 the caller pins into the lockfile).
func ResolveFromRegistry(name, registryURL string) (*Pack, RegistryEntry, error) {
	work, err := os.MkdirTemp("", "specd-pack-registry-*")
	if err != nil {
		return nil, RegistryEntry{}, err
	}
	defer os.RemoveAll(work)
	if err := core.SecureGitClone(registryURL, work, true); err != nil {
		return nil, RegistryEntry{}, err
	}
	raw, err := os.ReadFile(filepath.Join(work, registryIndexName))
	if err != nil {
		return nil, RegistryEntry{}, core.GateError("pack registry: repo has no registry.json index")
	}
	idx, err := ParseRegistryIndex(raw)
	if err != nil {
		return nil, RegistryEntry{}, err
	}
	entry, err := idx.LookupRegistryEntry(name)
	if err != nil {
		return nil, RegistryEntry{}, err
	}
	pk, err := resolveRegistryPack(entry)
	if err != nil {
		return nil, RegistryEntry{}, err
	}
	return pk, entry, nil
}

// resolveRegistryPack fetches and verifies the pack a registry entry points to.
// The entry URL may be an http(s) manifest (fetched + hashed) or a file path to
// a local manifest (for hermetic tests and air-gapped mirrors); either way the
// pinned SHA256 must match before the bytes are parsed.
func resolveRegistryPack(entry RegistryEntry) (*Pack, error) {
	var raw []byte
	switch {
	case strings.HasPrefix(entry.URL, "http://"), strings.HasPrefix(entry.URL, "https://"):
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(entry.URL)
		if err != nil {
			return nil, core.GateError(fmt.Sprintf("fetch pack %q: %v", entry.URL, err))
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, core.GateError(fmt.Sprintf("fetch pack %q: HTTP %d", entry.URL, resp.StatusCode))
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxPackBytes+1))
		if err != nil {
			return nil, core.GateError(fmt.Sprintf("read pack %q: %v", entry.URL, err))
		}
		if len(body) > maxPackBytes {
			return nil, core.GateError(fmt.Sprintf("pack %q exceeds the %d-byte limit", entry.URL, maxPackBytes))
		}
		raw = body
	case strings.HasPrefix(entry.URL, "file://"):
		p := strings.TrimPrefix(entry.URL, "file://")
		body, err := os.ReadFile(p)
		if err != nil {
			return nil, core.GateError(fmt.Sprintf("read pack %q: %v", entry.URL, err))
		}
		raw = body
	default:
		return nil, core.GateError(fmt.Sprintf("pack registry entry %q has an unsupported url scheme", entry.Name))
	}
	return VerifyAndParsePack(raw, entry.SHA256, entry.URL)
}

// PackLock is the checksum lockfile: a name→sha256 map recording every pack a
// project has resolved. It is the pin that turns a mutated registry into a hard
// failure rather than a silent swap.
type PackLock struct {
	Packs map[string]string `json:"packs"`
}

func packLockPath(root string) string {
	return filepath.Join(root, ".specd", packLockName)
}

// LoadPackLock reads the project's pack lockfile, returning an empty lock when
// none exists yet.
func LoadPackLock(root string) (PackLock, error) {
	raw, err := os.ReadFile(packLockPath(root))
	if err != nil {
		if os.IsNotExist(err) {
			return PackLock{Packs: map[string]string{}}, nil
		}
		return PackLock{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var l PackLock
	if err := dec.Decode(&l); err != nil {
		return PackLock{}, core.GateError(fmt.Sprintf("invalid pack lockfile: %v", err))
	}
	if l.Packs == nil {
		l.Packs = map[string]string{}
	}
	return l, nil
}

// CheckAndPin verifies name against any previously locked digest and records the
// new one. A disagreement is a hard failure (the registry changed a pack's bytes
// under a stable name); a first sighting is pinned.
func (l *PackLock) CheckAndPin(name, sha256 string) error {
	if l.Packs == nil {
		l.Packs = map[string]string{}
	}
	if prior, ok := l.Packs[name]; ok && prior != sha256 {
		return core.GateError(fmt.Sprintf(
			"pack %q lock mismatch: pinned %s but registry now offers %s — refusing (registry content changed)",
			name, prior, sha256))
	}
	l.Packs[name] = sha256
	return nil
}

// Save writes the lockfile deterministically (sorted keys via json map ordering).
func (l PackLock) Save(root string) error {
	out, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	return core.AtomicWrite(packLockPath(root), string(out)+"\n")
}
