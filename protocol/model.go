// Package protocol implements the offline EvalWitness verifier-audit protocol
// and its language-neutral adapter contract.
package protocol

import "encoding/json"

const (
	CurrentVersion       = "1.2.0"
	PreviousMinorVersion = "1.1.0"
	ProtocolName         = "EvalWitness Verifier Audit Protocol"
	SchemaDialect        = "https://json-schema.org/draft/2020-12/schema"
	CanonicalPolicy      = "evalwitness.protocol-canonical-json.v1"

	DescriptorSchema = "evalwitness.protocol.evaluator-descriptor.v1"
	CaseSchema       = "evalwitness.protocol.audit-case.v1"
	InvocationSchema = "evalwitness.protocol.audit-invocation.v1"
	ScoreSchema      = "evalwitness.protocol.score-evidence.v1"
	DecisionSchema   = "evalwitness.protocol.decision-evidence.v1"
	FindingSchema    = "evalwitness.protocol.audit-finding.v1"
	ResultSchema     = "evalwitness.protocol.invocation-result.v1"
	RunSchema        = "evalwitness.protocol.audit-run.v1"
	MessageSchema    = "evalwitness.protocol.adapter-message.v1"
	MatrixSchema     = "evalwitness.protocol.capability-matrix.v1"

	ReliabilityExtension = "org.evalwitness.reliability.v1"
)

type ConformanceLevel string

const (
	LevelSyntax                  ConformanceLevel = "syntax"
	LevelDeterministicReplay     ConformanceLevel = "deterministic_replay"
	LevelLiveScoreEvidence       ConformanceLevel = "live_score_evidence"
	LevelEmpiricalReliability    ConformanceLevel = "empirical_reliability"
	LevelIndependentReproduction ConformanceLevel = "independent_reproduction"
)

type CaseKind string

const (
	CaseCanonicalEncoding  CaseKind = "canonical_encoding"
	CaseRequestFingerprint CaseKind = "request_fingerprint"
	CaseScoreEvidence      CaseKind = "score_evidence"
	CaseDecisionEvidence   CaseKind = "decision_evidence"
	CaseReplayEvidence     CaseKind = "replay_evidence"
	CaseAttestation        CaseKind = "attestation"
	CaseExtension          CaseKind = "extension"
	CaseCompatibility      CaseKind = "compatibility"
)

type CaseExpectation string

const (
	ExpectationAccepted    CaseExpectation = "accepted"
	ExpectationRejected    CaseExpectation = "rejected"
	ExpectationUnsupported CaseExpectation = "unsupported"
)

type CaseOutcome string

const (
	OutcomePassed      CaseOutcome = "passed"
	OutcomeFailed      CaseOutcome = "failed"
	OutcomeSkipped     CaseOutcome = "skipped"
	OutcomeUnsupported CaseOutcome = "unsupported"
	OutcomeNotRun      CaseOutcome = "not_run"
)

type InvocationStatus string

const (
	InvocationAccepted    InvocationStatus = "accepted"
	InvocationRejected    InvocationStatus = "rejected"
	InvocationUnsupported InvocationStatus = "unsupported"
	InvocationCancelled   InvocationStatus = "cancelled"
	InvocationFailed      InvocationStatus = "failed"
)

type MessageType string

const (
	MessageHello            MessageType = "hello"
	MessageHelloAck         MessageType = "hello_ack"
	MessageDescribe         MessageType = "describe"
	MessageDescriptor       MessageType = "descriptor"
	MessageBeginRun         MessageType = "begin_run"
	MessageRunStarted       MessageType = "run_started"
	MessageEvaluate         MessageType = "evaluate"
	MessageEvaluationResult MessageType = "evaluation_result"
	MessageCancel           MessageType = "cancel"
	MessageCancelled        MessageType = "cancelled"
	MessageEndRun           MessageType = "end_run"
	MessageRunResult        MessageType = "run_result"
	MessageError            MessageType = "error"
)

type Extension struct {
	Namespace string          `json:"namespace"`
	Schema    string          `json:"schema"`
	Required  bool            `json:"required"`
	Payload   json.RawMessage `json:"payload"`
}

type ImplementationIdentity struct {
	Name           string `json:"name"`
	Version        string `json:"version"`
	IdentityDigest string `json:"identity_digest"`
}

type ResourceLimits struct {
	MaxMessageBytes      int `json:"max_message_bytes"`
	MaxCaseBytes         int `json:"max_case_bytes"`
	MaxCasesPerRun       int `json:"max_cases_per_run"`
	MaxDurationMillis    int `json:"max_duration_millis"`
	MaxExtensionsPerItem int `json:"max_extensions_per_item"`
}

type EvaluatorDescriptor struct {
	SchemaVersion         string                 `json:"schema_version"`
	ProtocolName          string                 `json:"protocol_name"`
	ProtocolVersions      []string               `json:"protocol_versions"`
	EvaluatorID           string                 `json:"evaluator_id"`
	Implementation        ImplementationIdentity `json:"implementation"`
	ExecutionModes        []string               `json:"execution_modes"`
	ConformanceLevels     []ConformanceLevel     `json:"conformance_levels"`
	ScoreEvidenceVersions []string               `json:"score_evidence_versions"`
	DecisionVersions      []string               `json:"decision_evidence_versions"`
	Limits                ResourceLimits         `json:"limits"`
	LiveCapable           bool                   `json:"live_capable"`
	Extensions            []Extension            `json:"extensions"`
}

type TrajectoryRef struct {
	CanonicalSchema      string          `json:"canonical_schema"`
	SourceFormat         string          `json:"source_format"`
	SourceDigest         string          `json:"source_digest"`
	TrajectoryDigest     string          `json:"trajectory_digest"`
	AccountingDigest     string          `json:"accounting_digest"`
	TraceEnvelopeDigest  string          `json:"trace_envelope_digest"`
	MappingReportDigest  string          `json:"mapping_report_digest"`
	MappingPolicyVersion string          `json:"mapping_policy_version"`
	Inline               json.RawMessage `json:"inline,omitempty"`
}

type RequestVectorInput struct {
	VectorID         string `json:"vector_id"`
	CanonicalUTF8Hex string `json:"canonical_utf8_hex"`
	ExpectedSHA256   string `json:"expected_sha256"`
}

type CanonicalJSONInput struct {
	SourceUTF8Hex        string `json:"source_utf8_hex"`
	ExpectedCanonicalHex string `json:"expected_canonical_utf8_hex"`
	ExpectedSHA256       string `json:"expected_sha256"`
}

type AlignedPosition struct {
	TokenIndex int `json:"token_index"`
	TokenByte  int `json:"token_byte"`
	StreamByte int `json:"stream_byte"`
	RawByte    int `json:"raw_byte"`
}

type ScoreSupport struct {
	Letter       string   `json:"letter"`
	ValueDecimal string   `json:"value_decimal"`
	Probability  string   `json:"probability_decimal"`
	SourceRanks  []int    `json:"source_ranks"`
	SourceForms  []string `json:"source_forms"`
}

type VisibleAlternative struct {
	Rank                 int    `json:"rank"`
	Token                string `json:"token"`
	LogprobDecimal       string `json:"logprob_decimal"`
	Probability          string `json:"probability_decimal"`
	Chosen               bool   `json:"chosen"`
	CanonicalLetter      string `json:"canonical_letter,omitempty"`
	CanonicalValue       string `json:"canonical_value_decimal,omitempty"`
	Diagnostic           string `json:"diagnostic,omitempty"`
	CanonicalSourceRanks []int  `json:"canonical_source_ranks,omitempty"`
}

type Degradation struct {
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

type ScoreEvidence struct {
	SchemaVersion               string               `json:"schema_version"`
	PolicyVersion               string               `json:"policy_version"`
	Tag                         string               `json:"tag"`
	ExtractionMode              string               `json:"extraction_mode"`
	AlignmentStatus             string               `json:"alignment_status"`
	AlignedPosition             AlignedPosition      `json:"aligned_position"`
	RequestedTopK               int                  `json:"requested_top_k"`
	ReturnedTopK                int                  `json:"returned_top_k"`
	VisibleProbabilityMass      string               `json:"visible_probability_mass_decimal"`
	ValidScoreMass              string               `json:"valid_score_mass_decimal"`
	UnobservedProbabilityMass   string               `json:"unobserved_probability_mass_decimal"`
	ScoreSupport                []ScoreSupport       `json:"score_support"`
	VisibleAlternatives         []VisibleAlternative `json:"visible_alternatives"`
	ConditionalExpectedScore    string               `json:"conditional_expected_score_decimal"`
	ConditionalVariance         string               `json:"conditional_variance_decimal"`
	Extracted                   bool                 `json:"extracted"`
	Degradations                []Degradation        `json:"degradations"`
	RequestedModel              string               `json:"requested_model"`
	ServedModel                 string               `json:"served_model,omitempty"`
	RouteLimitations            []string             `json:"route_limitations"`
	RequestFingerprint          string               `json:"request_fingerprint"`
	ResponseEvidenceDigest      string               `json:"response_evidence_digest"`
	CapabilityAttestationDigest string               `json:"capability_attestation_digest,omitempty"`
}

type DecisionEvidence struct {
	SchemaVersion        string   `json:"schema_version"`
	PolicyVersion        string   `json:"policy_version"`
	State                string   `json:"state"`
	AbstentionReason     string   `json:"abstention_reason,omitempty"`
	Winner               *int     `json:"winner,omitempty"`
	ConditionalScores    []string `json:"conditional_scores_decimal"`
	DecisionStrength     string   `json:"decision_strength_decimal"`
	ScoreEvidenceDigests []string `json:"score_evidence_digests"`
	BudgetDigest         string   `json:"budget_digest"`
	ProvenanceDigest     string   `json:"provenance_digest"`
}

type ReplayEvidence struct {
	RequestFingerprint string `json:"request_fingerprint"`
	ResponseDigest     string `json:"response_digest"`
	EvidenceDigest     string `json:"evidence_digest"`
	ReplayStatus       string `json:"replay_status"`
	ReplayReason       string `json:"replay_reason,omitempty"`
}

type AttestationEvidence struct {
	SchemaVersion      string   `json:"schema_version"`
	AttestationDigest  string   `json:"attestation_digest"`
	State              string   `json:"state"`
	EvidenceCeiling    string   `json:"evidence_ceiling"`
	ObservedAt         string   `json:"observed_at"`
	ExpiresAt          string   `json:"expires_at"`
	RequestFingerprint string   `json:"request_fingerprint"`
	RouteLimitations   []string `json:"route_limitations"`
}

type AuditInvocation struct {
	SchemaVersion  string               `json:"schema_version"`
	InvocationID   string               `json:"invocation_id"`
	Operation      CaseKind             `json:"operation"`
	Offline        bool                 `json:"offline"`
	TimeoutMillis  int                  `json:"timeout_millis"`
	MaxInputBytes  int                  `json:"max_input_bytes"`
	TrajectoryRefs []TrajectoryRef      `json:"trajectory_refs"`
	CanonicalJSON  *CanonicalJSONInput  `json:"canonical_json,omitempty"`
	RequestVector  *RequestVectorInput  `json:"request_vector,omitempty"`
	ScoreEvidence  *ScoreEvidence       `json:"score_evidence,omitempty"`
	Decision       *DecisionEvidence    `json:"decision_evidence,omitempty"`
	Replay         *ReplayEvidence      `json:"replay_evidence,omitempty"`
	Attestation    *AttestationEvidence `json:"attestation_evidence,omitempty"`
	Extensions     []Extension          `json:"extensions"`
}

type ExpectedOutcome struct {
	Status         CaseExpectation `json:"status"`
	FindingCode    string          `json:"finding_code,omitempty"`
	ScoreDecimal   string          `json:"score_decimal,omitempty"`
	DecisionState  string          `json:"decision_state,omitempty"`
	EvidenceDigest string          `json:"evidence_digest,omitempty"`
}

type AuditCase struct {
	SchemaVersion        string           `json:"schema_version"`
	ProtocolVersion      string           `json:"protocol_version"`
	CaseID               string           `json:"case_id"`
	Level                ConformanceLevel `json:"conformance_level"`
	Kind                 CaseKind         `json:"kind"`
	Description          string           `json:"description"`
	RequiredCapabilities []string         `json:"required_capabilities"`
	Invocation           AuditInvocation  `json:"invocation"`
	Expected             ExpectedOutcome  `json:"expected"`
	Extensions           []Extension      `json:"extensions"`
}

type AuditFinding struct {
	SchemaVersion string `json:"schema_version"`
	Code          string `json:"code"`
	Severity      string `json:"severity"`
	Path          string `json:"path"`
	Message       string `json:"message"`
	Invariant     string `json:"invariant"`
}

type InvocationResult struct {
	SchemaVersion  string            `json:"schema_version"`
	InvocationID   string            `json:"invocation_id"`
	Status         InvocationStatus  `json:"status"`
	ScoreEvidence  *ScoreEvidence    `json:"score_evidence,omitempty"`
	Decision       *DecisionEvidence `json:"decision_evidence,omitempty"`
	Findings       []AuditFinding    `json:"findings"`
	ObservedDigest string            `json:"observed_artifact_digest,omitempty"`
	EvidenceDigest string            `json:"evidence_digest"`
	Extensions     []Extension       `json:"extensions"`
}

type CaseResult struct {
	CaseID         string           `json:"case_id"`
	Level          ConformanceLevel `json:"conformance_level"`
	Kind           CaseKind         `json:"kind"`
	Outcome        CaseOutcome      `json:"outcome"`
	Reason         string           `json:"reason"`
	FindingCodes   []string         `json:"finding_codes"`
	ObservedDigest string           `json:"observed_artifact_digest,omitempty"`
	ResultDigest   string           `json:"result_digest,omitempty"`
}

type CapabilityStatus struct {
	Level       ConformanceLevel `json:"conformance_level"`
	Passed      int              `json:"passed"`
	Failed      int              `json:"failed"`
	Skipped     int              `json:"skipped"`
	Unsupported int              `json:"unsupported"`
	NotRun      int              `json:"not_run"`
	Reasons     []string         `json:"reasons"`
}

type CapabilityMatrix struct {
	SchemaVersion string             `json:"schema_version"`
	Statuses      []CapabilityStatus `json:"statuses"`
}

type AuditRun struct {
	SchemaVersion        string              `json:"schema_version"`
	ProtocolVersion      string              `json:"protocol_version"`
	RunID                string              `json:"run_id"`
	Evaluator            EvaluatorDescriptor `json:"evaluator"`
	CorpusDigest         string              `json:"corpus_digest"`
	RequestCorpusDigest  string              `json:"request_corpus_digest"`
	SchemaArtifactDigest string              `json:"schema_artifact_digest"`
	Offline              bool                `json:"offline"`
	Results              []CaseResult        `json:"results"`
	Matrix               CapabilityMatrix    `json:"capability_matrix"`
	Findings             []AuditFinding      `json:"findings"`
	RunDigest            string              `json:"run_digest"`
}

type AdapterMessage struct {
	SchemaVersion   string          `json:"schema_version"`
	ProtocolVersion string          `json:"protocol_version"`
	MessageType     MessageType     `json:"message_type"`
	MessageID       string          `json:"message_id"`
	ReplyTo         string          `json:"reply_to,omitempty"`
	RunID           string          `json:"run_id,omitempty"`
	Payload         json.RawMessage `json:"payload"`
}

type Hello struct {
	SupportedVersions []string `json:"supported_versions"`
	CanonicalPolicy   string   `json:"canonical_policy"`
}

type HelloAck struct {
	SelectedVersion string `json:"selected_version"`
	CanonicalPolicy string `json:"canonical_policy"`
}

type RunBoundary struct {
	RunID        string `json:"run_id"`
	CaseCount    int    `json:"case_count"`
	CorpusDigest string `json:"corpus_digest"`
}

type ProtocolError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}
