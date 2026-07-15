package core

import (
	"fmt"
	"os"
)

// IsJSONMode reports whether output should be emitted as JSON instead of the
// human-readable text format, based on the SPECD_JSON environment variable
// ("1" or "true").
func IsJSONMode() bool {
	v := os.Getenv("SPECD_JSON")
	return v == "1" || v == "true"
}

func useColor() bool {
	v := os.Getenv("NO_COLOR")
	return v == "" || v == "0"
}

const (
	colorReset  = "\x1b[0m"
	colorBold   = "\x1b[1m"
	colorRed    = "\x1b[31m"
	colorGreen  = "\x1b[32m"
	colorYellow = "\x1b[33m"
	colorBlue   = "\x1b[34m"
	colorGray   = "\x1b[90m"
	colorCyan   = "\x1b[36m"
)

func colorize(color, text string) string {
	if !useColor() {
		return text
	}
	return color + text + colorReset
}

// Info prints msg to stdout prefixed with a colorized "info" label.
func Info(msg string) {
	fmt.Printf("%s  %s\n", colorize(colorBlue, "info"), msg)
}

// Success prints msg to stdout prefixed with a colorized checkmark.
func Success(msg string) {
	fmt.Printf("%s %s\n", colorize(colorGreen, "✓"), msg)
}

// Warn prints msg to stdout prefixed with a colorized "warn" label.
func Warn(msg string) {
	fmt.Printf("%s  %s\n", colorize(colorYellow, "warn"), msg)
}

// Error prints msg to stderr prefixed with a colorized "error" label.
func Error(msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", colorize(colorRed, "error"), msg)
}

// Header prints title to stdout, upper-cased and bolded, preceded by a blank
// line. It is a no-op in JSON mode.
func Header(title string) {
	if IsJSONMode() {
		return
	}
	fmt.Printf("\n%s\n", colorize(colorBold, toUpper(title)))
}

// Divider prints a horizontal rule to stdout. It is a no-op in JSON mode.
func Divider() {
	if IsJSONMode() {
		return
	}
	fmt.Printf("%s\n", colorize(colorGray, "──────────────────────────────────────────────────"))
}

func toUpper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		}
	}
	return string(b)
}
