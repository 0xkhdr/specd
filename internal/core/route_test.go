package core

import "testing"

func TestGuidanceDispatchParity(t *testing.T) {
	tests := []struct {
		name       string
		operation  string
		context    RouteContext
		executable bool
		blocker    string
		handoff    OperationActor
	}{
		{"cli route", "status", RouteContext{Transport: RouteCLI, Phase: PhaseExecute, Actor: ActorAgent, Authority: RouteAuthorityAvailable}, true, "", ""},
		{"unknown route", "missing", RouteContext{Transport: RouteCLI, Phase: PhaseExecute, Actor: ActorAgent}, false, "ROUTE_DISPATCH_MISSING", ""},
		{"human handoff", "approve", RouteContext{Transport: RouteCLI, Phase: PhaseExecute, Actor: ActorAgent, Authority: RouteAuthorityAvailable}, false, "", ActorHuman},
		{"missing issuer", "verify.task", RouteContext{Transport: RouteCLI, Phase: PhaseExecute, Actor: ActorAgent, Authority: RouteAuthorityMissing}, false, "ROUTE_ISSUER_MISSING", ActorOperator},
		{"stale authority", "verify.task", RouteContext{Transport: RouteCLI, Phase: PhaseExecute, Actor: ActorAgent, Authority: RouteAuthorityStale, Issuer: "specd brain status demo", IssuerAvailable: true}, false, "", ActorOperator},
		{"mcp policy", "approve", RouteContext{Transport: RouteMCP, Phase: PhaseExecute, Actor: ActorAgent, Authority: RouteAuthorityAvailable}, false, "ROUTE_DISPATCH_MISSING", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProjectRoute(tt.operation, tt.context)
			if got.Executable != tt.executable {
				t.Fatalf("executable = %v, want %v: %+v", got.Executable, tt.executable, got)
			}
			if tt.blocker != "" && (len(got.Blockers) == 0 || got.Blockers[0].Code != tt.blocker) {
				t.Fatalf("blocker = %+v, want %s", got.Blockers, tt.blocker)
			}
			if tt.handoff != "" && (got.Handoff == nil || got.Handoff.Actor != tt.handoff) {
				t.Fatalf("handoff = %+v, want actor %s", got.Handoff, tt.handoff)
			}
		})
	}
}
