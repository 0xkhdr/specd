package core

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestProvenanceDecode(t *testing.T) {
	t.Run("typed", func(t *testing.T) {
		raw := []byte(`{"schema_version":1,"source_type":"incident","source_ref":"INC-42","systems":["api"],"affected_specs":["payments"],"severity":"high","risk":"customer-impact","owner":"sre","prior_links":["payments"]}`)
		got, err := DecodeProvenance(raw)
		if err != nil {
			t.Fatal(err)
		}
		if got.SchemaVersion != ProvenanceSchemaV1 || got.SourceType != SourceIncident || got.SourceRef != "INC-42" || got.Owner != "sre" {
			t.Fatalf("decoded provenance = %+v", got)
		}
		if !reflect.DeepEqual(got.Systems, []string{"api"}) || !reflect.DeepEqual(got.AffectedSpecs, []string{"payments"}) || len(got.PriorLinks) != 1 || got.PriorLinks[0].To != "payments" || got.PriorLinks[0].Kind != LinkKindFollows {
			t.Fatalf("decoded collections = %+v", got)
		}
	})

	t.Run("legacy-unversioned", func(t *testing.T) {
		got, err := DecodeProvenance([]byte(`{"source_type":"feature","source_ref":"roadmap","owner":"product"}`))
		if err != nil {
			t.Fatal(err)
		}
		if got.SchemaVersion != ProvenanceSchemaV1 || got.SourceType != SourceFeature {
			t.Fatalf("legacy decode = %+v", got)
		}
	})

	t.Run("unknown-source-type-refused", func(t *testing.T) {
		if _, err := DecodeProvenance([]byte(`{"schema_version":1,"source_type":"prompt"}`)); err == nil {
			t.Fatal("unknown source_type must fail closed")
		}
	})

	t.Run("load-missing-is-unconfigured", func(t *testing.T) {
		got, err := LoadProvenance(filepath.Join(t.TempDir(), "provenance.json"))
		if err != nil || got != nil {
			t.Fatalf("got=%+v err=%v", got, err)
		}
	})

	t.Run("load", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "provenance.json")
		if err := os.WriteFile(path, []byte(`{"schema_version":1,"source_type":"drift"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := LoadProvenance(path)
		if err != nil || got.SourceType != SourceDrift {
			t.Fatalf("got=%+v err=%v", got, err)
		}
	})

	t.Run("canonical-fixture", func(t *testing.T) {
		raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "maintenance", "provenance.json"))
		if err != nil {
			t.Fatal(err)
		}
		got, err := DecodeProvenance(raw)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.PriorLinks) != 1 || got.PriorLinks[0].Kind != LinkKindRegresses || got.PriorLinks[0].CreatedAt == "" {
			t.Fatalf("typed prior link = %+v", got.PriorLinks)
		}
	})
}
