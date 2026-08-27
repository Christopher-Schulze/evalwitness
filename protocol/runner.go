package protocol

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
)

func RunReferenceCorpus(corpus NormativeCorpus) (AuditRun, error) {
	evaluator := ReferenceEvaluator{}
	descriptor := evaluator.Descriptor()
	if err := ValidateDescriptor(descriptor); err != nil {
		return AuditRun{}, fmt.Errorf("validate reference descriptor: %w", err)
	}
	results := make([]CaseResult, 0, len(corpus.Vectors))
	findings := make([]AuditFinding, 0)
	for _, vector := range corpus.Vectors {
		caseResult, caseFindings := runReferenceVector(evaluator, vector, descriptor.Limits)
		results = append(results, caseResult)
		findings = append(findings, caseFindings...)
	}
	runID := "run-" + corpus.Digest[:16]
	run := AuditRun{
		SchemaVersion: RunSchema, ProtocolVersion: CurrentVersion, RunID: runID,
		Evaluator: descriptor, CorpusDigest: corpus.Digest, RequestCorpusDigest: corpus.RequestCorpusDigest,
		SchemaArtifactDigest: corpus.SchemaArtifactDigest,
		Offline:              true, Results: results, Matrix: BuildCapabilityMatrix(results), Findings: findings,
	}
	material := run
	material.RunDigest = ""
	digest, err := Digest(material)
	if err != nil {
		return AuditRun{}, err
	}
	run.RunDigest = digest
	if err := ValidateAuditRun(run); err != nil {
		return AuditRun{}, err
	}
	return run, nil
}

func runReferenceVector(evaluator ReferenceEvaluator, vector CaseVector, limits ResourceLimits) (CaseResult, []AuditFinding) {
	caseResult := CaseResult{
		CaseID: vector.CaseID, Level: vector.Level, Kind: vector.Kind,
		Outcome: OutcomeFailed, Reason: "case was not evaluated", FindingCodes: []string{},
	}
	result := evaluateAdapterPayload(evaluator, vector.Raw, limits)
	if err := ValidateInvocationResult(result, invocationIDFromRaw(vector.Raw)); err != nil {
		caseResult.Reason = err.Error()
		return caseResult, result.Findings
	}
	caseResult.ResultDigest = result.EvidenceDigest
	caseResult.ObservedDigest = result.ObservedDigest
	for _, finding := range result.Findings {
		caseResult.FindingCodes = append(caseResult.FindingCodes, finding.Code)
	}
	sort.Strings(caseResult.FindingCodes)
	matched, reason := resultMatchesExpected(vector.Expected, result)
	if matched {
		caseResult.Outcome = OutcomePassed
	} else {
		caseResult.Outcome = OutcomeFailed
	}
	caseResult.Reason = reason
	return caseResult, result.Findings
}

func resultMatchesExpected(expected ExpectedOutcome, result InvocationResult) (bool, string) {
	switch expected.Status {
	case ExpectationAccepted:
		if result.Status != InvocationAccepted {
			return false, fmt.Sprintf("expected accepted, got %s", result.Status)
		}
		if expected.ScoreDecimal != "" {
			if result.ScoreEvidence == nil || result.ScoreEvidence.ConditionalExpectedScore != expected.ScoreDecimal {
				return false, "accepted score evidence did not match expected closed-form score"
			}
		}
		if expected.DecisionState != "" {
			if result.Decision == nil || result.Decision.State != expected.DecisionState {
				return false, "accepted decision did not match expected state"
			}
		}
		if expected.EvidenceDigest != "" && result.ObservedDigest != expected.EvidenceDigest {
			return false, "accepted result did not emit the expected observed artifact digest"
		}
		return true, "accepted with complete evidence"
	case ExpectationRejected:
		if result.Status != InvocationRejected {
			return false, fmt.Sprintf("expected rejected, got %s", result.Status)
		}
		if expected.FindingCode != "" && !containsFinding(result.Findings, expected.FindingCode) {
			return false, "rejection omitted the expected finding code"
		}
		return true, "rejected without synthesizing evidence"
	case ExpectationUnsupported:
		if result.Status != InvocationUnsupported {
			return false, fmt.Sprintf("expected unsupported, got %s", result.Status)
		}
		return true, "unsupported capability reported explicitly"
	default:
		return false, "invalid expected status"
	}
}

func BuildCapabilityMatrix(results []CaseResult) CapabilityMatrix {
	levels := []ConformanceLevel{
		LevelSyntax, LevelDeterministicReplay, LevelLiveScoreEvidence,
		LevelEmpiricalReliability, LevelIndependentReproduction,
	}
	byLevel := make(map[ConformanceLevel][]CaseResult)
	for _, result := range results {
		byLevel[result.Level] = append(byLevel[result.Level], result)
	}
	statuses := make([]CapabilityStatus, 0, len(levels))
	for _, level := range levels {
		status := CapabilityStatus{Level: level, Reasons: []string{}}
		for _, result := range byLevel[level] {
			switch result.Outcome {
			case OutcomePassed:
				status.Passed++
			case OutcomeFailed:
				status.Failed++
			case OutcomeSkipped:
				status.Skipped++
			case OutcomeUnsupported:
				status.Unsupported++
			case OutcomeNotRun:
				status.NotRun++
			}
			if result.Outcome != OutcomePassed && result.Reason != "" {
				status.Reasons = append(status.Reasons, result.CaseID+": "+result.Reason)
			}
		}
		if len(byLevel[level]) == 0 {
			status.NotRun = 1
			status.Reasons = append(status.Reasons, "no normative case was run for this claim level")
		}
		sort.Strings(status.Reasons)
		statuses = append(statuses, status)
	}
	return CapabilityMatrix{SchemaVersion: MatrixSchema, Statuses: statuses}
}

func ValidateAuditRun(run AuditRun) error {
	if run.SchemaVersion != RunSchema || !supportedVersion(run.ProtocolVersion) || run.RunID == "" || !run.Offline {
		return errors.New("audit run schema, version, identity, or offline state is invalid")
	}
	if !validDigest(run.CorpusDigest) || run.RequestCorpusDigest != "" && !validDigest(run.RequestCorpusDigest) ||
		!validDigest(run.SchemaArtifactDigest) || !validDigest(run.RunDigest) {
		return errors.New("audit run corpus or run digest is invalid")
	}
	if err := ValidateDescriptor(run.Evaluator); err != nil {
		return err
	}
	if run.Results == nil || run.Findings == nil || run.Matrix.Statuses == nil {
		return errors.New("audit run omits a required array")
	}
	for _, result := range run.Results {
		if result.CaseID == "" || PermittedClaim(result.Level) == "" || !validCaseKind(result.Kind) || !validOutcome(result.Outcome) {
			return errors.New("audit run contains an invalid case result")
		}
		if result.ObservedDigest != "" && !validDigest(result.ObservedDigest) ||
			result.ResultDigest != "" && !validDigest(result.ResultDigest) {
			return errors.New("audit run contains an invalid case-result digest")
		}
	}
	if run.Matrix.SchemaVersion != MatrixSchema || len(run.Matrix.Statuses) != 5 {
		return errors.New("audit run capability matrix is incomplete")
	}
	for _, status := range run.Matrix.Statuses {
		if status.Reasons == nil {
			return errors.New("audit run capability status omits reasons")
		}
	}
	if expectedMatrix := BuildCapabilityMatrix(run.Results); !reflect.DeepEqual(run.Matrix, expectedMatrix) {
		return errors.New("audit run capability matrix does not match case results")
	}
	for _, finding := range run.Findings {
		if err := ValidateFinding(finding); err != nil {
			return err
		}
	}
	material := run
	material.RunDigest = ""
	expected, err := Digest(material)
	if err != nil {
		return err
	}
	if expected != run.RunDigest {
		return errors.New("audit run digest does not match content")
	}
	return nil
}

func validOutcome(outcome CaseOutcome) bool {
	return outcome == OutcomePassed || outcome == OutcomeFailed || outcome == OutcomeSkipped ||
		outcome == OutcomeUnsupported || outcome == OutcomeNotRun
}

func containsFinding(findings []AuditFinding, code string) bool {
	for _, finding := range findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}
