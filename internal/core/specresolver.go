package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type SpecResolution struct {
	Slug   string `json:"slug"`
	Source string `json:"source"`
}

type SpecResolutionError struct{ Code, Message, RecoveryAction string }

func (e SpecResolutionError) Error() string {
	return fmt.Sprintf("%s: %s; %s", e.Code, e.Message, e.RecoveryAction)
}
func FindingCode(err error) string {
	if e, ok := err.(SpecResolutionError); ok {
		return e.Code
	}
	return ""
}

func ResolveSpec(root, explicit, pinned string) (SpecResolution, error) {
	if explicit != "" {
		return resolveNamedSpec(root, explicit, "explicit", "SPEC_EXPLICIT_INVALID")
	}
	if pinned != "" {
		return resolveNamedSpec(root, pinned, "pinned", "SPEC_PIN_INVALID")
	}
	entries, err := os.ReadDir(filepath.Join(SpecdDir(root), "specs"))
	if err != nil && !os.IsNotExist(err) {
		return SpecResolution{}, err
	}
	var slugs []string
	for _, entry := range entries {
		if entry.IsDir() && ValidateSlug(entry.Name()) == nil {
			slugs = append(slugs, entry.Name())
		}
	}
	sort.Strings(slugs)
	switch len(slugs) {
	case 1:
		return SpecResolution{Slug: slugs[0], Source: "single"}, nil
	case 0:
		return SpecResolution{}, SpecResolutionError{Code: "SPEC_REQUIRED", Message: "no eligible spec found", RecoveryAction: "pass an explicit spec slug or create one with `specd new <slug>`"}
	default:
		return SpecResolution{}, SpecResolutionError{Code: "SPEC_AMBIGUOUS", Message: "multiple eligible specs: " + fmt.Sprint(slugs), RecoveryAction: "pass an explicit spec slug or set SPECD_SPEC"}
	}
}

func resolveNamedSpec(root, slug, source, code string) (SpecResolution, error) {
	if err := ValidateSlug(slug); err != nil {
		return SpecResolution{}, SpecResolutionError{Code: code, Message: err.Error(), RecoveryAction: "choose a valid existing spec slug"}
	}
	info, err := os.Stat(SpecDir(root, slug))
	if err != nil || !info.IsDir() {
		return SpecResolution{}, SpecResolutionError{Code: code, Message: "spec " + slug + " does not exist", RecoveryAction: "choose a valid existing spec slug"}
	}
	return SpecResolution{Slug: slug, Source: source}, nil
}
