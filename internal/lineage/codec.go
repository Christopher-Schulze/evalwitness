package lineage

import (
	"errors"
	"fmt"
	"io"

	"github.com/Christopher-Schulze/evalwitness/internal/study"
)

func DecodePlan(reader io.Reader) (VerificationLineagePlan, error) {
	var plan VerificationLineagePlan
	if err := study.DecodeStrict(reader, &plan); err != nil {
		return VerificationLineagePlan{}, fmt.Errorf("decode verification-lineage plan: %w", err)
	}
	if err := plan.Validate(); err != nil {
		return VerificationLineagePlan{}, err
	}
	return plan, nil
}

func DecodeParserLock(reader io.Reader) (VerificationLineageParserLock, error) {
	var lock VerificationLineageParserLock
	if err := study.DecodeStrict(reader, &lock); err != nil {
		return VerificationLineageParserLock{}, fmt.Errorf("decode verification-lineage parser lock: %w", err)
	}
	if err := lock.Validate(); err != nil {
		return VerificationLineageParserLock{}, err
	}
	return lock, nil
}

func DecodeSourceReadinessAudit(reader io.Reader) (VerificationLineageSourceReadinessAudit, error) {
	var audit VerificationLineageSourceReadinessAudit
	if err := study.DecodeStrict(reader, &audit); err != nil {
		return VerificationLineageSourceReadinessAudit{}, fmt.Errorf("decode verification-lineage source-readiness audit: %w", err)
	}
	if err := audit.Validate(); err != nil {
		return VerificationLineageSourceReadinessAudit{}, err
	}
	return audit, nil
}

func DecodeHoldoutReadinessAudit(repositoryRoot string, reader io.Reader) (VerificationLineageHoldoutReadinessAudit, error) {
	var audit VerificationLineageHoldoutReadinessAudit
	if err := study.DecodeStrict(reader, &audit); err != nil {
		return VerificationLineageHoldoutReadinessAudit{}, fmt.Errorf("decode verification-lineage holdout-readiness audit: %w", err)
	}
	if err := audit.Validate(repositoryRoot); err != nil {
		return VerificationLineageHoldoutReadinessAudit{}, err
	}
	return audit, nil
}

func DecodeCorpusFeasibilityDecision(repositoryRoot string, reader io.Reader) (VerificationLineageCorpusFeasibilityDecision, error) {
	var decision VerificationLineageCorpusFeasibilityDecision
	if err := study.DecodeStrict(reader, &decision); err != nil {
		return VerificationLineageCorpusFeasibilityDecision{}, fmt.Errorf("decode verification-lineage corpus-feasibility decision: %w", err)
	}
	if err := decision.Validate(repositoryRoot); err != nil {
		return VerificationLineageCorpusFeasibilityDecision{}, err
	}
	return decision, nil
}

func DecodeCapabilityMatrix(reader io.Reader) (VerificationLineageCapabilityMatrix, error) {
	var matrix VerificationLineageCapabilityMatrix
	if err := study.DecodeStrict(reader, &matrix); err != nil {
		return VerificationLineageCapabilityMatrix{}, fmt.Errorf("decode verification-lineage capability matrix: %w", err)
	}
	if err := matrix.Validate(); err != nil {
		return VerificationLineageCapabilityMatrix{}, err
	}
	return matrix, nil
}

func DecodeOfflineProof(repositoryRoot string, reader io.Reader) (VerificationLineageOfflineProof, error) {
	var proof VerificationLineageOfflineProof
	if err := study.DecodeStrict(reader, &proof); err != nil {
		return VerificationLineageOfflineProof{}, fmt.Errorf("decode verification-lineage offline proof: %w", err)
	}
	if err := proof.Validate(repositoryRoot); err != nil {
		return VerificationLineageOfflineProof{}, err
	}
	return proof, nil
}

func DecodeLossCertificate(repositoryRoot string, reader io.Reader) (VerificationLineageLossCertificate, error) {
	var certificate VerificationLineageLossCertificate
	if err := study.DecodeStrict(reader, &certificate); err != nil {
		return VerificationLineageLossCertificate{}, fmt.Errorf("decode verification-lineage loss certificate: %w", err)
	}
	if err := certificate.Validate(repositoryRoot); err != nil {
		return VerificationLineageLossCertificate{}, err
	}
	return certificate, nil
}

func DecodeLineageGraph(repositoryRoot string, reader io.Reader) (VerificationLineageGraph, error) {
	var graph VerificationLineageGraph
	if err := study.DecodeStrict(reader, &graph); err != nil {
		return VerificationLineageGraph{}, fmt.Errorf("decode verification-lineage graph: %w", err)
	}
	if err := graph.Validate(repositoryRoot); err != nil {
		return VerificationLineageGraph{}, err
	}
	return graph, nil
}

func DecodeOfflineBOM(reader io.Reader) (VerificationEvidenceBOM, error) {
	var bom VerificationEvidenceBOM
	if err := study.DecodeStrict(reader, &bom); err != nil {
		return VerificationEvidenceBOM{}, fmt.Errorf("decode verification-lineage offline BOM: %w", err)
	}
	if err := bom.Validate(); err != nil {
		return VerificationEvidenceBOM{}, err
	}
	return bom, nil
}

func DecodeOfflineAudit(reader io.Reader) (LineageAudit, error) {
	var audit LineageAudit
	if err := study.DecodeStrict(reader, &audit); err != nil {
		return LineageAudit{}, fmt.Errorf("decode verification-lineage offline audit: %w", err)
	}
	if err := audit.Validate(); err != nil {
		return LineageAudit{}, err
	}
	return audit, nil
}

func DecodeDevelopmentDatasetCard(reader io.Reader) (VerificationLineageDatasetCard, error) {
	var card VerificationLineageDatasetCard
	if err := study.DecodeStrict(reader, &card); err != nil {
		return VerificationLineageDatasetCard{}, fmt.Errorf("decode verification-lineage dataset card: %w", err)
	}
	if err := card.Validate(); err != nil {
		return VerificationLineageDatasetCard{}, err
	}
	return card, nil
}

func DecodeLimitationsLedger(reader io.Reader) (VerificationLineageLimitationsLedger, error) {
	var ledger VerificationLineageLimitationsLedger
	if err := study.DecodeStrict(reader, &ledger); err != nil {
		return VerificationLineageLimitationsLedger{}, fmt.Errorf("decode verification-lineage limitations: %w", err)
	}
	if err := ledger.Validate(); err != nil {
		return VerificationLineageLimitationsLedger{}, err
	}
	return ledger, nil
}

func DecodeDevelopmentRelease(reader io.Reader) (VerificationLineageRelease, error) {
	var release VerificationLineageRelease
	if err := study.DecodeStrict(reader, &release); err != nil {
		return VerificationLineageRelease{}, fmt.Errorf("decode verification-lineage development release: %w", err)
	}
	if err := release.Validate(); err != nil {
		return VerificationLineageRelease{}, err
	}
	return release, nil
}

func DecodeDocument(documentType string, reader io.Reader) (DocumentSummary, error) {
	switch documentType {
	case "plan":
		value, err := DecodePlan(reader)
		return DocumentSummary{SchemaVersion: value.SchemaVersion, ObjectID: value.Identity.PlanID, Valid: err == nil, Digest: value.Digest}, err
	case "source":
		return decodeArtifact[VerificationLineageSource](reader, func(value VerificationLineageSource) error { return value.Validate() })
	case "execution-witness":
		return decodeArtifact[ExecutionWitness](reader, func(value ExecutionWitness) error { return value.Validate() })
	case "candidate":
		return decodeArtifact[LineageCandidate](reader, func(value LineageCandidate) error { return value.Validate() })
	case "assessment":
		return decodeArtifact[LineageAssessment](reader, func(value LineageAssessment) error { return value.Validate() })
	case "capability-vector":
		return decodeArtifact[TraceCapabilityVector](reader, func(value TraceCapabilityVector) error { return value.Validate() })
	case "audit":
		return decodeArtifact[LineageAudit](reader, func(value LineageAudit) error { return value.Validate() })
	case "bom":
		return decodeArtifact[VerificationEvidenceBOM](reader, func(value VerificationEvidenceBOM) error { return value.Validate() })
	case "dataset-card":
		return decodeArtifact[VerificationLineageDatasetCard](reader, func(value VerificationLineageDatasetCard) error { return value.Validate() })
	case "release":
		return decodeArtifact[VerificationLineageRelease](reader, func(value VerificationLineageRelease) error { return value.Validate() })
	default:
		return DocumentSummary{}, fmt.Errorf("unsupported lineage document type %q", documentType)
	}
}

func decodeArtifact[T any](reader io.Reader, validate func(T) error) (DocumentSummary, error) {
	var value T
	if err := study.DecodeStrict(reader, &value); err != nil {
		return DocumentSummary{}, fmt.Errorf("decode verification-lineage artifact: %w", err)
	}
	if err := validate(value); err != nil {
		return DocumentSummary{}, err
	}
	header, err := headerOf(value)
	if err != nil {
		return DocumentSummary{}, err
	}
	return DocumentSummary{SchemaVersion: header.SchemaVersion, ObjectID: header.ObjectID, Valid: true, Digest: header.Digest}, nil
}

func headerOf(value any) (ArtifactHeader, error) {
	switch typed := value.(type) {
	case VerificationLineageSource:
		return typed.Header, nil
	case ExecutionWitness:
		return typed.Header, nil
	case LineageCandidate:
		return typed.Header, nil
	case LineageAssessment:
		return typed.Header, nil
	case TraceCapabilityVector:
		return typed.Header, nil
	case LineageAudit:
		return typed.Header, nil
	case VerificationEvidenceBOM:
		return typed.Header, nil
	case VerificationLineageDatasetCard:
		return typed.Header, nil
	case VerificationLineageRelease:
		return typed.Header, nil
	default:
		return ArtifactHeader{}, errors.New("unsupported lineage artifact")
	}
}

func EncodeIndented(value any) ([]byte, error) {
	return study.EncodeIndented(value)
}
