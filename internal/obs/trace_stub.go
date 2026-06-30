//go:build !specd_trace

package obs

// EndSpan completes a tracing span.
type EndSpan func()

// StartSpan is a no-op in default builds. Returning nil lets callers avoid
// allocating a closure and lets the compiler erase the default instrumentation.
func StartSpan(_ string) EndSpan { return nil }
