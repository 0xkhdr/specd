package cli

import (
	"fmt"
	"io"
	"strings"
)

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
				if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
					i++
					value = argv[i]
				} else {
					value = "true"
				}
			}
			args.Flags[name] = value
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
	return args, nil
}

func Usage(w io.Writer) {
	fmt.Fprintln(w, "usage: specd <command> [args] [--flag value|--flag=value]")
}
