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

func brainClaim(root, sessionPath, acpPath, slug string, args []string) error {
	if len(args) != 3 {
		return errors.New("usage: specd brain claim <spec> <mission-id> <worker-id> <role>")
	}
	missionID, workerID, role := args[0], args[1], args[2]
	_, err := core.WithSpecLock(root, func() (struct{}, error) {
		s, err := orchestration.LoadSession(sessionPath)
		if err != nil {
			return struct{}{}, err
		}
		idx := -1
		var m orchestration.MissionV1
		for i, candidate := range s.PendingMissions {
			if candidate.MissionID == missionID {
				idx, m = i, candidate
				break
			}
		}
		if idx < 0 {
			return struct{}{}, fmt.Errorf("MISSION_NOT_PENDING: %s", missionID)
		}
		now := time.Now()
		if err := orchestration.CheckClaimConflict(s.Leases, m, now); err != nil {
			return struct{}{}, err
		}
		e := orchestration.ClaimEcho{MissionID: m.MissionID, TaskID: m.TaskID, Role: role, ContextDigest: m.ContextDigest, ConfigDigest: m.ConfigDigest, PaletteDigest: m.PaletteDigest, AuthorityRef: m.AuthorityRef, SubjectHead: m.SubjectHead}
		l, err := orchestration.ClaimMission(m, orchestration.WorkerV1{WorkerID: workerID, Host: "local", Roles: []string{role}, Capabilities: []string{"edit", "verify"}}, e, now, brainLeaseTTL)
		if err != nil {
			return struct{}{}, err
		}
		s.PendingMissions = append(s.PendingMissions[:idx], s.PendingMissions[idx+1:]...)
		s.Missions = append(s.Missions, m)
		s.Leases = append(s.Leases, l)
		if err := orchestration.SaveSessionCAS(root, sessionPath, s.Revision, s); err != nil {
			return struct{}{}, err
		}
		payload, _ := json.Marshal(l)
		if err := orchestration.AppendClaim(acpPath, orchestration.ACPEvent{Time: now, MissionID: m.MissionID, TaskID: m.TaskID, Attempt: m.Attempt, Payload: string(payload)}); err != nil {
			return struct{}{}, err
		}
		fmt.Fprintf(os.Stdout, "brain claim: granted lease %s for mission %s to %s\n", l.LeaseID, m.MissionID, workerID)
		return struct{}{}, nil
	})
	return err
}
