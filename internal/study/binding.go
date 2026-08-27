package study

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

type ExecutionBinding struct {
	ArmID                      string   `json:"arm_id"`
	Entrypoint                 string   `json:"entrypoint"`
	RouteID                    string   `json:"route_id"`
	RequestContractDigest      string   `json:"request_contract_digest"`
	Commit                     string   `json:"commit"`
	Dirty                      bool     `json:"dirty"`
	BinaryDigest               string   `json:"binary_digest"`
	AnalysisDigest             string   `json:"analysis_digest"`
	AnalysisCommand            []string `json:"analysis_command"`
	AnalysisVersion            string   `json:"analysis_version"`
	InputPaths                 []string `json:"input_paths"`
	InputDigests               []string `json:"input_digests"`
	ExpectedCalls              int      `json:"expected_calls"`
	HardCalls                  int      `json:"hard_calls"`
	HardAttempts               int      `json:"hard_attempts"`
	HardInputTokens            int      `json:"hard_input_tokens"`
	HardOutputTokens           int      `json:"hard_output_tokens"`
	HardDurationSeconds        int64    `json:"hard_duration_seconds"`
	HardConcurrent             int      `json:"hard_concurrent"`
	HardCostUSD                float64  `json:"hard_cost_usd"`
	DecidableTasks             int      `json:"decidable_tasks"`
	NominalAlpha               float64  `json:"nominal_alpha"`
	TargetPower                float64  `json:"target_power"`
	MinimumEffect              float64  `json:"minimum_effect"`
	DisagreementRate           float64  `json:"disagreement_rate"`
	DiscordantWinProbability   float64  `json:"discordant_win_probability"`
	PowerAtMinimumEffect       float64  `json:"power_at_minimum_effect"`
	PrimaryFamilySize          int      `json:"primary_family_size"`
	ServedIdentityPolicy       string   `json:"served_identity_policy"`
	ExpectedServedModel        string   `json:"expected_served_model"`
	ExpectedServedModels       []string `json:"expected_served_models,omitempty"`
	RetryPolicyVersion         string   `json:"retry_policy_version"`
	MaxRetries                 int      `json:"max_retries"`
	RequestTimeoutSeconds      int      `json:"request_timeout_seconds"`
	MinDispatchIntervalSeconds int      `json:"min_dispatch_interval_seconds"`
}

func VerifyExecutionBinding(record Record, binding ExecutionBinding) error {
	if err := record.Validate(); err != nil {
		return err
	}
	if record.State != StateAuthorized {
		return fmt.Errorf("live study requires authorized lifecycle state, got %s", record.State)
	}
	manifest := record.Study.Manifest
	matchedArm := false
	for _, arm := range manifest.Arms {
		if arm.ID == binding.ArmID && arm.Entrypoint == binding.Entrypoint && arm.RouteID == binding.RouteID && arm.RequestContractDigest == binding.RequestContractDigest {
			matchedArm = true
			break
		}
	}
	if !matchedArm {
		return errors.New("entrypoint, route, or request contract is not a locked study arm")
	}
	execution := manifest.Execution
	if binding.Dirty || execution.Dirty || binding.Commit != execution.Commit || binding.BinaryDigest != execution.BinaryDigest || binding.AnalysisDigest != execution.AnalysisDigest ||
		binding.AnalysisVersion != execution.AnalysisVersion || !slices.Equal(binding.AnalysisCommand, execution.AnalysisCommand) {
		return errors.New("commit, clean state, binary, or analysis implementation differs from the locked execution")
	}
	if !slices.Equal(binding.InputPaths, execution.DeclaredInputPaths) || !slices.Equal(binding.InputDigests, execution.DeclaredInputDigests) {
		return errors.New("execution inputs differ in path, digest, count, or order from the locked manifest")
	}
	budget := manifest.Budget
	if binding.ExpectedCalls != budget.ExpectedCalls || binding.HardCalls != budget.HardCalls || binding.HardAttempts != budget.HardAttempts ||
		binding.HardInputTokens != budget.HardInputTokens || binding.HardOutputTokens != budget.HardOutputTokens ||
		binding.HardDurationSeconds != budget.HardDurationSeconds || binding.HardConcurrent != budget.HardConcurrent || binding.HardCostUSD != budget.HardCostUSD {
		return errors.New("execution budget differs from the locked manifest")
	}
	inference := manifest.Inference
	if binding.DecidableTasks != inference.DecidableTasks || binding.NominalAlpha != inference.NominalAlpha || binding.TargetPower != inference.TargetPower ||
		binding.MinimumEffect != inference.MinimumEffect || binding.DisagreementRate != inference.DisagreementRate ||
		binding.DiscordantWinProbability != inference.DiscordantWinProbability || binding.PowerAtMinimumEffect != inference.PowerAtMinimumEffect ||
		binding.PrimaryFamilySize != len(inference.PrimaryFamily) {
		return errors.New("statistical design differs from the locked manifest")
	}
	var providerPlan *ProviderPlan
	for index := range manifest.Providers {
		if manifest.Providers[index].ArmID == binding.ArmID {
			providerPlan = &manifest.Providers[index]
			break
		}
	}
	if providerPlan == nil {
		return errors.New("execution arm has no locked provider plan")
	}
	if binding.ServedIdentityPolicy != providerPlan.ServedIdentityPolicy || binding.ExpectedServedModel != providerPlan.ExpectedServedModel ||
		!slices.Equal(binding.ExpectedServedModels, providerPlan.ExpectedServedModels) {
		return errors.New("served identity policy differs from the locked manifest")
	}
	if binding.RetryPolicyVersion != providerPlan.RetryPolicyVersion || binding.MaxRetries != providerPlan.MaxRetries ||
		binding.RequestTimeoutSeconds != providerPlan.RequestTimeoutSeconds || binding.MinDispatchIntervalSeconds != providerPlan.MinDispatchIntervalSeconds {
		return errors.New("provider retry, timeout, or dispatch-interval policy differs from the locked manifest")
	}
	return nil
}

func VerifyDeclaredInputs(record Record, root string) error {
	if err := record.Validate(); err != nil {
		return err
	}
	for index, relativePath := range record.Study.Manifest.Execution.DeclaredInputPaths {
		digest, err := CanonicalPathDigest(root, relativePath)
		if err != nil {
			return fmt.Errorf("declared input %q: %w", relativePath, err)
		}
		if digest != record.Study.Manifest.Execution.DeclaredInputDigests[index] {
			return fmt.Errorf("declared input %q digest changed", relativePath)
		}
	}
	return nil
}

func CanonicalPathDigest(root, relativePath string) (string, error) {
	if filepath.IsAbs(relativePath) || filepath.Clean(relativePath) != relativePath || strings.HasPrefix(relativePath, "..") {
		return "", errors.New("input path must be clean and repository-relative")
	}
	rootAbsolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target := filepath.Join(rootAbsolute, relativePath)
	info, err := os.Lstat(target)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("input path must not be a symbolic link")
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return "", errors.New("input path must be a regular file or directory")
	}
	paths := []string{target}
	if info.IsDir() {
		paths = paths[:0]
		err = filepath.WalkDir(target, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("symbolic link %q is not a governed input", path)
			}
			if !entry.IsDir() && !entry.Type().IsRegular() {
				return fmt.Errorf("non-regular input %q is not supported", path)
			}
			paths = append(paths, path)
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	sort.Strings(paths)
	digest := sha256.New()
	for _, path := range paths {
		relative, err := filepath.Rel(rootAbsolute, path)
		if err != nil {
			return "", err
		}
		if _, err := io.WriteString(digest, filepath.ToSlash(relative)+"\x00"); err != nil {
			return "", err
		}
		entryInfo, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		if entryInfo.IsDir() {
			if _, err := io.WriteString(digest, "directory\x00"); err != nil {
				return "", err
			}
			continue
		}
		if _, err := io.WriteString(digest, "file\x00"); err != nil {
			return "", err
		}
		file, err := os.Open(path)
		if err != nil {
			return "", err
		}
		_, copyErr := io.Copy(digest, file)
		closeErr := file.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		if _, err := digest.Write([]byte{0}); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
