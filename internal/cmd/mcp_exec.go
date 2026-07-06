package cmd

import (
	"bytes"
	"io"
	"os"

	"github.com/0xkhdr/specd/internal/mcp"
)

// mcpExecutor adapts the dispatcher into an mcp.Executor. It lives here, not in
// the mcp package, because mcp must not import cmd (cmd already imports mcp — the
// reverse edge is an import cycle). Injecting the executor is how the MCP server
// runs verbs without that cycle.
func mcpExecutor(root string) mcp.Executor {
	return func(name string, args []string, flags map[string]string) (string, error) {
		return captureRunOutput(func() error { return Run(root, name, args, flags) })
	}
}

// captureRunOutput runs f with os.Stdout redirected to a pipe and returns what f
// wrote. The MCP stdio server processes one request at a time, so swapping the
// global os.Stdout for the duration of a single verb is safe; the JSON-RPC
// encoder holds the real *os.File captured at Serve start, so responses are
// unaffected by the swap. A draining goroutine prevents the pipe filling and
// blocking a verb that prints more than the pipe buffer.
// ponytail: global-stdout swap works because the server is single-threaded; if
// tool calls ever run concurrently, plumb an io.Writer through the handlers.
func captureRunOutput(f func() error) (string, error) {
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	runErr := f()
	_ = w.Close()
	os.Stdout = orig
	out := <-done
	_ = r.Close()
	return out, runErr
}
