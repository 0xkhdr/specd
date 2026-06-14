package cmd

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

var adrNumRE = regexp.MustCompile(`(?m)^##\s+ADR-(\d+)`)

func nextADRNumber(text string) int {
	matches := adrNumRE.FindAllStringSubmatch(core.StripHTMLComments(text), -1)
	max := 0
	for _, m := range matches {
		n, _ := strconv.Atoi(m[1])
		if n > max {
			max = n
		}
	}
	return max + 1
}

func RunDecision(args cli.Args) int {
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}
	slug := ""
	text := ""
	if len(args.Pos) > 0 {
		slug = args.Pos[0]
	}
	if len(args.Pos) > 1 {
		text = args.Pos[1]
	}
	if slug == "" || text == "" {
		return usageExit("usage: specd decision <slug> \"<decision text>\" [--supersedes <ADR-id>]")
	}
	if err := core.RequireSpec(root, slug); err != nil {
		return specdExit(err)
	}

	path := core.ArtifactPath(root, slug, "decisions.md")
	supersedes := args.Str("supersedes")
	if supersedes == "" {
		supersedes = "—"
	}
	date := core.Clock().UTC().Format("2006-01-02")

	rc, err := core.WithSpecLock[int](root, slug, func() (int, error) {
		existing := core.ReadOrDefault(path, "")
		id := fmt.Sprintf("ADR-%03d", nextADRNumber(existing))
		entry := fmt.Sprintf("\n## %s — %s · %s\n**Context:** TODO\n**Decision:** %s\n**Consequences:** TODO\n**Supersedes:** %s\n",
			id, text, date, text, supersedes)
		if err := core.AppendFile(path, entry); err != nil {
			return specdExit(err), err
		}
		fmt.Printf("decision: appended %s to decisions.md\n", id)
		return core.ExitOK, nil
	})
	if err != nil {
		return specdExit(err)
	}
	return rc
}
