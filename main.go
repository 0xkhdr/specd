package main

import (
	"fmt"
	"os"

	"github.com/0xkhdr/specd/internal/cli"
)

func main() {
	if _, err := cli.ParseArgs(os.Args[1:]); err != nil {
		cli.Usage(os.Stderr)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
