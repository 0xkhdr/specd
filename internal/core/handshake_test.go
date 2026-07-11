package core

import "testing"

func TestHandshakeToolContracts(t *testing.T) {
	hs := BootstrapHandshake(Config{})
	if len(hs.ToolContracts) == 0 {
		t.Fatal("tool contracts missing")
	}
	for _, tool := range hs.ToolContracts {
		if tool.Route == "" || len(tool.Phases) == 0 || len(tool.ExitCodes) == 0 {
			t.Fatalf("incomplete tool contract: %+v", tool)
		}
		if tool.HumanOnly && tool.Mutable == false {
			t.Fatalf("human-only mutation not labeled mutable: %+v", tool)
		}
	}
}
