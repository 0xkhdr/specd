package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

func runAgents(root string, args []string, flags map[string]string) error {
	if len(args) == 2 && args[0] == "guide" {
		guide, err := driverGuideForSpec(root, args[1])
		if err != nil {
			return err
		}
		if flagEnabled(flags, "json") {
			return writeJSON(guide)
		}
		for _, action := range guide.NextActions {
			fmt.Fprintf(os.Stdout, "%s\t%s\t%s %s\n", action.Actor, action.SideEffect, action.Command, strings.Join(action.Args, " "))
		}
		return nil
	}
	if len(args) == 1 && args[0] == "doctor" {
		result := core.Doctor(root, getenv()["SPECD_SPEC"])
		if flagEnabled(flags, "json") {
			return writeJSON(result)
		}
		for _, finding := range result.Findings {
			fmt.Fprintf(os.Stdout, "%s %s %s: %s; fix: %s\n", finding.Severity, finding.Code, finding.Ref, finding.Message, finding.RecoveryAction)
		}
		fmt.Fprintln(os.Stdout, result.NextAction)
		return nil
	}
	if len(args) != 0 {
		return errors.New("usage: specd agents [doctor | guide <slug>] [--json]")
	}
	discovery := core.DiscoverAgents(root)
	if flags["json"] == "true" {
		return writeJSON(discovery)
	}
	for _, agent := range discovery {
		fmt.Fprintf(os.Stdout, "%s\t%s\n", agent.Name, agent.Status)
		for _, rel := range agent.Missing {
			fmt.Fprintf(os.Stdout, "  missing\t%s\n", rel)
		}
		for _, rel := range agent.Invalid {
			fmt.Fprintf(os.Stdout, "  invalid\t%s\n", rel)
		}
	}
	return nil
}
