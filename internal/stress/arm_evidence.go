package stress

import (
	"errors"
	"fmt"
	"reflect"
	"sort"

	"github.com/Christopher-Schulze/evalwitness/internal/lineage"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

type ProtocolAdapterProof struct {
	SchemaVersion                   string `json:"schema_version"`
	CanonicalPolicy                 string `json:"canonical_policy"`
	ProtocolVersion                 string `json:"protocol_version"`
	NormativeCorpusDigest           string `json:"normative_corpus_digest"`
	ReferenceRunDigest              string `json:"reference_run_digest"`
	ExternalProcessRunDigest        string `json:"external_process_run_digest"`
	ResultParity                    bool   `json:"result_parity"`
	ApplicationEntrypoint           string `json:"application_entrypoint"`
	ApplicationInvocationSchema     string `json:"application_invocation_schema"`
	ApplicationResultSchema         string `json:"application_result_schema"`
	LineageAdapterConformanceDigest string `json:"lineage_adapter_conformance_digest"`
	ProviderCalls                   int    `json:"provider_calls"`
	EmpiricalReliability            bool   `json:"empirical_reliability"`
	Digest                          string `json:"digest"`
}

func BuildProtocolAdapterProof(reference, external protocolkit.AuditRun, mapping lineage.AdapterConformanceReport) (ProtocolAdapterProof, error) {
	if err := validateProtocolRuns(reference, external, mapping); err != nil {
		return ProtocolAdapterProof{}, err
	}
	value := ProtocolAdapterProof{
		SchemaVersion: ProtocolAdapterProofSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		ProtocolVersion: protocolkit.CurrentVersion, NormativeCorpusDigest: reference.CorpusDigest,
		ReferenceRunDigest: reference.RunDigest, ExternalProcessRunDigest: external.RunDigest, ResultParity: true,
		ApplicationEntrypoint: "protocol.application", ApplicationInvocationSchema: protocolkit.ApplicationInvocationSchema,
		ApplicationResultSchema: protocolkit.ApplicationResultSchema, LineageAdapterConformanceDigest: mapping.Digest,
		ProviderCalls: 0, EmpiricalReliability: false,
	}
	var err error
	value.Digest, err = protocolAdapterProofDigest(value)
	if err != nil {
		return ProtocolAdapterProof{}, err
	}
	if err := value.ValidateAgainst(reference, external, mapping); err != nil {
		return ProtocolAdapterProof{}, err
	}
	return value, nil
}

func (value ProtocolAdapterProof) ValidateAgainst(reference, external protocolkit.AuditRun, mapping lineage.AdapterConformanceReport) error {
	if err := validateProtocolRuns(reference, external, mapping); err != nil {
		return err
	}
	want := ProtocolAdapterProof{
		SchemaVersion: ProtocolAdapterProofSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		ProtocolVersion: protocolkit.CurrentVersion, NormativeCorpusDigest: reference.CorpusDigest,
		ReferenceRunDigest: reference.RunDigest, ExternalProcessRunDigest: external.RunDigest, ResultParity: true,
		ApplicationEntrypoint: "protocol.application", ApplicationInvocationSchema: protocolkit.ApplicationInvocationSchema,
		ApplicationResultSchema: protocolkit.ApplicationResultSchema, LineageAdapterConformanceDigest: mapping.Digest,
		ProviderCalls: 0, EmpiricalReliability: false,
	}
	var err error
	want.Digest, err = protocolAdapterProofDigest(want)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, want) {
		return errors.New("stress protocol-adapter proof differs from direct, subprocess, or mapping conformance evidence")
	}
	return nil
}

func (value ProtocolAdapterProof) Validate() error {
	if value.SchemaVersion != ProtocolAdapterProofSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		value.ProtocolVersion != protocolkit.CurrentVersion || !validDigest(value.NormativeCorpusDigest) ||
		!validDigest(value.ReferenceRunDigest) || !validDigest(value.ExternalProcessRunDigest) || !value.ResultParity ||
		value.ApplicationEntrypoint != "protocol.application" || value.ApplicationInvocationSchema != protocolkit.ApplicationInvocationSchema ||
		value.ApplicationResultSchema != protocolkit.ApplicationResultSchema || !validDigest(value.LineageAdapterConformanceDigest) ||
		value.ProviderCalls != 0 || value.EmpiricalReliability {
		return errors.New("stress protocol-adapter proof identity, parity, application boundary, or claim ceiling is invalid")
	}
	expectedDigest, err := protocolAdapterProofDigest(value)
	if err != nil || value.Digest != expectedDigest {
		return errors.New("stress protocol-adapter proof digest is invalid")
	}
	return nil
}

func validateProtocolRuns(reference, external protocolkit.AuditRun, mapping lineage.AdapterConformanceReport) error {
	if err := protocolkit.ValidateAuditRun(reference); err != nil {
		return fmt.Errorf("validate direct protocol run: %w", err)
	}
	if err := protocolkit.ValidateAuditRun(external); err != nil {
		return fmt.Errorf("validate external protocol run: %w", err)
	}
	if err := mapping.Validate(); err != nil {
		return fmt.Errorf("validate lineage adapter conformance: %w", err)
	}
	if reference.ProtocolVersion != protocolkit.CurrentVersion || external.ProtocolVersion != reference.ProtocolVersion ||
		reference.CorpusDigest != external.CorpusDigest || reference.RequestCorpusDigest != external.RequestCorpusDigest ||
		reference.SchemaArtifactDigest != external.SchemaArtifactDigest || !reflect.DeepEqual(reference.Results, external.Results) ||
		!reflect.DeepEqual(reference.Matrix, external.Matrix) || !reflect.DeepEqual(reference.Findings, external.Findings) {
		return errors.New("direct and external protocol runs lack exact corpus, result, capability, or finding parity")
	}
	return nil
}

func protocolAdapterProofDigest(value ProtocolAdapterProof) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}

type ArmReplayEvidence struct {
	SchemaVersion       string          `json:"schema_version"`
	CanonicalPolicy     string          `json:"canonical_policy"`
	PlanDigest          string          `json:"plan_digest"`
	CellID              string          `json:"cell_id"`
	ArmID               string          `json:"arm_id"`
	ProtocolProofDigest string          `json:"protocol_proof_digest,omitempty"`
	Execution           ReplayExecution `json:"execution"`
	Result              Result          `json:"result"`
	Digest              string          `json:"digest"`
}

func SealArmReplayEvidence(plan ArmComparisonPlan, spec Relation, admission ConstructAdmission, item ReplayedRelationCaseV3, execution ReplayExecution, protocolProof *ProtocolAdapterProof) (ArmReplayEvidence, error) {
	cell, arm, err := replayArmCell(plan, spec, item, execution)
	if err != nil {
		return ArmReplayEvidence{}, err
	}
	if admission.CaseID != item.CaseID || execution.EvidencePolicy != arm.EvidencePolicy || execution.Entrypoint != arm.Entrypoint {
		return ArmReplayEvidence{}, errors.New("stress arm replay admission, entrypoint, or evidence policy differs from its registered cell")
	}
	protocolDigest := ""
	if arm.Kind == ArmProtocolAdapter {
		if protocolProof == nil || protocolProof.Validate() != nil || protocolProof.ApplicationEntrypoint != arm.Entrypoint ||
			protocolProof.ProtocolVersion != arm.ProtocolVersion || protocolProof.ApplicationInvocationSchema != arm.ProtocolInvocationSchema ||
			protocolProof.ApplicationResultSchema != arm.ProtocolResultSchema || !protocolProof.ResultParity || protocolProof.ProviderCalls != 0 || protocolProof.EmpiricalReliability {
			return ArmReplayEvidence{}, errors.New("stress protocol arm lacks its exact provider-free subprocess and application-boundary proof")
		}
		protocolDigest = protocolProof.Digest
	} else if protocolProof != nil {
		return ArmReplayEvidence{}, errors.New("non-protocol stress arm carries protocol-only evidence")
	}
	result, err := SealReplayResult(spec, admission, item.TaskGroupID, execution)
	if err != nil {
		return ArmReplayEvidence{}, err
	}
	value := ArmReplayEvidence{
		SchemaVersion: ArmReplayEvidenceSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		PlanDigest: plan.Digest, CellID: cell.CellID, ArmID: arm.ID, ProtocolProofDigest: protocolDigest,
		Execution: execution, Result: result,
	}
	value.Digest, err = armReplayEvidenceDigest(value)
	if err != nil {
		return ArmReplayEvidence{}, err
	}
	if err := value.ValidateAgainst(plan, spec, admission, item, protocolProof); err != nil {
		return ArmReplayEvidence{}, err
	}
	return value, nil
}

func (value ArmReplayEvidence) ValidateAgainst(plan ArmComparisonPlan, spec Relation, admission ConstructAdmission, item ReplayedRelationCaseV3, protocolProof *ProtocolAdapterProof) error {
	cell, arm, err := replayArmCell(plan, spec, item, value.Execution)
	if err != nil {
		return err
	}
	if value.SchemaVersion != ArmReplayEvidenceSchemaVersion || value.CanonicalPolicy != CanonicalPolicy || value.PlanDigest != plan.Digest ||
		value.CellID != cell.CellID || value.ArmID != arm.ID || value.Execution.EvidencePolicy != arm.EvidencePolicy || value.Execution.Entrypoint != arm.Entrypoint {
		return errors.New("stress arm replay evidence identity, entrypoint, or policy is invalid")
	}
	if err := value.Execution.ValidateAgainst(spec); err != nil {
		return err
	}
	wantResult, err := SealReplayResult(spec, admission, item.TaskGroupID, value.Execution)
	if err != nil || !reflect.DeepEqual(value.Result, wantResult) {
		return errors.New("stress arm replay result does not reproduce from its execution")
	}
	if arm.Kind == ArmProtocolAdapter {
		if protocolProof == nil || protocolProof.Validate() != nil || value.ProtocolProofDigest != protocolProof.Digest {
			return errors.New("stress protocol arm replay evidence lacks its exact protocol proof")
		}
	} else if value.ProtocolProofDigest != "" || protocolProof != nil {
		return errors.New("non-protocol stress arm carries protocol-only evidence")
	}
	expectedDigest, err := armReplayEvidenceDigest(value)
	if err != nil || value.Digest != expectedDigest {
		return errors.New("stress arm replay evidence digest is invalid")
	}
	return nil
}

func replayArmCell(plan ArmComparisonPlan, spec Relation, item ReplayedRelationCaseV3, execution ReplayExecution) (ArmComparisonCell, ArmDefinition, error) {
	if execution.RelationDigest != spec.Digest || execution.CaseID != item.CaseID || !slicesContains(item.RelationIDs, spec.ID) {
		return ArmComparisonCell{}, ArmDefinition{}, errors.New("stress arm replay execution differs from its relation corpus cell")
	}
	var matches []ArmComparisonCell
	for _, cell := range plan.Cells {
		if cell.RelationID == spec.ID && cell.RelationDigest == spec.Digest && cell.CaseID == item.CaseID && cell.Support == ArmSupported {
			matches = append(matches, cell)
		}
	}
	sort.Slice(matches, func(left, right int) bool { return matches[left].ArmID < matches[right].ArmID })
	for _, cell := range matches {
		arm, exists := canonicalArmByID(cell.ArmID)
		if exists && arm.Kind != ArmZeroCostControl && arm.Entrypoint == execution.Entrypoint && arm.EvidencePolicy == execution.EvidencePolicy {
			return cell, arm, nil
		}
	}
	return ArmComparisonCell{}, ArmDefinition{}, errors.New("stress replay execution does not match one supported model or protocol arm")
}

func armReplayEvidenceDigest(value ArmReplayEvidence) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}
