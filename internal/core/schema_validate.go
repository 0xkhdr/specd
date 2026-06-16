package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// nodeSchema is the subset of JSON Schema (draft-07) that ValidateState
// interprets. specd deliberately ships no third-party JSON Schema validator
// (stdlib-only); this covers exactly the constructs schema/v1.json uses:
// $ref, object/array structure, required, additionalProperties (false or a
// value schema), items, and enum. Numeric bounds and string formats are left
// to the semantic gates — this mode checks *shape*, not policy.
type nodeSchema struct {
	Ref                  string                `json:"$ref"`
	Type                 string                `json:"type"`
	Required             []string              `json:"required"`
	Properties           map[string]nodeSchema `json:"properties"`
	AdditionalProperties json.RawMessage       `json:"additionalProperties"`
	Items                *nodeSchema           `json:"items"`
	Enum                 []json.RawMessage     `json:"enum"`
}

// ValidateState checks that a raw state.json document structurally conforms to
// the embedded schema of the given version ("" = default). It returns a sorted,
// stable list of human-readable violations (empty when conformant) and an error
// only when the inputs themselves are unusable (bad schema version or input that
// is not JSON). This is the `specd validate --schema` engine: a format check
// independent of the seven semantic gates.
func ValidateState(raw []byte, version string) ([]string, error) {
	schemaBytes, err := Schema(version)
	if err != nil {
		return nil, err
	}
	var root nodeSchema
	if err := json.Unmarshal(schemaBytes, &root); err != nil {
		return nil, GateError(fmt.Sprintf("embedded schema %q is not valid JSON: %v", version, err))
	}
	var defsRaw struct {
		Defs map[string]json.RawMessage `json:"$defs"`
	}
	if err := json.Unmarshal(schemaBytes, &defsRaw); err != nil {
		return nil, GateError(fmt.Sprintf("embedded schema %q has no usable $defs: %v", version, err))
	}

	var doc interface{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, GateError(fmt.Sprintf("document is not valid JSON: %v", err))
	}

	var viols []string
	validateNode("$", doc, root, defsRaw.Defs, &viols)
	sort.Strings(viols)
	return viols, nil
}

func resolveRef(sch nodeSchema, defs map[string]json.RawMessage) (nodeSchema, bool) {
	if sch.Ref == "" {
		return sch, true
	}
	name := strings.TrimPrefix(sch.Ref, "#/$defs/")
	raw, ok := defs[name]
	if !ok {
		return nodeSchema{}, false
	}
	var resolved nodeSchema
	if err := json.Unmarshal(raw, &resolved); err != nil {
		return nodeSchema{}, false
	}
	return resolved, true
}

func validateNode(path string, val interface{}, sch nodeSchema, defs map[string]json.RawMessage, viols *[]string) {
	sch, ok := resolveRef(sch, defs)
	if !ok {
		*viols = append(*viols, fmt.Sprintf("%s: schema $ref %q is unresolvable", path, sch.Ref))
		return
	}

	switch sch.Type {
	case "object":
		obj, ok := val.(map[string]interface{})
		if !ok {
			*viols = append(*viols, fmt.Sprintf("%s: expected object", path))
			return
		}
		for _, req := range sch.Required {
			if _, present := obj[req]; !present {
				*viols = append(*viols, fmt.Sprintf("%s: missing required property %q", path, req))
			}
		}
		// additionalProperties is either `false` (closed set) or a value schema
		// (used for the map-shaped `tasks`/`acceptance` objects).
		apFalse, apSchema := decodeAdditional(sch.AdditionalProperties)
		for key, child := range obj {
			childPath := path + "." + key
			if prop, declared := sch.Properties[key]; declared {
				validateNode(childPath, child, prop, defs, viols)
				continue
			}
			if apSchema != nil {
				validateNode(childPath, child, *apSchema, defs, viols)
				continue
			}
			if apFalse {
				*viols = append(*viols, fmt.Sprintf("%s: unknown property (additionalProperties: false)", childPath))
			}
		}
	case "array":
		arr, ok := val.([]interface{})
		if !ok {
			*viols = append(*viols, fmt.Sprintf("%s: expected array", path))
			return
		}
		if sch.Items != nil {
			for i, item := range arr {
				validateNode(fmt.Sprintf("%s[%d]", path, i), item, *sch.Items, defs, viols)
			}
		}
	case "string":
		if _, ok := val.(string); !ok {
			*viols = append(*viols, fmt.Sprintf("%s: expected string", path))
			return
		}
		checkEnum(path, val, sch, viols)
	case "integer":
		// JSON numbers decode to float64; require an integral value.
		f, ok := val.(float64)
		if !ok || f != float64(int64(f)) {
			*viols = append(*viols, fmt.Sprintf("%s: expected integer", path))
		}
	case "boolean":
		if _, ok := val.(bool); !ok {
			*viols = append(*viols, fmt.Sprintf("%s: expected boolean", path))
		}
	}
}

func checkEnum(path string, val interface{}, sch nodeSchema, viols *[]string) {
	if len(sch.Enum) == 0 {
		return
	}
	s, _ := val.(string)
	for _, e := range sch.Enum {
		var ev string
		if json.Unmarshal(e, &ev) == nil && ev == s {
			return
		}
	}
	*viols = append(*viols, fmt.Sprintf("%s: value %q not in enum", path, s))
}

// decodeAdditional interprets a JSON Schema additionalProperties value: a bare
// `false` (closed object) or a value schema applied to every extra key.
func decodeAdditional(raw json.RawMessage) (closed bool, value *nodeSchema) {
	if len(raw) == 0 {
		return false, nil
	}
	var b bool
	if json.Unmarshal(raw, &b) == nil {
		return !b, nil
	}
	var ns nodeSchema
	if json.Unmarshal(raw, &ns) == nil {
		return false, &ns
	}
	return false, nil
}
