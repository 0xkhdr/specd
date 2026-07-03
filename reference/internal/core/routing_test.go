package core

import (
	"strings"
	"testing"
)

func TestResolveRoutingStampsDeterministicPolicyAndEconomics(t *testing.T) {
	cfg := RoutingCfg{
		Enabled:     true,
		DefaultTier: "small",
		TaskTiers:   map[string]string{"T2": "large"},
		Tiers: map[string]TierCfg{
			"large": {Model: "host-large", BudgetUSD: "2.50", MaxTokens: 200000},
			"small": {Model: "host-small", BudgetUSD: "1.00", MaxTokens: 50000},
		},
	}
	tasks := map[string]TaskState{
		"T1": {Telemetry: &Telemetry{Tokens: 1000, Cost: "0.10"}},
		"T2": {Telemetry: &Telemetry{Tokens: 2000, Cost: "0.25"}},
	}

	stamps, economics, err := ResolveRoutingStamps(cfg, tasks)
	if err != nil {
		t.Fatalf("ResolveRoutingStamps error: %v", err)
	}
	if stamps["T1"].Tier != "small" || stamps["T1"].RuleIndex != 1 || stamps["T1"].BudgetUSD != 1 {
		t.Fatalf("T1 stamp = %+v", stamps["T1"])
	}
	if stamps["T2"].Tier != "large" || stamps["T2"].RuleIndex != 0 || stamps["T2"].BudgetUSD != 2.5 {
		t.Fatalf("T2 stamp = %+v", stamps["T2"])
	}
	if economics.TotalCostUSD != "0.35" || economics.TotalTokens != 3000 {
		t.Fatalf("economics = %+v", economics)
	}
	if economics.ByTier["large"].CostUSD != "0.25" || economics.ByTier["small"].CostUSD != "0.1" {
		t.Fatalf("by tier economics = %+v", economics.ByTier)
	}
}

func TestValidateRoutingConfigRejectsUnknownTierAndNegativeBudget(t *testing.T) {
	cfg := RoutingCfg{
		Enabled:     true,
		DefaultTier: "small",
		TaskTiers:   map[string]string{"T1": "missing"},
		Tiers:       map[string]TierCfg{"small": {BudgetUSD: "-1"}},
	}
	if err := ValidateRoutingConfig(&cfg); err == nil {
		t.Fatal("ValidateRoutingConfig accepted invalid routing config")
	}
}

func TestRoutingEconomicsReportFromState(t *testing.T) {
	state := &State{
		Tasks: map[string]TaskState{
			"T1": {Telemetry: &Telemetry{Tokens: 100, Cost: "0.01"}},
			"T2": {Telemetry: &Telemetry{Tokens: 200, Cost: "0.02"}},
		},
		Routing: map[string]RoutingStamp{
			"T1": {Tier: "small", BudgetUSD: 1, RuleIndex: 1},
			"T2": {Tier: "large", BudgetUSD: 2, RuleIndex: 0},
		},
	}

	md := RenderMarkdown(ReportData{State: state})
	for _, want := range []string{
		"Token Economics",
		"Total: **0.03 USD**",
		"| large | 1 | 200 | 0.02 |",
		"| small | 1 | 100 | 0.01 |",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("report missing %q:\n%s", want, md)
		}
	}
}
