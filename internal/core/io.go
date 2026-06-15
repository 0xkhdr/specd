package core

import (
	"fmt"
	"os"
	"path/filepath"
)

// FileExists reports whether a file or directory exists at path.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ReadOrDefault returns the file contents at path, or fallback if it cannot be
// read (missing or unreadable).
func ReadOrDefault(path, fallback string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return fallback
	}
	return string(b)
}

// ReadOrNull returns a pointer to the file contents at path, or nil if it
// cannot be read. The nil result lets callers distinguish a missing file from
// an empty one.
func ReadOrNull(path string) *string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	s := string(b)
	return &s
}

// AtomicWrite writes data to path atomically: it creates any missing parent
// dirs, writes to a temp file in the same directory, fsyncs, sets 0644 (honoring
// umask), and renames over path. A partial write never replaces the target, and
// any failure is propagated to the caller.
func AtomicWrite(path, data string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, fmt.Sprintf(".%d.*.tmp", os.Getpid()))
	if err != nil {
		return err
	}
	name := f.Name()
	defer func() {
		f.Close()
		os.Remove(name)
	}()
	if _, err := f.WriteString(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	// CreateTemp makes 0600 files; restore the documented 0644 (honoring umask)
	// so the renamed artifact is group/other readable for shared CI checkouts.
	if err := f.Chmod(0o644); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// AppendFile appends data to the file at path (creating it and any missing
// parent dirs), fsyncs, and propagates any write or close failure so a partial
// append is never reported as success.
func AppendFile(path, data string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	// Return the Close error: a deferred Close would swallow a flush failure and
	// let a partial append be reported as success.
	return f.Close()
}
