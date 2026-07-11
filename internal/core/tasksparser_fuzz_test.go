package core

import "testing"

func TestFuzzTasks(t *testing.T) {
	inputs := [][]byte{
		[]byte("| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n"),
		[]byte("| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | craftsman | a.go | - | go test ./... | ok |\n"),
	}
	for _, input := range inputs {
		parsed, err := ParseTasksMd(input)
		if err != nil {
			t.Fatalf("ParseTasksMd: %v", err)
		}
		if string(parsed.Raw) != string(input) {
			t.Fatalf("raw changed")
		}
	}
}
