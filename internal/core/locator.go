package core

// Locator is the "where am I" block carried on machine-readable responses. An
// agent that has to re-derive its own position — which spec, which phase, which
// revision, what it is allowed to do next — either spends a round trip on it or
// guesses. Both are avoidable, so every machine surface states it (R5.1).
//
// It is purely additive: it is emitted alongside an existing payload, never in
// place of one, so a consumer that does not know these fields still parses.
type Locator struct {
	Spec            string         `json:"spec"`
	Phase           Phase          `json:"phase"`
	Status          Status         `json:"status"`
	Revision        int64          `json:"revision"`
	ActorClass      OperationActor `json:"actor_class"`
	Authority       string         `json:"authority"`
	LegalOperations []string       `json:"legal_operations"`
	HumanOnly       []string       `json:"human_only"`
	Assurance       AssuranceLevel `json:"assurance"`
	Authoritative   bool           `json:"authoritative"`
}

// Authority states for Locator.Authority. "none" is the honest default: no
// token was presented, so nothing was delegated.
const (
	AuthorityNone    = "none"
	AuthorityScoped  = "scoped"
	AuthorityExpired = "expired"
)

// NewLocator assembles the block from state the caller already holds. It reads
// nothing and decides nothing — the guidance it is handed already computed what
// is legal in this phase, so the locator cannot widen it.
//
// authority is the caller's resolved authority state; pass AuthorityNone when no
// token was presented. host caps the assurance level: an unsandboxed host gets
// advisory, never a governed-looking response (R3.2).
func NewLocator(slug string, revision int64, g Guidance, actor OperationActor, authority string, host HostCapabilities) Locator {
	if authority == "" {
		authority = AuthorityNone
	}
	return Locator{
		Spec:            slug,
		Phase:           g.Phase,
		Status:          g.Status,
		Revision:        revision,
		ActorClass:      actor,
		Authority:       authority,
		LegalOperations: append([]string(nil), g.LegalCommands...),
		HumanOnly:       append([]string(nil), g.HumanOnly...),
		Assurance:       AssuranceCeiling(host),
		// A locator built from a loaded state.json describes committed state at
		// the stated revision. A projection or preview must pass this false.
		Authoritative: true,
	}
}
