package core

import "testing"

func TestValidateSlug(t *testing.T) {
	tests := []struct {
		name    string
		slug    string
		wantErr bool
		// wantCode is checked only when wantErr is true.
		wantCode int
	}{
		{"simple_lowercase_ok", "auth", false, 0},
		{"with_internal_hyphen_ok", "user-auth-2", false, 0},
		{"single_char_ok", "a", false, 0},
		{"empty_is_usage_error", "", true, ExitUsage},
		{"leading_hyphen_rejected", "-auth", true, ExitUsage},
		{"uppercase_rejected", "Auth", true, ExitUsage},
		{"underscore_rejected", "user_auth", true, ExitUsage},
		{"path_separator_rejected", "a/b", true, ExitUsage},
		{"parent_traversal_rejected", "..", true, ExitUsage},
		{"nested_traversal_rejected", "../../etc/passwd", true, ExitUsage},
		{"absolute_path_rejected", "/etc/passwd", true, ExitUsage},
		{"space_rejected", "a b", true, ExitUsage},
		{"shell_metachar_rejected", "a;rm", true, ExitUsage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			err := ValidateSlug(tt.slug)

			// Assert
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ValidateSlug(%q) = nil, want error", tt.slug)
				}
				se, ok := IsSpecdError(err)
				if !ok {
					t.Fatalf("ValidateSlug(%q) error is not *SpecdError: %v", tt.slug, err)
				}
				if se.Code != tt.wantCode {
					t.Errorf("ValidateSlug(%q) code = %d, want %d", tt.slug, se.Code, tt.wantCode)
				}
				return
			}
			if err != nil {
				t.Errorf("ValidateSlug(%q) = %v, want nil", tt.slug, err)
			}
		})
	}
}
