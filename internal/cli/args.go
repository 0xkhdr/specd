package cli

import "strings"

// Args holds the parsed command-line arguments for a specd invocation: ordered
// positional tokens and flag values keyed by flag name (without the leading
// "--").
type Args struct {
	Pos   []string
	Flags map[string]string
}

var booleanFlags = map[string]bool{
	"force": true, "json": true, "all": true, "guardrails": true, "unverified": true, "dry-run": true,
	"list-packs": true, "once": true, "pr-summary": true, "revert-on-fail": true,
	"schema": true, "repair": true, "refresh": true, "yes": true,
	"non-interactive": true, "verbose": true, "fix": true, "program": true,
	"bootstrap": true, "inline-roles": true, "orchestrated": true,
	"recommend": true, "list": true, "snapshot": true, "include-schema": true,
	"global": true, "migrate": true, "dispatch": true, "schema-only": true, "serve": true,
	"watch": true, "history": true, "diff": true, "auto-step": true, "prototype": true, "promote": true,
	"hud": true, "conductor": true,
	"ledger": true, "compact": true, "directive": true,
	"security": true, "override": true, "remove": true,
}

// ParseArgs splits argv into positional tokens and "--flag"/"--flag=value"
// entries. A "--flag=value" token binds the value explicitly; a known boolean
// flag (see booleanFlags) is recorded as "true" without consuming the next
// token; any other "--flag" consumes the following token as its value unless
// that token is itself a flag, in which case it is also recorded as "true".
func ParseArgs(argv []string) Args {
	args := Args{Flags: make(map[string]string)}
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		if strings.HasPrefix(tok, "--") {
			key := tok[2:]
			// --key=value: split on the first '=' so `--status=complete` binds
			// the value explicitly instead of silently consuming the next token.
			if eq := strings.IndexByte(key, '='); eq >= 0 {
				args.Flags[key[:eq]] = key[eq+1:]
				continue
			}
			switch {
			case booleanFlags[key]:
				args.Flags[key] = "true"
			case i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "--"):
				i++
				args.Flags[key] = argv[i]
			default:
				args.Flags[key] = "true"
			}
		} else {
			args.Pos = append(args.Pos, tok)
		}
	}
	return args
}

// Bool reports whether the flag key was set to the literal value "true"
// (the value ParseArgs assigns to boolean and bare flags).
func (a Args) Bool(key string) bool { return a.Flags[key] == "true" }

// Str returns the string value of flag key, or "" if the flag was not set.
func (a Args) Str(key string) string { return a.Flags[key] }

// Has reports whether flag key was set at all, regardless of its value.
func (a Args) Has(key string) bool { _, ok := a.Flags[key]; return ok }
