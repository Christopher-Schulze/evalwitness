package lineage

import (
	"errors"
	"slices"
)

type CapabilityState string

const (
	CapabilityRequired    CapabilityState = "required"
	CapabilityOptional    CapabilityState = "optional"
	CapabilityUnsupported CapabilityState = "unsupported"
	CapabilityUnspecified CapabilityState = "unspecified"
	CapabilityMalformed   CapabilityState = "malformed"
	CapabilityRedacted    CapabilityState = "redacted"
	CapabilityAbsent      CapabilityState = "absent"
	CapabilityObserved    CapabilityState = "observed"
	CapabilityNotObserved CapabilityState = "not_observed"
)

type CapabilityField struct {
	Field                 string          `json:"field"`
	RepresentableByFormat CapabilityState `json:"representable_by_format"`
	ObservedInCapture     CapabilityState `json:"observed_in_capture"`
	NormativeEvidence     string          `json:"normative_evidence"`
	GoldenVectorIDs       []string        `json:"golden_vector_ids"`
	Notes                 string          `json:"notes"`
}

type TraceCapabilityVector struct {
	Header                 ArtifactHeader    `json:"header"`
	Format                 string            `json:"format"`
	FormatVersion          string            `json:"format_version"`
	SpecificationIdentity  string            `json:"specification_identity"`
	SpecificationDigest    string            `json:"specification_digest"`
	SpecificationStability string            `json:"specification_stability"`
	MappingDiffDigest      string            `json:"mapping_diff_digest"`
	Fields                 []CapabilityField `json:"fields"`
}

func (vector TraceCapabilityVector) Validate() error {
	if err := validateHeader(vector.Header, CapabilitySchemaVersion, []ParentRequirement{
		{Relation: "plan", SchemaVersions: []string{PlanSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true},
	}); err != nil {
		return err
	}
	if missing(vector.Format, vector.FormatVersion, vector.SpecificationIdentity, vector.SpecificationStability) ||
		!validDigest(vector.SpecificationDigest) || !validDigest(vector.MappingDiffDigest) {
		return errors.New("trace capability specification identity is invalid")
	}
	expected := []string{"call_id", "command_display", "exit_status", "redaction_provenance", "result", "role", "source_record", "timestamps", "tool_call", "transaction_boundaries"}
	if len(vector.Fields) != len(expected) {
		return errors.New("trace capability vector requires the complete field surface")
	}
	representable := []CapabilityState{CapabilityRequired, CapabilityOptional, CapabilityUnsupported, CapabilityUnspecified}
	observed := []CapabilityState{CapabilityObserved, CapabilityNotObserved, CapabilityRedacted, CapabilityAbsent, CapabilityMalformed}
	for index, field := range vector.Fields {
		if field.Field != expected[index] || !slices.Contains(representable, field.RepresentableByFormat) ||
			!slices.Contains(observed, field.ObservedInCapture) || missing(field.NormativeEvidence, field.Notes) {
			return errors.New("trace capability field is incomplete, invalid, or out of order")
		}
		if err := validateSortedUnique("capability golden vectors", field.GoldenVectorIDs, 1); err != nil {
			return err
		}
	}
	copy := vector
	copy.Header.Digest = ""
	return validateArtifactDigest(vector.Header.Digest, copy)
}
