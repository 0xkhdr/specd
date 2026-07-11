package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/orchestration"
)

func brainHeartbeat(root, sessionPath, acpPath, slug string, args []string) error {
	if len(args) != 2 {
		return errors.New("usage: specd brain heartbeat <spec> <lease-id> <worker-id>")
	}
	leaseID, workerID := args[0], args[1]
	_, err := core.WithSpecLock(root, func() (struct{}, error) {
		s, err := orchestration.LoadSession(sessionPath)
		if err != nil {
			return struct{}{}, err
		}
		idx := -1
		for i := range s.Leases {
			if s.Leases[i].LeaseID == leaseID {
				idx = i
				break
			}
		}
		if idx < 0 {
			return struct{}{}, fmt.Errorf("LEASE_NOT_FOUND: %s", leaseID)
		}
		now := time.Now()
		l := s.Leases[idx]
		next, err := orchestration.RenewLease(l, orchestration.HeartbeatV1{LeaseID: leaseID, MissionID: l.MissionID, WorkerID: workerID, Attempt: l.Attempt, At: now}, brainLeaseTTL, 4*brainLeaseTTL)
		if err != nil {
			return struct{}{}, err
		}
		s.Leases[idx] = next
		if err := orchestration.SaveSessionCAS(root, sessionPath, s.Revision, s); err != nil {
			return struct{}{}, err
		}
		payload, _ := json.Marshal(next)
		if err := orchestration.AppendACP(acpPath, orchestration.ACPEvent{Time: now, Kind: "heartbeat", MissionID: l.MissionID, TaskID: l.TaskID, Attempt: l.Attempt, Payload: string(payload)}); err != nil {
			return struct{}{}, err
		}
		fmt.Fprintf(os.Stdout, "brain heartbeat: renewed lease %s\n", leaseID)
		return struct{}{}, nil
	})
	return err
}
