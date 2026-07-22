package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
	"github.com/0xkhdr/specd/internal/orchestration"
)

func brainWorkerReport(root, sessionPath, acpPath, slug string, args []string) error {
	if len(args) != 2 {
		return errors.New("usage: specd brain report <lease-id> <worker-id>")
	}
	leaseID, workerID := args[0], args[1]
	s, err := orchestration.LoadSession(sessionPath)
	if err != nil {
		return err
	}
	li := -1
	var l orchestration.Lease
	for i, candidate := range s.Leases {
		if candidate.LeaseID == leaseID {
			li, l = i, candidate
			break
		}
	}
	if li < 0 {
		return fmt.Errorf("LEASE_NOT_FOUND: %s", leaseID)
	}
	var m orchestration.MissionV1
	found := false
	for _, candidate := range s.Missions {
		if candidate.MissionID == l.MissionID {
			m, found = candidate, true
			break
		}
	}
	if !found {
		return fmt.Errorf("MISSION_NOT_FOUND: %s", l.MissionID)
	}
	now := time.Now()
	head := gitHead(root)
	r := orchestration.WorkerReportV1{MissionID: m.MissionID, LeaseID: l.LeaseID, WorkerID: workerID, TaskID: m.TaskID, Attempt: m.Attempt, Role: m.Role, SubjectHead: head, VerifyRef: "evidence.jsonl#" + m.TaskID, Status: "complete"}
	// R6.1: a lease bound to a driver session may only be acted on under that
	// session. An unbound lease is unaffected, and a closed or expired session
	// leaves nothing to match against, so this refuses only a genuine mismatch:
	// a second host reporting against work another session holds.
	if l.DriverSessionID != "" {
		driver, loadErr := core.LoadDriverSession(core.DriverSessionPath(root, slug))
		if loadErr != nil {
			return loadErr
		}
		if err := orchestration.ValidateLeaseSession(l, driver.ID); err != nil {
			return err
		}
	}
	if err := orchestration.ValidateWorkerReport(r, m, l, now); err != nil {
		return err
	}
	diff, err := core.DeriveDiff(root, m.SubjectHead)
	if err != nil {
		return err
	}
	if err := gates.CheckScope(diff.Paths, m.DeclaredFiles); err != nil {
		return err
	}
	records, err := core.LoadEvidence(core.EvidencePath(root, slug))
	if err != nil {
		return err
	}
	if err := acceptWorkerReport(records, workerReport{TaskID: m.TaskID, WorkerID: workerID, GitHead: head, Lease: l, Now: now}); err != nil {
		return err
	}
	// Share the manual path's run allocator so this Brain attempt lands on the
	// task's run chain (spec 07 R2.2). Deviation: T08 lists brain_worker.go but
	// the caller with root/slug in hand is here.
	if err := allocateWorkerRun(root, slug, m.TaskID, head, workerID); err != nil {
		return err
	}
	if err := runTaskComplete(root, []string{slug, m.TaskID}, nil); err != nil {
		return err
	}
	_, err = core.WithSpecLock(root, func() (struct{}, error) {
		current, err := orchestration.LoadSession(sessionPath)
		if err != nil {
			return struct{}{}, err
		}
		for i := range current.Leases {
			if current.Leases[i].LeaseID == leaseID {
				current.Leases = append(current.Leases[:i], current.Leases[i+1:]...)
				break
			}
		}
		if err := orchestration.SaveSessionCAS(root, sessionPath, current.Revision, current); err != nil {
			return struct{}{}, err
		}
		payload, _ := json.Marshal(r)
		if err := orchestration.AppendACP(acpPath, orchestration.ACPEvent{Time: now, Kind: orchestration.ACPKindReport, MissionID: m.MissionID, TaskID: m.TaskID, Attempt: m.Attempt, GitHead: head, VerifyRef: r.VerifyRef, Payload: string(payload)}); err != nil {
			return struct{}{}, err
		}
		fmt.Fprintf(os.Stdout, "brain report: completed task %s from worker %s\n", m.TaskID, workerID)
		return struct{}{}, nil
	})
	return err
}
