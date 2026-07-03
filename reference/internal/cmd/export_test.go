package cmd

import "github.com/0xkhdr/specd/internal/worker"

// SetBrainRunner swaps the package-level worker.Runner seam used by the live
// brain driver and returns a restore func. It exists only in test builds so the
// driver path can be exercised with a recording fake instead of spawning real
// `sh`. Production code never references it; the default stays ShellRunner.
func SetBrainRunner(r worker.Runner) (restore func()) {
	prev := brainRunner
	brainRunner = r
	return func() { brainRunner = prev }
}
