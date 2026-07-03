package cmd

import "github.com/0xkhdr/specd/internal/core"

// Handler is a deterministic command implementation boundary.
type Handler func(args []string) error

// Registry maps each supported top-level command to its handler.
var Registry = map[string]Handler{
	"help":   noOp,
	"init":   noOp,
	"new":    noOp,
	"status": noOp,
	"task":   noOp,
	"verify": noOp,
}

func noOp([]string) error { return nil }

// RegisteredCommandNames returns registry keys in help order.
func RegisteredCommandNames() []string {
	names := make([]string, 0, len(core.Commands))
	for _, command := range core.Commands {
		if _, ok := Registry[command.Name]; ok {
			names = append(names, command.Name)
		}
	}
	return names
}
