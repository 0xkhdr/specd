package orchestration

import "time"

type Lease struct {
	TaskID    string    `json:"task_id"`
	WorkerID  string    `json:"worker_id"`
	ExpiresAt time.Time `json:"expires_at"`
	Retries   int       `json:"retries"`
}

type Reclaim struct {
	TaskID string
	Retry  bool
	Reason string
}

func ReclaimExpired(leases []Lease, now time.Time, maxRetries int) []Reclaim {
	var reclaimed []Reclaim
	for _, lease := range leases {
		if now.Before(lease.ExpiresAt) {
			continue
		}
		reclaimed = append(reclaimed, Reclaim{
			TaskID: lease.TaskID,
			Retry:  lease.Retries < maxRetries,
			Reason: "lease expired",
		})
	}
	return reclaimed
}

func Escalation(leases []Lease, maxRetries int, now time.Time) Reclaim {
	for _, reclaim := range ReclaimExpired(leases, now, maxRetries) {
		if !reclaim.Retry {
			reclaim.Reason = "retry limit exceeded"
			return reclaim
		}
	}
	return Reclaim{}
}
