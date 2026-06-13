package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

var validCriticalities = map[string]bool{"minor": true, "important": true, "critical": true}

var memBlockRE = regexp.MustCompile(`(?m)^##\s+`)

func extractMemBlock(text, key string) string {
	lines := strings.Split(core.StripHTMLComments(text), "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "## "+key {
			start = i
			break
		}
	}
	if start == -1 {
		return ""
	}
	var body []string
	body = append(body, lines[start])
	for i := start + 1; i < len(lines); i++ {
		if memBlockRE.MatchString(lines[i]) {
			break
		}
		body = append(body, lines[i])
	}
	return strings.TrimRight(strings.Join(body, "\n"), " \t\n")
}

func RunMemory(args cli.Args) int {
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}
	slug := ""
	sub := ""
	if len(args.Pos) > 0 {
		slug = args.Pos[0]
	}
	if len(args.Pos) > 1 {
		sub = args.Pos[1]
	}
	if slug == "" || sub == "" {
		return usageExit("usage: specd memory <slug> <add|promote> [flags]")
	}
	if err := core.RequireSpec(root, slug); err != nil {
		return specdExit(err)
	}
	memPath := core.ArtifactPath(root, slug, "memory.md")

	if sub == "add" {
		key := args.Str("key")
		pattern := args.Str("pattern")
		body := args.Str("body")
		source := args.Str("source")
		crit := args.Str("criticality")
		if key == "" || pattern == "" || body == "" || source == "" {
			return usageExit("memory add requires --key --pattern \"..\" --body \"..\" --source \"..\" --criticality <c>")
		}
		if !validCriticalities[crit] {
			return usageExit("--criticality must be one of: minor, important, critical")
		}
		related := "—"
		if r := args.Str("related"); r != "" {
			parts := strings.Split(r, ",")
			var linked []string
			for _, p := range parts {
				linked = append(linked, "[["+strings.TrimSpace(p)+"]]")
			}
			related = strings.Join(linked, ", ")
		}
		entry := fmt.Sprintf("\n## %s\n**Pattern:** %s\n**Detail:** %s\n**Source:** %s\n**Criticality:** %s\n**Related:** %s\n",
			key, pattern, body, source, crit, related)
		rc, err := core.WithSpecLock[int](root, slug, func() (int, error) {
			if err := core.AppendFile(memPath, entry); err != nil {
				return specdExit(err), err
			}
			fmt.Printf("memory: added '%s' to %s/memory.md\n", key, slug)
			return 0, nil
		})
		if err != nil {
			return specdExit(err)
		}
		return rc
	}

	if sub == "promote" {
		key := args.Str("key")
		if key == "" {
			return usageExit("memory promote requires --key <slug>")
		}
		rc, err := core.WithSpecLock[int](root, slug, func() (int, error) {
			block := extractMemBlock(core.ReadOrDefault(memPath, ""), key)
			if block == "" {
				return specdExit(core.GateError(fmt.Sprintf("memory: key '%s' not found in %s/memory.md", key, slug))), nil
			}
			cfg := core.LoadConfig(root)
			threshold := cfg.PromotionThreshold
			specs := core.ListSpecs(root)
			occurrences := 0
			for _, s := range specs {
				raw := core.ReadArtifact(root, s, "memory.md")
				if raw != nil && extractMemBlock(*raw, key) != "" {
					occurrences++
				}
			}
			if occurrences < threshold && !args.Bool("force") {
				return specdExit(core.GateError(fmt.Sprintf("memory: pattern '%s' seen in %d spec(s); promotion threshold is %d. Re-run with --force to promote anyway.", key, occurrences, threshold))), nil
			}
			date := core.Clock().UTC().Format("2006-01-02")
			promoted := fmt.Sprintf("\n%s\n**Promoted:** from spec '%s' on %s (seen in %d spec(s))\n", block, slug, date, occurrences)
			globalPath := core.SteeringDir(root) + "/memory.md"
			if err := core.AppendFile(globalPath, promoted); err != nil {
				return specdExit(err), err
			}
			fmt.Printf("memory: promoted '%s' from %s to steering/memory.md (seen in %d spec(s), threshold %d)\n", key, slug, occurrences, threshold)
			return 0, nil
		})
		if err != nil {
			return specdExit(err)
		}
		return rc
	}

	return usageExit(fmt.Sprintf("unknown memory subcommand '%s' (expected add|promote)", sub))
}
