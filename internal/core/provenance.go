package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const ProvenanceSchemaV1 = 1

type ProvenanceSourceType string

const (
	SourceFeature       ProvenanceSourceType = "feature"
	SourceIncident      ProvenanceSourceType = "incident"
	SourceVulnerability ProvenanceSourceType = "vulnerability"
	SourceDrift         ProvenanceSourceType = "drift"
	SourceDependency    ProvenanceSourceType = "dependency"
	SourceMigration     ProvenanceSourceType = "migration"
	SourceDeprecation   ProvenanceSourceType = "deprecation"
	SourcePolicy        ProvenanceSourceType = "policy"
)

var provenanceSourceTypes = map[ProvenanceSourceType]struct{}{
	SourceFeature: {}, SourceIncident: {}, SourceVulnerability: {}, SourceDrift: {},
	SourceDependency: {}, SourceMigration: {}, SourceDeprecation: {}, SourcePolicy: {},
}

// ProvenanceV1 records bounded, operator-supplied intake facts. Unknown JSON
// fields are deliberately ignored so newer producers remain readable by v1.
type ProvenanceV1 struct {
	SchemaVersion  int                  `json:"schema_version"`
	SourceType     ProvenanceSourceType `json:"source_type"`
	SourceRef      string               `json:"source_ref,omitempty"`
	Systems        []string             `json:"systems,omitempty"`
	AffectedSpecs  []string             `json:"affected_specs,omitempty"`
	Severity       string               `json:"severity,omitempty"`
	Risk           string               `json:"risk,omitempty"`
	Owner          string               `json:"owner,omitempty"`
	PriorLinks     []ProvenanceLink     `json:"prior_links,omitempty"`
	RequiredFields []string             `json:"required_fields,omitempty"`
}

// ProvenanceLink traces intake to a prior spec without mutating that spec.
// String-form legacy entries decode as a follows link to that spec.
type ProvenanceLink struct {
	From      string   `json:"from,omitempty"`
	To        string   `json:"to"`
	Kind      LinkKind `json:"kind,omitempty"`
	Reason    string   `json:"reason,omitempty"`
	CreatedAt string   `json:"created_at,omitempty"`
}

func (l *ProvenanceLink) UnmarshalJSON(raw []byte) error {
	var legacy string
	if err := json.Unmarshal(raw, &legacy); err == nil {
		l.To, l.Kind = legacy, LinkKindFollows
		return nil
	}
	type plain ProvenanceLink
	var decoded plain
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	*l = ProvenanceLink(decoded)
	if l.Kind == "" {
		l.Kind = LinkKindFollows
	}
	if !l.Kind.Valid() {
		return fmt.Errorf("provenance prior link kind %q is unknown", l.Kind)
	}
	return nil
}

func DecodeProvenance(raw []byte) (ProvenanceV1, error) {
	var p ProvenanceV1
	if err := json.Unmarshal(raw, &p); err != nil {
		return ProvenanceV1{}, fmt.Errorf("decode provenance: %w", err)
	}
	if p.SchemaVersion == 0 {
		p.SchemaVersion = ProvenanceSchemaV1
	}
	if p.SourceType != "" {
		if _, ok := provenanceSourceTypes[p.SourceType]; !ok {
			return ProvenanceV1{}, fmt.Errorf("provenance source_type %q is unknown", p.SourceType)
		}
	}
	return p, nil
}

// LoadProvenance returns nil for an absent file: intake is opt-in and legacy
// feature specs must retain their existing behavior.
func LoadProvenance(path string) (*ProvenanceV1, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p, err := DecodeProvenance(raw)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func ProvenancePath(root, slug string) string {
	return filepath.Join(SpecdDir(root), "specs", slug, "provenance.json")
}
