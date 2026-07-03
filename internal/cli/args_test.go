package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestArgs(t *testing.T) {
	args, err := ParseArgs([]string{"new", "my-spec", "--title", "My Spec", "--json", "--agent=codex"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if args.Command != "new" || len(args.Pos) != 1 || args.Pos[0] != "my-spec" {
		t.Fatalf("positionals=%#v", args)
	}
	if args.Flags["title"] != "My Spec" || args.Flags["json"] != "true" || args.Flags["agent"] != "codex" {
		t.Fatalf("flags=%#v", args.Flags)
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
