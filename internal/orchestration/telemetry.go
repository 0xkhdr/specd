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
