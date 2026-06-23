// Package worker owns host worker process execution for Brain drivers.
//
// The package is the seam between deterministic orchestration policy and the
// unsafe OS-process layer: callers hand a Mission to a Runner, and concrete
// runners guarantee bounded execution, output prefixing, environment propagation,
// and process-tree cleanup.
package worker

import (
	"context"
	"time"
)

// Mission is the execution unit handed to a Runner.
type Mission struct {
	Command   string // worker command
	MissionID string // logical mission id, when available
	SessionID string
	WorkerID  string
	Spec      string
	TaskID    string
	Role      string
	Files     []string // SPECD_ARTIFACT hint
	Deadline  string   // RFC3339Nano

	// Payload is optional mission-file JSON content. When nil, the Mission itself
	// is written. This preserves the existing Pinky mission file contract while
	// keeping process execution decoupled from internal/core types.
	Payload any
}

// Result reports one completed runner invocation.
type Result struct {
	ExitErr  error
	TimedOut bool
	Duration time.Duration
}

// Runner executes one Mission to completion (or deadline). Implementations own
// child process groups and must not leave orphaned children behind Run.
type Runner interface {
	Run(ctx context.Context, m Mission) (Result, error)
}
