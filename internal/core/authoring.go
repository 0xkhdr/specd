package core

import (
	"fmt"
	"strings"
)

// AuthoringBrief is a gate-shaped description of what a faithful spec draft must
// contain. Every field is sourced from the same package vars the gates enforce
// (EarsForms, DesignSections, MandatoryKeys, ValidRoles) — never re-listed
// literals — so a faithful draft built from the brief passes `specd check`, and
// the brief cannot silently drift from the gates. It is a pure value: no
// network, no LLM, no IO.
type AuthoringBrief struct {
	EarsForms      []string        `json:"earsForms"`
	DesignSections []string        `json:"designSections"`
	TaskKeys       []string        `json:"taskKeys"`
	Roles          []string        `json:"roles"`
	ReadonlyRoles  []string        `json:"readonlyRoles"`
	Artifacts      []ArtifactBrief `json:"artifacts"`
	Prompt         string          `json:"prompt,omitempty"`
}

// ArtifactBrief is the per-artifact slice of the brief: the gate constraints an
// author must satisfy for one file.
type ArtifactBrief struct {
	Artifact    string   `json:"artifact"`
	Gate        string   `json:"gate"`
	Constraints []string `json:"constraints"`
}

// NewAuthoringBrief builds the brief from live gate constraints. prompt is the
// optional originating `--from` text; pass "" to omit it.
func NewAuthoringBrief(prompt string) AuthoringBrief {
	forms := EarsForms()
	sections := append([]string(nil), DesignSections...)
	keys := append([]string(nil), MandatoryKeys...)
	roles := append([]string(nil), ValidRoles...)
	ro := append([]string(nil), ReadonlyRoles...)

	earsConstraints := make([]string, 0, len(forms)+1)
	earsConstraints = append(earsConstraints, "every acceptance criterion matches one EARS form:")
	for _, f := range forms {
		earsConstraints = append(earsConstraints, "  "+f)
	}

	return AuthoringBrief{
		EarsForms:      forms,
		DesignSections: sections,
		TaskKeys:       keys,
		Roles:          roles,
		ReadonlyRoles:  ro,
		Prompt:         prompt,
		Artifacts: []ArtifactBrief{
			{
				Artifact: "requirements.md",
				Gate:     "ears",
				Constraints: append([]string{
					"each requirement is a `## Requirement N — <name>` section",
					"each requirement has a **User story:** line",
					"each requirement has **Acceptance criteria:** with ≥1 numbered item",
				}, earsConstraints...),
			},
			{
				Artifact:    "design.md",
				Gate:        "design",
				Constraints: append([]string{"contains every mandatory `## ` section:"}, indent(sections)...),
			},
			{
				Artifact: "tasks.md",
				Gate:     "task-schema, dag, traceability",
				Constraints: []string{
					"`# Tasks — <Title>` header, tasks grouped under `## Wave N`",
					"each task carries every key: " + strings.Join(keys, ", "),
					"`role` is one of: " + strings.Join(roles, ", "),
					"non-read-only roles (not " + strings.Join(ro, "/") + ") need a runnable `verify`",
					"`depends` only references earlier-or-equal waves; no cycles",
					"every requirement number is referenced by at least one task",
				},
			},
		},
	}
}

// InjectPrompt inserts an "Originating prompt" subsection carrying the verbatim
// `--from` text into a rendered requirements.md, placed just before the first
// `## Requirement` section (or appended if none is found). Returning the input
// unchanged when prompt is empty keeps the no-`--from` path byte-identical.
func InjectPrompt(reqMd, prompt string) string {
	if strings.TrimSpace(prompt) == "" {
		return reqMd
	}
	block := "## Originating prompt\n<!-- Verbatim `specd new --from` text. Refine the requirements below to satisfy it. -->\n> " +
		strings.ReplaceAll(strings.TrimRight(prompt, "\n"), "\n", "\n> ") + "\n\n"
	marker := "\n## Requirement "
	if idx := strings.Index(reqMd, marker); idx >= 0 {
		return reqMd[:idx+1] + block + reqMd[idx+1:]
	}
	return strings.TrimRight(reqMd, "\n") + "\n\n" + block
}

func indent(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = "  " + s
	}
	return out
}

// Text renders the brief as a human-readable authoring guide.
func (b AuthoringBrief) Text() string {
	var sb strings.Builder
	sb.WriteString("specd authoring brief — write a draft that passes `specd check`\n")
	if b.Prompt != "" {
		sb.WriteString("\nfrom prompt:\n  " + strings.ReplaceAll(b.Prompt, "\n", "\n  ") + "\n")
	}
	for _, a := range b.Artifacts {
		fmt.Fprintf(&sb, "\n%s  (gate: %s)\n", a.Artifact, a.Gate)
		for _, c := range a.Constraints {
			sb.WriteString("  - " + c + "\n")
		}
	}
	return sb.String()
}
