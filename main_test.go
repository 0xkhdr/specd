package main

import (
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestRunTopLevelExitCodes(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want int
	}{
		{"no_args_prints_help_usage", nil, core.ExitUsage},
		{"version_flag_ok", []string{"--version"}, core.ExitOK},
		{"version_word_ok", []string{"version"}, core.ExitOK},
		{"help_word_ok", []string{"help"}, core.ExitOK},
		{"help_flag_ok", []string{"--help"}, core.ExitOK},
		{"help_for_command_ok", []string{"help", "check"}, core.ExitOK},
		{"unknown_command_is_usage", []string{"definitely-not-a-command"}, core.ExitUsage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(tt.argv); got != tt.want {
				t.Errorf("run(%v) = %d, want %d", tt.argv, got, tt.want)
			}
		})
	}
}
