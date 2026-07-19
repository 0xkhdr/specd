package gates

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

var intakeFieldOrder = []string{
	"source_type", "source_ref", "systems", "affected_specs", "severity", "risk", "owner", "prior_links",
}

// intakeReadiness checks only explicitly configured fields. It is deterministic
// and pure over CheckCtx; absent policy is a compatibility-preserving no-op.
func intakeReadiness(ctx CheckCtx) []Finding {
	p := ctx.Provenance
	if ctx.ProvenanceError != "" {
		return []Finding{{Severity: Error, Message: "load provenance.json: " + ctx.ProvenanceError}}
	}
	if p == nil || len(p.RequiredFields) == 0 {
		return nil
	}
	required := make(map[string]bool, len(p.RequiredFields))
	for _, field := range p.RequiredFields {
		required[strings.TrimSpace(field)] = true
	}
	var findings []Finding
	for _, field := range intakeFieldOrder {
		if !required[field] {
			continue
		}
		delete(required, field)
		if provenanceFieldMissing(ctx, field) {
			findings = append(findings, Finding{Severity: Error, Message: fmt.Sprintf("intake field %s is required for source_type %s; record a non-empty, non-unknown value in provenance.json", field, p.SourceType)})
		}
	}
	// Unknown policy fields fail closed in lexical order without depending on map iteration.
	for _, field := range slices.Sorted(maps.Keys(required)) {
		findings = append(findings, Finding{Severity: Error, Message: fmt.Sprintf("intake policy requires unknown field %q", field)})
	}
	return findings
}

func provenanceFieldMissing(ctx CheckCtx, field string) bool {
	p := ctx.Provenance
	switch field {
	case "source_type":
		return unknown(string(p.SourceType))
	case "source_ref":
		return unknown(p.SourceRef)
	case "systems":
		return unknownSlice(p.Systems)
	case "affected_specs":
		return unknownSlice(p.AffectedSpecs)
	case "severity":
		return unknown(p.Severity)
	case "risk":
		return unknown(p.Risk)
	case "owner":
		return unknown(p.Owner)
	case "prior_links":
		if len(p.PriorLinks) == 0 {
			return true
		}
		for _, link := range p.PriorLinks {
			if unknown(link.To) {
				return true
			}
		}
		return false
	default:
		return true
	}
}

func unknown(value string) bool {
	value = strings.TrimSpace(value)
	return value == "" || strings.EqualFold(value, "unknown")
}

func unknownSlice(values []string) bool {
	if len(values) == 0 {
		return true
	}
	for _, value := range values {
		if unknown(value) {
			return true
		}
	}
	return false
}
