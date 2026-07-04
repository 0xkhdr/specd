package context

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

const ManifestVersion = "1"

type Item struct {
	Kind            string `json:"kind"`
	Path            string `json:"path,omitempty"`
	TaskID          string `json:"task_id,omitempty"`
	Mode            string `json:"mode,omitempty"`
	EstimatedTokens int    `json:"estimated_tokens"`
}

type Manifest struct {
	Version         string   `json:"version"`
	Mode            string   `json:"mode"`
	Slug            string   `json:"slug"`
	TaskID          string   `json:"task_id"`
	Items           []Item   `json:"items"`
	Notes           []string `json:"notes,omitempty"`
	EstimatedTokens int      `json:"estimated_tokens"`
}

// BuildManifest assembles the context references for one task. The steering
// constitution and memory (R4.3) enter as references + modes, never inlined
// content, bounded against maxTokens: when over budget, memory drops before
// steering (constitution wins), deterministically, with a note. maxTokens <= 0
// disables budget enforcement.
func BuildManifest(root, slug string, tasks []core.TaskRow, taskID string, maxTokens int) (Manifest, error) {
	task, ok := findTask(tasks, taskID)
	if !ok {
		return Manifest{}, fmt.Errorf("task %s not found", taskID)
	}
	mode := ModeForTask(task)
	items := []Item{
		{Kind: "spec", Path: fmt.Sprintf("specs/%s/requirements.md", slug)},
		{Kind: "tasks", Path: fmt.Sprintf("specs/%s/tasks.md", slug)},
		{Kind: "task", TaskID: task.ID},
		{Kind: "role", Path: fmt.Sprintf(".specd/roles/%s.md", task.Role)},
	}
	for i := range items {
		items[i].EstimatedTokens = EstimateText(items[i].Kind + items[i].Path + items[i].TaskID)
	}
	items = append(items, steeringItems(root, slug)...)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Kind == items[j].Kind {
			if items[i].Path == items[j].Path {
				return items[i].TaskID < items[j].TaskID
			}
			return items[i].Path < items[j].Path
		}
		return items[i].Kind < items[j].Kind
	})
	items, notes := enforceBudget(items, maxTokens)
	manifest := Manifest{Version: ManifestVersion, Mode: mode, Slug: slug, TaskID: taskID, Items: items, Notes: notes}
	for _, item := range items {
		manifest.EstimatedTokens += item.EstimatedTokens
	}
	return manifest, nil
}

// steeringItems references the constitution (.specd/steering/*.md) and memory
// files as manifest items. Steering carries static-instructions mode; memory
// (steering/memory.md and the spec's own memory.md) carries reference-if-needed.
// Token estimates come from on-disk size so the budget reflects what an agent
// would load. Missing files are skipped, never referenced.
func steeringItems(root, slug string) []Item {
	var items []Item
	steeringDir := filepath.Join(root, ".specd", "steering")
	entries, _ := os.ReadDir(steeringDir)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		rel := ".specd/steering/" + entry.Name()
		kind, mode := "steering", "static-instructions"
		if entry.Name() == "memory.md" {
			kind, mode = "memory", "reference-if-needed"
		}
		items = append(items, Item{Kind: kind, Path: rel, Mode: mode, EstimatedTokens: estimateFile(filepath.Join(root, rel))})
	}
	specMem := filepath.Join(".specd", "specs", slug, "memory.md")
	if fi, err := os.Stat(filepath.Join(root, specMem)); err == nil && !fi.IsDir() {
		items = append(items, Item{Kind: "memory", Path: filepath.ToSlash(specMem), Mode: "reference-if-needed", EstimatedTokens: tokensFromBytes(fi.Size())})
	}
	return items
}

func estimateFile(path string) int {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return 0
	}
	return tokensFromBytes(fi.Size())
}

func tokensFromBytes(n int64) int { return int((n + 3) / 4) }

// enforceBudget drops items until the total fits maxTokens, memory before
// steering (constitution wins). Core items (spec/tasks/task/role) are never
// dropped. Deterministic: items arrive sorted, droppable ones removed from the
// end. Returns the surviving items and one note per drop.
func enforceBudget(items []Item, maxTokens int) ([]Item, []string) {
	if maxTokens <= 0 {
		return items, nil
	}
	total := 0
	for _, item := range items {
		total += item.EstimatedTokens
	}
	var notes []string
	for total > maxTokens {
		idx := lastDroppable(items)
		if idx < 0 {
			break
		}
		total -= items[idx].EstimatedTokens
		notes = append(notes, fmt.Sprintf("dropped %s (%s) over context budget", items[idx].Path, items[idx].Kind))
		items = append(items[:idx], items[idx+1:]...)
	}
	return items, notes
}

// lastDroppable returns the index of the last memory item, else the last
// steering item, else -1. Memory always sheds before steering.
func lastDroppable(items []Item) int {
	steer := -1
	for i, item := range items {
		switch item.Kind {
		case "memory":
			memoryIdx := i
			// keep scanning for the last memory item
			for j := i + 1; j < len(items); j++ {
				if items[j].Kind == "memory" {
					memoryIdx = j
				}
			}
			return memoryIdx
		case "steering":
			steer = i
		}
	}
	return steer
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
	if len(manifest.Items) < 4 {
		return fmt.Errorf("manifest must contain the four core items")
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
