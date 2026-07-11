package adapter

// Class is a data-classification label from the fixed taxonomy (R4.1). Every
// adapter reference carries a class so the boundary layer can decide what may
// cross a process/network/A2A/CI/telemetry boundary and what must be redacted.
type Class string

const (
	ClassPublicMetadata     Class = "public-metadata"
	ClassSpecText           Class = "spec-text"
	ClassSourcePath         Class = "source-path"
	ClassSourceContent      Class = "source-content"
	ClassPrompt             Class = "prompt"
	ClassToolOutput         Class = "tool-output"
	ClassSecret             Class = "secret"
	ClassTelemetry          Class = "telemetry"
	ClassProductionFeedback Class = "production-feedback"
)

// AllClasses returns the taxonomy in a stable order. The count is asserted in
// tests so a class cannot be added or dropped without a deliberate change.
func AllClasses() []Class {
	return []Class{
		ClassPublicMetadata, ClassSpecText, ClassSourcePath, ClassSourceContent,
		ClassPrompt, ClassToolOutput, ClassSecret, ClassTelemetry, ClassProductionFeedback,
	}
}

// Valid reports whether c is a member of the taxonomy.
func (c Class) Valid() bool {
	for _, k := range AllClasses() {
		if c == k {
			return true
		}
	}
	return false
}

// Restricted reports whether c names content that must not cross a boundary in
// the clear by default (R4.2): secrets, raw source content, and prompts. These
// are absent or redacted unless a project policy explicitly opts them in.
func (c Class) Restricted() bool {
	switch c {
	case ClassSecret, ClassSourceContent, ClassPrompt:
		return true
	default:
		return false
	}
}

// Redaction records that a reference's inline content was removed at a boundary
// so an audit can see a transfer policy applied rather than a silent drop.
type Redaction struct {
	Ref    string `json:"ref"`
	Class  Class  `json:"class"`
	Reason string `json:"reason"`
}

// ExportPolicy governs which inline content may cross a boundary. The zero value
// is the safe default: only public-metadata inline is exported; every other
// class is reference+digest only. AllowInline opts specific classes in.
type ExportPolicy struct {
	AllowInline []Class
}

func (p ExportPolicy) inlineAllowed(c Class) bool {
	if c == ClassPublicMetadata {
		return true
	}
	for _, a := range p.AllowInline {
		if a == c {
			return true
		}
	}
	return false
}

// RedactForExport strips inline content that policy does not permit to cross a
// boundary, returning the sanitized references and a record of every removal
// (R4.2/R4.3). References and digests are always preserved; only inline bytes
// are removed, so the result still identifies its inputs by content address.
func RedactForExport(refs []Ref, p ExportPolicy) (kept []Ref, redactions []Redaction) {
	kept = make([]Ref, len(refs))
	for i, r := range refs {
		if r.Inline != "" && !p.inlineAllowed(r.Class) {
			redactions = append(redactions, Redaction{
				Ref:    r.Name,
				Class:  r.Class,
				Reason: "inline content of restricted class removed at export boundary",
			})
			r.Inline = ""
		}
		kept[i] = r
	}
	return kept, redactions
}
