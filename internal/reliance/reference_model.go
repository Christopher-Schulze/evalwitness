package reliance

import (
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/stats"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

const (
	ReferenceCorpusSchemaVersion   = "evalwitness.reliance-reference-corpus.v1"
	ReferenceAnalysisSchemaVersion = "evalwitness.reliance-reference-analysis.v1"
	ReferenceOutputSchemaVersion   = "evalwitness.reliance-reference-adapter-output.v1"
	ReferenceAdapterAlgorithm      = "evalwitness.reliance-planted-walsh64.v1"
	ReferenceSourceTasks           = 16
	ReferenceCellsPerTask          = 64
	referenceScoreTag              = "<reliance_score>"
	referenceScoreCloseTag         = "</reliance_score>"
)

type ReferenceFactorMask struct {
	FactorID string `json:"factor_id"`
	Mask     uint64 `json:"mask"`
}

type PlantedEffect struct {
	TermID                 string  `json:"term_id"`
	ConditionalScoreEffect float64 `json:"conditional_score_effect"`
	ValidMassEffect        float64 `json:"valid_mass_effect"`
}

type ReferenceAdapterOutput struct {
	SchemaVersion        string                   `json:"schema_version"`
	CanonicalPolicy      string                   `json:"canonical_policy"`
	RawText              string                   `json:"raw_text"`
	HasLogprobs          bool                     `json:"has_logprobs"`
	ObservedTopLogprobs  int                      `json:"observed_top_logprobs"`
	OrderedTokenEvidence []provider.TokenEvidence `json:"ordered_token_evidence"`
	Digest               string                   `json:"digest"`
}

type ReferenceRecord struct {
	ObservationID        string                           `json:"observation_id"`
	SourceTaskID         string                           `json:"source_task_id"`
	Cell                 int                              `json:"cell"`
	Levels               []stats.FactorialLevel           `json:"levels"`
	BaselineOutput       ReferenceAdapterOutput           `json:"baseline_output"`
	InterventionOutput   ReferenceAdapterOutput           `json:"intervention_output"`
	BaselineEvidence     verifier.ScoreEvidence           `json:"baseline_evidence"`
	InterventionEvidence verifier.ScoreEvidence           `json:"intervention_evidence"`
	Comparison           verifier.ScoreEvidenceComparison `json:"comparison"`
	BaselineState        verifier.DecisionState           `json:"baseline_state"`
	InterventionState    verifier.DecisionState           `json:"intervention_state"`
	DecisionFlip         bool                             `json:"decision_flip"`
	AbstentionTransition bool                             `json:"abstention_transition"`
}

type ReferenceCorpus struct {
	SchemaVersion         string                `json:"schema_version"`
	CanonicalPolicy       string                `json:"canonical_policy"`
	Algorithm             string                `json:"algorithm"`
	PreregistrationDigest string                `json:"preregistration_digest"`
	SourceTasks           int                   `json:"source_tasks"`
	CellsPerTask          int                   `json:"cells_per_task"`
	FactorMasks           []ReferenceFactorMask `json:"factor_masks"`
	PlantedEffects        []PlantedEffect       `json:"planted_effects"`
	NullFactors           []FactorID            `json:"null_factors"`
	Records               []ReferenceRecord     `json:"records"`
	ProviderCalls         int                   `json:"provider_calls"`
	NetworkRequired       bool                  `json:"network_required"`
	Digest                string                `json:"digest"`
}

type ReferenceOutcomeFit struct {
	OutcomeID OutcomeID                   `json:"outcome_id"`
	Fit       stats.ClusteredFactorialFit `json:"fit"`
}

type ReferenceAnalysis struct {
	SchemaVersion         string                `json:"schema_version"`
	CanonicalPolicy       string                `json:"canonical_policy"`
	CorpusDigest          string                `json:"corpus_digest"`
	PreregistrationDigest string                `json:"preregistration_digest"`
	OutcomeFits           []ReferenceOutcomeFit `json:"outcome_fits"`
	Digest                string                `json:"digest"`
}
