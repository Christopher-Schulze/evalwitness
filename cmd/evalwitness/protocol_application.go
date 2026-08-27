package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

const applicationAdapterVersion = "evalwitness.application-adapter.v1"

var canonicalApplicationDecimal = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]*[1-9])?$`)

type applicationProtocolEvaluator struct {
	service *verification.Service
}

func (applicationProtocolEvaluator) Descriptor() protocolkit.EvaluatorDescriptor {
	payload := json.RawMessage(`{"schema_version":"evalwitness.protocol.application-extension-descriptor.v1"}`)
	return protocolkit.EvaluatorDescriptor{
		SchemaVersion: protocolkit.DescriptorSchema, ProtocolName: protocolkit.ProtocolName,
		ProtocolVersions: protocolkit.SupportedVersions(), EvaluatorID: "org.evalwitness.application-service",
		Implementation: protocolkit.ImplementationIdentity{
			Name: "EvalWitness application service adapter", Version: applicationAdapterVersion,
			IdentityDigest: protocolkit.DigestBytes([]byte(applicationAdapterVersion)),
		},
		ExecutionModes:        []string{"application_service", "sealed_replay"},
		ConformanceLevels:     []protocolkit.ConformanceLevel{protocolkit.LevelDeterministicReplay},
		ScoreEvidenceVersions: []string{protocolkit.ScoreSchema}, DecisionVersions: []string{protocolkit.DecisionSchema},
		Limits: protocolkit.DefaultLimits(), LiveCapable: false,
		Extensions: []protocolkit.Extension{{
			Namespace: protocolkit.ApplicationExtensionNamespace, Schema: protocolkit.ApplicationInvocationSchema,
			Required: false, Payload: payload,
		}},
	}
}

func (evaluator applicationProtocolEvaluator) Evaluate(auditCase protocolkit.AuditCase) protocolkit.InvocationResult {
	invocationID := auditCase.Invocation.InvocationID
	result := protocolkit.InvocationResult{
		SchemaVersion: protocolkit.ResultSchema, InvocationID: invocationID,
		Status: protocolkit.InvocationRejected, Findings: []protocolkit.AuditFinding{}, Extensions: []protocolkit.Extension{},
	}
	request, err := applicationRequest(auditCase)
	if err != nil {
		result.Findings = append(result.Findings, applicationFinding("protocol.application.invalid", err))
		return protocolkit.SealInvocationResult(result)
	}
	criteria := make([]verifier.Criterion, len(request.Criteria))
	for index, criterion := range request.Criteria {
		criteria[index] = verifier.Criterion{ID: criterion.ID, Name: criterion.Name, Description: criterion.Description}
	}
	trajectories := make([]string, len(auditCase.Invocation.TrajectoryRefs))
	for index, reference := range auditCase.Invocation.TrajectoryRefs {
		if len(reference.Inline) == 0 || string(reference.Inline) == "null" {
			result.Findings = append(result.Findings, applicationFinding("protocol.application.trajectory-inline", errors.New("application trajectory reference requires inline canonical content")))
			return protocolkit.SealInvocationResult(result)
		}
		canonical, canonicalErr := protocolkit.CanonicalizeJSON(reference.Inline)
		if canonicalErr != nil {
			result.Findings = append(result.Findings, applicationFinding("protocol.application.trajectory-inline", canonicalErr))
			return protocolkit.SealInvocationResult(result)
		}
		if !bytes.Equal(canonical, reference.Inline) {
			result.Findings = append(result.Findings, applicationFinding("protocol.application.trajectory-inline", errors.New("application inline trajectory source is not canonical JSON")))
			return protocolkit.SealInvocationResult(result)
		}
		trajectories[index] = string(canonical)
	}
	policy, err := applicationVerificationPolicy(request.Policy)
	if err != nil {
		result.Findings = append(result.Findings, applicationFinding("protocol.application.policy", err))
		return protocolkit.SealInvocationResult(result)
	}
	input := verification.Input{
		Entrypoint: "protocol.application", Mode: verification.Mode(request.Mode), Task: request.Task,
		Trajectories: trajectories, Criteria: criteria, Policy: policy,
		DisableCache: true,
		Lineage: verification.LineageReferences{
			AuditCaseID: request.Lineage.AuditCaseID, TransformationID: request.Lineage.TransformationID,
			OutcomeEvidenceDigest: request.Lineage.OutcomeEvidenceDigest, ProfilePolicyDigest: request.Lineage.ProfilePolicyDigest,
			CapsuleDigest: request.Lineage.CapsuleDigest, StudyCellID: request.Lineage.StudyCellID,
		},
	}
	plan, err := evaluator.service.Plan(input)
	if err != nil {
		result.Findings = append(result.Findings, applicationFinding("protocol.application.plan", err))
		return protocolkit.SealInvocationResult(result)
	}
	if err := verifyApplicationTrajectoryRefs(auditCase.Invocation.TrajectoryRefs, plan.TrajectoryEvidence); err != nil {
		result.Findings = append(result.Findings, applicationFinding("protocol.application.trajectory-binding", err))
		return protocolkit.SealInvocationResult(result)
	}
	timeout := time.Duration(auditCase.Invocation.TimeoutMillis) * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	runResult, err := evaluator.service.Execute(ctx, plan)
	if err != nil {
		result.Status = protocolkit.InvocationFailed
		result.Findings = append(result.Findings, applicationFinding("protocol.application.execution", err))
		return protocolkit.SealInvocationResult(result)
	}
	decision, err := applicationDecisionBytes(runResult)
	if err != nil {
		result.Status = protocolkit.InvocationFailed
		result.Findings = append(result.Findings, applicationFinding("protocol.application.result", err))
		return protocolkit.SealInvocationResult(result)
	}
	stableBudget := runResult.Budget
	stableBudget.ElapsedSeconds = 0
	budget, err := json.Marshal(stableBudget)
	if err != nil {
		result.Status = protocolkit.InvocationFailed
		result.Findings = append(result.Findings, applicationFinding("protocol.application.budget", err))
		return protocolkit.SealInvocationResult(result)
	}
	trajectoryDigests := make([]string, len(plan.TrajectoryEvidence))
	for index, evidence := range plan.TrajectoryEvidence {
		trajectoryDigests[index] = evidence.TrajectoryDigest
	}
	applicationResult := protocolkit.ApplicationResult{
		SchemaVersion: protocolkit.ApplicationResultSchema, RunFingerprint: plan.RunFingerprint,
		RequestSetFingerprint: plan.Requests.SetFingerprint,
		RequestFingerprints:   append([]string(nil), plan.Requests.Fingerprints...),
		DecisionUTF8Hex:       hex.EncodeToString(decision), BudgetDigest: digestApplicationBytes(budget),
		TrajectoryDigests: trajectoryDigests,
	}
	payload, err := json.Marshal(applicationResult)
	if err != nil {
		result.Status = protocolkit.InvocationFailed
		result.Findings = append(result.Findings, applicationFinding("protocol.application.result", err))
		return protocolkit.SealInvocationResult(result)
	}
	result.Status = protocolkit.InvocationAccepted
	result.ObservedDigest = plan.RunFingerprint
	result.Extensions = []protocolkit.Extension{{
		Namespace: protocolkit.ApplicationExtensionNamespace, Schema: protocolkit.ApplicationResultSchema,
		Required: false, Payload: payload,
	}}
	return protocolkit.SealInvocationResult(result)
}

func applicationRequest(auditCase protocolkit.AuditCase) (protocolkit.ApplicationInvocation, error) {
	if auditCase.Kind != protocolkit.CaseExtension || auditCase.Invocation.Operation != protocolkit.CaseExtension || !auditCase.Invocation.Offline {
		return protocolkit.ApplicationInvocation{}, errors.New("application adapter accepts only offline extension cases")
	}
	var requestExtension *protocolkit.Extension
	for index := range auditCase.Invocation.Extensions {
		extension := &auditCase.Invocation.Extensions[index]
		if extension.Namespace == protocolkit.ApplicationExtensionNamespace {
			if requestExtension != nil {
				return protocolkit.ApplicationInvocation{}, errors.New("application invocation extension is duplicated")
			}
			requestExtension = extension
		}
	}
	if requestExtension == nil || requestExtension.Schema != protocolkit.ApplicationInvocationSchema || !requestExtension.Required {
		return protocolkit.ApplicationInvocation{}, errors.New("required application invocation extension is missing")
	}
	var request protocolkit.ApplicationInvocation
	if err := protocolkit.DecodeStrict(requestExtension.Payload, &request); err != nil {
		return protocolkit.ApplicationInvocation{}, err
	}
	if request.SchemaVersion != protocolkit.ApplicationInvocationSchema {
		return protocolkit.ApplicationInvocation{}, errors.New("application invocation schema is unsupported")
	}
	return request, nil
}

func applicationVerificationPolicy(policy protocolkit.ApplicationPolicy) (verification.Policy, error) {
	epsilon, err := applicationDecimal(policy.Epsilon, "epsilon")
	if err != nil {
		return verification.Policy{}, err
	}
	confidence, err := applicationDecimal(policy.ConfidenceThreshold, "confidence threshold")
	if err != nil {
		return verification.Policy{}, err
	}
	calibration, err := applicationDecimal(policy.CalibrationSigma, "calibration sigma")
	if err != nil {
		return verification.Policy{}, err
	}
	alpha, err := applicationDecimal(policy.SPRT.Alpha, "SPRT alpha")
	if err != nil {
		return verification.Policy{}, err
	}
	beta, err := applicationDecimal(policy.SPRT.Beta, "SPRT beta")
	if err != nil {
		return verification.Policy{}, err
	}
	sigma, err := applicationDecimal(policy.SPRT.Sigma, "SPRT sigma")
	if err != nil {
		return verification.Policy{}, err
	}
	sprtEpsilon, err := applicationDecimal(policy.SPRT.Epsilon, "SPRT epsilon")
	if err != nil {
		return verification.Policy{}, err
	}
	return verification.Policy{
		Evidence: verification.EvidencePolicy(policy.Evidence), NReps: policy.NReps, Epsilon: epsilon,
		BiasMitigation: policy.BiasMitigation, InconsistencyPolicy: policy.InconsistencyPolicy,
		SelectionStrategy: policy.SelectionStrategy, SingleElim: policy.SingleElimination, UseSPRT: policy.UseSPRT,
		SPRTParams: verification.SPRTParameters{
			Alpha: alpha, Beta: beta, Sigma: sigma,
			Epsilon: sprtEpsilon, MinReps: policy.SPRT.MinReps, MaxReps: policy.SPRT.MaxReps,
		},
		MaxWorkers: policy.MaxWorkers, MaxPairCalls: policy.MaxPairCalls,
		ConfidenceThreshold: confidence, CalibrationSigma: calibration,
	}, nil
}

func applicationDecimal(value, name string) (float64, error) {
	if !canonicalApplicationDecimal.MatchString(value) {
		return 0, fmt.Errorf("application %s decimal is not canonical", name)
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, fmt.Errorf("application %s decimal is invalid", name)
	}
	return parsed, nil
}

func verifyApplicationTrajectoryRefs(references []protocolkit.TrajectoryRef, evidence []preprocess.AccountingSummary) error {
	if len(references) != len(evidence) {
		return errors.New("trajectory reference and prepared evidence counts differ")
	}
	for index, reference := range references {
		observed := evidence[index]
		if reference.CanonicalSchema != "evalwitness.trajectory.v1" || reference.SourceFormat != string(observed.SourceFormat) ||
			reference.SourceDigest != observed.SourceDigest || reference.TrajectoryDigest != observed.TrajectoryDigest ||
			reference.AccountingDigest != observed.IngestionDigest || reference.TraceEnvelopeDigest != observed.TraceEnvelopeDigest ||
			reference.MappingReportDigest != observed.TraceMappingDigest || reference.MappingPolicyVersion != observed.TraceMappingPolicy {
			return fmt.Errorf("trajectory reference %d does not match canonical preprocessing evidence", index)
		}
	}
	return nil
}

func applicationDecisionBytes(result verification.Result) ([]byte, error) {
	switch result.Mode {
	case verification.ModeDelta:
		return json.Marshal(result.Delta)
	case verification.ModeAbsolute:
		return json.Marshal(result.Absolute)
	case verification.ModePairwise:
		return json.Marshal(result.Selection)
	default:
		return nil, errors.New("application result mode is unsupported")
	}
}

func applicationFinding(code string, err error) protocolkit.AuditFinding {
	return protocolkit.AuditFinding{
		SchemaVersion: protocolkit.FindingSchema, Code: code, Severity: "error", Path: "/invocation",
		Message: err.Error(), Invariant: "application adapter inputs and outputs must remain bound to the production verification service",
	}
}

func digestApplicationBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}
