package orchestration

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/0xkhdr/specd/internal/core"
)

var ErrSessionRevisionConflict = errors.New("session revision conflict")

type Session struct {
	Revision int64   `json:"revision"`
	Leases   []Lease `json:"leases,omitempty"`
}

func LoadSession(path string) (Session, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Session{Revision: 0}, nil
	}
	if err != nil {
		return Session{}, fmt.Errorf("read session: %w", err)
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return Session{}, fmt.Errorf("decode session: %w", err)
	}
	return session, nil
}

func SaveSessionCAS(root, path string, expectedRevision int64, next Session) error {
	_, err := core.WithSpecLock(root, func() (struct{}, error) {
		current, err := LoadSession(path)
		if err != nil {
			return struct{}{}, err
		}
		if current.Revision != expectedRevision {
			return struct{}{}, ErrSessionRevisionConflict
		}
		next.Revision = expectedRevision + 1
		data, err := json.MarshalIndent(next, "", "  ")
		if err != nil {
			return struct{}{}, fmt.Errorf("encode session: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return struct{}{}, fmt.Errorf("mkdir session: %w", err)
		}
		return struct{}{}, core.AtomicWrite(path, string(append(data, '\n')))
	})
	return err
}
