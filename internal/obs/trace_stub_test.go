//go:build !specd_trace

package obs

import "testing"

func TestDefaultTraceSpanNoop(t *testing.T) {
	if end := StartSpan("frontier.observe"); end != nil {
		t.Fatal("default StartSpan should return nil")
	}
}
