package lineage

import "errors"

type LayerBinding struct {
	Layer               string   `json:"layer"`
	ObjectDigest        string   `json:"object_digest"`
	RecordIDs           []string `json:"record_ids"`
	RequiredFields      []string `json:"required_fields"`
	DecisiveChannels    []string `json:"decisive_channels"`
	StructuredPresence  bool     `json:"structured_presence"`
	SemanticSufficiency bool     `json:"semantic_sufficiency"`
}

type AlignmentEvidence struct {
	CallIDMatched          bool   `json:"call_id_matched"`
	ParentEdgesMatched     bool   `json:"parent_edges_matched"`
	CommandOperandsDigest  string `json:"command_operands_digest"`
	TemporalWindowMillis   int64  `json:"temporal_window_millis"`
	TimestampOnlyCausality bool   `json:"timestamp_only_causality"`
}

type LineageCandidate struct {
	Header                        ArtifactHeader    `json:"header"`
	CandidateID                   string            `json:"candidate_id"`
	ClaimID                       string            `json:"claim_id"`
	InvocationID                  string            `json:"invocation_id"`
	ParserIdentityDigest          string            `json:"parser_identity_digest"`
	RequestFingerprint            string            `json:"request_fingerprint"`
	CanonicalRequestLineageDigest string            `json:"canonical_request_lineage_digest"`
	Alignment                     AlignmentEvidence `json:"alignment"`
	Layers                        []LayerBinding    `json:"layers"`
}

func (candidate LineageCandidate) Validate() error {
	if err := validateHeader(candidate.Header, CandidateSchemaVersion, []ParentRequirement{
		{Relation: "source", SchemaVersions: []string{SourceSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true, SameTaskGroup: true},
		{Relation: "witness", SchemaVersions: []string{WitnessSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true, SameTaskGroup: true},
	}); err != nil {
		return err
	}
	if missing(candidate.CandidateID, candidate.ClaimID, candidate.InvocationID) || !validDigest(candidate.ParserIdentityDigest) ||
		!validDigest(candidate.RequestFingerprint) || !validDigest(candidate.CanonicalRequestLineageDigest) || candidate.Alignment.TimestampOnlyCausality ||
		!validDigest(candidate.Alignment.CommandOperandsDigest) || candidate.Alignment.TemporalWindowMillis < 0 {
		return errors.New("lineage candidate identity or alignment is invalid")
	}
	expectedLayers := []string{"runtime_witness", "native_export", "canonical_graph", "retained_bundle", "verifier_request"}
	if len(candidate.Layers) != len(expectedLayers) {
		return errors.New("lineage candidate requires exactly five ordered layers")
	}
	for index, layer := range candidate.Layers {
		if layer.Layer != expectedLayers[index] || !validDigest(layer.ObjectDigest) || len(layer.RecordIDs) == 0 ||
			len(layer.RequiredFields) == 0 || len(layer.DecisiveChannels) == 0 {
			return errors.New("lineage candidate layer binding is incomplete or out of order")
		}
		if err := validateSortedUnique("layer record IDs", layer.RecordIDs, 1); err != nil {
			return err
		}
		if err := validateSortedUnique("layer required fields", layer.RequiredFields, 1); err != nil {
			return err
		}
		if err := validateSortedUnique("layer decisive channels", layer.DecisiveChannels, 1); err != nil {
			return err
		}
	}
	copy := candidate
	copy.Header.Digest = ""
	return validateArtifactDigest(candidate.Header.Digest, copy)
}
