package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/mcp"
)

func runMCP(root string, args []string, flags map[string]string) error {
	// `--config <host>` prints a paste-ready MCP config snippet instead of serving
	// (spec 11 R1). --root/--spec pin the server's cwd and active spec.
	if host, ok := flags["config"]; ok {
		if len(args) != 0 {
			return errors.New("usage: specd mcp --config <host> [--root <path>] [--spec <slug>]")
		}
		snippet, err := core.MCPConfigSnippet(host, flags["root"], flags["spec"])
		if err != nil {
			return fmt.Errorf("%w: %v", ErrUsage, err)
		}
		fmt.Fprint(os.Stdout, snippet)
		return nil
	}
	if len(args) != 0 {
		return errors.New("usage: mcp")
	}
	return mcp.Serve(os.Stdin, os.Stdout, mcp.CoreTools(), mcpExecutor(root))
}

// mcpExecutor adapts the dispatcher into an mcp.Executor. It lives here, not in
// the mcp package, because mcp must not import cmd (cmd already imports mcp — the
// reverse edge is an import cycle). Injecting the executor is how the MCP server
// runs verbs without that cycle.
func mcpExecutor(root string) mcp.Executor {
	return func(name string, args []string, flags map[string]string, authority *core.AuthorityV1, now time.Time) (string, error) {
		operation, ok := core.ResolveOperation(name, args, flags)
		if !ok {
			return "", core.Refusef("ROUTE_DISPATCH_MISSING", "MCP route for %s is not declared", name)
		}
		slug := ""
		if command, found := core.CommandByName(name); found && command.SpecSlugArg != nil && *command.SpecSlugArg < len(args) {
			slug = args[*command.SpecSlugArg]
		}
		route := routeContextForSpec(root, slug, core.RouteMCP, authority != nil)
		if authority != nil {
			route.Phase = core.Phase(authority.Phase)
		} else if slug != "" {
			if state, err := core.LoadState(core.StatePath(root, slug)); err == nil {
				route.Phase = state.Phase
			}
		}
		decision := core.ProjectRoute(operation.ID, route)
		if !decision.Executable {
			actor := core.ActorOperator
			if decision.Handoff != nil {
				actor = decision.Handoff.Actor
			}
			return "", core.Refusef("ROUTE_HANDOFF_REQUIRED", "operation %s requires a %s handoff", operation.ID, actor)
		}
		return captureRunOutput(func() error {
			if authority != nil {
				return RunAuthorized(root, name, args, flags, *authority, nil, now)
			}
			return Run(root, name, args, flags)
		})
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
