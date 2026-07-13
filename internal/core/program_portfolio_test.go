package core

import (
	"reflect"
	"testing"
)

func TestPortfolioView(t *testing.T) {
	program := Program{Links: []ProgramLink{{From: "checkout", To: "auth"}}}
	inputs := []PortfolioSpec{
		{SpecID: "checkout", Deployments: []DeploymentV1{
			{ReleaseID: "r1", Environment: EnvironmentProduction, Status: StatusHealthy, Attempt: 1},
			{ReleaseID: "r2", Environment: EnvironmentProduction, Status: StatusFailed, Attempt: 2},
		}},
		{SpecID: "auth", Complete: false},
	}
	want := PortfolioView{Environments: []PortfolioEnvironment{{
		SpecID: "checkout", Environment: EnvironmentProduction, ReleaseID: "r2", Status: StatusFailed,
	}}, Blockers: []PortfolioBlocker{{SpecID: "checkout", BlockedBy: []string{"auth"}}}}
	got, err := BuildPortfolioView(program, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("portfolio = %#v, want %#v", got, want)
	}
	again, err := BuildPortfolioView(program, append([]PortfolioSpec(nil), inputs...))
	if err != nil || !reflect.DeepEqual(got, again) {
		t.Fatalf("portfolio not deterministic: %#v / %v", again, err)
	}
}
