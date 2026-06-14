package cli

import "strings"

type Args struct {
	Pos   []string
	Flags map[string]string
}

var booleanFlags = map[string]bool{
	"force": true, "json": true, "all": true, "unverified": true, "dry-run": true, "boot": true, "enrich": true,
}

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
			if booleanFlags[key] {
				args.Flags[key] = "true"
			} else if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "--") {
				i++
				args.Flags[key] = argv[i]
			} else {
				args.Flags[key] = "true"
			}
		} else {
			args.Pos = append(args.Pos, tok)
		}
	}
	return args
}

func (a Args) Bool(key string) bool  { return a.Flags[key] == "true" }
func (a Args) Str(key string) string { return a.Flags[key] }
func (a Args) Has(key string) bool   { _, ok := a.Flags[key]; return ok }
