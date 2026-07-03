package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

// uriFromParams extracts the "uri" field from a resources/read params object.
func uriFromParams(raw json.RawMessage) string {
	var p struct {
		URI string `json:"uri"`
	}
	_ = json.Unmarshal(raw, &p)
	return p.URI
}

// MCP Resources (spec B1). Spec artifacts and steering files reach a host through
// the native `resources` channel instead of a `specd_context` tool call. The
// channel is strictly read-only and strictly contained: only files under
// `.specd/` are addressable, and any URI whose resolved path escapes that root is
// rejected before a single byte is read (spec R6/R8).

// errResourceNotFound is the JSON-RPC error code returned for an unknown URI or a
// rejected (out-of-tree) path. The same code covers both so a traversal probe is
// indistinguishable from a genuine miss — no filesystem disclosure (spec R5).
const errResourceNotFound = -32002

// resourceURIScheme prefixes every specd resource URI (spec R3).
const resourceURIScheme = "specd://"

// artifactResources is the fixed, deterministic per-spec artifact order used by
// resources/list (spec R7). It is the `specd new` artifact set plus the runtime
// state file, all of which live directly under a spec directory.
var artifactResources = append(append([]string{}, core.Artifacts...), "state.json")

// handleResourcesList enumerates every existing spec artifact and steering file
// as a resource entry (spec R2). Ordering is deterministic: specs in slug order,
// each spec's artifacts in artifactResources order, then steering files sorted by
// name. Only files that exist on disk are emitted.
func handleResourcesList(root string) map[string]any {
	resources := make([]map[string]any, 0, 16)
	if root != "" {
		for _, slug := range core.ListSpecs(root) {
			for _, name := range artifactResources {
				path := core.ArtifactPath(root, slug, name)
				if fileExists(path) {
					resources = append(resources, resourceEntry(
						resourceURIScheme+"specs/"+slug+"/"+name, slug+"/"+name, name))
				}
			}
		}
		for _, name := range steeringFiles(root) {
			resources = append(resources, resourceEntry(
				resourceURIScheme+"steering/"+name, "steering/"+name, name))
		}
	}
	return map[string]any{"resources": resources}
}

// handleResourceRead resolves a known URI to file content with the matching mime
// type (spec R4). Unknown URIs, traversal attempts, and missing files all yield a
// resource-not-found error with no filesystem detail (spec R5/R6).
func handleResourceRead(root, uri string) (map[string]any, *rpcError) {
	path, name, ok := resolveResourceURI(root, uri)
	if !ok {
		return nil, &rpcError{Code: errResourceNotFound, Message: "resource not found: " + uri}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &rpcError{Code: errResourceNotFound, Message: "resource not found: " + uri}
	}
	return map[string]any{
		"contents": []map[string]any{{
			"uri":      uri,
			"mimeType": mimeForName(name),
			"text":     string(data),
		}},
	}, nil
}

// resolveResourceURI maps a specd:// URI to a contained on-disk path. It returns
// ok=false for an unknown scheme/shape, an invalid slug or filename, or any path
// that escapes `.specd/` (spec R6). The returned name is the bare filename, used
// to infer the mime type.
func resolveResourceURI(root, uri string) (path, name string, ok bool) {
	if root == "" {
		return "", "", false
	}
	rest, found := strings.CutPrefix(uri, resourceURIScheme)
	if !found {
		return "", "", false
	}
	parts := strings.Split(rest, "/")
	switch {
	case len(parts) == 3 && parts[0] == "specs":
		slug, file := parts[1], parts[2]
		if core.ValidateSlug(slug) != nil || !isArtifactResource(file) {
			return "", "", false
		}
		path = core.ArtifactPath(root, slug, file)
		name = file
	case len(parts) == 2 && parts[0] == "steering":
		file := parts[1]
		if !isSafeResourceFile(file) || !strings.HasSuffix(file, ".md") {
			return "", "", false
		}
		path = filepath.Join(core.SteeringDir(root), file)
		name = file
	default:
		return "", "", false
	}
	if !withinSpecd(root, path) {
		return "", "", false
	}
	return path, name, true
}

// withinSpecd verifies a resolved path stays inside the project's `.specd/`
// directory after cleaning (defence-in-depth against traversal, spec R6).
func withinSpecd(root, path string) bool {
	specd := filepath.Clean(core.SpecdDir(root))
	clean := filepath.Clean(path)
	return clean == specd || strings.HasPrefix(clean, specd+string(os.PathSeparator))
}

// steeringFiles lists the steering directory's markdown files in sorted order.
func steeringFiles(root string) []string {
	entries, err := os.ReadDir(core.SteeringDir(root))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

func resourceEntry(uri, name, file string) map[string]any {
	return map[string]any{"uri": uri, "name": name, "mimeType": mimeForName(file)}
}

func isArtifactResource(name string) bool {
	for _, a := range artifactResources {
		if a == name {
			return true
		}
	}
	return false
}

// isSafeResourceFile rejects path separators and traversal in a bare filename.
func isSafeResourceFile(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	return !strings.ContainsAny(name, "/\\") && !strings.Contains(name, "..")
}

func mimeForName(name string) string {
	switch {
	case strings.HasSuffix(name, ".json"):
		return "application/json"
	case strings.HasSuffix(name, ".md"):
		return "text/markdown"
	default:
		return "text/plain"
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
