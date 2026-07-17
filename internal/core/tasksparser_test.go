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

func TestTasksRoutingMetadataByteStable(t *testing.T) {
	raw := []byte("| id | role | files | depends-on | verify | acceptance | risk | complexity | capabilities |\n|---|---|---|---|---|---|---|---|---|\n| [ ] T1 | craftsman | a.go | | go test ./... | R5 | high | complex | context, sandbox, review |\n")
	doc, err := ParseTasksMd(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Tasks) != 1 {
		t.Fatalf("tasks = %d", len(doc.Tasks))
	}
	task := doc.Tasks[0]
	if task.Complexity != "complex" || !reflect.DeepEqual(task.Capabilities, []string{"context", "review", "sandbox"}) {
		t.Fatalf("routing metadata = %#v", task)
	}
	if !bytes.Equal(doc.Raw, raw) {
		t.Fatal("routing metadata changed source bytes")
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

func TestTasksTraceMetadata(t *testing.T) {
	// Minimal 6-column tables carry no trace metadata and still parse (spec 01 R3.1).
	minimal, err := ParseTasksMd([]byte(sampleTasksMd))
	if err != nil {
		t.Fatal(err)
	}
	if minimal.Tasks[0].Risk != "" || len(minimal.Tasks[0].Refs) != 0 {
		t.Fatalf("minimal task carried unexpected metadata: %+v", minimal.Tasks[0])
	}

	// A table that declares the optional columns parses the trace/risk metadata.
	src := "# Tasks\n\n" +
		"| id | role | files | depends-on | verify | acceptance | refs | kind | risk | context | evidence | checks |\n" +
		"|---|---|---|---|---|---|---|---|---|---|---|---|\n" +
		"| T1 | craftsman | a.go | - | go test ./... | first | R1.1, D2 | feature | high | design.md 2 | unit, integration | empty-input |\n"
	doc, err := ParseTasksMd([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	task := doc.Tasks[0]
	if !reflect.DeepEqual(task.Refs, []string{"R1.1", "D2"}) {
		t.Fatalf("Refs = %#v", task.Refs)
	}
	if task.Kind != "feature" || task.Risk != "high" || task.Context != "design.md 2" ||
		task.Evidence != "unit, integration" || task.Checks != "empty-input" {
		t.Fatalf("trace metadata = %+v", task)
	}
	// Byte-stable: the extended table round-trips unchanged.
	if got := SerializeTasksMd(doc); !bytes.Equal(got, []byte(src)) {
		t.Fatalf("extended table did not round-trip:\n%s", got)
	}
}

func TestTasksDeclaredFilesNormalized(t *testing.T) {
	src := "| id | role | files | depends-on | verify | acceptance |\n" +
		"|---|---|---|---|---|---|\n" +
		"| T1 | craftsman | ./b.go; a_test.go, a.go; ./b.go | - | go test ./... | R2.1 |\n"
	doc, err := ParseTasksMd([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.go", "a_test.go", "b.go"}
	if got := doc.Tasks[0].DeclaredFiles; !reflect.DeepEqual(got, want) {
		t.Fatalf("DeclaredFiles = %#v, want %#v", got, want)
	}
	if got := SerializeTasksMd(doc); !bytes.Equal(got, []byte(src)) {
		t.Fatal("normalization changed author bytes")
	}
}

func TestTasksDeclaredFilesRejectEscape(t *testing.T) {
	for _, files := range []string{"../secret", "/etc/passwd", "a/../../secret", "C:\\temp\\x"} {
		src := "| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | craftsman | " + files + " | - | go test ./... | R2.1 |\n"
		if _, err := ParseTasksMd([]byte(src)); err == nil {
			t.Fatalf("unsafe files %q accepted", files)
		}
	}
}

func TestTasksQualityDeclarationCompatibility(t *testing.T) {
	src := "| id | role | files | depends-on | verify | acceptance | evidence | checks |\n|---|---|---|---|---|---|---|---|\n| T1 | craftsman | a.go | - | go test ./... | R1 | test/unit, output_eval/rubric-v1 | unit, rubric-v1 |\n"
	doc, err := ParseTasksMd([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Tasks[0].Evidence != "test/unit, output_eval/rubric-v1" || doc.Tasks[0].Checks != "unit, rubric-v1" {
		t.Fatalf("quality declaration = %+v", doc.Tasks[0])
	}
	if got := SerializeTasksMd(doc); !bytes.Equal(got, []byte(src)) {
		t.Fatal("quality columns changed bytes")
	}
}

func TestRewriteKeepsMetadataColumns(t *testing.T) {
	src := "# Tasks\n\n" +
		"| id | role | files | depends-on | verify | acceptance | risk |\n" +
		"|---|---|---|---|---|---|---|\n" +
		"| T1 | craftsman | a.go | - | go test ./... | first | high |\n"
	out, err := RewriteTaskStatusLine([]byte(src), "T1", "✅")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "high") {
		t.Fatalf("rewrite dropped the risk column:\n%s", out)
	}
	if !strings.Contains(string(out), "✅ T1") {
		t.Fatalf("rewrite missed the marker:\n%s", out)
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
