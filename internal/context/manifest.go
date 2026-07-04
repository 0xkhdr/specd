package context

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/0xkhdr/specd/internal/core"
)

const ManifestVersion = "1"

type Item struct {
	Kind            string `json:"kind"`
	Path            string `json:"path,omitempty"`
	TaskID          string `json:"task_id,omitempty"`
	EstimatedTokens int    `json:"estimated_tokens"`
}

type Manifest struct {
	Version         string `json:"version"`
	Mode            string `json:"mode"`
	Slug            string `json:"slug"`
	TaskID          string `json:"task_id"`
	Items           []Item `json:"items"`
	EstimatedTokens int    `json:"estimated_tokens"`
}

func BuildManifest(slug string, tasks []core.TaskRow, taskID string) (Manifest, error) {
	task, ok := findTask(tasks, taskID)
	if !ok {
		return Manifest{}, fmt.Errorf("task %s not found", taskID)
	}
	mode := ModeForTask(task)
	items := []Item{
		{Kind: "spec", Path: fmt.Sprintf("specs/%s/spec.md", slug)},
		{Kind: "tasks", Path: fmt.Sprintf("specs/%s/tasks.md", slug)},
		{Kind: "task", TaskID: task.ID},
		{Kind: "role", Path: fmt.Sprintf(".specd/roles/%s.md", task.Role)},
	}
	for i := range items {
		items[i].EstimatedTokens = EstimateText(items[i].Kind + items[i].Path + items[i].TaskID)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Kind == items[j].Kind {
			if items[i].Path == items[j].Path {
				return items[i].TaskID < items[j].TaskID
			}
			return items[i].Path < items[j].Path
		}
		return items[i].Kind < items[j].Kind
	})
	manifest := Manifest{Version: ManifestVersion, Mode: mode, Slug: slug, TaskID: taskID, Items: items}
	for _, item := range items {
		manifest.EstimatedTokens += item.EstimatedTokens
	}
	return manifest, nil
}

func ModeForTask(task core.TaskRow) string {
	switch task.Role {
	case "validator":
		return "validator"
	case "scout":
		return "scout"
	case "scribe":
		return "scribe"
	default:
		return "craftsman"
	}
}

func EstimateText(text string) int {
	if text == "" {
		return 0
	}
	return (len(text) + 3) / 4
}

func ValidateManifest(raw []byte) error {
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return err
	}
	if manifest.Version != ManifestVersion {
		return fmt.Errorf("unsupported manifest version %q", manifest.Version)
	}
	if manifest.Mode == "" || manifest.Slug == "" || manifest.TaskID == "" {
		return fmt.Errorf("manifest mode, slug, and task_id are required")
	}
	if len(manifest.Items) != 4 {
		return fmt.Errorf("manifest must contain four items")
	}
	for _, item := range manifest.Items {
		if item.Kind == "" {
			return fmt.Errorf("manifest item kind is required")
		}
	}
	return nil
}

func findTask(tasks []core.TaskRow, taskID string) (core.TaskRow, bool) {
	for _, task := range tasks {
		if task.ID == taskID {
			return task, true
		}
	}
	return core.TaskRow{}, false
}
