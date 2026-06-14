package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
)

func TestParseArgs(t *testing.T) {
	t.Run("separates_positionals_from_flags", func(t *testing.T) {
		a := ParseArgs([]string{"auth", "t1", "--title", "Hello"})
		if len(a.Pos) != 2 || a.Pos[0] != "auth" || a.Pos[1] != "t1" {
			t.Errorf("Pos = %v, want [auth t1]", a.Pos)
		}
		if a.Str("title") != "Hello" {
			t.Errorf("title = %q, want Hello", a.Str("title"))
		}
	})

	t.Run("boolean_flags_take_no_value", func(t *testing.T) {
		a := ParseArgs([]string{"--json", "auth"})
		if !a.Bool("json") {
			t.Error("json should be true")
		}
		// "auth" must remain a positional, not be eaten as json's value.
		if len(a.Pos) != 1 || a.Pos[0] != "auth" {
			t.Errorf("Pos = %v, want [auth]", a.Pos)
		}
	})

	t.Run("value_flag_at_end_without_value_is_true", func(t *testing.T) {
		a := ParseArgs([]string{"--title"})
		if a.Str("title") != "true" {
			t.Errorf("title = %q, want true", a.Str("title"))
		}
	})

	t.Run("value_flag_followed_by_flag_is_true", func(t *testing.T) {
		a := ParseArgs([]string{"--title", "--json"})
		if a.Str("title") != "true" {
			t.Errorf("title = %q, want true", a.Str("title"))
		}
		if !a.Bool("json") {
			t.Error("json should be true")
		}
	})

	t.Run("has_distinguishes_set_from_unset", func(t *testing.T) {
		a := ParseArgs([]string{"--criterion", "1.2"})
		if !a.Has("criterion") {
			t.Error("Has(criterion) should be true")
		}
		if a.Has("status") {
			t.Error("Has(status) should be false")
		}
	})

	t.Run("key_equals_value_form", func(t *testing.T) {
		a := ParseArgs([]string{"--status=complete"})
		if a.Str("status") != "complete" {
			t.Errorf("status = %q, want complete", a.Str("status"))
		}
	})

	t.Run("key_equals_value_with_following_positional", func(t *testing.T) {
		a := ParseArgs([]string{"--status=complete", "auth"})
		if a.Str("status") != "complete" {
			t.Errorf("status = %q, want complete", a.Str("status"))
		}
		if len(a.Pos) != 1 || a.Pos[0] != "auth" {
			t.Errorf("Pos = %v, want [auth]", a.Pos)
		}
	})

	t.Run("key_equals_value_preserves_embedded_equals", func(t *testing.T) {
		a := ParseArgs([]string{"--evidence=a=b=c"})
		if a.Str("evidence") != "a=b=c" {
			t.Errorf("evidence = %q, want a=b=c", a.Str("evidence"))
		}
	})

	t.Run("key_equals_empty_value", func(t *testing.T) {
		a := ParseArgs([]string{"--title="})
		if !a.Has("title") || a.Str("title") != "" {
			t.Errorf("title = %q present=%v, want empty present", a.Str("title"), a.Has("title"))
		}
	})

	t.Run("two_bools_then_value", func(t *testing.T) {
		a := ParseArgs([]string{"--force", "--json", "--evidence", "x y"})
		if !a.Bool("force") || !a.Bool("json") {
			t.Error("force and json should both be true")
		}
		if a.Str("evidence") != "x y" {
			t.Errorf("evidence = %q, want 'x y'", a.Str("evidence"))
		}
	})
}

// TestBooleanFlagsRegistered guards against the silent-next-token-consume
// footgun: every flag consumed via args.Bool(...) in internal/cmd must be
// registered in booleanFlags, or a forgotten registration would eat the next
// positional as the flag's value. The list is derived from source so it can
// never drift from actual usage.
func TestBooleanFlagsRegistered(t *testing.T) {
	cmdDir := filepath.Join("..", "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		t.Fatalf("read cmd dir: %v", err)
	}
	boolCall := regexp.MustCompile(`\.Bool\("([a-z0-9-]+)"\)`)
	used := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || filepath.Ext(name) != ".go" || hasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(cmdDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, m := range boolCall.FindAllStringSubmatch(string(src), -1) {
			used[m[1]] = true
		}
	}
	if len(used) == 0 {
		t.Fatal("found no args.Bool(...) calls in internal/cmd — scanner likely broken")
	}
	var missing []string
	for flag := range used {
		if !booleanFlags[flag] {
			missing = append(missing, flag)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("boolean flags used in internal/cmd but missing from booleanFlags: %v", missing)
	}
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
