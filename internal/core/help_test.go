package core_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

func TestHelpListsRegistryCommands(t *testing.T) {
	if got := len(core.Commands); got != 27 {
		t.Fatalf("len(core.Commands) = %d, want 27", got)
	}

	var buf bytes.Buffer
	cli.Usage(&buf)
	help := buf.String()
	for _, command := range core.Commands {
		if !strings.Contains(help, command.Name) {
			t.Fatalf("help output missing command %q", command.Name)
		}
	}
}
