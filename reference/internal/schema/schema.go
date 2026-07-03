package schema

import (
	"embed"
	"encoding/json"
	"fmt"
)

//go:embed schema/v1.json
var schemaFS embed.FS

// SchemaVersionID is the explicit, user-visible version id of the open spec
// format. It is independent of the state.json SchemaVersion (which versions the
// on-disk migration shape); this versions the published JSON Schema contract.
const SchemaVersionID = "1"

// DefaultSchemaVersion is served when `specd schema` is invoked without an
// explicit --version.
const DefaultSchemaVersion = SchemaVersionID

// Schema returns the raw, embedded JSON Schema document for the given version.
// Passing "" yields DefaultSchemaVersion. Unknown versions are an error so the
// command surface fails closed rather than emitting an empty document.
func Schema(version string) ([]byte, error) {
	if version == "" {
		version = DefaultSchemaVersion
	}
	switch version {
	case "1", "v1":
		return schemaFS.ReadFile("schema/v1.json")
	default:
		return nil, fmt.Errorf("unknown schema version %q (known: %s)", version, SchemaVersionID)
	}
}

// SchemaDoc is the minimal decoded view of a JSON Schema document specd needs:
// its definitions and declared version ids. Full JSON Schema validation is
// intentionally out of scope (stdlib-only, no validator dependency); specd uses
// the structural definitions for conformance checks and format reporting.
type SchemaDoc struct {
	ID                 string                     `json:"$id"`
	SpecdSchemaVersion string                     `json:"specdSchemaVersion"`
	StateSchemaVersion int                        `json:"stateSchemaVersion"`
	Defs               map[string]SchemaDef       `json:"$defs"`
	Raw                map[string]json.RawMessage `json:"-"`
}

// SchemaDef is one named definition (a Go type's mirror) within the schema.
type SchemaDef struct {
	Type                 string                     `json:"type"`
	Required             []string                   `json:"required"`
	AdditionalProperties *bool                      `json:"additionalProperties"`
	Properties           map[string]json.RawMessage `json:"properties"`
}

// ParseSchema decodes an embedded schema document into a SchemaDoc.
func ParseSchema(version string) (*SchemaDoc, error) {
	b, err := Schema(version)
	if err != nil {
		return nil, err
	}
	var doc SchemaDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("embedded schema %q is not valid JSON: %v", version, err)
	}
	return &doc, nil
}
