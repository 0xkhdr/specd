package core

import "regexp"

// SlugRE is the canonical spec-slug grammar. A slug is a path segment under
// .specd/specs/, so it must never contain path separators, "..", or other
// shell/filesystem metacharacters. Lowercase alnum plus internal hyphens only.
var SlugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// ValidateSlug rejects any slug that could escape the specs directory (path
// traversal) or otherwise resolve to an unintended path. Every command that
// accepts a user-supplied slug must call this before touching the filesystem —
// defense in depth, independent of whether the spec already exists.
func ValidateSlug(slug string) error {
	if slug == "" {
		return UsageError("spec slug is required")
	}
	if !SlugRE.MatchString(slug) {
		return UsageError("invalid slug '" + slug + "' (must match ^[a-z0-9][a-z0-9-]*$)")
	}
	return nil
}
