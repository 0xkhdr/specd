package core

import (
	"fmt"
	"os"
)

func IsJSONMode() bool {
	v := os.Getenv("SPECd_JSON")
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

func Info(msg string) {
	fmt.Printf("%s  %s\n", colorize(colorBlue, "info"), msg)
}

func Success(msg string) {
	fmt.Printf("%s %s\n", colorize(colorGreen, "✓"), msg)
}

func Warn(msg string) {
	fmt.Printf("%s  %s\n", colorize(colorYellow, "warn"), msg)
}

func Error(msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", colorize(colorRed, "error"), msg)
}

func Header(title string) {
	if IsJSONMode() {
		return
	}
	fmt.Printf("\n%s\n", colorize(colorBold, toUpper(title)))
}

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
