package pack

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// maxPackBytes bounds a remote pack download so a hostile or runaway endpoint
// cannot exhaust memory before the SHA256 pin is even checked.
const maxPackBytes = 1 << 20 // 1 MiB

// ResolvePack resolves a pack reference to a validated Pack. A bare name (no
// scheme) resolves against the embedded built-in packs. An http(s) URL is a
// remote pack and MUST carry a pinned sha256 digest: the bytes are downloaded,
// hashed, and compared before parsing — on any mismatch nothing is returned
// (fail-closed), mirroring `specd update`'s SHA256SUMS contract. Either way the
// manifest passes ParsePack, so a resolved pack is always declarative-only and
// path-safe.
func ResolvePack(ref, sha256Pin string) (*Pack, error) {
	if isRemoteRef(ref) {
		return resolveRemotePack(ref, sha256Pin)
	}
	if strings.TrimSpace(sha256Pin) != "" {
		return nil, core.GateError("--sha256 is only meaningful for a remote (http/https) pack reference")
	}
	return BuiltinPack(ref)
}

func isRemoteRef(ref string) bool {
	return strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://")
}

func resolveRemotePack(url, sha256Pin string) (*Pack, error) {
	pin := strings.ToLower(strings.TrimSpace(sha256Pin))
	if pin == "" {
		return nil, core.GateError(fmt.Sprintf("remote pack %q requires a pinned --sha256 digest — refusing to fetch unpinned", url))
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, core.GateError(fmt.Sprintf("fetch pack %q: %v", url, err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, core.GateError(fmt.Sprintf("fetch pack %q: HTTP %d", url, resp.StatusCode))
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxPackBytes+1))
	if err != nil {
		return nil, core.GateError(fmt.Sprintf("read pack %q: %v", url, err))
	}
	if len(raw) > maxPackBytes {
		return nil, core.GateError(fmt.Sprintf("pack %q exceeds the %d-byte limit", url, maxPackBytes))
	}

	return VerifyAndParsePack(raw, pin, url)
}

// VerifyAndParsePack checks raw bytes against a pinned SHA256 digest and, only
// on an exact match, parses them as a pack manifest. A digest mismatch returns
// an error and no pack — the caller must write nothing. Exposed for direct
// testing of the fail-closed contract without a network round-trip.
func VerifyAndParsePack(raw []byte, sha256Pin, source string) (*Pack, error) {
	want := strings.ToLower(strings.TrimSpace(sha256Pin))
	sum := sha256.Sum256(raw)
	got := hex.EncodeToString(sum[:])
	if got != want {
		return nil, core.GateError(fmt.Sprintf("pack %q SHA256 mismatch: got %s, pinned %s — refusing to apply", source, got, want))
	}
	return ParsePack(raw)
}
