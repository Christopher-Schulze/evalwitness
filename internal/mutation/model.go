package mutation

import "github.com/Christopher-Schulze/evalwitness/internal/preprocess"

const (
	ManifestSchemaVersion            = "evalwitness.mutation-manifest.v1"
	WitnessSchemaVersion             = "evalwitness.mutation-witness.v1"
	BlindPacketSchemaVersion         = "evalwitness.blind-review-packet.v1"
	ConstructFirewallSchemaVersion   = "evalwitness.construct-firewall.v1"
	ConstructFirewallSchemaVersionV2 = "evalwitness.construct-firewall.v2"
	EvidenceBoundaryVersionV1        = "evalwitness.evidence-boundary.v1"
	EvidenceBoundaryVersionV2        = "evalwitness.evidence-boundary.v2"
	EvidenceBoundaryVersionV3        = "evalwitness.evidence-boundary.v3"
	MutationProgramVersionV1         = "evalwitness.trajectory-mutation.v1"
	MutationProgramVersionV2         = "evalwitness.trajectory-mutation.v2"
	MutationProgramVersionV3         = "evalwitness.trajectory-mutation.v3"
	RelationContractVersionV1        = "evalwitness.controlled-relation.v1"
	RelationContractVersionV2        = "evalwitness.controlled-relation.v2"
	RelationContractVersionV3        = "evalwitness.controlled-relation.v3"
	EvidenceBoundaryVersion          = EvidenceBoundaryVersionV1
	MutationProgramVersion           = MutationProgramVersionV1
	RelationContractVersion          = RelationContractVersionV1
	CanonicalPolicy                  = "evalwitness.mutation-canonical-json.v1"
	MaximumMutationDocumentSize      = 16 * 1024 * 1024
)

type Family string

const (
	FamilyPatchHunkRemoval          Family = "necessary_patch_hunk_removal"
	FamilyFailingChangeReintroduced Family = "known_failing_change_reintroduction"
	FamilyTestEvidenceOmitted       Family = "omitted_test_evidence"
	FamilyTestEvidenceFalsified     Family = "falsified_test_evidence"
	FamilyCommandFailureHidden      Family = "command_failure_hidden"
	FamilyToolOutputIncomplete      Family = "incomplete_tool_output"
	FamilyIrrelevantVerbosity       Family = "irrelevant_verbosity"
	FamilyNeutralFormatting         Family = "neutral_formatting"
	FamilyStablePathAlias           Family = "stable_path_aliasing"
	FamilyCandidateOrderReversal    Family = "candidate_order_reversal"
	FamilyCausalIndependentReorder  Family = "causally_independent_event_reorder"
	FamilyUntrustedScoreInjection   Family = "untrusted_score_tag_injection"
	FamilyAmbiguousSemanticEdit     Family = "ambiguous_semantic_edit"
)

type InterventionClass string

const (
	ClassSemanticQuality      InterventionClass = "semantic_quality"
	ClassPresentation         InterventionClass = "presentation"
	ClassEvidenceAvailability InterventionClass = "evidence_availability"
	ClassAdversarialClaim     InterventionClass = "adversarial_claim"
	ClassParserOnly           InterventionClass = "parser_only"
)

type Relation string

const (
	RelationOriginalBetter          Relation = "original_better"
	RelationQualityEqual            Relation = "quality_equal"
	RelationQualityEqualEvidenceLow Relation = "quality_equal_evidence_weaker"
	RelationVerifiedOutcomeWins     Relation = "verified_outcome_dominates"
	RelationNoControlEffect         Relation = "no_control_effect"
	RelationAmbiguous               Relation = "ambiguous"
)

type LabelState string

const (
	LabelProven    LabelState = "proven"
	LabelAmbiguous LabelState = "ambiguous"
	LabelInvalid   LabelState = "invalid"
)

type ConstructStatus string

const (
	ConstructApplied  ConstructStatus = "applied"
	ConstructRejected ConstructStatus = "rejected"
)

type ConstructRejectionReason string

const (
	RejectionNoApplicableTarget     ConstructRejectionReason = "no_applicable_target"
	RejectionPreservationFailure    ConstructRejectionReason = "preservation_failure"
	RejectionUnverifiedEvidenceRole ConstructRejectionReason = "unverified_evidence_role"
	RejectionUnnaturalFormatting    ConstructRejectionReason = "unnatural_formatting"
	RejectionTokenSequenceChanged   ConstructRejectionReason = "token_sequence_changed"
	RejectionTransactionDependency  ConstructRejectionReason = "transaction_dependency"
	RejectionTemporalDependency     ConstructRejectionReason = "temporal_dependency"
)

type ValidationKind string

const (
	ValidationFormal       ValidationKind = "formal"
	ValidationHermetic     ValidationKind = "hermetic_executable"
	ValidationPreservation ValidationKind = "preservation"
)

type NamedValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type SourceOutcome struct {
	Kind          string `json:"kind"`
	Value         string `json:"value"`
	WitnessDigest string `json:"witness_digest"`
}

type SourceRef struct {
	TaskID                  string                  `json:"task_id"`
	RepositoryID            string                  `json:"repository_id"`
	SourceFamily            string                  `json:"source_family"`
	SourceFormat            preprocess.SourceFormat `json:"source_format"`
	SourceLocation          string                  `json:"source_location"`
	SourceRevision          string                  `json:"source_revision"`
	SourceDigest            string                  `json:"source_digest"`
	TrajectoryDigest        string                  `json:"trajectory_digest"`
	PairedTrajectoryDigests []string                `json:"paired_trajectory_digests,omitempty"`
	Outcome                 SourceOutcome           `json:"outcome"`
}

type Program struct {
	Version    string       `json:"version"`
	Family     Family       `json:"family"`
	Seed       string       `json:"seed"`
	Operator   string       `json:"operator"`
	Parameters []NamedValue `json:"parameters"`
}

type AffectedSurface struct {
	EventIDs    []string               `json:"event_ids"`
	FieldPaths  []preprocess.FieldPath `json:"field_paths"`
	FileAliases []string               `json:"file_aliases"`
}

type ValidatorSpec struct {
	ID                 string         `json:"id"`
	Version            string         `json:"version"`
	Kind               ValidationKind `json:"kind"`
	ContractDigest     string         `json:"contract_digest"`
	TimeoutMillis      int            `json:"timeout_millis"`
	MaximumOutputBytes int            `json:"maximum_output_bytes"`
}

type OutcomeProof struct {
	Mechanism          ValidationKind `json:"mechanism"`
	ContractDigest     string         `json:"contract_digest"`
	OriginalPassed     bool           `json:"original_passed"`
	MutatedPassed      bool           `json:"mutated_passed"`
	IndependentOfTrace bool           `json:"independent_of_trace"`
	WitnessDigest      string         `json:"witness_digest"`
}

type PreservationRecord struct {
	BoundaryVersion          string   `json:"boundary_version"`
	QualityProjectionBefore  string   `json:"quality_projection_before"`
	QualityProjectionAfter   string   `json:"quality_projection_after"`
	EvidenceProjectionBefore string   `json:"evidence_projection_before"`
	EvidenceProjectionAfter  string   `json:"evidence_projection_after"`
	CausalGraphBefore        string   `json:"causal_graph_before"`
	CausalGraphAfter         string   `json:"causal_graph_after"`
	ChangedGroups            []string `json:"changed_groups"`
	PreservedGroups          []string `json:"preserved_groups"`
	AmbiguityReasons         []string `json:"ambiguity_reasons"`
}

type Check struct {
	Name     string `json:"name"`
	Expected string `json:"expected"`
	Observed string `json:"observed"`
	Passed   bool   `json:"passed"`
}

type Witness struct {
	SchemaVersion    string     `json:"schema_version"`
	ValidatorID      string     `json:"validator_id"`
	ValidatorVersion string     `json:"validator_version"`
	Relation         Relation   `json:"relation"`
	LabelState       LabelState `json:"label_state"`
	Checks           []Check    `json:"checks"`
	Digest           string     `json:"digest"`
}

type LicenseMetadata struct {
	SPDX           string `json:"spdx"`
	SourceURL      string `json:"source_url"`
	SourceRevision string `json:"source_revision"`
	Redistribution string `json:"redistribution"`
	Attribution    string `json:"attribution"`
}

type PrivacyMetadata struct {
	Classification        string `json:"classification"`
	RedactionPolicyDigest string `json:"redaction_policy_digest"`
	PublicReleaseAllowed  bool   `json:"public_release_allowed"`
}

type ReviewState struct {
	Required          bool   `json:"required"`
	SamplingStratum   string `json:"sampling_stratum"`
	BlindPacketDigest string `json:"blind_packet_digest"`
}

type Manifest struct {
	SchemaVersion            string             `json:"schema_version"`
	CanonicalPolicy          string             `json:"canonical_policy"`
	RelationContractVersion  string             `json:"relation_contract_version"`
	CorpusVersion            string             `json:"corpus_version"`
	MutationID               string             `json:"mutation_id"`
	Source                   SourceRef          `json:"source"`
	Program                  Program            `json:"program"`
	Class                    InterventionClass  `json:"intervention_class"`
	ExpectedRelation         Relation           `json:"expected_relation"`
	Affected                 AffectedSurface    `json:"affected"`
	Validator                ValidatorSpec      `json:"validator"`
	OutcomeProof             *OutcomeProof      `json:"outcome_proof,omitempty"`
	Preservation             PreservationRecord `json:"preservation"`
	Witness                  Witness            `json:"witness"`
	License                  LicenseMetadata    `json:"license"`
	Privacy                  PrivacyMetadata    `json:"privacy"`
	SplitGroupID             string             `json:"split_group_id"`
	OriginalTrajectoryDigest string             `json:"original_trajectory_digest"`
	MutatedTrajectoryDigest  string             `json:"mutated_trajectory_digest"`
	Review                   ReviewState        `json:"review"`
	ConstructFirewallDigest  string             `json:"construct_firewall_digest,omitempty"`
	Digest                   string             `json:"digest"`
}

type ConstructFirewallReport struct {
	SchemaVersion           string                     `json:"schema_version"`
	CanonicalPolicy         string                     `json:"canonical_policy"`
	ProgramVersion          string                     `json:"program_version"`
	Family                  Family                     `json:"family"`
	Status                  ConstructStatus            `json:"status"`
	SourceTrajectoryDigest  string                     `json:"source_trajectory_digest"`
	MutatedTrajectoryDigest string                     `json:"mutated_trajectory_digest,omitempty"`
	TargetEventIDs          []string                   `json:"target_event_ids"`
	ProofEventIDs           []string                   `json:"proof_event_ids"`
	SemanticRole            string                     `json:"semantic_role,omitempty"`
	Checks                  []Check                    `json:"checks"`
	RejectionReasons        []ConstructRejectionReason `json:"rejection_reasons"`
	Digest                  string                     `json:"digest"`
}

type InvocationParseStatus string

const (
	InvocationParsed   InvocationParseStatus = "parsed"
	InvocationRejected InvocationParseStatus = "rejected"
)

type PresentationContentKind string

const (
	PresentationAssistantProse     PresentationContentKind = "assistant_prose"
	PresentationTerminalCommand    PresentationContentKind = "terminal_command"
	PresentationTerminalTranscript PresentationContentKind = "terminal_transcript"
	PresentationCode               PresentationContentKind = "code"
	PresentationStructuredData     PresentationContentKind = "structured_data"
	PresentationNonAssistantRole   PresentationContentKind = "non_assistant_role"
	PresentationUnknown            PresentationContentKind = "unknown"
)

type InvocationProof struct {
	ParserVersion    string                `json:"parser_version"`
	ToolCallEventID  string                `json:"tool_call_event_id"`
	CommandEventID   string                `json:"command_event_id"`
	EvidenceEventID  string                `json:"evidence_event_id"`
	SegmentIndex     int                   `json:"segment_index"`
	Executable       string                `json:"executable"`
	Arguments        []string              `json:"arguments"`
	WrapperChain     []string              `json:"wrapper_chain"`
	SemanticRole     string                `json:"semantic_role,omitempty"`
	DirectInvocation bool                  `json:"direct_invocation"`
	ParseStatus      InvocationParseStatus `json:"parse_status"`
	Decision         string                `json:"decision"`
}

type PresentationProof struct {
	ClassifierVersion string                  `json:"classifier_version"`
	EventID           string                  `json:"event_id"`
	MessageRole       string                  `json:"message_role"`
	ContentKind       PresentationContentKind `json:"content_kind"`
	TextPartCount     int                     `json:"text_part_count"`
	TokenCount        int                     `json:"token_count"`
	LineCount         int                     `json:"line_count"`
	MaximumLineBytes  int                     `json:"maximum_line_bytes"`
	Decision          string                  `json:"decision"`
}

type ConstructFirewallReportV2 struct {
	SchemaVersion           string                     `json:"schema_version"`
	CanonicalPolicy         string                     `json:"canonical_policy"`
	ProgramVersion          string                     `json:"program_version"`
	Family                  Family                     `json:"family"`
	Status                  ConstructStatus            `json:"status"`
	SourceTrajectoryDigest  string                     `json:"source_trajectory_digest"`
	MutatedTrajectoryDigest string                     `json:"mutated_trajectory_digest,omitempty"`
	TargetEventIDs          []string                   `json:"target_event_ids"`
	ProofEventIDs           []string                   `json:"proof_event_ids"`
	SemanticRole            string                     `json:"semantic_role,omitempty"`
	Invocation              *InvocationProof           `json:"invocation,omitempty"`
	Presentation            *PresentationProof         `json:"presentation,omitempty"`
	Checks                  []Check                    `json:"checks"`
	RejectionReasons        []ConstructRejectionReason `json:"rejection_reasons"`
	Digest                  string                     `json:"digest"`
}

type BlindReviewPacket struct {
	SchemaVersion          string                  `json:"schema_version"`
	PacketID               string                  `json:"packet_id"`
	MutationMaterialDigest string                  `json:"mutation_material_digest"`
	TaskAlias              string                  `json:"task_alias"`
	SourceFormat           preprocess.SourceFormat `json:"source_format"`
	OriginalDigest         string                  `json:"original_digest"`
	MutatedDigest          string                  `json:"mutated_digest"`
	AffectedEventCount     int                     `json:"affected_event_count"`
	ReviewQuestions        []string                `json:"review_questions"`
	Digest                 string                  `json:"digest"`
}

type ApplyRequest struct {
	CorpusVersion         string
	TaskID                string
	RepositoryID          string
	SourceFamily          string
	SourceLocation        string
	SourceRevision        string
	SplitGroupID          string
	Seed                  string
	Family                Family
	TargetEventID         string
	RequiredFragment      string
	Replacement           string
	Outcome               SourceOutcome
	Validator             ValidatorSpec
	OutcomeProof          *OutcomeProof
	License               LicenseMetadata
	Privacy               PrivacyMetadata
	ReviewSampled         bool
	ReviewSamplingStratum string
}

type ApplyResult struct {
	Manifest  Manifest
	Mutated   preprocess.Trajectory
	Packet    BlindReviewPacket
	PairOrder [2]string
}

type ApplyV2Outcome struct {
	Status   ConstructStatus         `json:"status"`
	Applied  *ApplyResult            `json:"applied,omitempty"`
	Firewall ConstructFirewallReport `json:"firewall"`
}

type ApplyV3Outcome struct {
	Status   ConstructStatus           `json:"status"`
	Applied  *ApplyResult              `json:"applied,omitempty"`
	Firewall ConstructFirewallReportV2 `json:"firewall"`
}
