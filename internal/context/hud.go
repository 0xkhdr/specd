package context

import (
	"fmt"
	"strings"
	"text/tabwriter"
)

// RenderHUD formats an already-built Manifest as a human-readable operator view:
// a table of load items with byte size and estimated token cost, a total row,
// and the spec's mode/tier line. It is a pure projection of the Manifest — no
// new estimation, no LLM, no I/O (ADR-8). The token total equals the value the
// --json surface serializes (manifest.EstimatedTokens), so the two renders never
// diverge numerically (RH.3).
func RenderHUD(m Manifest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "mode: %s  spec: %s  task: %s\n\n", m.Mode, m.Slug, m.TaskID)

	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "LOAD\tBYTES\tTOKENS")
	totalBytes := 0
	for _, item := range m.Items {
		bytes := itemBytes(item)
		totalBytes += bytes
		fmt.Fprintf(tw, "%s\t%d\t%d\n", itemLabel(item), bytes, item.EstimatedTokens)
	}
	fmt.Fprintf(tw, "TOTAL\t%d\t%d\n", totalBytes, m.EstimatedTokens)
	tw.Flush()
	return b.String()
}

// RenderHUDV2 renders typed context metadata in canonical order. It exposes
// references and status only; payload bytes remain outside the HUD.
func RenderHUDV2(m ManifestV2) string {
	copyManifest := m
	copyManifest.Items = append([]ItemV2(nil), m.Items...)
	CanonicalizeV2(&copyManifest)
	var b strings.Builder
	fmt.Fprintf(&b, "schema: %s  spec: %s  task: %s\n\n", copyManifest.SchemaVersion, copyManifest.Slug, copyManifest.TaskID)
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PATH\tREASON\tPRIORITY\tDIGEST\tREQUIRED\tTOKENS")
	for _, item := range copyManifest.Items {
		path := item.Source
		if path == "" {
			path = item.Selector
		}
		digest := item.RepresentationDigest
		if digest == "" {
			digest = item.SourceDigest
		}
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%t\t%d\n", path, item.Reason, item.Priority, digest, item.Required, item.EstimatedTokens)
	}
	tw.Flush()
	return b.String()
}

// RenderHUDQuality renders only quality metadata and proof labels. It is a
// pure projection, suitable for operator context without leaking corpora or traces.
func RenderHUDQuality(p QualityPacket) string {
	return RenderQualityPacket(p)
}

func RenderQualityPacket(p QualityPacket) string {
	var b strings.Builder
	fmt.Fprintf(&b, "quality contract: task=%s freshness=%s revision=%s\n", p.TaskID, p.Freshness, p.Revision)
	if p.Verify != "" {
		fmt.Fprintf(&b, "verify: %s\n", p.Verify)
	}
	if len(p.Review.HardRisks) > 0 {
		fmt.Fprintf(&b, "review risks: %s\n", strings.Join(p.Review.HardRisks, ","))
	}
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PROOF\tSTATUS\tREF\tDIGEST")
	for _, req := range p.Required {
		fmt.Fprintf(tw, "%s/%s\t%s\t%s\t%s\n", req.Class, req.Check, req.Status, req.ArtifactRef, req.Digest)
	}
	tw.Flush()
	for _, item := range []struct{ name, value string }{{"dataset", p.Dataset}, {"rubric", p.Rubric}, {"output", p.Output}, {"trace", p.Trace}} {
		if item.value != "" {
			fmt.Fprintf(&b, "%s: %s\n", item.name, item.value)
		}
	}
	return b.String()
}

// itemBytes is the payload the estimator counted for this item — the on-disk
// file size for path-backed items (R3.1), else the metadata string length — so
// tokens == (bytes+3)/4 holds by construction.
func itemBytes(item Item) int {
	return item.Bytes
}

func itemLabel(item Item) string {
	if item.Path != "" {
		return item.Path
	}
	if item.TaskID != "" {
		return item.Kind + ":" + item.TaskID
	}
	return item.Kind
}
