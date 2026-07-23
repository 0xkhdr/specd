package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

// globalFlags are recognized for every command by the dispatcher itself (e.g.
// `--help` prints per-command help), so palette flag validation permits them
// regardless of the command entry.
var globalFlags = map[string]bool{"help": true}

type Args struct {
	Command string
	Pos     []string
	Flags   map[string]string
}

func ParseArgs(argv []string) (Args, error) {
	args := Args{Flags: map[string]string{}}
	for i := 0; i < len(argv); i++ {
		token := argv[i]
		if token == "" {
			continue
		}
		if strings.HasPrefix(token, "--") {
			name, value, hasValue := strings.Cut(strings.TrimPrefix(token, "--"), "=")
			if name == "" {
				return args, fmt.Errorf("empty flag")
			}
			if !hasValue {
				if name == "help" {
					value = "true"
				} else if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
					i++
					value = argv[i]
				} else {
					value = "true"
				}
			}
			args.Flags[name] = value
			continue
		}
		if token == "-h" {
			args.Flags["help"] = "true"
			continue
		}
		if strings.HasPrefix(token, "-") {
			return args, fmt.Errorf("unsupported short flag %q", token)
		}
		if args.Command == "" {
			args.Command = token
		} else {
			args.Pos = append(args.Pos, token)
		}
	}
	if args.Command == "" {
		return args, fmt.Errorf("missing command")
	}
	if err := rejectUndocumentedFlags(args); err != nil {
		return args, err
	}
	return args, nil
}

// rejectUndocumentedFlags fails closed when an invocation carries a flag the
// command palette does not declare (spec R4.2): a flag a handler reads but the
// palette omits is functional-but-undocumented, and this is the deterministic
// lint that keeps it from shipping. Flags are validated against the union of the
// command's declared flags (subcommand flags live on the parent entry) plus the
// dispatcher's global flags. An unrecognized command is left to the dispatcher's
// fail-closed unknown-verb path, so its flags are not judged here. On a
// violation the flags are sorted and the first is named, so the refusal is
// deterministic.
func rejectUndocumentedFlags(args Args) error {
	allowed := paletteFlags(args.Command)
	if allowed == nil {
		return nil
	}
	var bad []string
	for name := range args.Flags {
		if !globalFlags[name] && !allowed[name] {
			bad = append(bad, name)
		}
	}
	if len(bad) == 0 {
		return nil
	}
	sort.Strings(bad)
	return fmt.Errorf("unknown flag --%s for command %q; run `specd help %s` for its documented flags", bad[0], args.Command, args.Command)
}

// paletteFlags returns the declared flag set for a command, or nil when the
// command is not in the palette.
func paletteFlags(name string) map[string]bool {
	for _, c := range core.Commands {
		if c.Name != name {
			continue
		}
		set := make(map[string]bool, len(c.Flags))
		for _, f := range c.Flags {
			set[f.Name] = true
		}
		return set
	}
	return nil
}

func Usage(w io.Writer) {
	fmt.Fprintln(w, "usage: specd <command> [args] [--flag value|--flag=value]")
	for _, command := range core.Commands {
		fmt.Fprintf(w, "  %-10s %s\n", command.Name, command.Description)
	}
}
