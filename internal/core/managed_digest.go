package core

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ManagedDigest returns one stable identity for managed AGENTS.md, role, and
// steering regions. User-owned bytes outside managed markers are excluded.
// Missing or malformed regions remain part of the identity as "<missing>" so
// bootstrap pinning detects drift without preventing an operator from reading
// the current packet and repairing it.
func ManagedDigest(root string) (string, error) {
	assets, err := ManagedAssets()
	if err != nil {
		return "", err
	}
	entries := make([]string, 0, len(assets)+1)
	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	entries = append(entries, "AGENTS.md\x00"+markedRegion(string(agents), agentsBegin, agentsEnd))
	for _, asset := range assets {
		raw, readErr := os.ReadFile(filepath.Join(root, asset.RelPath))
		if readErr != nil && !os.IsNotExist(readErr) {
			return "", readErr
		}
		entries = append(entries, filepath.ToSlash(asset.RelPath)+"\x00"+markedRegion(string(raw), managedBegin(asset.Name, asset.Version), managedEnd(asset.Name, asset.Version)))
	}
	sort.Strings(entries)
	return Digest([]byte(strings.Join(entries, "\x00"))), nil
}

func markedRegion(content, begin, end string) string {
	start := strings.Index(content, begin)
	if start < 0 {
		return "<missing>"
	}
	finish := strings.Index(content[start+len(begin):], end)
	if finish < 0 {
		return "<missing>"
	}
	finish += start + len(begin) + len(end)
	return content[start:finish]
}
