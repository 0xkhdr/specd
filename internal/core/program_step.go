package core

type ProgramChildStep struct {
	Slug      string                  `json:"slug"`
	SessionID string                  `json:"sessionId"`
	Result    OrchestrationStepResult `json:"result"`
}

type ProgramStepResult struct {
	Snapshot ProgramSnapshot     `json:"snapshot"`
	Decision ProgramDecision     `json:"decision"`
	Started  []ProgramChildLease `json:"started"`
	Stepped  []ProgramChildStep  `json:"stepped"`
	Leases   []ProgramChildLease `json:"leases"`
}

func StepProgramOrchestration(root, parentSessionID string, policy OrchestrationPolicy, cfg OrchestrationCfg) (ProgramStepResult, error) {
	if err := validateACPOpaqueID("parent session ID", parentSessionID); err != nil {
		return ProgramStepResult{}, err
	}
	if err := ValidateOrchestrationPolicy(policy); err != nil {
		return ProgramStepResult{}, err
	}
	if err := ValidateOrchestrationConfig(&cfg); err != nil {
		return ProgramStepResult{}, err
	}
	programSession, err := ensureProgramSession(root, parentSessionID)
	if err != nil {
		return ProgramStepResult{}, err
	}

	graph, err := BuildProgram(root, nil)
	if err != nil {
		return ProgramStepResult{}, err
	}
	if err := releaseCompleteProgramChildren(root, graph); err != nil {
		return ProgramStepResult{}, err
	}
	runtime, err := programChildRuntime(root)
	if err != nil {
		return ProgramStepResult{}, err
	}
	snapshot, err := BuildProgramSnapshotWithRuntime(graph, runtime, cfg.Program.MaxConcurrentSpecs)
	if err != nil {
		return ProgramStepResult{}, err
	}
	result := ProgramStepResult{Snapshot: snapshot, Started: []ProgramChildLease{}, Stepped: []ProgramChildStep{}, Leases: []ProgramChildLease{}}

	switch programSession.Status {
	case OrchestrationSessionPaused:
		result.Decision = programControlDecision(ProgramDecisionWait, "program paused — new child dispatch suspended")
		result.Leases, err = LoadProgramChildLeases(root)
		return result, err
	case OrchestrationSessionCancelling:
		if err := propagateProgramControl(root, parentSessionID, CancelOrchestration); err != nil {
			return ProgramStepResult{}, err
		}
		leases, err := LoadProgramChildLeases(root)
		if err != nil {
			return ProgramStepResult{}, err
		}
		result.Leases = leases
		active := programLeasesToStep(graph, leases, parentSessionID, cfg.Program.MaxConcurrentSpecs)
		if len(active) == 0 {
			if _, err := markProgramSessionStatus(root, parentSessionID, OrchestrationSessionComplete); err != nil {
				return ProgramStepResult{}, err
			}
			result.Decision = programControlDecision(ProgramDecisionComplete, "program cancelled — no active child leases remain")
			return result, nil
		}
		result.Decision = programControlDecision(ProgramDecisionWait, "program cancelling — cooperative cancel propagated")
		for _, lease := range active {
			step, err := StepOrchestration(root, lease.Slug, lease.ChildSessionID, policy, cfg)
			if err != nil {
				return ProgramStepResult{}, err
			}
			result.Stepped = append(result.Stepped, ProgramChildStep{Slug: lease.Slug, SessionID: lease.ChildSessionID, Result: step})
		}
		return result, nil
	case OrchestrationSessionComplete:
		result.Decision = programControlDecision(ProgramDecisionComplete, "program session complete")
		result.Leases, err = LoadProgramChildLeases(root)
		return result, err
	case OrchestrationSessionFailed:
		result.Decision = programControlDecision(ProgramDecisionEscalate, "program session failed — no new child dispatch")
		result.Leases, err = LoadProgramChildLeases(root)
		return result, err
	}

	decision, err := DecideProgram(snapshot)
	if err != nil {
		return ProgramStepResult{}, err
	}
	result.Decision = decision
	if decision.Action == ProgramDecisionStart {
		for _, slug := range decision.Specs {
			lease, err := AcquireProgramChildLease(root, parentSessionID, slug, cfg)
			if err != nil {
				return ProgramStepResult{}, err
			}
			if err := ensureProgramChildSession(root, lease, policy); err != nil {
				return ProgramStepResult{}, err
			}
			result.Started = append(result.Started, lease)
		}
	}

	leases, err := LoadProgramChildLeases(root)
	if err != nil {
		return ProgramStepResult{}, err
	}
	result.Leases = leases
	if decision.Action == ProgramDecisionEscalate {
		if _, err := markProgramSessionStatus(root, parentSessionID, OrchestrationSessionFailed); err != nil {
			return ProgramStepResult{}, err
		}
		return result, nil
	}
	if decision.Action == ProgramDecisionComplete {
		if _, err := markProgramSessionStatus(root, parentSessionID, OrchestrationSessionComplete); err != nil {
			return ProgramStepResult{}, err
		}
		return result, nil
	}
	for _, lease := range programLeasesToStep(graph, leases, parentSessionID, cfg.Program.MaxConcurrentSpecs) {
		step, err := StepOrchestration(root, lease.Slug, lease.ChildSessionID, policy, cfg)
		if err != nil {
			return ProgramStepResult{}, err
		}
		result.Stepped = append(result.Stepped, ProgramChildStep{Slug: lease.Slug, SessionID: lease.ChildSessionID, Result: step})
		if step.Decision.Action == OrchestrationEscalate {
			if _, err := markProgramChildLeaseEscalated(root, parentSessionID, lease.Slug); err != nil {
				return ProgramStepResult{}, err
			}
			if _, err := markProgramSessionStatus(root, parentSessionID, OrchestrationSessionFailed); err != nil {
				return ProgramStepResult{}, err
			}
			break
		}
	}
	result.Leases, err = LoadProgramChildLeases(root)
	return result, err
}

func programControlDecision(action ProgramDecisionAction, reason string) ProgramDecision {
	return ProgramDecision{Version: OrchestrationModelVersion, Action: action, Reason: reason}
}
