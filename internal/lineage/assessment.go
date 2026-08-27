package lineage

import (
	"errors"
	"slices"
	"time"
)

type ProvenanceClass string

const (
	ProvenanceStdout      ProvenanceClass = "subprocess_stdout"
	ProvenanceStderr      ProvenanceClass = "subprocess_stderr"
	ProvenanceExitStatus  ProvenanceClass = "exit_status"
	ProvenanceStructured  ProvenanceClass = "structured_tool_metadata"
	ProvenanceSynthesized ProvenanceClass = "adapter_synthesized_text"
	ProvenanceNarration   ProvenanceClass = "agent_narration"
)

type FailabilityFinding struct {
	Property         string `json:"property"`
	OperandsDigest   string `json:"operands_digest"`
	Observable       string `json:"observable"`
	FailureCondition string `json:"failure_condition"`
	Failable         bool   `json:"failable"`
	Reason           string `json:"reason"`
}

type FreshnessState string

const (
	FreshnessCurrent    FreshnessState = "current"
	FreshnessClosed     FreshnessState = "closed"
	FreshnessUnresolved FreshnessState = "unresolved"
)

type InvalidationEdge struct {
	EdgeID            string    `json:"edge_id"`
	Kind              string    `json:"kind"`
	SubjectAlias      string    `json:"subject_alias"`
	BeforeStateDigest string    `json:"before_state_digest"`
	AfterStateDigest  string    `json:"after_state_digest"`
	ObservedAt        time.Time `json:"observed_at"`
}

type FreshnessInterval struct {
	State               FreshnessState     `json:"state"`
	ObservedStateDigest string             `json:"observed_state_digest"`
	ValidFrom           time.Time          `json:"valid_from"`
	ValidUntil          time.Time          `json:"valid_until"`
	EvaluatedAt         time.Time          `json:"evaluated_at"`
	InvalidationEdges   []InvalidationEdge `json:"invalidation_edges"`
}

type LineageAssessment struct {
	Header                    ArtifactHeader     `json:"header"`
	CandidateID               string             `json:"candidate_id"`
	TerminalState             TerminalState      `json:"terminal_state"`
	StateDisposition          StateDisposition   `json:"state_disposition"`
	PrecedenceApplied         int                `json:"precedence_applied"`
	ProofRecordIDs            []string           `json:"proof_record_ids"`
	ResultProvenance          []ProvenanceClass  `json:"result_provenance"`
	Failability               FailabilityFinding `json:"failability"`
	Freshness                 FreshnessInterval  `json:"freshness"`
	EquivalentChannelSurvives bool               `json:"equivalent_channel_survives"`
	ReviewerMode              string             `json:"reviewer_mode"`
}

func (assessment LineageAssessment) Validate() error {
	if err := validateHeader(assessment.Header, AssessmentSchemaVersion, []ParentRequirement{
		{Relation: "candidate", SchemaVersions: []string{CandidateSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true, SameTaskGroup: true},
		{Relation: "witness", SchemaVersions: []string{WitnessSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true, SameTaskGroup: true},
	}); err != nil {
		return err
	}
	if missing(assessment.CandidateID, assessment.ReviewerMode) || len(assessment.ProofRecordIDs) == 0 || len(assessment.ResultProvenance) == 0 {
		return errors.New("lineage assessment evidence is incomplete")
	}
	contract := terminalStateContract()
	index := slices.IndexFunc(contract, func(rule TerminalStateRule) bool { return rule.State == assessment.TerminalState })
	if index < 0 || assessment.PrecedenceApplied != index+1 || assessment.StateDisposition != contract[index].Disposition {
		return errors.New("lineage assessment terminal state violates the precedence contract")
	}
	if err := validateSortedUnique("assessment proof record IDs", assessment.ProofRecordIDs, 1); err != nil {
		return err
	}
	allowedProvenance := []ProvenanceClass{ProvenanceNarration, ProvenanceSynthesized, ProvenanceExitStatus, ProvenanceStderr, ProvenanceStdout, ProvenanceStructured}
	for index, provenance := range assessment.ResultProvenance {
		if !slices.Contains(allowedProvenance, provenance) || (index > 0 && assessment.ResultProvenance[index-1] >= provenance) {
			return errors.New("assessment result provenance must be supported, unique, and sorted")
		}
	}
	if missing(assessment.Failability.Property, assessment.Failability.Observable, assessment.Failability.FailureCondition, assessment.Failability.Reason) ||
		!validDigest(assessment.Failability.OperandsDigest) {
		return errors.New("lineage assessment failability finding is incomplete")
	}
	if err := assessment.Freshness.Validate(); err != nil {
		return err
	}
	if assessment.TerminalState == StateFreshnessUnresolved && assessment.Freshness.State != FreshnessUnresolved {
		return errors.New("freshness-unresolved state cannot carry a resolved finding")
	}
	if assessment.TerminalState == StateNonFailableVerification && assessment.Failability.Failable {
		return errors.New("non-failable terminal state cannot carry a failable finding")
	}
	if assessment.TerminalState == StateClaimSpecificEvidenceNotWeakened && !assessment.EquivalentChannelSurvives {
		return errors.New("evidence-not-weakened state requires an equivalent surviving channel")
	}
	if assessment.TerminalState == StateDirectVerificationInvocation && (!assessment.Failability.Failable || assessment.Freshness.State != FreshnessCurrent || assessment.EquivalentChannelSurvives) {
		return errors.New("eligible lineage requires failable, fresh, claim-weakened evidence")
	}
	copy := assessment
	copy.Header.Digest = ""
	return validateArtifactDigest(assessment.Header.Digest, copy)
}

func (interval FreshnessInterval) Validate() error {
	if !slices.Contains([]FreshnessState{FreshnessCurrent, FreshnessClosed, FreshnessUnresolved}, interval.State) {
		return errors.New("freshness interval state is invalid")
	}
	if interval.EvaluatedAt.IsZero() {
		return errors.New("freshness interval requires an evaluation time")
	}
	if interval.State == FreshnessUnresolved {
		if interval.ObservedStateDigest != "" || !interval.ValidFrom.IsZero() || !interval.ValidUntil.IsZero() || len(interval.InvalidationEdges) != 0 {
			return errors.New("unresolved freshness cannot claim state or interval evidence")
		}
		return nil
	}
	if !validDigest(interval.ObservedStateDigest) || interval.ValidFrom.IsZero() || interval.EvaluatedAt.Before(interval.ValidFrom) {
		return errors.New("resolved freshness requires observed state and start")
	}
	if interval.State == FreshnessCurrent {
		if !interval.ValidUntil.IsZero() || len(interval.InvalidationEdges) != 0 {
			return errors.New("current freshness cannot carry a closed boundary")
		}
		return nil
	}
	if interval.ValidUntil.Before(interval.ValidFrom) || interval.EvaluatedAt.Before(interval.ValidUntil) || len(interval.InvalidationEdges) != 1 {
		return errors.New("closed freshness requires an ordered invalidation boundary")
	}
	allowedKinds := []string{"artifact_replacement", "checkout_boundary", "dependency_change", "file_change"}
	previous := ""
	for _, edge := range interval.InvalidationEdges {
		if missing(edge.EdgeID, edge.Kind, edge.SubjectAlias) || edge.EdgeID <= previous || !slices.Contains(allowedKinds, edge.Kind) ||
			!validDigest(edge.BeforeStateDigest) || !validDigest(edge.AfterStateDigest) || edge.BeforeStateDigest == edge.AfterStateDigest ||
			edge.ObservedAt.Before(interval.ValidFrom) || edge.ObservedAt.After(interval.ValidUntil) {
			return errors.New("freshness invalidation edges must be typed, ordered, state-changing, and inside the interval")
		}
		previous = edge.EdgeID
	}
	edge := interval.InvalidationEdges[0]
	if edge.BeforeStateDigest != interval.ObservedStateDigest || !edge.ObservedAt.Equal(interval.ValidUntil) {
		return errors.New("freshness interval must close at its first state-changing edge")
	}
	return nil
}
