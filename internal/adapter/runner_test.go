package adapter

import (
	"os"
	"testing"
)

// TestMain runs the adapter package test suite.
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
