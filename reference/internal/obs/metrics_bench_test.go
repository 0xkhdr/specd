package obs

import (
	"os"
	"testing"
	"time"
)

func BenchmarkRecordDurationDisabled(b *testing.B) {
	resetMetricsForTest()
	b.Setenv("SPECD_LOG", "")
	b.Setenv("SPECD_METRICS_ENDPOINT", "")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		RecordDuration("command_duration", time.Millisecond)
	}
}

func BenchmarkRecordDurationLoggingEnabled(b *testing.B) {
	resetMetricsForTest()
	b.Setenv("SPECD_LOG", "info")
	b.Setenv("SPECD_METRICS_ENDPOINT", "")
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		b.Fatalf("open devnull: %v", err)
	}
	old := os.Stderr
	os.Stderr = devNull
	b.Cleanup(func() {
		os.Stderr = old
		_ = devNull.Close()
	})
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		RecordDuration("command_duration", time.Millisecond)
	}
}
