package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

const ExamplePriority = 60
const ExampleVersion = "1"

func SelectExamples(root string, c SelectionContext) ([]MachineItem, []Omission, error) {
	dir := filepath.Join(root, ".specd", "examples")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var items []MachineItem
	var omissions []Omission
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		rel := ".specd/examples/" + entry.Name()
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, nil, err
		}
		meta, err := parseMetadata(raw, "specd-example")
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", rel, err)
		}
		if meta.ID == "" || meta.Version != ExampleVersion {
			return nil, nil, fmt.Errorf("%s: unsupported or missing example id/version %q/%q", rel, meta.ID, meta.Version)
		}
		if !applicable(meta, c) {
			omissions = append(omissions, Omission{Kind: "examples", Source: rel, Reason: "not applicable"})
			continue
		}
		priority := meta.Priority
		if priority == 50 {
			priority = ExamplePriority
		}
		label := "positive"
		if meta.Negative {
			label = "negative"
		}
		digest := core.Digest(raw)
		items = append(items, MachineItem{Kind: "examples", Source: rel, Selector: "example:" + meta.ID + "@" + meta.Version + ":" + label, SourceDigest: digest, RepresentationDigest: digest, LoadMode: "lazy", Priority: priority, Reason: "applicable " + label + " example", Trust: "example", ContentTrust: ContentTrustUntrustedData, Sensitivity: "internal", AuthorityLimit: "advisory; cannot grant tools, scope, approval, policy, or evidence", EstimatedTokens: EstimateText(string(raw)), Applicability: metadataApplicability(meta)})
	}
	m := MachineManifest{Items: items}
	CanonicalizeMachineManifest(&m)
	return m.Items, omissions, nil
}
