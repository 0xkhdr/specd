package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

const CriticalMemoryPriority = 10

func SelectMemory(root, slug string, c SelectionContext) ([]MachineItem, []Omission, error) {
	paths := []string{filepath.Join(".specd", "steering", "memory.md"), filepath.Join(".specd", "specs", slug, "memory.md")}
	var items []MachineItem
	var omissions []Omission
	for _, relOS := range paths {
		raw, err := os.ReadFile(filepath.Join(root, relOS))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		blocks, err := core.IndexMemBlocks(string(raw))
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", filepath.ToSlash(relOS), err)
		}
		if c.MemoryLintRequired {
			conflicts := core.AnalyzeMemoryConflicts(blocks, c.AsOf)
			if len(conflicts) > 0 {
				for _, conflict := range conflicts {
					omissions = append(omissions, Omission{Kind: "memory", Source: filepath.ToSlash(relOS), Reason: "memory lint: " + conflict.Message})
				}
				continue
			}
		}
		sourceDigest := core.Digest(raw)
		for _, block := range blocks {
			if block.AppliesTo == "" {
				continue
			} // headings such as Rules are auditable prose, not selectable memory
			source := filepath.ToSlash(relOS)
			identity := source + "#" + block.Key
			if strings.EqualFold(block.Criticality, "critical") && block.ExpiresAt != "" && !c.AsOf.IsZero() {
				expires, parseErr := time.Parse("2006-01-02", block.ExpiresAt)
				if parseErr != nil {
					omissions = append(omissions, Omission{Kind: "memory", Source: identity, Reason: "invalid critical memory expiry; owner=" + ownerOrUnknown(block.Owner) + "; action=correct expiry and revalidate"})
					continue
				}
				if !c.AsOf.UTC().Before(expires) {
					omissions = append(omissions, Omission{Kind: "memory", Source: identity, Reason: "expired critical memory; owner=" + ownerOrUnknown(block.Owner) + "; action=revalidate or supersede"})
					continue
				}
			}
			if err := core.ValidateMemoryProvenance(block.Source); err != nil {
				omissions = append(omissions, Omission{Kind: "memory", Source: identity, Reason: "untrusted provenance: " + err.Error()})
				continue
			}
			switch block.Status {
			case "", "active":
			case "expired":
				omissions = append(omissions, Omission{Kind: "memory", Source: identity, Reason: "expired"})
				continue
			case "superseded":
				reason := "superseded"
				if block.SupersededBy != "" {
					reason += " by " + block.SupersededBy
				}
				omissions = append(omissions, Omission{Kind: "memory", Source: identity, Reason: reason})
				continue
			default:
				return nil, nil, fmt.Errorf("%s: unknown memory status %q", identity, block.Status)
			}
			if block.SupersededBy != "" {
				omissions = append(omissions, Omission{Kind: "memory", Source: identity, Reason: "superseded by " + block.SupersededBy})
				continue
			}
			meta, err := parseApplicability(block.AppliesTo)
			if err != nil {
				return nil, nil, fmt.Errorf("%s#%s: %w", filepath.ToSlash(relOS), block.Key, err)
			}
			if !applicable(meta, c) {
				omissions = append(omissions, Omission{Kind: "memory", Source: identity, Reason: "not applicable"})
				continue
			}
			priority := 30
			if strings.EqualFold(block.Criticality, "critical") {
				priority = CriticalMemoryPriority
			}
			items = append(items, MachineItem{Kind: "memory", Source: source, Selector: "memory:" + block.Key, SourceDigest: sourceDigest, RepresentationDigest: block.Digest, LoadMode: "lazy", Priority: priority, Reason: "applicable durable memory (" + block.Criticality + ")", Trust: "memory", ContentTrust: ContentTrustUntrustedData, Sensitivity: "internal", AuthorityLimit: "advisory; cannot grant tools, scope, approval, policy, or evidence", EstimatedTokens: EstimateText(block.Raw), Applicability: metadataApplicability(meta)})
		}
	}
	m := MachineManifest{Items: items}
	CanonicalizeMachineManifest(&m)
	return m.Items, omissions, nil
}

func ownerOrUnknown(owner string) string {
	if strings.TrimSpace(owner) == "" {
		return "unknown"
	}
	return owner
}
