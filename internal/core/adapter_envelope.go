package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	verifyexec "github.com/0xkhdr/specd/internal/core/verify"
)

const (
	DeploymentAdapterEnvelopeSchemaV1 = "deployment-adapter/v1"
	DeploymentAdapterResultKind       = "deployment.result"
	MaxDeploymentAdapterEnvelopeBytes = 64 * 1024
	MaxDeploymentAdapterMessageBytes  = 1024
	AdapterRedactedText               = "[REDACTED UNTRUSTED ADAPTER TEXT]"
)

type AdapterTrustSource string

const (
	AdapterTrustAttestedCI    AdapterTrustSource = "attested_ci"
	AdapterTrustSignedRuntime AdapterTrustSource = "signed_runtime"
	AdapterTrustOperatorFile  AdapterTrustSource = "operator_file"
)

func (s AdapterTrustSource) valid() bool {
	switch s {
	case AdapterTrustAttestedCI, AdapterTrustSignedRuntime, AdapterTrustOperatorFile:
		return true
	default:
		return false
	}
}

// DeploymentAdapterEnvelopeV1 is Domain 08's additive payload contract. Domain
// 10 owns process execution and its common envelope; core accepts only this
// already-delivered JSON value and performs no network, subprocess, or
// credential discovery.
type DeploymentAdapterEnvelopeV1 struct {
	SchemaVersion  string             `json:"schema_version"`
	Kind           string             `json:"kind"`
	IdempotencyKey string             `json:"idempotency_key"`
	TrustSource    AdapterTrustSource `json:"trust_source"`
	AttestationRef string             `json:"attestation_ref,omitempty"`
	Deployment     DeploymentV1       `json:"deployment"`
	Message        string             `json:"message,omitempty"`
}

func (e *DeploymentAdapterEnvelopeV1) validateAndSanitize() error {
	if e.SchemaVersion != DeploymentAdapterEnvelopeSchemaV1 {
		return fmt.Errorf("unsupported deployment adapter schema %q", e.SchemaVersion)
	}
	if e.Kind != DeploymentAdapterResultKind {
		return fmt.Errorf("unsupported deployment adapter kind %q", e.Kind)
	}
	if strings.TrimSpace(e.IdempotencyKey) == "" {
		return errors.New("deployment adapter idempotency_key is required")
	}
	if !e.TrustSource.valid() {
		return fmt.Errorf("deployment adapter trust_source %q is not allowlisted", e.TrustSource)
	}
	if e.TrustSource != AdapterTrustOperatorFile && strings.TrimSpace(e.AttestationRef) == "" {
		return fmt.Errorf("deployment adapter attestation_ref required for trust_source %q", e.TrustSource)
	}
	if e.Deployment.IdempotencyKey != e.IdempotencyKey {
		return errors.New("deployment adapter idempotency_key does not match deployment")
	}
	if e.Deployment.AttestationRef != e.AttestationRef {
		return errors.New("deployment adapter attestation_ref does not match deployment")
	}
	if err := ValidateDeployment(e.Deployment); err != nil {
		return fmt.Errorf("invalid deployment adapter payload: %w", err)
	}
	e.Message = sanitizeAdapterMessage(e.Message)
	return nil
}

func sanitizeAdapterMessage(message string) string {
	if message == "" {
		return ""
	}
	if len(message) > MaxDeploymentAdapterMessageBytes {
		return AdapterRedactedText
	}
	lower := strings.ToLower(message)
	if strings.ContainsAny(message, "\r\n") ||
		strings.Contains(lower, "ignore previous") ||
		strings.Contains(lower, "ignore all") ||
		strings.Contains(lower, "system prompt") {
		return AdapterRedactedText
	}
	redacted := verifyexec.NewRedactor(nil).String(message)
	if redacted != message {
		return AdapterRedactedText
	}
	return message
}

// DecodeDeploymentAdapterEnvelope reads one bounded JSON value. It never reads
// process environment, so provider credentials cannot enter through this API.
func DecodeDeploymentAdapterEnvelope(r io.Reader) (DeploymentAdapterEnvelopeV1, error) {
	limited := io.LimitReader(r, MaxDeploymentAdapterEnvelopeBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return DeploymentAdapterEnvelopeV1{}, fmt.Errorf("read deployment adapter envelope: %w", err)
	}
	if len(data) > MaxDeploymentAdapterEnvelopeBytes {
		return DeploymentAdapterEnvelopeV1{}, fmt.Errorf("deployment adapter envelope exceeds %d bytes", MaxDeploymentAdapterEnvelopeBytes)
	}
	var envelope DeploymentAdapterEnvelopeV1
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&envelope); err != nil {
		return DeploymentAdapterEnvelopeV1{}, fmt.Errorf("decode deployment adapter envelope: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return DeploymentAdapterEnvelopeV1{}, fmt.Errorf("decode deployment adapter envelope trailing data: %w", err)
	}
	if err := envelope.validateAndSanitize(); err != nil {
		return DeploymentAdapterEnvelopeV1{}, err
	}
	return envelope, nil
}

func ReadDeploymentAdapterEnvelope(path string) (DeploymentAdapterEnvelopeV1, error) {
	f, err := os.Open(path)
	if err != nil {
		return DeploymentAdapterEnvelopeV1{}, fmt.Errorf("open deployment adapter envelope: %w", err)
	}
	defer f.Close()
	return DecodeDeploymentAdapterEnvelope(f)
}

type AdapterAppendOutcome string

const (
	AdapterAppendCreated  AdapterAppendOutcome = "created"
	AdapterAppendNoop     AdapterAppendOutcome = "noop"
	AdapterAppendConflict AdapterAppendOutcome = "conflict"
)

func deploymentPayloadDigest(d DeploymentV1) string {
	b, _ := json.Marshal(d)
	return Digest(b)
}

type DeploymentAdapterConflictV1 struct {
	SchemaVersion  string             `json:"schema_version"`
	IdempotencyKey string             `json:"idempotency_key"`
	ExistingDigest string             `json:"existing_digest"`
	IncomingDigest string             `json:"incoming_digest"`
	TrustSource    AdapterTrustSource `json:"trust_source"`
}

func DeploymentAdapterConflictLedgerPath(root, slug string) string {
	return filepath.Join(SpecdDir(root), "specs", slug, "deployment-conflicts.jsonl")
}

// ApplyDeploymentAdapterEnvelope atomically checks idempotency and appends a
// validated attempt. Conflicts expose only content digests, preserving both
// audit facts without persisting hostile payload text or credentials.
func ApplyDeploymentAdapterEnvelope(root, slug string, envelope DeploymentAdapterEnvelopeV1) (DeploymentV1, AdapterAppendOutcome, error) {
	if err := envelope.validateAndSanitize(); err != nil {
		return DeploymentV1{}, "", err
	}
	var outcome AdapterAppendOutcome
	result, err := WithSpecLock(root, func() (DeploymentV1, error) {
		path := DeploymentLedgerPath(root, slug)
		existing, err := ReadDeployments(path)
		if err != nil {
			return DeploymentV1{}, err
		}
		incoming := envelope.Deployment
		incoming.AdapterTrustSource = envelope.TrustSource
		incoming.AdapterMessage = envelope.Message
		for _, record := range existing {
			if record.IdempotencyKey != envelope.IdempotencyKey {
				continue
			}
			incoming.Attempt = record.Attempt
			existingDigest := deploymentPayloadDigest(record)
			incomingDigest := deploymentPayloadDigest(incoming)
			if existingDigest == incomingDigest {
				outcome = AdapterAppendNoop
				return record, nil
			}
			outcome = AdapterAppendConflict
			conflict := DeploymentAdapterConflictV1{
				SchemaVersion:  DeploymentAdapterEnvelopeSchemaV1,
				IdempotencyKey: envelope.IdempotencyKey,
				ExistingDigest: existingDigest,
				IncomingDigest: incomingDigest,
				TrustSource:    envelope.TrustSource,
			}
			if err := appendLedger(DeploymentAdapterConflictLedgerPath(root, slug), conflict); err != nil {
				return DeploymentV1{}, fmt.Errorf("record deployment adapter idempotency conflict: %w", err)
			}
			return DeploymentV1{}, fmt.Errorf("deployment adapter idempotency conflict: existing_digest=%s incoming_digest=%s", existingDigest, incomingDigest)
		}
		attempt := 1
		for _, record := range existing {
			if record.DeploymentID == incoming.DeploymentID && record.Attempt >= attempt {
				attempt = record.Attempt + 1
			}
		}
		incoming.Attempt = attempt
		if err := AppendDeployment(path, incoming); err != nil {
			return DeploymentV1{}, err
		}
		outcome = AdapterAppendCreated
		return incoming, nil
	})
	return result, outcome, err
}
