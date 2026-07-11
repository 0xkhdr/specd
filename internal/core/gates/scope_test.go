package gates

import "testing"

func TestScopeRejectsOutsideDeclaredFiles(t *testing.T) {
	if err := CheckScope([]string{"a.go"}, []string{"a.go", "a_test.go"}); err != nil {
		t.Fatal(err)
	}
	if err := CheckScope([]string{"a.go", "x.go"}, []string{"a.go"}); err == nil {
		t.Fatal("outside scope accepted")
	}
}
