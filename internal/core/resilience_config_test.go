package core

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestResilienceConfigByteStable proves the default config marshals without a
// `resilience` key, so adding the block keeps existing config.json byte-identical.
func TestResilienceConfigByteStable(t *testing.T) {
	if DefaultConfig.Orchestration.Resilience != nil {
		t.Fatal("default config must leave resilience unset")
	}
	raw, err := json.Marshal(DefaultConfig.Orchestration)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "resilience") {
		t.Fatalf("default orchestration config must not emit resilience: %s", raw)
	}
}

func TestResilienceConfigMaxAgeValidation(t *testing.T) {
	cfg := DefaultConfig.Orchestration
	cfg.Resilience = &ResilienceCfg{
		CheckpointEnabled: true,
		AutoResume:        AutoResumeCfg{Enabled: true, MaxAgeMinutes: -1},
	}
	if err := ValidateOrchestrationConfig(&cfg); err == nil {
		t.Fatal("negative maxAgeMinutes must fail validation")
	}

	cfg.Resilience.AutoResume.MaxAgeMinutes = 30
	if err := ValidateOrchestrationConfig(&cfg); err != nil {
		t.Fatalf("valid resilience config rejected: %v", err)
	}
}
