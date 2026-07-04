package orchestration

import (
	"encoding/json"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

type Snapshot struct {
	Revision int64
	Phase    core.Phase
	Records  map[string]json.RawMessage
	Frontier []core.FrontierTask
	Leases   []Lease
	Now      time.Time
	Cost     int
}

func Sense(state core.State, frontier []core.FrontierTask, leases []Lease, now time.Time) Snapshot {
	records := make(map[string]json.RawMessage, len(state.Records))
	for key, value := range state.Records {
		records[key] = append(json.RawMessage(nil), value...)
	}
	return Snapshot{
		Revision: state.Revision,
		Phase:    state.Phase,
		Records:  records,
		Frontier: append([]core.FrontierTask(nil), frontier...),
		Leases:   append([]Lease(nil), leases...),
		Now:      now,
	}
}
