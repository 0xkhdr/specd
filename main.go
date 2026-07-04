package main

import (
	"fmt"
	"os"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/cmd"
)

func main() {
	if len(os.Args) == 1 {
		cli.Usage(os.Stdout)
		return
	}
	args, err := cli.ParseArgs(os.Args[1:])
	if err != nil {
		cli.Usage(os.Stderr)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := cmd.Run(".", args.Command, args.Pos, args.Flags); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
