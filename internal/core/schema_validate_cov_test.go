package core

import "testing"

// schema_validate_cov_test.go drives validateNode's type-mismatch and
// required/unknown-property violation branches via ValidateState with crafted
// malformed state documents.

func TestValidateStateViolations(t *testing.T) {
	docs := []string{
		`{"spec": 123}`,                           // spec: expected string, got number
		`{"tasks": "not-an-object"}`,              // tasks: expected object
		`{"turn": "not-an-int"}`,                  // turn: expected integer
		`{"tasks": {"T1": {"wave": "x"}}}`,        // nested: wave expected integer
		`{"blockers": "not-an-array"}`,            // blockers: expected array
		`{"unknownTopLevelField": true}`,          // additionalProperties violation
		`{"acceptance": {"R1C1": {"status": 7}}}`, // nested string field got number
	}
	for _, doc := range docs {
		viols, err := ValidateState([]byte(doc), "")
		if err != nil {
			t.Fatalf("ValidateState(%s) unexpected error: %v", doc, err)
		}
		if len(viols) == 0 {
			t.Errorf("ValidateState(%s) found no violations", doc)
		}
	}

	// Malformed JSON is a hard error, not a violation list.
	if _, err := ValidateState([]byte("{not json"), ""); err == nil {
		t.Error("malformed JSON should error")
	}
}
