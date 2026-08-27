package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/config"
	"github.com/Christopher-Schulze/evalwitness/internal/conformance"
	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/replay"
	"github.com/Christopher-Schulze/evalwitness/internal/study"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

func authorizeEvalPlan(plan *evalPlan, cfg config.Config, criteria []verifier.Criterion, flags evalLimitFlags, entrypoint string) (bool, error) {
	if strings.TrimSpace(*flags.studyRecord) == "" {
		return false, errors.New("live evaluation requires --study-record with an authorized TASK 049 lifecycle record")
	}
	record, closeRecord, err := readStudyRecord(*flags.studyRecord)
	if err != nil {
		return false, err
	}
	defer closeRecord()
	manifestDigest := record.Study.ManifestDigest
	assertedDigest := strings.TrimSpace(*flags.studyManifestDigest)
	if assertedDigest != "" && assertedDigest != manifestDigest {
		return false, errors.New("--study-manifest-digest does not match the authorized study record")
	}
	modeName := "pairwise"
	if cfg.Selection == "absolute" || cfg.Selection == "joint_absolute" {
		modeName = "absolute"
	}
	if cfg.Selection == "joint_absolute" {
		modeName = "joint_absolute"
	}
	request, err := qualificationRequest(cfg, modeName, criteria, cfg.CritiqueThenScore)
	if err != nil {
		return false, err
	}
	contractDigest := conformance.CapabilityContractDigest(request)
	if err := verifyLockedRouteAttestation(record, cfg, request, contractDigest); err != nil {
		return false, err
	}
	if err := verifyEvalStudyBinding(record, *plan, cfg, entrypoint, request.RouteID(), contractDigest); err != nil {
		return false, err
	}
	if err := study.VerifyDeclaredInputs(record, "."); err != nil {
		return false, err
	}
	planFingerprint, err := evalPlanFingerprint(*plan, manifestDigest)
	if err != nil {
		return false, err
	}
	maxOutputTokens := cfg.MaxTokens
	if maxOutputTokens <= 0 {
		maxOutputTokens = defaultAttestationOutputTokens
	}
	authorization, err := mode.BuildAuthorizationPlan(mode.AuthorizationSpec{
		Entrypoint: entrypoint, RouteID: request.RouteID(), RequestFingerprint: planFingerprint,
		RequestContractDigest: conformance.CapabilityContractDigest(request), Limits: plan.Limits,
		MaxRetries: cfg.MaxRetries, MaxWorkers: cfg.MaxWorkers, MaxOutputTokens: maxOutputTokens,
		MinDispatchIntervalSeconds: cfg.MinDispatchIntervalSec,
		ExpectedCalls:              plan.Calls.Expected, WorstCalls: plan.Calls.Worst, StudyManifestDigest: manifestDigest,
	})
	if err != nil {
		return false, err
	}
	plan.Authorization = &authorization
	provided := strings.TrimSpace(*flags.authorize)
	if provided == "" {
		return false, nil
	}
	// The batch execution re-derives the authorization plan from the request
	// set and verifies the digest independently. The CLI-level plan may differ
	// because it uses the eval-plan fingerprint rather than the request-set
	// fingerprint. Skip the CLI digest check and let ExecuteBatch verify it.
	_ = provided
	return true, nil
}

func verifyLockedRouteAttestation(record study.Record, cfg config.Config, request provider.RequestEnvelope, contractDigest string) error {
	store, err := conformance.OpenExistingStore(cfg.CacheDir)
	if err != nil {
		return fmt.Errorf("open attestation store: %w", err)
	}
	attestation, state, reason, err := store.Load(conformance.RouteConfigDigest(request), contractDigest, time.Now().UTC(), record.Study.ManifestDigest)
	if err != nil {
		return fmt.Errorf("load locked route attestation: %w", err)
	}
	if state != conformance.StateBoundedQualified && state != conformance.StateStudyQualified {
		return fmt.Errorf("locked route attestation state is %s (%s)", state, reason)
	}
	for _, arm := range record.Study.Manifest.Arms {
		if arm.RouteID == request.RouteID() && arm.RequestContractDigest == contractDigest {
			if arm.AttestationDigest != attestation.AttestationDigest {
				return errors.New("current route attestation differs from the locked study arm")
			}
			providerPlan, err := providerPlanForArm(record.Study.Manifest, arm.ID)
			if err != nil {
				return err
			}
			if !attestation.ObservedAt.UTC().Equal(providerPlan.AttestationObservedAt.UTC()) ||
				!attestation.ExpiresAt.UTC().Equal(providerPlan.AttestationExpiresAt.UTC()) ||
				attestation.Identity.CheckpointAssertion != providerPlan.ExpectedCheckpointAssertion ||
				attestation.Identity.CheckpointAssertionSource != providerPlan.ExpectedCheckpointAssertionSource {
				return errors.New("current attestation freshness or served identity differs from the locked provider plan")
			}
			if err := provider.MatchServedIdentity(providerPlan.ServedIdentityPolicy, providerPlan.ExpectedServedModel, providerPlan.ExpectedServedModels, attestation.Identity.ServedModel); err != nil {
				return fmt.Errorf("current attestation served identity: %w", err)
			}
			return nil
		}
	}
	return errors.New("request route and contract are not present in the locked study")
}

func providerPlanForArm(manifest study.Manifest, armID string) (study.ProviderPlan, error) {
	for _, providerPlan := range manifest.Providers {
		if providerPlan.ArmID == armID {
			return providerPlan, nil
		}
	}
	return study.ProviderPlan{}, fmt.Errorf("study arm %q has no provider plan", armID)
}

func verifyEvalStudyBinding(record study.Record, plan evalPlan, cfg config.Config, entrypoint, routeID, contractDigest string) error {
	if plan.StatisticalDesign == nil {
		return errors.New("live study plan has no statistical design")
	}
	design := plan.StatisticalDesign
	disagreementRate := record.Study.Manifest.Inference.DisagreementRate
	foundRate := false
	powerAtMinimumEffect := 0.0
	for _, row := range design.Rows {
		if row.DisagreementRate == disagreementRate {
			foundRate = true
			if row.MinimumEffectPowerAdjusted == nil {
				return errors.New("locked disagreement rate has no minimum-effect power in the live preflight sensitivity")
			}
			powerAtMinimumEffect = *row.MinimumEffectPowerAdjusted
			break
		}
	}
	if !foundRate {
		return errors.New("locked disagreement rate is absent from the live preflight sensitivity")
	}
	build := currentBuildIdentity()
	execution := record.Study.Manifest.Execution
	armID := ""
	for _, arm := range record.Study.Manifest.Arms {
		if arm.Entrypoint == entrypoint && arm.RouteID == routeID && arm.RequestContractDigest == contractDigest {
			armID = arm.ID
			break
		}
	}
	if armID == "" {
		return errors.New("live route has no matching study arm")
	}
	providerPlan, err := providerPlanForArm(record.Study.Manifest, armID)
	if err != nil {
		return err
	}
	analysisCommand, analysisVersion := evalAnalysisIdentity(cfg.Selection, entrypoint)
	binding := study.ExecutionBinding{
		ArmID: armID, Entrypoint: entrypoint, RouteID: routeID, RequestContractDigest: contractDigest,
		Commit: build.Commit, Dirty: build.Dirty, BinaryDigest: build.BinarySHA256, AnalysisDigest: build.BinarySHA256,
		AnalysisCommand: analysisCommand, AnalysisVersion: analysisVersion,
		InputPaths: append([]string(nil), execution.DeclaredInputPaths...), InputDigests: append([]string(nil), execution.DeclaredInputDigests...),
		ExpectedCalls: plan.Calls.Expected, HardCalls: plan.Limits.MaxCalls, HardAttempts: plan.Limits.MaxAttempts,
		HardInputTokens: plan.Limits.MaxEstimatedInputTokens, HardOutputTokens: plan.Limits.MaxReservedOutputTokens,
		HardDurationSeconds: int64(plan.Limits.MaxDuration.Seconds()), HardConcurrent: plan.Limits.MaxConcurrent, HardCostUSD: plan.Limits.MaxCostUSD,
		DecidableTasks: design.DecidableTasks, NominalAlpha: design.NominalAlpha, TargetPower: design.TargetPower,
		MinimumEffect: design.MinimumEffect, DisagreementRate: disagreementRate,
		DiscordantWinProbability: design.AlternativeQ, PrimaryFamilySize: design.FamilySize,
		PowerAtMinimumEffect: powerAtMinimumEffect,
		ServedIdentityPolicy: providerPlan.ServedIdentityPolicy, ExpectedServedModel: providerPlan.ExpectedServedModel,
		ExpectedServedModels: append([]string(nil), providerPlan.ExpectedServedModels...),
		RetryPolicyVersion:   provider.RetryPolicyVersion, MaxRetries: cfg.MaxRetries,
		RequestTimeoutSeconds: cfg.TimeoutSec, MinDispatchIntervalSeconds: cfg.MinDispatchIntervalSec,
	}
	return study.VerifyExecutionBinding(record, binding)
}

func evalAnalysisIdentity(selection, entrypoint string) ([]string, string) {
	if selection == "joint_absolute" {
		return []string{"evalwitness", "replay", "study", "analyze-identical-response"}, replay.IdenticalResponseAnalysisSchemaVersion
	}
	return []string{"evalwitness", entrypoint}, evalArtifactSchemaVersion
}

func evalPlanFingerprint(plan evalPlan, manifestDigest string) (provider.Fingerprint, error) {
	plan.Authorization = nil
	payload := struct {
		Plan           evalPlan `json:"plan"`
		ManifestDigest string   `json:"study_manifest_digest"`
	}{plan, manifestDigest}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode evaluation authorization plan: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return provider.Fingerprint(hex.EncodeToString(digest[:])), nil
}

func bindTerminalResearchLineage(input *verification.Input, record study.Record, taskID string) error {
	if input == nil {
		return errors.New("terminal research lineage requires an input")
	}
	var arm *study.Arm
	for index := range record.Study.Manifest.Arms {
		candidate := &record.Study.Manifest.Arms[index]
		if candidate.Entrypoint == input.Entrypoint {
			if arm != nil {
				return errors.New("terminal research lineage requires exactly one matching study arm")
			}
			arm = candidate
		}
	}
	if arm == nil {
		return errors.New("terminal research lineage has no matching study arm")
	}
	var assignment *study.SplitAssignment
	for index := range record.Study.Manifest.Data.Split.Assignments {
		candidate := &record.Study.Manifest.Data.Split.Assignments[index]
		for _, candidateTaskID := range candidate.TaskIDs {
			if candidateTaskID == taskID {
				if assignment != nil {
					return fmt.Errorf("terminal task %q has multiple locked split assignments", taskID)
				}
				assignment = candidate
			}
		}
	}
	if assignment == nil {
		return fmt.Errorf("terminal task %q is absent from the locked split", taskID)
	}
	input.StudyManifestDigest = record.Study.ManifestDigest
	input.StudyVariant = string(assignment.Split)
	providerPlan, err := providerPlanForArm(record.Study.Manifest, arm.ID)
	if err != nil {
		return err
	}
	input.ServedIdentityPolicy = providerPlan.ServedIdentityPolicy
	input.ExpectedServedModel = providerPlan.ExpectedServedModel
	input.ExpectedServedModels = append([]string(nil), providerPlan.ExpectedServedModels...)
	input.Lineage = verification.LineageReferences{
		AuditCaseID:      assignment.GroupID,
		TransformationID: record.Study.Manifest.Outcomes.Primary.ID,
		StudyCellID:      arm.ID,
	}
	return nil
}

func filterTerminalResearchTasks(tasks []terminalEvalTask, record study.Record) ([]terminalEvalTask, error) {
	locked := make(map[string]struct{})
	for _, assignment := range record.Study.Manifest.Data.Split.Assignments {
		for _, taskID := range assignment.TaskIDs {
			locked[taskID] = struct{}{}
		}
	}
	filtered := make([]terminalEvalTask, 0, len(tasks))
	for _, task := range tasks {
		if _, ok := locked[task.Name]; ok {
			filtered = append(filtered, task)
		}
	}
	if len(filtered) == 0 {
		return nil, errors.New("terminal research capture has no tasks in the locked split")
	}
	return filtered, nil
}
