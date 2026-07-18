// Command evalstatusassert structurally validates the JSON emitted by
// `specd eval status --json` for the domain regression harness.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/0xkhdr/specd/internal/core"
)

type statusReport struct {
	SchemaVersion string                    `json:"schema_version"`
	Count         int                       `json:"count"`
	Records       []core.EvidenceEnvelopeV1 `json:"records"`
}

func validate(r io.Reader, specSlug, taskID, checkID string) error {
	var report statusReport
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return fmt.Errorf("decode eval status: %w", err)
	}
	if err := requireEOF(decoder); err != nil {
		return err
	}
	if report.SchemaVersion != core.EvalSchemaVersion {
		return fmt.Errorf("status schema_version = %q, want %q", report.SchemaVersion, core.EvalSchemaVersion)
	}
	if report.Count != len(report.Records) {
		return fmt.Errorf("status count = %d, records = %d", report.Count, len(report.Records))
	}
	if len(report.Records) != 1 {
		return fmt.Errorf("status records = %d, want 1", len(report.Records))
	}
	record := report.Records[0]
	if err := core.ValidateEvidenceEnvelope(record); err != nil {
		return fmt.Errorf("invalid evidence envelope: %w", err)
	}
	if record.SpecSlug != specSlug || record.TaskID != taskID || record.CheckID != checkID {
		return fmt.Errorf("evidence identity = %s/%s/%s, want %s/%s/%s", record.SpecSlug, record.TaskID, record.CheckID, specSlug, taskID, checkID)
	}
	if record.EvidenceClass != core.EvidenceTest {
		return fmt.Errorf("evidence_class = %q, want %q", record.EvidenceClass, core.EvidenceTest)
	}
	if record.Producer != core.VerifyProducer {
		return fmt.Errorf("producer = %q, want %q", record.Producer, core.VerifyProducer)
	}
	return nil
}

func requireEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode eval status: trailing JSON value")
		}
		return fmt.Errorf("decode eval status trailer: %w", err)
	}
	return nil
}

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: evalstatusassert <spec> <task> <check>")
		os.Exit(2)
	}
	if err := validate(os.Stdin, os.Args[1], os.Args[2], os.Args[3]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
