package core

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

const sampleTasksMd = `# Tasks

## Wave 1

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T1 | craftsman | a.go | - | go test ./... | first task |
| T2 | craftsman | b.go | T1 | go test ./... | second task |
| T3 | craftsman | c.go | T1 | go test ./... | third task |
| T4 | craftsman | d.go | T2, T3 | go test ./... | final task |
`

func TestParseTasksMdRoundTrip(t *testing.T) {
	doc, err := ParseTasksMd([]byte(sampleTasksMd))
	if err != nil {
		t.Fatalf("ParseTasksMd() error = %v", err)
	}
	if got := SerializeTasksMd(doc); !bytes.Equal(got, []byte(sampleTasksMd)) {
		t.Fatalf("SerializeTasksMd() changed bytes\nwant %q\ngot  %q", sampleTasksMd, string(got))
	}
	if got, want := len(doc.Tasks), 4; got != want {
		t.Fatalf("len(Tasks) = %d, want %d", got, want)
	}
	if got, want := doc.Tasks[3].DependsOn, []string{"T2", "T3"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("DependsOn = %#v, want %#v", got, want)
	}
}

func TestTasksRoundTrip(t *testing.T) {
	TestParseTasksMdRoundTrip(t)
}

func TestParseTasksMdRejectsDuplicateIDs(t *testing.T) {
	input := strings.Replace(sampleTasksMd, "| T2 |", "| T1 |", 1)
	if _, err := ParseTasksMd([]byte(input)); err == nil {
		t.Fatal("ParseTasksMd() error = nil, want duplicate id error")
	}
}

func TestSingleLineRewrite(t *testing.T) {
	input := []byte(sampleTasksMd)
	rewritten, err := RewriteTaskStatusLine(input, "T2", "✅")
	if err != nil {
		t.Fatal(err)
	}
	oldLines := strings.Split(string(input), "\n")
	newLines := strings.Split(string(rewritten), "\n")
	if len(oldLines) != len(newLines) {
		t.Fatalf("line count changed: got %d, want %d", len(newLines), len(oldLines))
	}
	changed := 0
	for i := range oldLines {
		if oldLines[i] != newLines[i] {
			changed++
			if !strings.Contains(newLines[i], "✅ T2") {
				t.Fatalf("changed line %d = %q, want T2 marker", i, newLines[i])
			}
		}
	}
	if changed != 1 {
		t.Fatalf("changed lines = %d, want 1", changed)
	}
}

func TestTaskDAGTopologicalWaves(t *testing.T) {
	doc, err := ParseTasksMd([]byte(sampleTasksMd))
	if err != nil {
		t.Fatal(err)
	}
	dag, err := NewTaskDAG(doc.Tasks)
	if err != nil {
		t.Fatal(err)
	}
	waves, err := dag.TopologicalWaves()
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"T1"}, {"T2", "T3"}, {"T4"}}
	if !reflect.DeepEqual(waves, want) {
		t.Fatalf("TopologicalWaves() = %#v, want %#v", waves, want)
	}
}

func TestDAG(t *testing.T) {
	TestTaskDAGTopologicalWaves(t)
	TestTaskDAGRunnableFrontier(t)
	TestTaskDAGRejectsUnknownDependencyAndCycle(t)
}

func TestTaskDAGRunnableFrontier(t *testing.T) {
	doc, err := ParseTasksMd([]byte(sampleTasksMd))
	if err != nil {
		t.Fatal(err)
	}
	dag, err := NewTaskDAG(doc.Tasks)
	if err != nil {
		t.Fatal(err)
	}
	frontier, err := dag.RunnableFrontier(map[string]TaskRunStatus{"T1": TaskComplete})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"T2", "T3"}
	if !reflect.DeepEqual(frontier, want) {
		t.Fatalf("RunnableFrontier() = %#v, want %#v", frontier, want)
	}
}

func TestTaskDAGRejectsUnknownDependencyAndCycle(t *testing.T) {
	if _, err := NewTaskDAG([]TaskRow{{ID: "T1", DependsOn: []string{"T2"}}}); err == nil {
		t.Fatal("NewTaskDAG() unknown dependency error = nil")
	}
	if _, err := NewTaskDAG([]TaskRow{
		{ID: "T1", DependsOn: []string{"T2"}},
		{ID: "T2", DependsOn: []string{"T1"}},
	}); err == nil {
		t.Fatal("NewTaskDAG() cycle error = nil")
	}
}

func TestTaskDAGAllBlocked(t *testing.T) {
	dag, err := NewTaskDAG([]TaskRow{{ID: "T1"}, {ID: "T2"}})
	if err != nil {
		t.Fatal(err)
	}
	if !dag.AllBlocked(map[string]TaskRunStatus{"T1": TaskBlocked, "T2": TaskBlocked}) {
		t.Fatal("AllBlocked() = false, want true")
	}
	if dag.AllBlocked(map[string]TaskRunStatus{"T1": TaskBlocked}) {
		t.Fatal("AllBlocked() = true with pending task, want false")
	}
}

func FuzzParseTasksMdRoundTrip(f *testing.F) {
	f.Add(sampleTasksMd)
	f.Add("| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n")
	f.Add("not a table\n")
	f.Fuzz(func(t *testing.T, input string) {
		doc, err := ParseTasksMd([]byte(input))
		if err != nil {
			return
		}
		if got := SerializeTasksMd(doc); !bytes.Equal(got, []byte(input)) {
			t.Fatalf("round trip changed bytes")
		}
	})
}
