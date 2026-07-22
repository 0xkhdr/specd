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

// TestTaskContractConformance pins the one typed task contract every consumer
// reads (spec 05 R1.1–R1.4). Each subtest names a required check: legacy
// delimiters normalize with a warning, unknown values in a closed vocabulary are
// refused against task id and column, a deferred task carries no evidence
// obligation, and the shipped scaffold example satisfies every armed consumer
// with capability ids that intersect the routing vocabulary.
func TestTaskContractConformance(t *testing.T) {
	t.Run("legacy_delimiter_normalizes_with_warning", func(t *testing.T) {
		row := TaskRow{
			ID: "T1", Role: "craftsman", Files: "b.go;a.go", Kind: "feature", Risk: "high",
			Context: "design;requirements", Evidence: "test/unit;review/design-review", Checks: "empty;error",
		}
		c, err := ParseTaskContract(row)
		if err != nil {
			t.Fatalf("legacy `;` delimiter refused: %v", err)
		}
		if !reflect.DeepEqual(c.OutputPaths, []string{"a.go", "b.go"}) {
			t.Fatalf("files not normalized: %#v", c.OutputPaths)
		}
		if !reflect.DeepEqual(c.Context, []string{"design", "requirements"}) {
			t.Fatalf("context not normalized: %#v", c.Context)
		}
		if len(c.Quality.Required) != 2 || !reflect.DeepEqual(c.Checks, []string{"empty", "error"}) {
			t.Fatalf("evidence/checks not normalized: %#v", c.Quality)
		}
		for _, column := range []string{"files", "context", "evidence", "checks"} {
			if !strings.Contains(strings.Join(c.Warnings, "\n"), "column "+column+" uses ';'") {
				t.Fatalf("no deprecation warning for column %s: %#v", column, c.Warnings)
			}
		}
		// Same authored intent through the canonical delimiter must produce the
		// identical contract apart from the warnings.
		canonical, err := ParseTaskContract(TaskRow{
			ID: "T1", Role: "craftsman", Files: "b.go,a.go", Kind: "feature", Risk: "high",
			Context: "design,requirements", Evidence: "test/unit,review/design-review", Checks: "empty,error",
		})
		if err != nil {
			t.Fatal(err)
		}
		c.Warnings = nil
		if !reflect.DeepEqual(c, canonical) {
			t.Fatalf("legacy and canonical spellings disagree:\n%#v\n%#v", c, canonical)
		}
	})

	t.Run("unknown_value_fails_against_task_and_column", func(t *testing.T) {
		for column, row := range map[string]TaskRow{
			"kind":         {ID: "T7", Kind: "widget"},
			"risk":         {ID: "T7", Risk: "spicy"},
			"capabilities": {ID: "T7", Capabilities: []string{"telepathy"}},
		} {
			_, err := ParseTaskContract(row)
			if err == nil {
				t.Fatalf("unknown %s value accepted", column)
			}
			for _, want := range []string{"TASK_FIELD_UNKNOWN", "task T7", "column " + column, "is not one of"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("%s error missing %q: %v", column, want, err)
				}
			}
		}
	})

	t.Run("deferred_task_carries_no_evidence_obligation", func(t *testing.T) {
		row := TaskRow{ID: "T9", Role: "craftsman", Files: "-", Kind: DeferredTaskKind, Risk: "low", Refs: []string{"R1.1"}}
		c, err := ParseTaskContract(row)
		if err != nil {
			t.Fatalf("deferred task refused: %v", err)
		}
		if !c.Deferred {
			t.Fatal("kind=deferred did not project Deferred")
		}
		if len(c.Quality.Required) != 0 {
			t.Fatalf("deferred task carries evidence requirements: %#v", c.Quality.Required)
		}
		// Coverage analysis must agree with the contract's deferral identity:
		// both read the same kind token, so a deferred task suppresses the
		// missing-task-coverage finding for its requirement.
		requirements := RequirementsDoc{Requirements: []Requirement{{ID: "R1", Criteria: []Criterion{{ID: "R1.1"}}}}}
		design := DesignDoc{Refs: []string{"R1"}}
		if findings := AnalyzeCoverage(requirements, design, []TaskRow{row}); len(findings) != 0 {
			t.Fatalf("coverage disagrees with contract deferral: %+v", findings)
		}
		if findings := AnalyzeCoverage(requirements, design, []TaskRow{{ID: "T9", Kind: "feature", Refs: []string{"R1.1"}}}); len(findings) != 0 {
			t.Fatalf("non-deferred control produced findings: %+v", findings)
		}
	})

	t.Run("capability_parity_across_registry_and_routing", func(t *testing.T) {
		canonical := CanonicalTaskCapabilities()
		if len(canonical) == 0 {
			t.Fatal("capability registry is empty")
		}
		defaultClass := DefaultConfig.Routing.ClassCapabilities[DefaultConfig.Routing.DefaultClass]
		if !reflect.DeepEqual(canonical, sortedUnique(defaultClass)) {
			t.Fatalf("task capability registry %v != default routing class capabilities %v", canonical, defaultClass)
		}
		for _, capability := range canonical {
			if _, err := ParseTaskContract(TaskRow{ID: "T1", Capabilities: []string{capability}}); err != nil {
				t.Fatalf("routing capability %q is not a legal task capability: %v", capability, err)
			}
		}
	})

	t.Run("scaffold_example_satisfies_every_armed_consumer", func(t *testing.T) {
		row := TasksScaffoldExampleRow()
		if row.Role == "" || row.Kind == "" || row.Risk == "" || len(row.Capabilities) == 0 {
			t.Fatalf("scaffold example row did not parse: %#v", row)
		}
		c, err := ParseTaskContract(row)
		if err != nil {
			t.Fatalf("scaffold example rejected by ParseTaskContract: %v", err)
		}
		if len(c.Warnings) != 0 {
			t.Fatalf("scaffold example teaches a deprecated spelling: %v", c.Warnings)
		}
		if findings := ValidateTaskTrace([]TaskRow{row}, map[string]bool{"R1": true, "R1.1": true}, true); len(findings) != 0 {
			t.Fatalf("scaffold example fails the production task-trace gate: %+v", findings)
		}
	})
}
