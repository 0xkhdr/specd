package orchestration

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type ACPEvent struct {
	Seq     int       `json:"seq"`
	Time    time.Time `json:"time"`
	Kind    string    `json:"kind"`
	TaskID  string    `json:"task_id,omitempty"`
	Payload string    `json:"payload,omitempty"`
}

func AppendACP(path string, event ACPEvent) error {
	events, err := ReadACP(path)
	if err != nil {
		return err
	}
	event.Seq = len(events) + 1
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode acp event: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir acp: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open acp: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("append acp: %w", err)
	}
	return file.Sync()
}

func ReadACP(path string) ([]ACPEvent, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open acp: %w", err)
	}
	defer file.Close()

	var events []ACPEvent
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event ACPEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, fmt.Errorf("decode acp: %w", err)
		}
		events = append(events, event)
	}
	return events, scanner.Err()
}
