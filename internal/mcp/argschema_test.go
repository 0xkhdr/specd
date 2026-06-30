package mcp

import "testing"

func TestValidateToolArgs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		command string
		args    map[string]any
		wantErr bool
	}{
		{"unknown_command_passes_through", "no-such-command", map[string]any{"anything": "x"}, false},
		{"no_arguments_ok", "status", map[string]any{}, false},
		{"nil_arguments_map_ok", "status", nil, false},
		{"declared_boolean_flag_with_bool_value_ok", "status", map[string]any{"all": true}, false},
		{"declared_boolean_flag_with_string_value_rejected", "status", map[string]any{"all": "yes"}, true},
		{"declared_boolean_flag_with_number_value_rejected", "status", map[string]any{"all": float64(1)}, true},
		{"nil_value_for_declared_flag_treated_as_omitted", "status", map[string]any{"all": nil}, false},
		{"undeclared_key_rejected", "status", map[string]any{"bogus": "x"}, true},
		{"args_array_of_strings_ok", "status", map[string]any{"args": []any{"widget"}}, false},
		{"args_must_be_array_not_string", "status", map[string]any{"args": "widget"}, true},
		{"non_boolean_flag_accepts_string", "fusion", map[string]any{"expect-config-digest": "abc123"}, false},
		{"non_boolean_flag_accepts_number", "brain", map[string]any{"max-retries": float64(2)}, false},
		{"non_boolean_flag_rejects_array", "fusion", map[string]any{"expect-config-digest": []any{"a", "b"}}, true},
		{"non_boolean_flag_rejects_object", "fusion", map[string]any{"expect-config-digest": map[string]any{"a": "b"}}, true},
		{"non_boolean_flag_rejects_bool", "fusion", map[string]any{"expect-config-digest": true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateToolArgs(tt.command, tt.args)
			if tt.wantErr && err == nil {
				t.Fatalf("validateToolArgs(%q, %v) = nil, want error", tt.command, tt.args)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("validateToolArgs(%q, %v) = %v, want nil", tt.command, tt.args, err)
			}
		})
	}
}
