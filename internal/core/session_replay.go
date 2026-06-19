package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// SessionTimelineEvent is a human-facing, replayable view of ACP session events.
// It preserves deterministic ordering while surfacing Brain decision intent.
type SessionTimelineEvent struct {
	At         string `json:"at,omitempty"`
	Sequence   uint64 `json:"sequence"`
	Type       string `json:"type"`
	Spec       string `json:"spec,omitempty"`
	Task       string `json:"task,omitempty"`
	Action     string `json:"action,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Escalation string `json:"escalation,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

// ReplaySessionTimeline normalizes ACP envelopes into a stable session timeline.
func ReplaySessionTimeline(events []ACPEnvelope) []SessionTimelineEvent {
	timeline := make([]SessionTimelineEvent, 0, len(events))
	for _, event := range events {
		item := SessionTimelineEvent{
			At:       event.CreatedAt,
			Sequence: event.Sequence,
			Type:     string(event.Type),
			Spec:     event.Spec,
			Task:     event.Task,
		}
		if event.Decision != nil {
			item.Action = string(event.Decision.Action)
			item.Reason = event.Decision.Reason
			item.Escalation = string(event.Decision.Escalation.Code)
			if item.Spec == "" {
				item.Spec = event.Decision.Spec
			}
			if item.Task == "" {
				item.Task = event.Decision.TaskID
			}
		}
		applyPayloadDetail(&item, event.Payload)
		timeline = append(timeline, item)
	}
	sort.SliceStable(timeline, func(i, j int) bool {
		if timeline[i].Sequence != timeline[j].Sequence {
			return timeline[i].Sequence < timeline[j].Sequence
		}
		if timeline[i].At != timeline[j].At {
			return timeline[i].At < timeline[j].At
		}
		return timeline[i].Type < timeline[j].Type
	})
	return timeline
}

// ExplainCurrentSessionDecision returns a concise explanation for the latest Brain decision.
func ExplainCurrentSessionDecision(events []ACPEnvelope) (SessionTimelineEvent, bool) {
	timeline := ReplaySessionTimeline(events)
	for i := len(timeline) - 1; i >= 0; i-- {
		if timeline[i].Action != "" || timeline[i].Reason != "" || timeline[i].Escalation != "" {
			return timeline[i], true
		}
	}
	if len(timeline) == 0 {
		return SessionTimelineEvent{}, false
	}
	return timeline[len(timeline)-1], true
}

func FormatSessionTimelineEvent(event SessionTimelineEvent) string {
	parts := []string{fmt.Sprintf("%020d", event.Sequence), event.Type}
	if event.Action != "" {
		parts = append(parts, "action="+event.Action)
	}
	if event.Spec != "" {
		parts = append(parts, "spec="+event.Spec)
	}
	if event.Task != "" {
		parts = append(parts, "task="+event.Task)
	}
	if event.Escalation != "" && event.Escalation != string(EscalationNone) {
		parts = append(parts, "escalation="+event.Escalation)
	}
	if event.Reason != "" {
		parts = append(parts, "reason="+event.Reason)
	}
	if event.Detail != "" {
		parts = append(parts, "detail="+event.Detail)
	}
	return strings.Join(parts, " ")
}

func applyPayloadDetail(item *SessionTimelineEvent, raw json.RawMessage) {
	if len(raw) == 0 {
		return
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}
	if item.Reason == "" {
		item.Reason = stringFromPayload(payload, "reason")
	}
	switch item.Type {
	case string(ACPMessageProgress):
		msg := stringFromPayload(payload, "message")
		if percent, ok := payload["percent"].(float64); ok && msg != "" {
			item.Detail = fmt.Sprintf("%.0f%% %s", percent, msg)
		} else if msg != "" {
			item.Detail = msg
		}
	case string(ACPMessageEvidence), string(ACPMessageBlocker):
		item.Detail = stringFromPayload(payload, "summary")
		if item.Detail == "" {
			item.Detail = stringFromPayload(payload, "blocker")
		}
	case string(ACPMessageMission):
		role := stringFromPayload(payload, "role")
		if role != "" {
			item.Detail = "role=" + role
		}
	case string(ACPMessageAccepted), string(ACPMessageHeartbeat):
		worker := stringFromPayload(payload, "workerId")
		if worker != "" {
			item.Detail = "worker=" + worker
		}
	}
}

func stringFromPayload(payload map[string]any, key string) string {
	value, ok := payload[key].(string)
	if !ok {
		return ""
	}
	return value
}
