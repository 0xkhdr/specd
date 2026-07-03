package core

// ExitOK, ExitGate, ExitUsage, and ExitNotFound are the process exit codes
// returned by specd commands: success, a gate/validation failure, invalid
// usage, and a missing resource, respectively.
const (
	ExitOK       = 0
	ExitGate     = 1
	ExitUsage    = 2
	ExitNotFound = 3
)

// SpecdError is an error that carries the process exit Code it should
// produce alongside its Message.
type SpecdError struct {
	Code    int
	Message string
}

func (e *SpecdError) Error() string { return e.Message }

// GateError constructs a SpecdError with ExitGate, used for spec/gate
// validation failures.
func GateError(msg string) *SpecdError { return &SpecdError{Code: ExitGate, Message: msg} }

// UsageError constructs a SpecdError with ExitUsage, used for invalid
// command-line usage.
func UsageError(msg string) *SpecdError { return &SpecdError{Code: ExitUsage, Message: msg} }

// NotFoundError constructs a SpecdError with ExitNotFound, used when a
// required resource (e.g. a .specd root) cannot be located.
func NotFoundError(msg string) *SpecdError { return &SpecdError{Code: ExitNotFound, Message: msg} }
