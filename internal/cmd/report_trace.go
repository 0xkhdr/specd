package cmd

import (
	"github.com/0xkhdr/specd/internal/core"
)

// runTrace projects a spec's metadata-only run trace and renders it as stable
// JSON Lines (spec 07 R6.2). It reuses the audit-history projection (the same
// on-disk records) and the SpanKind mapping, correlates each span to its W2 run
// chain via runs.jsonl, and links a task's activity spans to its dispatch span.
// It writes nothing; two exports over the same tree are byte-identical.
func runTrace(root, slug string, model core.ReportModel) (string, error) {
	events, err := gatherHistory(root, slug, model)
	if err != nil {
		return "", err
	}
	core.SortHistory(events)

	// All attempts of a task share one run_id (W2 R2.2), so first-seen wins.
	runs, err := core.ReadRuns(core.RunLedgerPath(root, slug))
	if err != nil {
		return "", err
	}
	runByTask := map[string]string{}
	for _, r := range runs {
		if _, ok := runByTask[r.TaskID]; !ok {
			runByTask[r.TaskID] = r.RunID
		}
	}

	// Pass 1: one span per trace-worthy event; remember each task's dispatch span
	// so activity spans can point at it as a parent (spec 07 R6.2).
	spans := make([]core.RunSpan, 0, len(events))
	dispatchByTask := map[string]string{}
	for _, e := range events {
		kind, ok := e.SpanKind()
		if !ok {
			continue
		}
		span := core.RunSpan{
			SpanID:     core.NewSpanID(slug, kind, e.SourceRank, e.Seq, e.Reference),
			RunID:      runByTask[e.TaskID],
			SpecID:     slug,
			TaskID:     e.TaskID,
			Kind:       kind,
			StartedAt:  e.Timestamp,
			GitHead:    e.GitHead,
			Actor:      e.Actor,
			Status:     e.Event,
			Reference:  e.Reference,
			SourceRank: e.SourceRank,
			Seq:        e.Seq,
		}
		if kind == core.SpanDispatch && e.TaskID != "" {
			dispatchByTask[e.TaskID] = span.SpanID
		}
		spans = append(spans, span)
	}
	// Pass 2: link each task's activity spans to its dispatch span, when present.
	for i := range spans {
		s := &spans[i]
		if s.Kind == core.SpanDispatch || s.TaskID == "" {
			continue
		}
		if pid, ok := dispatchByTask[s.TaskID]; ok {
			s.ParentSpanID = pid
		}
	}
	return core.RenderTraceJSON(spans)
}
