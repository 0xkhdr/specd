package scope

import (
	"fmt"
	"path"
	"strings"
)

func Normalize(rel string) (string, error) {
	rel = strings.ReplaceAll(strings.TrimSpace(rel), "\\", "/")
	clean := path.Clean(rel)
	if rel == "" || clean == "." || path.IsAbs(clean) || (len(clean) >= 2 && clean[1] == ':') || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("path %q escapes repository base", rel)
	}
	return clean, nil
}
