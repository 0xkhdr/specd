package core

import "strings"

type RequestMode string

const (
	RequestModeGeneral RequestMode = "general"
	RequestModeConsult RequestMode = "consult"
	RequestModeManaged RequestMode = "managed"
)

type RequestModeSource string

const (
	RequestModeSourceDirective RequestModeSource = "explicit_directive"
	RequestModeSourceSession   RequestModeSource = "active_session"
	RequestModeSourceRule      RequestModeSource = "repository_rule"
	RequestModeSourceConfig    RequestModeSource = "configured_default"
	RequestModeSourceCompiled  RequestModeSource = "compiled_default"
)

type RequestModeRule struct {
	Mode        RequestMode
	Path        string
	Enforceable bool
}

type RequestModeInput struct {
	ExplicitDirective RequestMode
	ActiveSession     RequestMode
	SessionSlug       string
	RepositoryRule    *RequestModeRule
	ConfiguredDefault RequestMode
	SelectedSpec      string
	IntakeRoute       string
	HostCapabilities  HostContract
	PreviousMode      RequestMode
	PreviousSpec      string
}

type RequestModeResolution struct {
	Mode                    RequestMode       `json:"request_mode"`
	Source                  RequestModeSource `json:"request_mode_source"`
	Enforcement             string            `json:"request_mode_enforcement"`
	SelectedSpec            string            `json:"selected_spec,omitempty"`
	Assurance               AssuranceLevel    `json:"host_assurance"`
	MissingHostCapabilities []string          `json:"missing_host_capabilities"`
	PermittedOperations     []string          `json:"permitted_operations"`
	HandshakeRequired       bool              `json:"handshake_required"`
	AuthorityInvalidated    bool              `json:"authority_invalidated"`
	Blocker                 string            `json:"blocker,omitempty"`
	Recovery                string            `json:"recovery,omitempty"`
}

// ResolveRequestMode is the single deterministic request-routing policy. A
// repository existing on disk is deliberately not an input: absence of an
// activation signal therefore always resolves to general.
func ResolveRequestMode(input RequestModeInput) (RequestModeResolution, error) {
	if input.RepositoryRule != nil && input.RepositoryRule.Enforceable &&
		input.RepositoryRule.Mode != RequestModeGeneral && input.ExplicitDirective == RequestModeGeneral {
		message := "general mode conflicts with enforced repository rule at " + input.RepositoryRule.Path
		return RequestModeResolution{Mode: RequestModeGeneral, Source: RequestModeSourceDirective, Enforcement: "required", Blocker: message, Recovery: "select " + string(input.RepositoryRule.Mode) + " mode or change the governing rule"},
			Refuse("REQUEST_MODE_CONFLICT", message)
	}

	mode, source, slug := input.ExplicitDirective, RequestModeSourceDirective, input.SelectedSpec
	if mode == "" {
		mode, source, slug = input.ActiveSession, RequestModeSourceSession, input.SessionSlug
	}
	if mode == "" && input.RepositoryRule != nil && input.RepositoryRule.Enforceable {
		mode, source = input.RepositoryRule.Mode, RequestModeSourceRule
	}
	if mode == "" {
		mode, source = input.ConfiguredDefault, RequestModeSourceConfig
	}
	if mode == "" {
		mode, source = RequestModeGeneral, RequestModeSourceCompiled
	}
	if !validRequestMode(mode) {
		return RequestModeResolution{}, Refusef("REQUEST_MODE_INVALID", "unknown request mode %q", mode)
	}

	conformance := EvaluateHostContract(input.HostCapabilities)
	resolution := RequestModeResolution{
		Mode: mode, Source: source, Enforcement: "advisory", SelectedSpec: slug,
		Assurance: conformance.Assurance, MissingHostCapabilities: append([]string{}, conformance.Unmet...),
		PermittedOperations: []string{},
	}
	if source == RequestModeSourceRule && input.RepositoryRule.Enforceable {
		resolution.Enforcement = "required"
	}
	if mode == RequestModeManaged {
		resolution.HandshakeRequired = true
		if slug == "" && strings.TrimSpace(input.IntakeRoute) == "" {
			resolution.Blocker = "managed mode requires an explicit spec or intake route"
			resolution.Recovery = "select a spec or provide an intake route"
			return resolution, Refuse("MANAGED_TARGET_REQUIRED", resolution.Blocker)
		}
	}
	for _, operation := range Operations {
		if ForbiddenTool(operation.Command) || mode == RequestModeGeneral || (mode == RequestModeConsult && operation.Effect != EffectRead) {
			continue
		}
		resolution.PermittedOperations = append(resolution.PermittedOperations, operation.ID)
	}
	resolution.AuthorityInvalidated = input.PreviousMode != "" &&
		(input.PreviousMode != mode || (mode == RequestModeManaged && input.PreviousSpec != slug))
	return resolution, nil
}

func validRequestMode(mode RequestMode) bool {
	return mode == RequestModeGeneral || mode == RequestModeConsult || mode == RequestModeManaged
}
