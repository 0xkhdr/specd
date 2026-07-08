package mcp

import (
	"reflect"
	"testing"
)

// TestSplitArgumentsContract pins the MCP tool-call marshaling contract
// (SPEC-05 T-05-03): the reserved "args" key becomes ordered positional
// operands, every other key becomes a flag, and JSON scalar types render into
// the string shape the CLI dispatcher expects.
func TestSplitArgumentsContract(t *testing.T) {
	args, flags := splitArguments(map[string]any{
		"args":     []any{"demo", "T1"},
		"json":     true,
		"sandbox":  false, // a false bool flag is dropped (empty value)
		"tokens":   float64(1200),
		"format":   "prometheus",
		"duration": 2.5,
	})

	if want := []string{"demo", "T1"}; !reflect.DeepEqual(args, want) {
		t.Fatalf("positional args = %v, want %v", args, want)
	}
	want := map[string]string{
		"json":     "true",
		"sandbox":  "",
		"tokens":   "1200", // float64 whole number renders without a decimal point
		"format":   "prometheus",
		"duration": "2.5",
	}
	if !reflect.DeepEqual(flags, want) {
		t.Fatalf("flags = %v, want %v", flags, want)
	}
}

func TestValueToStringFallback(t *testing.T) {
	// A non-scalar JSON value (object/array under a flag key) falls back to Go's
	// default formatting rather than panicking — the contract is total.
	if got := valueToString([]any{1, 2}); got == "" {
		t.Fatalf("fallback rendering produced empty string for %v", []any{1, 2})
	}
}
