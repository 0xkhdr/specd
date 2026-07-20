package cmd

import (
	"fmt"
	"os"

	"github.com/0xkhdr/specd/internal/core"
)

func runRecurring(root string, args []string, flags map[string]string) error {
	if len(args) != 2 || args[0] != "record" {
		return usageError("recurring")
	}
	result := core.RecurringResultV1{SchemaVersion: core.RecurringSchemaV1, CheckID: flags["check"], GitHead: flags["head"], ReleaseID: flags["release"], ConfigID: flags["config"], Verdict: core.RecurringVerdict(flags["verdict"]), ObservedAt: flags["observed-at"]}
	if err := core.RecordRecurringResult(root, args[1], result); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "recorded recurring check %s = %s for %s at %s\n", result.CheckID, result.Verdict, args[1], result.GitHead)
	return nil
}
