package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/integration"
)

// Onboarding performance + deterministic-output gates (spec R1.3, R4.1, R5.2;
// task T26). The benchmarks record latency baselines (see
// docs/agent-harness-baselines.md); the Test* functions are the CI gate — they
// assert byte-stable receipts without any flaky wall-clock assertion.

// benchRuntime is an offline onboarding runtime: empty registry (no host
// detection or mutation) and a passing in-memory probe. Deterministic.
func benchRuntime() onboardingRuntime {
	return onboardingRuntime{
		Registry:    integration.MustRegistry(),
		Probe:       passingProbe,
		Input:       strings.NewReader(""),
		Interactive: func() bool { return false },
	}
}

// chdirTemp moves into a fresh temp dir for the duration of the test/benchmark.
func chdirTemp(tb testing.TB) string {
	tb.Helper()
	root := tb.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		tb.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = os.Chdir(previous) })
	tb.Setenv("NO_COLOR", "1")
	tb.Setenv("SPECD_JSON", "")
	return root
}

// silenceStdout redirects os.Stdout to the null device so benchmarks measure
// init work rather than pipe plumbing. Returns a restore func.
func silenceStdout(tb testing.TB) func() {
	tb.Helper()
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		tb.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = null
	return func() {
		os.Stdout = orig
		_ = null.Close()
	}
}

// initArgs is the offline, non-interactive, scaffold-only invocation the
// onboarding baselines are measured against.
func initArgs() cli.Args {
	return cli.ParseArgs([]string{"--agent", "none", "--non-interactive", "--json"})
}

func BenchmarkInitFresh(b *testing.B) {
	restore := silenceStdout(b)
	defer restore()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		chdirTemp(b)
		b.StartTimer()
		if code := runInitWithRuntime(initArgs(), core.DefaultInitExecutor(), benchRuntime()); code != core.ExitOK {
			b.Fatalf("init exit=%d", code)
		}
	}
}

func BenchmarkInitRerun(b *testing.B) {
	chdirTemp(b)
	restore := silenceStdout(b)
	defer restore()
	// Prime: first run scaffolds; the benchmark measures the steady-state rerun.
	if code := runInitWithRuntime(initArgs(), core.DefaultInitExecutor(), benchRuntime()); code != core.ExitOK {
		b.Fatalf("prime init exit=%d", code)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if code := runInitWithRuntime(initArgs(), core.DefaultInitExecutor(), benchRuntime()); code != core.ExitOK {
			b.Fatalf("rerun exit=%d", code)
		}
	}
}

func BenchmarkAgentDetection(b *testing.B) {
	root := chdirTemp(b)
	registry := integration.DefaultRegistry()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = registry.Detect(root)
	}
}

// TestInitOutputDeterministic asserts a healthy rerun emits a byte-identical JSON
// receipt (R1.3, R5.2): the deterministic-output gate that runs in CI.
func TestInitOutputDeterministic(t *testing.T) {
	chdirTemp(t)
	// Prime so subsequent runs are reruns on an unchanged, healthy project.
	if _, _, code := captureOutput(t, func() int {
		return runInitWithRuntime(initArgs(), core.DefaultInitExecutor(), benchRuntime())
	}); code != core.ExitOK {
		t.Fatalf("prime init exit=%d", code)
	}

	first, _, code := captureOutput(t, func() int {
		return runInitWithRuntime(initArgs(), core.DefaultInitExecutor(), benchRuntime())
	})
	if code != core.ExitOK {
		t.Fatalf("first rerun exit=%d", code)
	}
	second, _, code := captureOutput(t, func() int {
		return runInitWithRuntime(initArgs(), core.DefaultInitExecutor(), benchRuntime())
	})
	if code != core.ExitOK {
		t.Fatalf("second rerun exit=%d", code)
	}
	if first != second {
		t.Errorf("rerun output not byte-identical:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
	// Receipt must be valid JSON with non-null arrays (R5.2).
	var result core.InitResult
	if err := json.Unmarshal([]byte(first), &result); err != nil {
		t.Fatalf("receipt is not valid JSON: %v", err)
	}
}

// TestInitBenchmarkContract guards that the benchmarked operations produce
// deterministic results across invocations, so recorded latency baselines
// compare like for like (task T26 acceptance).
func TestInitBenchmarkContract(t *testing.T) {
	root := chdirTemp(t)

	// Detection (BenchmarkAgentDetection input) is stable across calls.
	registry := integration.DefaultRegistry()
	a, err := json.Marshal(registry.Detect(root))
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(registry.Detect(root))
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Errorf("detection nondeterministic:\n%s\n%s", a, b)
	}

	// Fresh init (BenchmarkInitFresh input) reaches a ready, exit-0 state.
	stdout, _, code := captureOutput(t, func() int {
		return runInitWithRuntime(initArgs(), core.DefaultInitExecutor(), benchRuntime())
	})
	if code != core.ExitOK {
		t.Fatalf("fresh init exit=%d stdout=%s", code, stdout)
	}
	var result core.InitResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("fresh init receipt invalid: %v", err)
	}
	if result.Status != "ready" {
		t.Errorf("fresh init status = %q, want ready", result.Status)
	}
}
