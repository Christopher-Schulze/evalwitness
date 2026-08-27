package protocol

import (
	"errors"
	"fmt"
)

const referenceImplementationVersion = "evalwitness.closed-form-reference-adapter.v1"

type ReferenceEvaluator struct{}

func (ReferenceEvaluator) Descriptor() EvaluatorDescriptor {
	return EvaluatorDescriptor{
		SchemaVersion:    DescriptorSchema,
		ProtocolName:     ProtocolName,
		ProtocolVersions: SupportedVersions(),
		EvaluatorID:      "org.evalwitness.closed-form-reference",
		Implementation: ImplementationIdentity{
			Name: "EvalWitness closed-form protocol adapter", Version: referenceImplementationVersion,
			IdentityDigest: DigestBytes([]byte(referenceImplementationVersion)),
		},
		ExecutionModes:        []string{"closed_form", "sealed_replay"},
		ConformanceLevels:     []ConformanceLevel{LevelSyntax, LevelDeterministicReplay},
		ScoreEvidenceVersions: []string{ScoreSchema},
		DecisionVersions:      []string{DecisionSchema},
		Limits:                DefaultLimits(),
		LiveCapable:           false,
		Extensions:            []Extension{},
	}
}

func (ReferenceEvaluator) Evaluate(auditCase AuditCase) InvocationResult {
	result := InvocationResult{
		SchemaVersion: ResultSchema, InvocationID: auditCase.Invocation.InvocationID,
		Status: InvocationAccepted, Findings: []AuditFinding{}, Extensions: cloneExtensions(auditCase.Invocation.Extensions),
	}
	if err := ValidateInvocation(auditCase.Invocation); err != nil {
		result.Status = InvocationRejected
		result.Findings = append(result.Findings, findingForError(err))
		return sealInvocationResult(result)
	}
	switch auditCase.Kind {
	case CaseCanonicalEncoding:
		result.ObservedDigest = auditCase.Invocation.CanonicalJSON.ExpectedSHA256
	case CaseRequestFingerprint:
		result.ObservedDigest = auditCase.Invocation.RequestVector.ExpectedSHA256
	case CaseScoreEvidence:
		evidence := *auditCase.Invocation.ScoreEvidence
		result.ScoreEvidence = &evidence
	case CaseDecisionEvidence:
		decision := *auditCase.Invocation.Decision
		result.Decision = &decision
	case CaseExtension:
		if err := ValidateExtensions(auditCase.Invocation.Extensions, DefaultLimits().MaxExtensionsPerItem); err != nil {
			result.Status = InvocationRejected
			result.Findings = append(result.Findings, findingForError(err))
		}
	case CaseReplayEvidence:
		result.ObservedDigest = auditCase.Invocation.Replay.EvidenceDigest
	case CaseAttestation:
		result.ObservedDigest = auditCase.Invocation.Attestation.AttestationDigest
	case CaseCompatibility:
	default:
		result.Status = InvocationUnsupported
		result.Findings = append(result.Findings, AuditFinding{
			SchemaVersion: FindingSchema, Code: "protocol.operation.unsupported", Severity: "error",
			Path: "/invocation/operation", Message: "reference adapter does not implement the operation",
			Invariant: "an adapter must report unsupported instead of inventing a result",
		})
	}
	return sealInvocationResult(result)
}

func sealInvocationResult(result InvocationResult) InvocationResult {
	material := result
	material.EvidenceDigest = ""
	digest, err := Digest(material)
	if err != nil {
		result.Status = InvocationFailed
		result.EvidenceDigest = ""
		result.Findings = append(result.Findings, findingForError(fmt.Errorf("seal invocation result: %w", err)))
		return result
	}
	result.EvidenceDigest = digest
	return result
}

func SealInvocationResult(result InvocationResult) InvocationResult {
	return sealInvocationResult(result)
}

func ValidateInvocationResult(result InvocationResult, invocationID string) error {
	if result.SchemaVersion != ResultSchema || result.InvocationID != invocationID {
		return errors.New("invocation result schema or identity is invalid")
	}
	if result.Status != InvocationAccepted && result.Status != InvocationRejected &&
		result.Status != InvocationUnsupported && result.Status != InvocationCancelled && result.Status != InvocationFailed {
		return errors.New("invocation result status is invalid")
	}
	if result.Findings == nil || result.Extensions == nil {
		return errors.New("invocation result omits a required array")
	}
	if !validDigest(result.EvidenceDigest) {
		return errors.New("invocation result evidence digest is invalid")
	}
	if result.ObservedDigest != "" && !validDigest(result.ObservedDigest) {
		return errors.New("invocation result observed artifact digest is invalid")
	}
	material := result
	material.EvidenceDigest = ""
	expected, err := Digest(material)
	if err != nil {
		return err
	}
	if expected != result.EvidenceDigest {
		return errors.New("invocation result evidence digest does not match content")
	}
	for _, finding := range result.Findings {
		if err := ValidateFinding(finding); err != nil {
			return err
		}
	}
	if result.ScoreEvidence != nil {
		if err := ValidateScoreEvidence(*result.ScoreEvidence); err != nil {
			return fmt.Errorf("result score evidence: %w", err)
		}
	}
	if result.Decision != nil {
		if err := ValidateDecisionEvidence(*result.Decision); err != nil {
			return fmt.Errorf("result decision evidence: %w", err)
		}
	}
	return ValidateExtensions(result.Extensions, DefaultLimits().MaxExtensionsPerItem)
}

func ValidateFinding(finding AuditFinding) error {
	if finding.SchemaVersion != FindingSchema || !identifierPattern.MatchString(finding.Code) ||
		finding.Message == "" || finding.Invariant == "" || finding.Path == "" {
		return errors.New("audit finding is incomplete")
	}
	if finding.Severity != "info" && finding.Severity != "warning" && finding.Severity != "error" {
		return errors.New("audit finding severity is invalid")
	}
	return nil
}

func findingForError(err error) AuditFinding {
	return AuditFinding{
		SchemaVersion: FindingSchema,
		Code:          "protocol.semantic.invalid",
		Severity:      "error",
		Path:          "/invocation",
		Message:       err.Error(),
		Invariant:     "invalid evidence must be rejected without a synthesized score or decision",
	}
}

func cloneExtensions(extensions []Extension) []Extension {
	result := make([]Extension, len(extensions))
	for index, extension := range extensions {
		result[index] = extension
		result[index].Payload = append([]byte(nil), extension.Payload...)
	}
	return result
}
