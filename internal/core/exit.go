package core

const (
	ExitOK       = 0
	ExitGate     = 1
	ExitUsage    = 2
	ExitNotFound = 3
)

type SpecdError struct {
	Code    int
	Message string
}

func (e *SpecdError) Error() string { return e.Message }

func GateError(msg string) *SpecdError     { return &SpecdError{Code: ExitGate, Message: msg} }
func UsageError(msg string) *SpecdError    { return &SpecdError{Code: ExitUsage, Message: msg} }
func NotFoundError(msg string) *SpecdError { return &SpecdError{Code: ExitNotFound, Message: msg} }
