package verification

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Christopher-Schulze/evalwitness/internal/audit"
	"github.com/Christopher-Schulze/evalwitness/internal/log"
	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

func writeRunAudit(runner *mode.Runner, plan Plan, result Result, executionErr, cleanupErr error) error {
	if runner == nil || runner.Audit == nil {
		return nil
	}
	policyBytes, err := json.Marshal(plan.Input.Policy)
	if err != nil {
		return fmt.Errorf("encode verification decision policy: %w", err)
	}
	budget := audit.BudgetRecord{
		Calls: result.Budget.Calls, Attempts: result.Budget.Attempts,
		EstimatedInputTokens: result.Budget.EstimatedInputTokens, ReservedOutputTokens: result.Budget.ReservedOutputTokens,
		PeakConcurrent: result.Budget.PeakConcurrent, EstimatedCostUSD: result.Budget.EstimatedCostUSD,
		ElapsedSeconds: result.Budget.ElapsedSeconds,
	}
	status := "decision_complete"
	errorMessage := ""
	if executionErr != nil {
		status = "execution_failed"
		errorMessage = log.Redact(errors.Join(executionErr, cleanupErr).Error())
	} else if cleanupErr != nil {
		status = "cleanup_failed"
		errorMessage = log.Redact(cleanupErr.Error())
	}
	entry := audit.Entry{
		EventType: "verification_run", RunFingerprint: plan.RunFingerprint,
		RequestSetFingerprint: plan.Requests.SetFingerprint, VerificationMode: string(plan.Input.Mode),
		DecisionState: string(result.State), EvidencePolicy: string(plan.Input.Policy.Evidence),
		DecisionPolicyDigest: preprocess.Hash(string(policyBytes)), ProviderAttempts: result.Budget.Attempts,
		Budget: &budget, Lifecycle: &audit.LifecycleRecord{
			RuntimeOpen: string(result.Lifecycle.RuntimeOpen), Execution: string(result.Lifecycle.Execution),
			Cleanup: string(result.Lifecycle.Cleanup), Audit: string(LifecycleComplete),
		},
		OutputStatus: status, Error: errorMessage,
		CalibrationPolicy: &audit.CalibrationPolicyRecord{
			Status: result.CalibrationPolicy.Status, Reason: result.CalibrationPolicy.Reason,
		},
		Fallback: &audit.FallbackRecord{
			Kind: result.Fallback.Kind, Charged: result.Fallback.Charged, Calls: result.Fallback.Calls,
			Tokens: result.Fallback.Tokens, Reason: result.Fallback.Reason,
		},
	}
	entry.RunLineage = &audit.RunLineageRecord{
		AuditCaseID: plan.Input.Lineage.AuditCaseID, TransformationID: plan.Input.Lineage.TransformationID,
		OutcomeEvidenceDigest: plan.Input.Lineage.OutcomeEvidenceDigest,
		ProfilePolicyDigest:   plan.Input.Lineage.ProfilePolicyDigest, CapsuleDigest: plan.Input.Lineage.CapsuleDigest,
		StudyCellID: plan.Input.Lineage.StudyCellID, StudyManifestDigest: plan.Input.StudyManifestDigest,
		StudyVariant:       plan.Input.StudyVariant,
		SourceTraceDigests: make([]string, len(plan.TrajectoryEvidence)), TraceMapDigests: make([]string, len(plan.TrajectoryEvidence)),
	}
	for index, evidence := range plan.TrajectoryEvidence {
		entry.RunLineage.SourceTraceDigests[index] = evidence.SourceDigest
		entry.RunLineage.TraceMapDigests[index] = evidence.TraceMappingDigest
	}
	switch {
	case result.Delta != nil:
		entry.AbstentionReason = string(result.Delta.AbstentionReason)
		if result.Delta.Inconsistent {
			entry.InconsistentPairs = [][2]int{{0, 1}}
		}
	case result.Selection != nil:
		entry.AbstentionReason = string(result.Selection.AbstentionReason)
		entry.InconsistentPairs = append([][2]int(nil), result.Selection.InconsistentPairs...)
	}
	if err := runner.Audit.Write(entry); err != nil {
		return fmt.Errorf("write verification run audit: %w", err)
	}
	return nil
}
