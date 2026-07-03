package core

import (
	"os"
	"strconv"
)

// EnvInt reads name as an int, clamps to [min,max], and warns once on
// malformed input, returning def. max<=0 means "no upper bound".
func EnvInt(name string, def, min, max int) int {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		Warn(name + ": not an integer (" + v + ") — using default")
		return def
	}
	if n < min {
		n = min
	}
	if max > 0 && n > max {
		n = max
	}
	return n
}
