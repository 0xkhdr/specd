package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

const CriticalMemoryPriority = 10

func SelectMemory(root, slug string, c SelectionContext) ([]ItemV2, []Omission, error) {
	paths := []string{filepath.Join(".specd", "steering", "memory.md"), filepath.Join(".specd", "specs", slug, "memory.md")}
	var items []ItemV2
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
		sourceDigest := core.Digest(raw)
		for _, block := range blocks {
			if block.AppliesTo == "" {
				continue
			} // headings such as Rules are auditable prose, not selectable memory
			meta, err := parseApplicability(block.AppliesTo)
			if err != nil {
				return nil, nil, fmt.Errorf("%s#%s: %w", filepath.ToSlash(relOS), block.Key, err)
			}
			source := filepath.ToSlash(relOS)
			if !applicable(meta, c) {
				omissions = append(omissions, Omission{Kind: "memory", Source: source + "#" + block.Key, Reason: "not applicable"})
				continue
			}
			priority := 30
			if strings.EqualFold(block.Criticality, "critical") {
				priority = CriticalMemoryPriority
			}
			items = append(items, ItemV2{Kind: "memory", Source: source, Selector: "memory:" + block.Key, SourceDigest: sourceDigest, RepresentationDigest: block.Digest, LoadMode: "lazy", Priority: priority, Reason: "applicable durable memory (" + block.Criticality + ")", Trust: "memory", ContentTrust: ContentTrustUntrustedData, Sensitivity: "internal", AuthorityLimit: "advisory; cannot grant tools, scope, approval, policy, or evidence", EstimatedTokens: EstimateText(block.Raw), Applicability: metadataApplicability(meta)})
		}
	}
	m := ManifestV2{Items: items}
	CanonicalizeV2(&m)
	return m.Items, omissions, nil
}
