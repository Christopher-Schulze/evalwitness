package mode

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

const AuthorizationSchemaVersion = "evalwitness.live-authorization.v1"

type AuthorizationSpec struct {
	Entrypoint                 string
	RouteID                    string
	RequestFingerprint         provider.Fingerprint
	RequestContractDigest      string
	Limits                     BudgetLimits
	MaxRetries                 int
	MaxWorkers                 int
	MinDispatchIntervalSeconds int
	MaxOutputTokens            int
	ExpectedCalls              int
	WorstCalls                 int
	StudyManifestDigest        string
}

type AuthorizationLimits struct {
	MaxCalls                int     `json:"max_calls"`
	MaxAttempts             int     `json:"max_attempts"`
	MaxEstimatedInputTokens int     `json:"max_estimated_input_tokens"`
	MaxReservedOutputTokens int     `json:"max_reserved_output_tokens"`
	MaxConcurrent           int     `json:"max_concurrent"`
	MaxCostUSD              float64 `json:"max_cost_usd,omitempty"`
	MaxDurationSeconds      int64   `json:"max_duration_seconds"`
}

type AuthorizationPlan struct {
	SchemaVersion              string               `json:"schema_version"`
	Entrypoint                 string               `json:"entrypoint"`
	RouteID                    string               `json:"route_id"`
	RequestFingerprint         provider.Fingerprint `json:"request_fingerprint"`
	RequestContractDigest      string               `json:"request_contract_digest"`
	RequestSchemaVersion       int                  `json:"request_schema_version"`
	PromptBuilderVersion       string               `json:"prompt_builder_version"`
	ParserContractVersion      string               `json:"parser_contract_version"`
	ScorePolicyVersion         string               `json:"score_policy_version"`
	MaxRetries                 int                  `json:"max_retries"`
	MaxWorkers                 int                  `json:"max_workers"`
	MinDispatchIntervalSeconds int                  `json:"min_dispatch_interval_seconds,omitempty"`
	MaxOutputTokens            int                  `json:"max_output_tokens"`
	ExpectedCalls              int                  `json:"expected_calls"`
	WorstCalls                 int                  `json:"worst_calls"`
	Limits                     AuthorizationLimits  `json:"limits"`
	StudyManifestDigest        string               `json:"study_manifest_digest,omitempty"`
	AuthorizationDigest        string               `json:"authorization_digest"`
}

type AuthorizationError struct {
	Reason   string
	Expected string
	Provided string
}

func (e *AuthorizationError) Error() string {
	if e.Expected == "" && e.Provided == "" {
		return "live authorization rejected: " + e.Reason
	}
	return fmt.Sprintf("live authorization rejected: %s (expected %s, provided %s)", e.Reason, e.Expected, e.Provided)
}

func BuildAuthorizationPlan(spec AuthorizationSpec) (AuthorizationPlan, error) {
	if strings.TrimSpace(spec.Entrypoint) == "" || strings.TrimSpace(spec.RouteID) == "" {
		return AuthorizationPlan{}, errors.New("authorization entrypoint and route ID are required")
	}
	if strings.TrimSpace(string(spec.RequestFingerprint)) == "" || strings.TrimSpace(spec.RequestContractDigest) == "" {
		return AuthorizationPlan{}, errors.New("authorization request fingerprint and contract digest are required")
	}
	if spec.MaxRetries < 0 || spec.MaxWorkers <= 0 || spec.MinDispatchIntervalSeconds < 0 || spec.MaxOutputTokens <= 0 || spec.ExpectedCalls < 0 || spec.WorstCalls <= 0 || spec.ExpectedCalls > spec.WorstCalls {
		return AuthorizationPlan{}, errors.New("authorization execution bounds are invalid")
	}
	if err := validateAuthorizationLimits(spec.Limits); err != nil {
		return AuthorizationPlan{}, err
	}
	if spec.MaxWorkers > spec.Limits.MaxConcurrent {
		return AuthorizationPlan{}, errors.New("authorization max workers exceeds hard concurrency limit")
	}
	if spec.WorstCalls > spec.Limits.MaxCalls {
		return AuthorizationPlan{}, errors.New("authorization worst calls exceed hard call limit")
	}
	plan := AuthorizationPlan{
		SchemaVersion:              AuthorizationSchemaVersion,
		Entrypoint:                 spec.Entrypoint,
		RouteID:                    spec.RouteID,
		RequestFingerprint:         spec.RequestFingerprint,
		RequestContractDigest:      spec.RequestContractDigest,
		RequestSchemaVersion:       provider.RequestSchemaVersion,
		PromptBuilderVersion:       provider.PromptBuilderVersion,
		ParserContractVersion:      provider.ParserContractVersion,
		ScorePolicyVersion:         verifier.StrictPolicyVersion,
		MaxRetries:                 spec.MaxRetries,
		MaxWorkers:                 spec.MaxWorkers,
		MinDispatchIntervalSeconds: spec.MinDispatchIntervalSeconds,
		MaxOutputTokens:            spec.MaxOutputTokens,
		ExpectedCalls:              spec.ExpectedCalls,
		WorstCalls:                 spec.WorstCalls,
		Limits:                     authorizationLimits(spec.Limits),
		StudyManifestDigest:        strings.TrimSpace(spec.StudyManifestDigest),
	}
	digest, err := plan.Digest()
	if err != nil {
		return AuthorizationPlan{}, err
	}
	plan.AuthorizationDigest = digest
	return plan, nil
}

func (p AuthorizationPlan) Digest() (string, error) {
	p.AuthorizationDigest = ""
	encoded, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("encode authorization plan: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func (p AuthorizationPlan) Verify(provided string) error {
	expected, err := p.Digest()
	if err != nil {
		return err
	}
	if p.SchemaVersion != AuthorizationSchemaVersion || p.AuthorizationDigest != expected {
		return &AuthorizationError{Reason: "plan digest is internally inconsistent", Expected: expected, Provided: p.AuthorizationDigest}
	}
	provided = strings.TrimSpace(provided)
	if provided == "" {
		return &AuthorizationError{Reason: "explicit --authorize digest is required", Expected: expected, Provided: "missing"}
	}
	if provided != expected {
		return &AuthorizationError{Reason: "execution plan changed or a different plan was approved", Expected: expected, Provided: provided}
	}
	return nil
}

func validateAuthorizationLimits(limits BudgetLimits) error {
	switch {
	case limits.MaxCalls <= 0:
		return errors.New("live authorization requires max calls > 0")
	case limits.MaxAttempts <= 0:
		return errors.New("live authorization requires max attempts > 0")
	case limits.MaxEstimatedInputTokens <= 0:
		return errors.New("live authorization requires max input tokens > 0")
	case limits.MaxReservedOutputTokens <= 0:
		return errors.New("live authorization requires max output tokens > 0")
	case limits.MaxConcurrent <= 0:
		return errors.New("live authorization requires max concurrency > 0")
	case limits.MaxDuration < time.Second:
		return errors.New("live authorization requires max duration >= 1s")
	case limits.MaxCostUSD < 0:
		return errors.New("live authorization max cost must be >= 0")
	}
	return nil
}

func authorizationLimits(limits BudgetLimits) AuthorizationLimits {
	return AuthorizationLimits{
		MaxCalls:                limits.MaxCalls,
		MaxAttempts:             limits.MaxAttempts,
		MaxEstimatedInputTokens: limits.MaxEstimatedInputTokens,
		MaxReservedOutputTokens: limits.MaxReservedOutputTokens,
		MaxConcurrent:           limits.MaxConcurrent,
		MaxCostUSD:              limits.MaxCostUSD,
		MaxDurationSeconds:      int64(limits.MaxDuration.Seconds()),
	}
}
