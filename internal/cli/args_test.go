package cli

import "testing"

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
}
