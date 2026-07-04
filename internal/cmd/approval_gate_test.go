package cmd

import (
	"strings"
	"testing"
)

func TestNextGatedOnApproval(t *testing.T) {
	root := t.TempDir()
	if err := Run(root, "new", []string{"demo"}, nil); err != nil {
		t.Fatalf("new: %v", err)
	}

	out, err := captureStdout(t, func() error {
		return Run(root, "next", []string{"demo"}, map[string]string{"json": "1"})
	})
	if err != nil {
		t.Fatalf("next before approval: %v", err)
	}
	if !strings.Contains(out, `"items": []`) || !strings.Contains(out, "missing approval") {
		t.Fatalf("next before approval = %s", out)
	}
	if err := Run(root, "verify", []string{"demo", "T1"}, nil); err == nil {
		t.Fatalf("verify before approval succeeded")
	}

	if err := Run(root, "approve", []string{"demo", "requirements"}, nil); err != nil {
		t.Fatalf("approve requirements: %v", err)
	}
	if err := Run(root, "approve", []string{"demo", "design"}, nil); err != nil {
		t.Fatalf("approve design: %v", err)
	}

	out, err = captureStdout(t, func() error {
		return Run(root, "next", []string{"demo"}, map[string]string{"json": "1"})
	})
	if err != nil {
		t.Fatalf("next after approval: %v", err)
	}
	if !strings.Contains(out, `"id": "T1"`) {
		t.Fatalf("next after approval = %s", out)
	}
	if err := Run(root, "verify", []string{"demo", "T1"}, nil); err != nil {
		t.Fatalf("verify after approval: %v", err)
	}
}
