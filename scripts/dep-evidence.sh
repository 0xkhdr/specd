#!/usr/bin/env sh
# dep-evidence.sh — offline dependency-evidence adapter (Domain 06, R6.2).
#
# Emits a pinned `dep-evidence/v1` artifact for the current manifest state. The
# security gate never touches the network: this adapter runs out of band and its
# output is validated by security.ScanDepEvidence. A production adapter attaches
# advisory findings from a LOCALLY PINNED database; this baseline emits a
# well-formed, digest-pinned artifact with no findings so the offline path is
# exercised deterministically.
#
# Usage: scripts/dep-evidence.sh [repo-root] > .specd/security/dep-evidence.json
set -eu

root="${1:-.}"

# Digest must match security.ManifestDigest: sha256 of go.mod then go.sum bytes.
digest=$(cat "$root/go.mod" "$root/go.sum" 2>/dev/null | sha256sum | cut -d' ' -f1)
generated_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)

printf '{"schema":"dep-evidence/v1","generated_at":"%s","manifest_digest":"%s","findings":[]}\n' \
	"$generated_at" "$digest"
