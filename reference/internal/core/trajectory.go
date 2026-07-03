package core

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

const (
	TrajectoryFileName     = "trajectory.jsonl"
	MaxTrajectoryLineBytes = 64 * 1024
)

// TrajectoryEvent is the digest-only record written to trajectory.jsonl.
// Argument and result payloads must be recorded through their sha256 digests,
// never as raw values.
type TrajectoryEvent struct {
	Seq          int64    `json:"seq"`
	At           string   `json:"at"`
	Actor        string   `json:"actor,omitempty"`
	Kind         string   `json:"kind"`
	Tool         string   `json:"tool,omitempty"`
	TaskIDs      []string `json:"taskIds,omitempty"`
	ArgsDigest   string   `json:"argsDigest,omitempty"`
	ResultDigest string   `json:"resultDigest,omitempty"`
	CwdDigest    string   `json:"cwdDigest,omitempty"`
	ExitCode     *int     `json:"exitCode,omitempty"`
	WallMs       int64    `json:"wallMs,omitempty"`
}

func TrajectoryPath(root, slug string) string {
	return filepath.Join(SpecDir(root, slug), TrajectoryFileName)
}

func AppendTrajectoryEvent(root, slug string, event TrajectoryEvent) error {
	if err := ValidateSlug(slug); err != nil {
		return err
	}
	path := TrajectoryPath(root, slug)
	seq, err := nextTrajectorySeq(path)
	if err != nil {
		return err
	}
	if event.Seq == 0 {
		event.Seq = seq
	} else if event.Seq != seq {
		return fmt.Errorf("trajectory seq %d does not match next seq %d", event.Seq, seq)
	}
	if event.At == "" {
		event.At = Clock().UTC().Format(time.RFC3339Nano)
	}
	line, err := MarshalTrajectoryEvent(event)
	if err != nil {
		return err
	}
	return AppendFile(path, line)
}

// TryAppendTrajectoryEvent records an event when possible and intentionally
// ignores failures so optional producers can remain inert on unwritable ledgers.
func TryAppendTrajectoryEvent(root, slug string, event TrajectoryEvent) {
	_ = AppendTrajectoryEvent(root, slug, event)
}

func ReadTrajectory(root, slug string) ([]TrajectoryEvent, error) {
	if err := ValidateSlug(slug); err != nil {
		return nil, err
	}
	return ReadTrajectoryFile(TrajectoryPath(root, slug))
}

func ReadTrajectoryFile(path string) ([]TrajectoryEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	return DecodeTrajectory(f)
}

func DecodeTrajectory(r io.Reader) ([]TrajectoryEvent, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 4096), MaxTrajectoryLineBytes+1)
	var events []TrajectoryEvent
	var prev int64
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		if line == "" {
			continue
		}
		if len(line)+1 > MaxTrajectoryLineBytes {
			return nil, fmt.Errorf("trajectory line %d exceeds %d bytes", lineNo, MaxTrajectoryLineBytes)
		}
		if strings.ContainsRune(line, 0) {
			return nil, fmt.Errorf("trajectory line %d contains NUL", lineNo)
		}
		var event TrajectoryEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("trajectory line %d: %w", lineNo, err)
		}
		if err := validateTrajectoryEvent(event); err != nil {
			return nil, fmt.Errorf("trajectory line %d: %w", lineNo, err)
		}
		if event.Seq != prev+1 {
			return nil, fmt.Errorf("trajectory line %d: seq %d after %d", lineNo, event.Seq, prev)
		}
		prev = event.Seq
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		if strings.Contains(err.Error(), "token too long") {
			return nil, fmt.Errorf("trajectory line exceeds %d bytes", MaxTrajectoryLineBytes)
		}
		return nil, err
	}
	return events, nil
}

func MarshalTrajectoryEvent(event TrajectoryEvent) (string, error) {
	if err := validateTrajectoryEvent(event); err != nil {
		return "", err
	}
	b, err := json.Marshal(event)
	if err != nil {
		return "", err
	}
	if len(b)+1 > MaxTrajectoryLineBytes {
		return "", fmt.Errorf("trajectory line exceeds %d bytes", MaxTrajectoryLineBytes)
	}
	return string(b) + "\n", nil
}

func TrajectoryDigestBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func TrajectoryDigestJSON(v any) (string, error) {
	if err := rejectNULValue(reflect.ValueOf(v)); err != nil {
		return "", err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return TrajectoryDigestBytes(b), nil
}

func nextTrajectorySeq(path string) (int64, error) {
	events, err := ReadTrajectoryFile(path)
	if err != nil {
		return 0, err
	}
	return int64(len(events) + 1), nil
}

func validateTrajectoryEvent(event TrajectoryEvent) error {
	if event.Seq < 1 {
		return fmt.Errorf("trajectory seq must be positive")
	}
	if event.At == "" {
		return fmt.Errorf("trajectory at is required")
	}
	if event.Kind == "" {
		return fmt.Errorf("trajectory kind is required")
	}
	for name, value := range map[string]string{
		"at":           event.At,
		"actor":        event.Actor,
		"kind":         event.Kind,
		"tool":         event.Tool,
		"argsDigest":   event.ArgsDigest,
		"resultDigest": event.ResultDigest,
		"cwdDigest":    event.CwdDigest,
	} {
		if strings.ContainsRune(value, 0) {
			return fmt.Errorf("trajectory %s contains NUL", name)
		}
	}
	for i, taskID := range event.TaskIDs {
		if strings.ContainsRune(taskID, 0) {
			return fmt.Errorf("trajectory taskIds[%d] contains NUL", i)
		}
	}
	for name, digest := range map[string]string{
		"argsDigest":   event.ArgsDigest,
		"resultDigest": event.ResultDigest,
		"cwdDigest":    event.CwdDigest,
	} {
		if digest != "" && !isSHA256Digest(digest) {
			return fmt.Errorf("trajectory %s must be sha256 digest", name)
		}
	}
	if event.WallMs < 0 {
		return fmt.Errorf("trajectory wallMs must be non-negative")
	}
	return nil
}

func isSHA256Digest(value string) bool {
	if len(value) != len("sha256:")+64 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil
}

func rejectNULValue(v reflect.Value) error {
	if !v.IsValid() {
		return nil
	}
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.String:
		if strings.ContainsRune(v.String(), 0) {
			return fmt.Errorf("trajectory digest input contains NUL")
		}
	case reflect.Slice, reflect.Array:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			for i := 0; i < v.Len(); i++ {
				if v.Index(i).Uint() == 0 {
					return fmt.Errorf("trajectory digest input contains NUL")
				}
			}
			return nil
		}
		for i := 0; i < v.Len(); i++ {
			if err := rejectNULValue(v.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			if err := rejectNULValue(iter.Key()); err != nil {
				return err
			}
			if err := rejectNULValue(iter.Value()); err != nil {
				return err
			}
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if err := rejectNULValue(v.Field(i)); err != nil {
				return err
			}
		}
	}
	return nil
}
