package registry

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// IntakeEntry is one submitted conformance artifact pending validation.
type IntakeEntry struct {
	// Identity
	EntryID   string `json:"entry_id"`
	Submitter string `json:"submitter"`
	CapsuleID string `json:"capsule_id"`

	// Content binding
	CapsuleDigest   string  `json:"capsule_digest"`
	ProfileDigest   string  `json:"profile_digest"`
	RequestContract string  `json:"request_contract"`
	ScorePolicy     string  `json:"score_policy"`
	TraceMapping    string  `json:"trace_mapping"`
	SchemaVersion   int     `json:"schema_version"`
	EndpointKind    string  `json:"endpoint_kind"`
	ThinkingMode    string  `json:"thinking_mode"`
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens int     `json:"max_output_tokens"`
	TopLogprobs     int     `json:"top_logprobs"`
	ScoreAlphabet   string  `json:"score_alphabet"`

	// Evidence metadata
	EvidenceLevel       string    `json:"evidence_level"`
	ObservedAt          time.Time `json:"observed_at"`
	ExpiresAt           time.Time `json:"expires_at"`
	ServedModel         string    `json:"served_model"`
	CheckpointAssertion string    `json:"checkpoint_assertion"`
	ProviderRequestID   string    `json:"provider_request_id"`
	ChallengeNonce      string    `json:"challenge_nonce"`
	LatencyMs           int64     `json:"latency_milliseconds"`
	HTTPAttempts        int       `json:"http_attempts"`

	RelianceOntologyDigest     string `json:"reliance_ontology_digest,omitempty"`
	ReliancePanelDigest        string `json:"reliance_panel_digest,omitempty"`
	RelianceEstimatorDigest    string `json:"reliance_estimator_digest,omitempty"`
	RelianceInterventionDigest string `json:"reliance_intervention_digest,omitempty"`
	RelianceOutcomeDigest      string `json:"reliance_outcome_digest,omitempty"`
	RelianceProfileDigest      string `json:"reliance_profile_digest,omitempty"`

	// Governance
	Status             string `json:"status"`
	License            string `json:"license"`
	PrivacyClass       string `json:"privacy_class"`
	PublicReleaseOK    bool   `json:"public_release_allowed"`
	ContaminationFree  bool   `json:"contamination_free"`
	CommunityValidated bool   `json:"community_validated"`
}

const IntakeStatusFormatVerified = "format_verified"

// ValidateIntake checks a single intake entry for schema and content validity.
func ValidateIntake(e IntakeEntry) error {
	if err := rejectUnsafeIntakeText("entry_id", e.EntryID); err != nil {
		return err
	}
	if err := rejectUnsafeIntakeText("submitter", e.Submitter); err != nil {
		return err
	}
	if err := rejectUnsafeIntakeText("capsule_id", e.CapsuleID); err != nil {
		return err
	}
	if err := rejectUnsafeIntakeText("request_contract", e.RequestContract); err != nil {
		return err
	}
	if err := rejectUnsafeIntakeText("score_policy", e.ScorePolicy); err != nil {
		return err
	}
	if err := rejectUnsafeIntakeText("trace_mapping", e.TraceMapping); err != nil {
		return err
	}
	if err := rejectUnsafeIntakeText("endpoint_kind", e.EndpointKind); err != nil {
		return err
	}
	if err := rejectUnsafeIntakeText("thinking_mode", e.ThinkingMode); err != nil {
		return err
	}
	if err := rejectUnsafeIntakeText("score_alphabet", e.ScoreAlphabet); err != nil {
		return err
	}
	if err := rejectUnsafeIntakeText("evidence_level", e.EvidenceLevel); err != nil {
		return err
	}
	if err := rejectUnsafeIntakeText("served_model", e.ServedModel); err != nil {
		return err
	}
	if err := rejectUnsafeIntakeText("checkpoint_assertion", e.CheckpointAssertion); err != nil {
		return err
	}
	if err := rejectUnsafeIntakeText("provider_request_id", e.ProviderRequestID); err != nil {
		return err
	}
	if err := rejectUnsafeIntakeText("license", e.License); err != nil {
		return err
	}
	if err := rejectUnsafeIntakeText("privacy_class", e.PrivacyClass); err != nil {
		return err
	}
	if strings.TrimSpace(e.EntryID) == "" {
		return fmt.Errorf("intake: entry_id required")
	}
	if strings.TrimSpace(e.Submitter) == "" {
		return fmt.Errorf("intake: submitter required")
	}
	if e.Status != IntakeStatusFormatVerified {
		return fmt.Errorf("intake: status must be %s; capsule_verified and independently_reproduced require later offline proofs", IntakeStatusFormatVerified)
	}
	if e.CommunityValidated {
		return fmt.Errorf("intake: community_validated must be false until an independent contributor entry is admitted")
	}
	if len(e.CapsuleDigest) != 64 {
		return fmt.Errorf("intake: capsule_digest must be SHA-256 hex (got %d chars)", len(e.CapsuleDigest))
	}
	if _, err := hex.DecodeString(e.CapsuleDigest); err != nil {
		return fmt.Errorf("intake: capsule_digest is not valid hex: %w", err)
	}
	if len(e.ProfileDigest) != 64 {
		return fmt.Errorf("intake: profile_digest must be SHA-256 hex")
	}
	if _, err := hex.DecodeString(e.ProfileDigest); err != nil {
		return fmt.Errorf("intake: profile_digest is not valid hex: %w", err)
	}
	if strings.TrimSpace(e.RequestContract) == "" {
		return fmt.Errorf("intake: request_contract required")
	}
	if e.SchemaVersion < 1 {
		return fmt.Errorf("intake: schema_version must be >= 1")
	}
	if strings.TrimSpace(e.EndpointKind) == "" {
		return fmt.Errorf("intake: endpoint_kind required")
	}
	if e.TopLogprobs < 20 {
		return fmt.Errorf("intake: top_logprobs must be >= 20 for strict verification")
	}
	if e.EvidenceLevel == "" {
		return fmt.Errorf("intake: evidence_level required")
	}
	if e.ObservedAt.IsZero() {
		return fmt.Errorf("intake: observed_at required")
	}
	if e.ExpiresAt.IsZero() || !e.ExpiresAt.After(e.ObservedAt) {
		return fmt.Errorf("intake: expires_at must be after observed_at")
	}
	if len(e.ChallengeNonce) != 64 {
		return fmt.Errorf("intake: challenge_nonce must be SHA-256 hex")
	}
	if _, err := hex.DecodeString(e.ChallengeNonce); err != nil {
		return fmt.Errorf("intake: challenge_nonce is not valid hex: %w", err)
	}
	if e.License == "" {
		return fmt.Errorf("intake: license required")
	}
	if e.PrivacyClass == "" {
		return fmt.Errorf("intake: privacy_class required")
	}
	if !e.PublicReleaseOK {
		return fmt.Errorf("intake: public_release_allowed must be true for registry submission")
	}
	if !e.ContaminationFree {
		return fmt.Errorf("intake: contamination_free must be true for registry submission")
	}
	return validateRelianceParents(e)
}

func (e IntakeEntry) hasRelianceParents() bool {
	return e.RelianceOntologyDigest != "" || e.ReliancePanelDigest != "" || e.RelianceEstimatorDigest != "" ||
		e.RelianceInterventionDigest != "" || e.RelianceOutcomeDigest != "" || e.RelianceProfileDigest != ""
}

func validateRelianceParents(e IntakeEntry) error {
	if !e.hasRelianceParents() {
		return nil
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"reliance_ontology_digest", e.RelianceOntologyDigest},
		{"reliance_panel_digest", e.ReliancePanelDigest},
		{"reliance_estimator_digest", e.RelianceEstimatorDigest},
		{"reliance_intervention_digest", e.RelianceInterventionDigest},
		{"reliance_outcome_digest", e.RelianceOutcomeDigest},
		{"reliance_profile_digest", e.RelianceProfileDigest},
	} {
		if err := rejectUnsafeIntakeText(field.name, field.value); err != nil {
			return err
		}
		if len(field.value) != 64 {
			return fmt.Errorf("intake: %s must be SHA-256 hex when any reliance parent is set", field.name)
		}
		if _, err := hex.DecodeString(field.value); err != nil {
			return fmt.Errorf("intake: %s is not valid hex: %w", field.name, err)
		}
	}
	return nil
}

func relianceCompatibilityKey(e IntakeEntry) string {
	return strings.Join([]string{
		e.RelianceOntologyDigest,
		e.ReliancePanelDigest,
		e.RelianceEstimatorDigest,
		e.RelianceInterventionDigest,
		e.RelianceOutcomeDigest,
		e.RelianceProfileDigest,
	}, "\x1f")
}

// IntakeValidationResult reports the outcome of validating one submission.
type IntakeValidationResult struct {
	EntryID     string   `json:"entry_id"`
	Valid       bool     `json:"valid"`
	DigestMatch bool     `json:"digest_match"`
	Freshness   string   `json:"freshness"`
	Errors      []string `json:"errors,omitempty"`
}

// IntakeValidator validates submitted intake entries against a reference
// capsule file. It performs offline-only checks: schema, digest, freshness,
// duplicate detection, and privacy policy compliance.
type IntakeValidator struct {
	entries   map[string]IntakeEntry // entry_id -> entry
	digestSet map[string]bool        // capsule+profile duplicate detection
	nonceSet  map[string]bool        // challenge_nonce replay detection
	nowFunc   func() time.Time       // injectable clock for testing
}

func NewIntakeValidator() *IntakeValidator {
	return &IntakeValidator{
		entries:   map[string]IntakeEntry{},
		digestSet: map[string]bool{},
		nonceSet:  map[string]bool{},
		nowFunc:   time.Now,
	}
}

// Add validates an intake entry and adds it to the validator's known set.
// Returns an error if the entry fails validation or duplicates a prior entry.
func (v *IntakeValidator) Add(e IntakeEntry) error {
	if err := ValidateIntake(e); err != nil {
		return err
	}
	now := v.nowFunc()
	if !now.Before(e.ExpiresAt) {
		return fmt.Errorf("intake: entry %q expired", e.EntryID)
	}
	if now.Before(e.ObservedAt) {
		return fmt.Errorf("intake: entry %q observed_at is in the future", e.EntryID)
	}
	if _, dup := v.entries[e.EntryID]; dup {
		return fmt.Errorf("intake: duplicate entry_id %q", e.EntryID)
	}
	key := e.CapsuleDigest + ":" + e.ProfileDigest
	if v.digestSet[key] {
		return fmt.Errorf("intake: duplicate capsule+profile pair %q", key[:24])
	}
	if v.nonceSet[e.ChallengeNonce] {
		return fmt.Errorf("intake: replayed challenge_nonce")
	}
	v.entries[e.EntryID] = e
	v.digestSet[key] = true
	v.nonceSet[e.ChallengeNonce] = true
	return nil
}

// Count returns the number of validated entries.
func (v *IntakeValidator) Count() int {
	return len(v.entries)
}

// IntakeReport summarizes all validated entries.
type IntakeReport struct {
	TotalEntries int           `json:"total_entries"`
	Submitters   []string      `json:"submitters"`
	Entries      []IntakeEntry `json:"entries"`
	GeneratedAt  time.Time     `json:"generated_at"`
}

// Report produces a deterministic summary of all validated intake entries.
func (v *IntakeValidator) Report() IntakeReport {
	submitterSet := map[string]bool{}
	var entries []IntakeEntry
	for _, e := range v.entries {
		submitterSet[e.Submitter] = true
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].EntryID < entries[j].EntryID })
	submitters := make([]string, 0, len(submitterSet))
	for s := range submitterSet {
		submitters = append(submitters, s)
	}
	sort.Strings(submitters)
	return IntakeReport{
		TotalEntries: len(entries),
		Submitters:   submitters,
		Entries:      entries,
		GeneratedAt:  v.nowFunc(),
	}
}

// Save writes the current intake report to a JSON file.
func (v *IntakeValidator) Save(path string) error {
	report := v.Report()
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode report: %w", err)
	}
	return os.WriteFile(path, b, 0600)
}

const MaxIntakeTextRunes = 256

func rejectUnsafeIntakeText(field, value string) error {
	if value == "" {
		return nil
	}
	if len([]rune(value)) > MaxIntakeTextRunes {
		return fmt.Errorf("intake: %s exceeds %d characters", field, MaxIntakeTextRunes)
	}
	lower := strings.ToLower(value)
	if strings.ContainsAny(value, "<>&`") {
		return fmt.Errorf("intake: %s contains markup or shell metacharacters", field)
	}
	if strings.Contains(value, "../") || strings.Contains(value, `..\`) {
		return fmt.Errorf("intake: %s contains a path traversal", field)
	}
	if strings.Contains(value, "\x00") {
		return fmt.Errorf("intake: %s contains a NUL byte", field)
	}
	if strings.Contains(lower, "javascript:") || strings.Contains(lower, "data:") {
		return fmt.Errorf("intake: %s contains a script or data URI", field)
	}
	return nil
}
