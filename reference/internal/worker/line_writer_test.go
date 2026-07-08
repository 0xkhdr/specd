package worker

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

func TestLineWriterPrefixing(t *testing.T) {
	var dst bytes.Buffer
	w := newLineWriter("[w1] ", &dst)

	if _, err := w.Write([]byte("hel")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("lo\nsecond\nthird")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(" line")); err != nil {
		t.Fatal(err)
	}
	w.Flush()

	want := "[w1] hello\n[w1] second\n[w1] third line\n"
	if got := dst.String(); got != want {
		t.Fatalf("output mismatch\nwant: %q\n got: %q", want, got)
	}
}

func TestLineWriterConcurrentWritersDoNotInterleaveLines(t *testing.T) {
	var dst bytes.Buffer
	w1 := newLineWriter("[a] ", &dst)
	w2 := newLineWriter("[b] ", &dst)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = w1.Write([]byte("alpha\n"))
		}()
		go func() {
			defer wg.Done()
			_, _ = w2.Write([]byte("beta\n"))
		}()
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSuffix(dst.String(), "\n"), "\n")
	if len(lines) != 100 {
		t.Fatalf("line count = %d, want 100", len(lines))
	}
	for _, line := range lines {
		if line != "[a] alpha" && line != "[b] beta" {
			t.Fatalf("interleaved or unprefixed line: %q", line)
		}
	}
}
