package core

import "fmt"

func ValidSlug(slug string) bool {
	if slug == "" {
		return false
	}
	lastHyphen := false
	for i, r := range slug {
		ok := r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-'
		if !ok {
			return false
		}
		if i == 0 && r == '-' {
			return false
		}
		if r == '-' && lastHyphen {
			return false
		}
		lastHyphen = r == '-'
	}
	return !lastHyphen
}

func ValidateSlug(slug string) error {
	if ValidSlug(slug) {
		return nil
	}
	return fmt.Errorf("invalid slug %q: want ^[a-z0-9][a-z0-9-]*$", slug)
}
