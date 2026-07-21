package core

import "testing"

func TestRequestModeResolution(t *testing.T) {
	t.Run("defaults_general_without_activation", func(t *testing.T) {
		got, err := ResolveRequestMode(RequestModeInput{})
		if err != nil || got.Mode != RequestModeGeneral || got.Source != RequestModeSourceCompiled || got.HandshakeRequired || len(got.PermittedOperations) != 0 {
			t.Fatalf("resolution = %+v, err = %v", got, err)
		}
	})
	t.Run("precedence_and_consult_filter", func(t *testing.T) {
		got, err := ResolveRequestMode(RequestModeInput{ExplicitDirective: RequestModeConsult, ActiveSession: RequestModeManaged, SessionSlug: "demo"})
		if err != nil || got.Mode != RequestModeConsult || got.Source != RequestModeSourceDirective {
			t.Fatalf("resolution = %+v, err = %v", got, err)
		}
		for _, id := range got.PermittedOperations {
			op, _ := OperationByID(id)
			if op.Effect != EffectRead {
				t.Fatalf("consult exposed mutable operation %s", id)
			}
		}
	})
	t.Run("managed_requires_target", func(t *testing.T) {
		got, err := ResolveRequestMode(RequestModeInput{ExplicitDirective: RequestModeManaged})
		if err == nil || got.Blocker == "" || !got.HandshakeRequired {
			t.Fatalf("resolution = %+v, err = %v", got, err)
		}
	})
	t.Run("enforced_rule_refuses_general", func(t *testing.T) {
		rule := &RequestModeRule{Mode: RequestModeManaged, Path: "/repo/AGENTS.md", Enforceable: true}
		got, err := ResolveRequestMode(RequestModeInput{ExplicitDirective: RequestModeGeneral, RepositoryRule: rule})
		if err == nil || got.Blocker == "" || got.Enforcement != "required" {
			t.Fatalf("resolution = %+v, err = %v", got, err)
		}
	})
	t.Run("mode_or_slug_change_invalidates_authority", func(t *testing.T) {
		got, err := ResolveRequestMode(RequestModeInput{ExplicitDirective: RequestModeManaged, SelectedSpec: "new", PreviousMode: RequestModeManaged, PreviousSpec: "old"})
		if err != nil || !got.AuthorityInvalidated {
			t.Fatalf("resolution = %+v, err = %v", got, err)
		}
	})
}
