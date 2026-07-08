package mcp

import (
	"reflect"
	"strings"
	"testing"
)

// TestCompositeTranslate pins the (command, argv) each composite produces so the
// round-trip to its atomic equivalent is exact (spec R8/AC1/AC3/AC4).
func TestCompositeTranslate(t *testing.T) {
	cases := []struct {
		tool     string
		args     map[string]any
		wantCmd  string
		wantArgv []string
	}{
		{"specd_inspect", map[string]any{"view": "status", "slug": "auth"}, "status", []string{"auth"}},
		{"specd_inspect", map[string]any{"view": "diff", "slug": "auth", "from": "HEAD~1"}, "diff", []string{"auth", "--from", "HEAD~1"}},
		{"specd_inspect", map[string]any{"view": "validate", "slug": "auth"}, "validate", []string{"auth", "--schema"}},
		{"specd_read", map[string]any{"view": "report", "slug": "auth", "format": "html"}, "report", []string{"auth", "--format", "html"}},
		{"specd_query", map[string]any{"view": "next", "slug": "auth", "all": true}, "next", []string{"auth", "--all"}},
		{"specd_orchestrate", map[string]any{"action": "status", "session": "S1"}, "brain", []string{"status", "--session", "S1"}},
		{"specd_orchestrate", map[string]any{"action": "start", "spec": "auth", "approval_policy": "planning"}, "brain", []string{"start", "auth", "--approval-policy", "planning"}},
		{"specd_worker", map[string]any{"action": "claim"}, "pinky", []string{"claim"}},
	}
	for _, tc := range cases {
		ct, ok := compositeByName[tc.tool]
		if !ok {
			t.Fatalf("composite %s not registered", tc.tool)
		}
		cmd, argv, err := ct.translate(tc.args)
		if err != nil {
			t.Fatalf("%s%v: unexpected error: %v", tc.tool, tc.args, err)
		}
		if cmd != tc.wantCmd {
			t.Errorf("%s%v: command = %q, want %q", tc.tool, tc.args, cmd, tc.wantCmd)
		}
		if !reflect.DeepEqual(argv, tc.wantArgv) {
			t.Errorf("%s%v: argv = %v, want %v", tc.tool, tc.args, argv, tc.wantArgv)
		}
	}
}

// TestCompositeEnumErrors covers R6/AC2/AC5: an unknown or missing view/action,
// and a diff without a from ref, all error without dispatching.
func TestCompositeEnumErrors(t *testing.T) {
	cases := []struct {
		tool string
		args map[string]any
		want string
	}{
		{"specd_inspect", map[string]any{"view": "bogus"}, "valid values"},
		{"specd_inspect", map[string]any{}, "is required"},
		{"specd_inspect", map[string]any{"view": "diff", "slug": "auth"}, "requires a 'from'"},
		{"specd_orchestrate", map[string]any{"action": "explode"}, "valid values"},
		{"specd_worker", map[string]any{}, "is required"},
	}
	for _, tc := range cases {
		ct := compositeByName[tc.tool]
		_, _, err := ct.translate(tc.args)
		if err == nil {
			t.Errorf("%s%v: expected error", tc.tool, tc.args)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s%v: error %q missing %q", tc.tool, tc.args, err.Error(), tc.want)
		}
	}
}

// TestCompositeSchemaEnum covers the schema test (spec §7): the view/action
// selector advertises its closed value set in inputSchema.
func TestCompositeSchemaEnum(t *testing.T) {
	for tool, key := range map[string]string{
		"specd_inspect":     "view",
		"specd_query":       "view",
		"specd_orchestrate": "action",
		"specd_worker":      "action",
	} {
		def := compositeByName[tool].def()
		prop, ok := def.InputSchema.Properties[key]
		if !ok {
			t.Errorf("%s: missing %q property", tool, key)
			continue
		}
		if len(prop.Enum) == 0 {
			t.Errorf("%s: %q property advertises no enum", tool, key)
		}
	}
}
