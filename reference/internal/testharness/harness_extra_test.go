package testharness_test

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

func TestHarnessInitScaffolds(t *testing.T) {
	h := th.New(t)
	h.Init()
	// init writes the steering + config layout.
	h.AssertFileExists(".specd/config.yml")
	h.AssertFileExists("AGENTS.md")
}

func TestHarnessPathAndReadFile(t *testing.T) {
	h := th.New(t)
	h.Init()
	if !strings.HasPrefix(h.Path(".specd/config.yml"), h.Root) {
		t.Error("Path did not anchor under root")
	}
	if got := h.ReadFile(".specd/config.yml"); got == "" {
		t.Error("ReadFile returned empty config")
	}
	h.AssertFileContains(".specd/config.yml", "version:")
	h.AssertFileAbsent(".specd/does-not-exist")
}

func TestHarnessSpecPathAndArtifact(t *testing.T) {
	h := th.New(t)
	h.Spec("demo").
		Req("Core", "story", "THE SYSTEM SHALL do the thing.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Verify: "true"}).
		Status(core.StatusTasks).
		Build()

	if !strings.Contains(h.SpecPath("demo", "requirements.md"), "demo") {
		t.Error("SpecPath missing slug")
	}
	if got := h.SpecArtifact("demo", "tasks.md"); !strings.Contains(got, "T1") {
		t.Errorf("SpecArtifact(tasks.md) = %q, want to mention T1", got)
	}
}

func TestRunExpectAssertsCode(t *testing.T) {
	h := th.New(t)
	// `check` on an unknown spec is a not-found error; RunExpect asserts the code.
	res := h.RunExpect(core.ExitUsage, "new")
	if res.Code != core.ExitUsage {
		t.Errorf("RunExpect returned code %d", res.Code)
	}
	if res.OK() {
		t.Error("Result.OK() should be false for a usage error")
	}
	if !strings.Contains(res.Out(), "usage") {
		t.Errorf("Result.Out() = %q, want combined stream with usage", res.Out())
	}
}

func TestSpecBuilderTitleAndDesignSection(t *testing.T) {
	h := th.New(t)
	h.Spec("custom").
		Title("Custom Title").
		Req("Core", "story", "THE SYSTEM SHALL do the thing.").
		FullDesign().
		DesignSection("Risks", "No notable risks.").
		AddTask(th.TaskSpec{ID: "T1", Verify: "true"}).
		Status(core.StatusTasks).
		Build()

	h.AssertFileContains(".specd/specs/custom/requirements.md", "Custom Title")
	h.AssertFileContains(".specd/specs/custom/design.md", "Risks")
}
