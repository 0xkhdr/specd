package orchestration

import (
	"encoding/json"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

type Snapshot struct {
	Revision       int64
	Phase          core.Phase
	Records        map[string]json.RawMessage
	Frontier       []core.FrontierTask
	Leases         []Lease
	Now            time.Time
	CostMicros     int64
	Tokens         int64
	TelemetryKnown bool
	// TelemetryTrusted is true only when the known cost derives entirely from
	// trusted sources; a worker-reported accounting hint leaves it false so a
	// brake fired on it is labelled untrusted (spec 07 R4.1, R4.3).
	TelemetryTrusted bool
}

// Sense builds the immutable decision snapshot. Cost/token accounting is
// populated only from accepted telemetry (see AccrueTelemetry): production and
// tests share one honest population path, and absent telemetry stays unknown
// rather than being zero-filled (spec 07 R4.1).
func Sense(state core.State, frontier []core.FrontierTask, leases []Lease, telemetry Telemetry, now time.Time) Snapshot {
	records := make(map[string]json.RawMessage, len(state.Records))
	for key, value := range state.Records {
		records[key] = append(json.RawMessage(nil), value...)
	}
	return Snapshot{
		Revision:         state.Revision,
		Phase:            state.Phase,
		Records:          records,
		Frontier:         append([]core.FrontierTask(nil), frontier...),
		Leases:           append([]Lease(nil), leases...),
		Now:              now,
		CostMicros:       telemetry.CostMicros,
		Tokens:           telemetry.Tokens,
		TelemetryKnown:   telemetry.Known,
		TelemetryTrusted: telemetry.Trusted,
	}
}
