package core

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Digest returns the hex-encoded SHA-256 of data — the canonical content
// address used to pin approved requirement/design source bytes into a record so
// a later amendment can detect drift (spec 01 R5). Deterministic and pure.
func Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ReadOrNull returns the file contents, or nil when path does not exist.
func ReadOrNull(path string) *string {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return nil
	}
	text := string(data)
	return &text
}

// AtomicWrite writes data via a temp file in the target directory, fsyncs it,
// chmods to the non-secret artifact mode, then renames over the target.
func AtomicWrite(path, data string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}

	file, err := os.CreateTemp(dir, fmt.Sprintf(".%d.*.tmp", os.Getpid()))
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tempName := file.Name()
	cleanup := true
	defer func() {
		_ = file.Close()
		if cleanup {
			_ = os.Remove(tempName)
		}
	}()

	if _, err := file.WriteString(data); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := file.Chmod(0o644); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	cleanup = false
	if err := syncDir(dir); err != nil {
		return fmt.Errorf("sync parent dir: %w", err)
	}
	return nil
}

// AppendFile appends data to a non-secret artifact and fsyncs before returning.
func AppendFile(path, data string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open append file: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(data); err != nil {
		return fmt.Errorf("append file: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync append file: %w", err)
	}
	return nil
}

func syncDir(dir string) error {
	file, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

// RemoveFileDurable removes a transaction sidecar and fsyncs its directory so
// recovery cannot resurrect a completed transaction after a process or host
// crash.
func RemoveFileDurable(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return syncDir(filepath.Dir(path))
}
