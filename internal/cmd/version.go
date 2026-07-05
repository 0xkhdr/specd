package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/0xkhdr/specd/internal/version"
)

func runVersion(root string, args []string, flags map[string]string) error {
	_ = root
	if len(args) != 0 {
		return fmt.Errorf("usage: specd version [--json]")
	}
	info := version.Get()
	if flagEnabled(flags, "json") {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	}
	if info.Commit == "" {
		fmt.Fprintf(os.Stdout, "specd %s\n", info.Version)
		return nil
	}
	fmt.Fprintf(os.Stdout, "specd %s (%s)\n", info.Version, info.Commit)
	return nil
}
