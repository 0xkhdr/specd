package core

import "testing"

func TestConfigDocsSamplesParse(t *testing.T) {
	samples := map[string]string{
		"command-reference-config-yml": `version: 2
defaults:
  verify_command: "npm test"
  report_format: "md"
  subagent_mode: "inline"
  promotion_threshold: 3
report:
  format: "md"
  auto_refresh_seconds: 0
roles:
  subagent_mode: "inline"
gates:
  traceability: "warn"
  acceptance: "off"
  scope: "off"
  custom: []
verify:
  sandbox: "none"
orchestration:
  enabled: false
  approval_policy: "manual"
  worker_mode: "host"
  max_workers: 4
  max_retries: 2
  session_timeout_minutes: 120
  host_reported_cost_limit_usd: 0
  transport:
    kind: "file"
    poll_interval_millis: 500
    message_ttl_seconds: 3600
    lease_seconds: 120
    heartbeat_seconds: 30
  program:
    max_concurrent_specs: 2
`,
		"user-guide-project-config": `version: 2
defaults:
  verify_command: "make test"
verify:
  sandbox: "bwrap"
orchestration:
  enabled: true
  approval_policy: "planning"
  max_workers: 4
`,
	}
	for name, raw := range samples {
		t.Run(name, func(t *testing.T) {
			doc, err := parseSimpleYAML(raw)
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateConfigDoc(doc); err != nil {
				t.Fatal(err)
			}
			cfg := DefaultConfig
			applyConfigDoc(&cfg, doc)
			if err := ValidateOrchestrationConfig(&cfg.Orchestration); err != nil {
				t.Fatal(err)
			}
		})
	}
}
