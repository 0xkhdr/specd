package core

// RouteTransport identifies the dispatcher an advertised operation must reach.
type RouteTransport string

const (
	RouteCLI RouteTransport = "cli"
	RouteMCP RouteTransport = "mcp"
)

// RouteAuthority is the authority state observed by the guidance issuer.
type RouteAuthority string

const (
	RouteAuthorityAvailable RouteAuthority = "available"
	RouteAuthorityMissing   RouteAuthority = "missing"
	RouteAuthorityStale     RouteAuthority = "stale"
)

// RouteContext contains only facts a guidance issuer already resolved.
type RouteContext struct {
	Transport       RouteTransport
	Phase           Phase
	Actor           OperationActor
	Authority       RouteAuthority
	Issuer          string
	IssuerAvailable bool
}

type RouteBlocker struct {
	Code      string `json:"code"`
	Operation string `json:"operation"`
	Missing   string `json:"missing"`
}

type RouteHandoff struct {
	Operation        string         `json:"operation"`
	Actor            OperationActor `json:"actor"`
	MissingAuthority string         `json:"missing_authority,omitempty"`
	Command          string         `json:"command,omitempty"`
}

type RouteDecision struct {
	Operation  string         `json:"operation"`
	Transport  RouteTransport `json:"transport"`
	Executable bool           `json:"executable"`
	Handoff    *RouteHandoff  `json:"handoff,omitempty"`
	Blockers   []RouteBlocker `json:"blockers,omitempty"`
}

// ProjectRoute proves that an advertised operation can reach the selected
// transport with the current actor and authority. It reads nothing and keeps
// unavailable human/host authority out of agent-executable routes.
func ProjectRoute(operationID string, context RouteContext) RouteDecision {
	decision := RouteDecision{Operation: operationID, Transport: context.Transport}
	operation, ok := OperationByID(operationID)
	if !ok || !routeDispatches(operation, context.Transport) {
		decision.Blockers = append(decision.Blockers, RouteBlocker{Code: "ROUTE_DISPATCH_MISSING", Operation: operationID, Missing: "dispatch"})
		return decision
	}
	if !phaseAllowed(operation.AllowedPhases, context.Phase) {
		decision.Blockers = append(decision.Blockers, RouteBlocker{Code: "ROUTE_PHASE_INVALID", Operation: operationID, Missing: "phase"})
		return decision
	}
	if operation.Actor != context.Actor {
		decision.Handoff = &RouteHandoff{Operation: operation.ID, Actor: operation.Actor, Command: operationCommand(operation)}
		return decision
	}
	if operation.AuthorityRequired && context.Authority != RouteAuthorityAvailable {
		decision.Handoff = &RouteHandoff{Operation: operation.ID, Actor: ActorOperator, MissingAuthority: string(context.Authority), Command: context.Issuer}
		if !context.IssuerAvailable || context.Issuer == "" {
			decision.Blockers = append(decision.Blockers, RouteBlocker{Code: "ROUTE_ISSUER_MISSING", Operation: operation.ID, Missing: "issuer"})
		}
		return decision
	}
	decision.Executable = true
	return decision
}

func routeDispatches(operation Operation, transport RouteTransport) bool {
	switch transport {
	case RouteCLI:
		_, ok := CommandByName(operation.Command)
		return ok
	case RouteMCP:
		return !ForbiddenTool(operation.Command)
	default:
		return false
	}
}

func operationCommand(operation Operation) string {
	command := "specd " + operation.Command
	if operation.Subcommand != "" {
		command += " " + operation.Subcommand
	}
	return command
}
