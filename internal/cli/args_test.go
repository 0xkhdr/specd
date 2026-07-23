package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestArgs(t *testing.T) {
	// status documents --program (value), --json and --guide (bool); exercise the
	// space form, the = form, and bare bool parsing on a documented invocation.
	args, err := ParseArgs([]string{"status", "demo", "--program", "My Prog", "--json", "--guide"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if args.Command != "status" || len(args.Pos) != 1 || args.Pos[0] != "demo" {
		t.Fatalf("positionals=%#v", args)
	}
	if args.Flags["program"] != "My Prog" || args.Flags["json"] != "true" || args.Flags["guide"] != "true" {
		t.Fatalf("flags=%#v", args.Flags)
	}
	if a, err := ParseArgs([]string{"status", "demo", "--program=prod"}); err != nil || a.Flags["program"] != "prod" {
		t.Fatalf("= form: %v %#v", err, a.Flags)
	}

	for _, argv := range [][]string{{}, {"-x"}, {"--"}} {
		if _, err := ParseArgs(argv); err == nil {
			t.Fatalf("ParseArgs(%v) expected error", argv)
		}
	}

	var buf bytes.Buffer
	Usage(&buf)
	if !strings.Contains(buf.String(), "usage: specd") {
		t.Fatalf("usage missing: %q", buf.String())
	}
}

// TestArgsRejectsUndocumentedFlags pins spec R4.2: a flag absent from the
// command palette fails closed, the refusal names the flag and the command, and
// a genuinely-global flag (--help) is always allowed. An unknown command is left
// to the dispatcher's fail-closed path (flags not judged here).
func TestArgsRejectsUndocumentedFlags(t *testing.T) {
	_, err := ParseArgs([]string{"new", "my-spec", "--title", "x"})
	if err == nil {
		t.Fatal("undocumented --title on new was accepted")
	}
	if !strings.Contains(err.Error(), "--title") || !strings.Contains(err.Error(), `"new"`) {
		t.Fatalf("refusal does not name flag and command: %v", err)
	}

	// Deterministic: two undocumented flags name the first sorted.
	if _, err := ParseArgs([]string{"new", "s", "--zebra", "--alpha"}); err == nil || !strings.Contains(err.Error(), "--alpha") {
		t.Fatalf("expected --alpha named first: %v", err)
	}

	// Global --help is allowed on any command.
	if _, err := ParseArgs([]string{"status", "demo", "--help"}); err != nil {
		t.Fatalf("--help rejected: %v", err)
	}

	// Newly documented flags parse.
	if _, err := ParseArgs([]string{"agents", "doctor", "--compat"}); err != nil {
		t.Fatalf("documented --compat on agents rejected: %v", err)
	}
	if _, err := ParseArgs([]string{"report", "demo", "--workflow-metrics"}); err != nil {
		t.Fatalf("documented --workflow-metrics on report rejected: %v", err)
	}

	// Unknown command: flags not judged (dispatcher fails closed on the verb).
	if _, err := ParseArgs([]string{"bogus", "--whatever"}); err != nil {
		t.Fatalf("unknown-command flags should not be judged here: %v", err)
	}
}
