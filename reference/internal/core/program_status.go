package core

// ProgramCounts summarizes how many child specs in a program fall into each
// status bucket.
type ProgramCounts struct {
	Total     int `json:"total"`
	Complete  int `json:"complete"`
	Active    int `json:"active"`
	Blocked   int `json:"blocked"`
	Escalated int `json:"escalated"`
}

// ProgramWaveSummary summarizes one dependency wave's specs and how many of
// them are complete or active.
type ProgramWaveSummary struct {
	Wave     int      `json:"wave"`
	Specs    []string `json:"specs"`
	Complete int      `json:"complete"`
	Active   int      `json:"active"`
}

// ProgramStatusReport is the full status view of a program orchestration
// session: its session and snapshot state, the next decision, aggregate
// counts, the runnable frontier, per-wave summaries, and any escalated specs.
type ProgramStatusReport struct {
	Session    ProgramSession       `json:"session"`
	Snapshot   ProgramSnapshot      `json:"snapshot"`
	Decision   ProgramDecision      `json:"decision"`
	Counts     ProgramCounts        `json:"counts"`
	Frontier   []string             `json:"frontier"`
	Waves      []ProgramWaveSummary `json:"waves"`
	Escalation []string             `json:"escalation"`
}

// SenseProgramOrchestration builds the current ProgramStatusReport for a
// program session: it loads the session and program graph, builds the
// runtime snapshot, derives the next decision (honoring paused/cancelling/
// complete/failed session states before falling back to DecideProgram), and
// assembles the counts, frontier, and wave summaries.
func SenseProgramOrchestration(root, parentSessionID string, cfg OrchestrationCfg) (ProgramStatusReport, error) {
	if err := validateACPOpaqueID("program session ID", parentSessionID); err != nil {
		return ProgramStatusReport{}, err
	}
	if err := ValidateOrchestrationConfig(&cfg); err != nil {
		return ProgramStatusReport{}, err
	}
	session, err := LoadProgramSession(root, parentSessionID)
	if err != nil {
		return ProgramStatusReport{}, err
	}
	graph, err := BuildProgram(root, nil)
	if err != nil {
		return ProgramStatusReport{}, err
	}
	runtime, err := programChildRuntime(root)
	if err != nil {
		return ProgramStatusReport{}, err
	}
	snapshot, err := BuildProgramSnapshotWithRuntime(graph, runtime, cfg.Program.MaxConcurrentSpecs)
	if err != nil {
		return ProgramStatusReport{}, err
	}
	decision, err := programStatusDecision(session, snapshot)
	if err != nil {
		return ProgramStatusReport{}, err
	}
	return buildProgramStatusReport(session, snapshot, decision), nil
}

func programStatusDecision(session ProgramSession, snapshot ProgramSnapshot) (ProgramDecision, error) {
	switch session.Status {
	case OrchestrationSessionPaused:
		return programControlDecision(ProgramDecisionWait, "program paused — new child dispatch suspended"), nil
	case OrchestrationSessionCancelling:
		return programControlDecision(ProgramDecisionWait, "program cancelling — cooperative cancel in progress"), nil
	case OrchestrationSessionComplete:
		return programControlDecision(ProgramDecisionComplete, "program session complete"), nil
	case OrchestrationSessionFailed:
		return programControlDecision(ProgramDecisionEscalate, "program session failed — no new child dispatch"), nil
	default:
		return DecideProgram(snapshot)
	}
}

func buildProgramStatusReport(session ProgramSession, snapshot ProgramSnapshot, decision ProgramDecision) ProgramStatusReport {
	counts := ProgramCounts{Total: len(snapshot.Children), Active: snapshot.ActiveCount}
	frontier := programRunnableChildren(snapshot.Children)
	escalation := []string{}
	waveIndex := map[int]int{}
	waves := []ProgramWaveSummary{}
	for _, child := range snapshot.Children {
		if child.Complete {
			counts.Complete++
		}
		if child.Blocked {
			counts.Blocked++
			escalation = append(escalation, child.Slug)
		}
		if child.Escalated {
			counts.Escalated++
			escalation = append(escalation, child.Slug)
		}
		idx, ok := waveIndex[child.Wave]
		if !ok {
			waves = append(waves, ProgramWaveSummary{Wave: child.Wave, Specs: []string{}})
			idx = len(waves) - 1
			waveIndex[child.Wave] = idx
		}
		waves[idx].Specs = append(waves[idx].Specs, child.Slug)
		if child.Complete {
			waves[idx].Complete++
		}
		if child.Active {
			waves[idx].Active++
		}
	}
	if frontier == nil {
		frontier = []string{}
	}
	if escalation == nil {
		escalation = []string{}
	}
	return ProgramStatusReport{Session: session, Snapshot: snapshot, Decision: decision, Counts: counts, Frontier: frontier, Waves: waves, Escalation: escalation}
}
