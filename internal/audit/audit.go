package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

type Entry struct {
	EventType             string                            `json:"event_type,omitempty"`
	TS                    int64                             `json:"ts"`
	Provider              string                            `json:"provider"`
	Model                 string                            `json:"model"`
	CriterionID           string                            `json:"criterion_id"`
	RequestFingerprint    string                            `json:"request_fingerprint,omitempty"`
	ResponseDigest        string                            `json:"response_digest,omitempty"`
	ServedModel           string                            `json:"served_model,omitempty"`
	ProviderRequestID     string                            `json:"provider_request_id,omitempty"`
	ReplayStatus          provider.ReplayStatus             `json:"replay_status,omitempty"`
	ReplayReason          string                            `json:"replay_reason,omitempty"`
	ParserContractVersion string                            `json:"parser_contract_version,omitempty"`
	ScoreEvidence         map[string]verifier.ScoreEvidence `json:"score_evidence,omitempty"`
	Lineage               provider.RequestLineage           `json:"lineage"`
	InputTokens           int                               `json:"input_tokens"`
	OutputTokens          int                               `json:"output_tokens"`
	CachedTokens          int                               `json:"cached_tokens"`
	EstCostUSD            *float64                          `json:"est_cost_usd"`
	CacheHit              bool                              `json:"cache_hit"`
	CacheNamespace        string                            `json:"cache_namespace,omitempty"`
	Logprobs              bool                              `json:"logprobs"`
	RedactionHits         int                               `json:"redaction_hits,omitempty"`
	TrajectoryEvidence    []preprocess.AccountingSummary    `json:"trajectory_evidence,omitempty"`
	InconsistentPair      bool                              `json:"inconsistent_pair,omitempty"`
	RunFingerprint        string                            `json:"run_fingerprint,omitempty"`
	RequestSetFingerprint string                            `json:"request_set_fingerprint,omitempty"`
	VerificationMode      string                            `json:"verification_mode,omitempty"`
	DecisionState         string                            `json:"decision_state,omitempty"`
	AbstentionReason      string                            `json:"abstention_reason,omitempty"`
	EvidencePolicy        string                            `json:"evidence_policy,omitempty"`
	DecisionPolicyDigest  string                            `json:"decision_policy_digest,omitempty"`
	ProviderAttempts      int                               `json:"provider_attempts,omitempty"`
	ProviderAttempt       int                               `json:"provider_attempt,omitempty"`
	AttemptStatus         string                            `json:"attempt_status,omitempty"`
	InconsistentPairs     [][2]int                          `json:"inconsistent_pairs,omitempty"`
	Budget                *BudgetRecord                     `json:"budget,omitempty"`
	Lifecycle             *LifecycleRecord                  `json:"lifecycle,omitempty"`
	OutputStatus          string                            `json:"output_status,omitempty"`
	RunLineage            *RunLineageRecord                 `json:"run_lineage,omitempty"`
	CalibrationPolicy     *CalibrationPolicyRecord          `json:"calibration_policy,omitempty"`
	Fallback              *FallbackRecord                   `json:"fallback,omitempty"`
	Error                 string                            `json:"error,omitempty"`
}

type CalibrationPolicyRecord struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type FallbackRecord struct {
	Kind    string `json:"kind"`
	Charged bool   `json:"charged"`
	Calls   int    `json:"calls"`
	Tokens  int    `json:"tokens"`
	Reason  string `json:"reason,omitempty"`
}

type RunLineageRecord struct {
	AuditCaseID           string   `json:"audit_case_id,omitempty"`
	SourceTraceDigests    []string `json:"source_trace_digests"`
	TraceMapDigests       []string `json:"trace_map_digests"`
	TransformationID      string   `json:"transformation_id,omitempty"`
	OutcomeEvidenceDigest string   `json:"outcome_evidence_digest,omitempty"`
	ProfilePolicyDigest   string   `json:"profile_policy_digest,omitempty"`
	CapsuleDigest         string   `json:"capsule_digest,omitempty"`
	StudyCellID           string   `json:"study_cell_id,omitempty"`
	StudyManifestDigest   string   `json:"study_manifest_digest,omitempty"`
	StudyVariant          string   `json:"study_variant,omitempty"`
}

type BudgetRecord struct {
	Calls                int     `json:"calls"`
	Attempts             int     `json:"attempts"`
	EstimatedInputTokens int     `json:"estimated_input_tokens"`
	ReservedOutputTokens int     `json:"reserved_output_tokens"`
	PeakConcurrent       int     `json:"peak_concurrent"`
	EstimatedCostUSD     float64 `json:"estimated_cost_usd"`
	ElapsedSeconds       float64 `json:"elapsed_seconds"`
}

type LifecycleRecord struct {
	RuntimeOpen string `json:"runtime_open"`
	Execution   string `json:"execution"`
	Cleanup     string `json:"cleanup"`
	Audit       string `json:"audit"`
}

type Logger struct {
	mu      sync.Mutex
	file    *os.File
	enabled bool
}

func New(path string) (*Logger, error) {
	if path == "" {
		return &Logger{enabled: false}, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), safety.SensitiveDirectoryMode); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, safety.SensitiveFileMode)
	if err != nil {
		return nil, err
	}
	if err := f.Chmod(safety.SensitiveFileMode); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &Logger{file: f, enabled: true}, nil
}

func (l *Logger) Enabled() bool { return l != nil && l.enabled }

func (l *Logger) Write(e Entry) error {
	if !l.Enabled() {
		return nil
	}
	if e.TS == 0 {
		e.TS = time.Now().Unix()
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	enc := json.NewEncoder(l.file)
	if err := enc.Encode(e); err != nil {
		return err
	}
	return l.file.Sync()
}

func (l *Logger) Close() error {
	if !l.Enabled() {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}
