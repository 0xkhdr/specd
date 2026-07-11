package gates

import (
	"fmt"
	"path/filepath"
)

func CheckScope(changed, declared []string) error {
	for _, path := range changed {
		matched := false
		for _, pattern := range declared {
			if pattern == path {
				matched = true
				break
			}
			if ok, _ := filepath.Match(pattern, path); ok {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("outside_scope: %s is not declared", path)
		}
	}
	return nil
}
