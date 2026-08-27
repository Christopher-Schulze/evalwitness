package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

const bonManifestSchemaVersion = "evalwitness.best-of-n-run.v1"
const maxBonManifestBytes = 8 << 20

type bonRunManifest struct {
	SchemaVersion          string                  `json:"schema_version"`
	CreatedAt              string                  `json:"created_at"`
	RepositoryRoot         string                  `json:"repository_root"`
	RunDirectory           string                  `json:"run_directory"`
	Source                 bonSourceSnapshot       `json:"source"`
	Destination            bonDestinationState     `json:"destination_before_attempts"`
	Task                   string                  `json:"task"`
	Command                []string                `json:"command"`
	ApplyAuthorized        bool                    `json:"apply_authorized"`
	Criteria               []verifier.Criterion    `json:"criteria"`
	Policy                 verification.Policy     `json:"policy"`
	Limits                 bonBudgetLimits         `json:"limits"`
	BudgetStatePath        string                  `json:"budget_state_path,omitempty"`
	Attempts               []bonAttempt            `json:"attempts"`
	PlanFingerprint        string                  `json:"plan_fingerprint"`
	SelectionAuthorization *mode.AuthorizationPlan `json:"selection_authorization,omitempty"`
}

type bonBudgetLimits struct {
	MaxCalls                int     `json:"max_calls"`
	MaxAttempts             int     `json:"max_attempts"`
	MaxEstimatedInputTokens int     `json:"max_estimated_input_tokens"`
	MaxReservedOutputTokens int     `json:"max_reserved_output_tokens"`
	MaxConcurrent           int     `json:"max_concurrent"`
	MaxCostUSD              float64 `json:"max_cost_usd"`
	MaxDurationNanoseconds  int64   `json:"max_duration_nanoseconds"`
}

func newBonBudgetLimits(limits mode.BudgetLimits) bonBudgetLimits {
	return bonBudgetLimits{
		limits.MaxCalls, limits.MaxAttempts, limits.MaxEstimatedInputTokens, limits.MaxReservedOutputTokens,
		limits.MaxConcurrent, limits.MaxCostUSD, int64(limits.MaxDuration),
	}
}

func (limits bonBudgetLimits) modeLimits() mode.BudgetLimits {
	return mode.BudgetLimits{
		MaxCalls: limits.MaxCalls, MaxAttempts: limits.MaxAttempts,
		MaxEstimatedInputTokens: limits.MaxEstimatedInputTokens, MaxReservedOutputTokens: limits.MaxReservedOutputTokens,
		MaxConcurrent: limits.MaxConcurrent, MaxCostUSD: limits.MaxCostUSD,
		MaxDuration: time.Duration(limits.MaxDurationNanoseconds),
	}
}

func writeBonManifest(manifest bonRunManifest) (string, error) {
	manifest.SchemaVersion = bonManifestSchemaVersion
	if manifest.CreatedAt == "" {
		manifest.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode Best-of-N run manifest: %w", err)
	}
	encoded = append(encoded, '\n')
	path := filepath.Join(manifest.RunDirectory, "run.json")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, safety.SensitiveFileMode)
	if err != nil {
		return "", fmt.Errorf("create Best-of-N run manifest: %w", err)
	}
	if _, err := file.Write(encoded); err != nil {
		return "", errors.Join(err, file.Close())
	}
	if err := file.Sync(); err != nil {
		return "", errors.Join(err, file.Close())
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return path, nil
}

func loadBonManifest(path string) (bonRunManifest, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return bonRunManifest{}, err
	}
	if !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > maxBonManifestBytes {
		return bonRunManifest{}, errors.New("Best-of-N run manifest must be a bounded regular file")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return bonRunManifest{}, errors.New("Best-of-N run manifest permissions are not private")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return bonRunManifest{}, err
	}
	var manifest bonRunManifest
	if err := decodeStrictJSON(raw, &manifest); err != nil {
		return bonRunManifest{}, fmt.Errorf("decode Best-of-N run manifest: %w", err)
	}
	if manifest.SchemaVersion != bonManifestSchemaVersion || manifest.RepositoryRoot == "" || manifest.RunDirectory == "" ||
		manifest.Task == "" || len(manifest.Command) == 0 || len(manifest.Criteria) == 0 || len(manifest.Attempts) < 2 ||
		manifest.PlanFingerprint == "" {
		return bonRunManifest{}, errors.New("Best-of-N run manifest is incomplete")
	}
	cleanPath := filepath.Clean(path)
	if filepath.Clean(manifest.RunDirectory) != filepath.Dir(cleanPath) {
		return bonRunManifest{}, errors.New("Best-of-N run manifest directory does not match its path")
	}
	for index := range manifest.Attempts {
		if manifest.Attempts[index].Index != index {
			return bonRunManifest{}, fmt.Errorf("attempt %d identity does not match its manifest position", index)
		}
		if err := loadBonAttemptArtifacts(&manifest.Attempts[index], manifest.RunDirectory); err != nil {
			return bonRunManifest{}, fmt.Errorf("verify attempt %d artifacts: %w", index, err)
		}
	}
	return manifest, nil
}

func loadBonAttemptArtifacts(attempt *bonAttempt, runDirectory string) error {
	expectedDiff := filepath.Join(runDirectory, fmt.Sprintf("attempt-%d.diff", attempt.Index))
	expectedTranscript := filepath.Join(runDirectory, fmt.Sprintf("attempt-%d.transcript", attempt.Index))
	if attempt.Index < 0 || filepath.Clean(attempt.DiffPath) != filepath.Clean(expectedDiff) ||
		filepath.Clean(attempt.TranscriptPath) != filepath.Clean(expectedTranscript) {
		return errors.New("attempt artifact path does not match its immutable run location")
	}
	diff, err := readBonArtifact(attempt.DiffPath, maxAttemptDiffBytes)
	if err != nil {
		return err
	}
	transcript, err := readBonArtifact(attempt.TranscriptPath, maxTranscriptBytes)
	if err != nil {
		return err
	}
	if digestBytes(diff) != attempt.DiffDigest || digestBytes(transcript) != attempt.TranscriptDigest {
		return errors.New("attempt artifact digest mismatch")
	}
	attempt.Diff = string(diff)
	attempt.Transcript = string(transcript)
	return nil
}

func readBonArtifact(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("Best-of-N artifact must be a regular file")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, errors.New("Best-of-N artifact permissions are not private")
	}
	if info.Size() < 0 || info.Size() > limit {
		return nil, fmt.Errorf("Best-of-N artifact exceeds %d bytes", limit)
	}
	return os.ReadFile(path)
}

func decodeStrictJSON(raw []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("JSON contains a trailing value")
		}
		return err
	}
	return nil
}
