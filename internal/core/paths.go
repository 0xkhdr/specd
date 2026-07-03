package core

import (
	"fmt"
	"os"
	"path/filepath"
)

const specdDirName = ".specd"

type NotFoundError struct {
	Start string
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf("specd root not found from %s", e.Start)
}

func (e NotFoundError) ExitCode() int {
	return 3
}

func SpecdDir(root string) string {
	return filepath.Join(root, specdDirName)
}

func FindRoot(start string) (string, error) {
	if start == "" {
		start = "."
	}
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(dir)
	if err == nil && !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	for {
		if st, err := os.Stat(SpecdDir(dir)); err == nil && st.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", NotFoundError{Start: start}
		}
		dir = parent
	}
}
