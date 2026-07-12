package orchestration

import (
	"fmt"
	"strings"
	"unicode"
)

const MaxObservationText = 256
const maxRouteIdentity = 128

type RouteFact struct {
	Class    string `json:"class,omitempty"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// ObservationV1 carries bounded audit facts. It is never evidence or proof of
// completion. Cost uses integer micro-units; Known distinguishes zero from absent.
type ObservationV1 struct {
	Version    string     `json:"version"`
	Known      bool       `json:"known"`
	Source     string     `json:"source,omitempty"`
	Unit       string     `json:"unit,omitempty"`
	CostMicros int64      `json:"cost_micros,omitempty"`
	Tokens     int64      `json:"tokens,omitempty"`
	DurationMs int64      `json:"duration_ms,omitempty"`
	Route      *RouteFact `json:"route,omitempty"`
}

// Telemetry is the cost/token total accrued from accepted report observations.
// Known distinguishes an unknown total (nil) from a real zero — an honest cost
// brake never treats an absent measurement as zero. Trusted is true only when
// every contributing observation came from a trusted (non worker-self-reported)
// source; worker telemetry is an accounting hint (spec 07 R4.1, R4.3).
type Telemetry struct {
	CostMicros int64
	Tokens     int64
	Known      bool
	Trusted    bool
}

// AccrueTelemetry folds accepted report observations into one honest total.
// Only observations already accepted onto the ledger reach here, so production
// and tests share exactly one population path (R4.1). A single unknown
// observation makes the whole total unknown — it is never zero-filled — so the
// aggregate is Known only when every report is Known.
func AccrueTelemetry(events []ACPEvent) Telemetry {
	seen := false
	total := Telemetry{Trusted: true}
	for _, event := range events {
		if event.Kind != ACPKindReport || event.Observation == nil {
			continue
		}
		obs := *event.Observation
		if !obs.Known {
			return Telemetry{}
		}
		seen = true
		if !trustedSource(obs.Source) {
			total.Trusted = false
		}
		total.CostMicros += obs.CostMicros
		total.Tokens += obs.Tokens
	}
	if !seen {
		return Telemetry{}
	}
	total.Known = true
	return total
}

// trustedSource reports whether a report source is authoritative rather than a
// self-reported accounting hint. Worker telemetry is an unverified hint;
// host-measured, adapter-mediated, and provider-attested usage are trusted.
func trustedSource(source string) bool {
	return source == "host" || source == "adapter" || source == "attested"
}

func NormalizeObservation(in ObservationV1) (ObservationV1, error) {
	if in.Version != "1" {
		return ObservationV1{}, fmt.Errorf("OBSERVATION_VERSION_UNSUPPORTED")
	}
	if in.CostMicros < 0 || in.Tokens < 0 || in.DurationMs < 0 {
		return ObservationV1{}, fmt.Errorf("OBSERVATION_VALUE_INVALID")
	}
	if !in.Known {
		if in.Source != "" || in.Unit != "" || in.CostMicros != 0 || in.Tokens != 0 || in.DurationMs != 0 || in.Route != nil {
			return ObservationV1{}, fmt.Errorf("OBSERVATION_UNKNOWN_HAS_FACTS")
		}
		return in, nil
	}
	if !oneOf(in.Source, "worker", "adapter", "host", "attested") {
		return ObservationV1{}, fmt.Errorf("OBSERVATION_SOURCE_INVALID")
	}
	if in.Unit != "micro-usd" {
		return ObservationV1{}, fmt.Errorf("OBSERVATION_UNIT_INVALID")
	}
	if in.Route != nil {
		route := *in.Route
		route.Class = strings.TrimSpace(route.Class)
		route.Provider = strings.TrimSpace(route.Provider)
		route.Model = strings.TrimSpace(route.Model)
		route.Reason = truncate(strings.TrimSpace(route.Reason), MaxObservationText)
		for _, value := range []string{route.Class, route.Provider, route.Model, route.Reason} {
			if unsafeObservationText(value) {
				return ObservationV1{}, fmt.Errorf("OBSERVATION_ROUTE_REDACTION_REQUIRED")
			}
		}
		if len(route.Class) > maxRouteIdentity || len(route.Provider) > maxRouteIdentity || len(route.Model) > maxRouteIdentity {
			return ObservationV1{}, fmt.Errorf("OBSERVATION_ROUTE_TOO_LARGE")
		}
		in.Route = &route
	}
	return in, nil
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}
func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
func unsafeObservationText(value string) bool {
	lower := strings.ToLower(value)
	if strings.Contains(lower, "secret=") || strings.Contains(lower, "token=") || strings.Contains(lower, "api_key") {
		return true
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}
