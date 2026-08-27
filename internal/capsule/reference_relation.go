package capsule

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/relation"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	referencePrivateRelationInventoryType = "evalwitness.relation-owner-private-package-inventory.v1"
	referencePrivateCommitmentType        = "evalwitness.private-omission-commitment.v1"
	referenceScarcityBriefType            = "evalwitness.relation-scarcity-public-brief.v1"
	referenceRelationBindingValidatorID   = "evalwitness.validator.reference-relation-bindings.v1"
	referenceOmissionClass                = "restricted-owner-inspection-chain"
)

type privateOmissionCommitment struct {
	SchemaVersion      string `json:"schema_version"`
	SubjectType        string `json:"subject_type"`
	SubjectDigest      string `json:"subject_digest"`
	OmissionClass      string `json:"omission_class"`
	VerificationState  string `json:"verification_state"`
	PublicReproduction bool   `json:"public_reproduction"`
	Digest             string `json:"digest"`
}

func referenceRelationTypes() []ComponentType {
	public := []Visibility{VisibilityPublic}
	restricted := []Visibility{VisibilityRestricted, VisibilityPrivate}
	exactJSON := func(typeID string, role Role, binding string, rules ...ParentRule) ComponentType {
		return ComponentType{
			TypeID: typeID, SchemaID: typeID, Role: role, AllowedVisibilities: public,
			MediaType: "application/json", PayloadProfile: PayloadExactBytes,
			ValidatorID: referenceRelationValidatorID(typeID), BindingValidatorID: binding, ParentRules: rules,
		}
	}
	types := []ComponentType{
		exactJSON(mutation.CorpusDevelopmentPlanSchemaVersion, RoleGovernance, ""),
		{TypeID: mutation.CorpusDevelopmentAuditSchemaVersion, SchemaID: mutation.CorpusDevelopmentAuditSchemaVersion, Role: RoleDerivation, AllowedVisibilities: restricted, MediaType: "application/json", PayloadProfile: PayloadExactBytes, ValidatorID: referenceRelationValidatorID(mutation.CorpusDevelopmentAuditSchemaVersion), ParentRules: []ParentRule{parentRule(EdgeGovernedBy, mutation.CorpusDevelopmentPlanSchemaVersion, 1, 1)}},
		{TypeID: mutation.CorpusReleaseSchemaVersion, SchemaID: mutation.CorpusReleaseSchemaVersion, Role: RoleDerivation, AllowedVisibilities: restricted, MediaType: "application/json", PayloadProfile: PayloadExactBytes, ValidatorID: referenceRelationValidatorID(mutation.CorpusReleaseSchemaVersion), ParentRules: []ParentRule{parentRule(EdgeDerivedFrom, mutation.CorpusDevelopmentAuditSchemaVersion, 1, 1), parentRule(EdgeGovernedBy, mutation.CorpusDevelopmentPlanSchemaVersion, 1, 1)}},
		exactJSON(mutation.ConstructRepairEvidenceSchemaVersion, RoleDerivation, referenceRelationBindingValidatorID,
			parentRuleWithResolutions(EdgeRedacts, mutation.CorpusDevelopmentAuditSchemaVersion, 1, 1, ParentOmitted),
			parentRuleWithResolutions(EdgeRedacts, mutation.CorpusReleaseSchemaVersion, 1, 1, ParentOmitted),
			parentRule(EdgeGovernedBy, mutation.CorpusDevelopmentPlanSchemaVersion, 1, 1)),
		exactJSON(mutation.ConstructChallengeSchemaVersion, RoleDerivation, ""),
		exactJSON(mutation.CorpusDevelopmentPlanSchemaVersionV3, RoleGovernance, ""),
		exactJSON(mutation.CorpusDevelopmentAuditSchemaVersionV3, RoleDerivation, referenceRelationBindingValidatorID,
			parentRule(EdgeGovernedBy, mutation.CorpusDevelopmentPlanSchemaVersionV3, 1, 1)),
		exactJSON(mutation.CorpusReleaseSchemaVersionV3, RoleDerivation, referenceRelationBindingValidatorID,
			parentRule(EdgeDerivedFrom, mutation.CorpusDevelopmentAuditSchemaVersionV3, 1, 1),
			parentRule(EdgeGovernedBy, mutation.CorpusDevelopmentPlanSchemaVersionV3, 1, 1)),
		exactJSON(relation.PlanSchemaVersionV3, RoleGovernance, referenceRelationBindingValidatorID,
			parentRule(EdgeDerivedFrom, mutation.CorpusDevelopmentPlanSchemaVersionV3, 1, 1),
			parentRule(EdgeDerivedFrom, mutation.CorpusDevelopmentAuditSchemaVersionV3, 1, 1),
			parentRule(EdgeDerivedFrom, mutation.CorpusReleaseSchemaVersionV3, 1, 1)),
		exactJSON(relation.PrimarySampleSchemaVersionV3, RoleDerivation, referenceRelationBindingValidatorID,
			parentRule(EdgeDerivedFrom, mutation.CorpusDevelopmentPlanSchemaVersionV3, 1, 1),
			parentRule(EdgeDerivedFrom, mutation.CorpusDevelopmentAuditSchemaVersionV3, 1, 1),
			parentRule(EdgeDerivedFrom, mutation.CorpusReleaseSchemaVersionV3, 1, 1),
			parentRule(EdgeGovernedBy, relation.PlanSchemaVersionV3, 1, 1)),
		exactJSON(relation.ScarcitySentinelSchemaVersionV3, RoleDerivation, referenceRelationBindingValidatorID,
			parentRule(EdgeDerivedFrom, mutation.CorpusDevelopmentPlanSchemaVersionV3, 1, 1),
			parentRule(EdgeDerivedFrom, mutation.CorpusDevelopmentAuditSchemaVersionV3, 1, 1),
			parentRule(EdgeDerivedFrom, mutation.CorpusReleaseSchemaVersionV3, 1, 1),
			parentRule(EdgeDerivedFrom, relation.PrimarySampleSchemaVersionV3, 1, 1),
			parentRule(EdgeGovernedBy, relation.PlanSchemaVersionV3, 1, 1)),
		exactJSON(relation.ScarcityPublicEvidenceSchemaVersion, RoleDerivation, referenceRelationBindingValidatorID,
			parentRule(EdgeDerivedFrom, mutation.CorpusDevelopmentPlanSchemaVersionV3, 1, 1),
			parentRule(EdgeDerivedFrom, mutation.CorpusDevelopmentAuditSchemaVersionV3, 1, 1),
			parentRule(EdgeDerivedFrom, mutation.CorpusReleaseSchemaVersionV3, 1, 1),
			parentRule(EdgeDerivedFrom, relation.PlanSchemaVersionV3, 1, 1),
			parentRule(EdgeDerivedFrom, relation.PrimarySampleSchemaVersionV3, 1, 1),
			parentRule(EdgeDerivedFrom, relation.ScarcitySentinelSchemaVersionV3, 1, 1)),
		{TypeID: referencePrivateRelationInventoryType, SchemaID: referencePrivateRelationInventoryType, Role: RoleObservation, AllowedVisibilities: []Visibility{VisibilityPrivate}, MediaType: "application/json", PayloadProfile: PayloadExactBytes, ValidatorID: referenceRelationValidatorID(referencePrivateRelationInventoryType)},
		{TypeID: referencePrivateCommitmentType, SchemaID: referencePrivateCommitmentType, Role: RoleCommitment, AllowedVisibilities: public, MediaType: "application/json", PayloadProfile: PayloadCanonicalJSON, ValidatorID: referenceRelationValidatorID(referencePrivateCommitmentType), BindingValidatorID: referenceRelationBindingValidatorID, ParentRules: []ParentRule{parentRuleWithResolutions(EdgeCommitsTo, referencePrivateRelationInventoryType, 1, 1, ParentInternal, ParentOmitted)}},
		exactJSON(relation.OwnerInspectionPublicAttestationSchemaVersion, RoleAttestation, referenceRelationBindingValidatorID,
			parentRule(EdgeAttests, referencePrivateCommitmentType, 1, 1)),
		{TypeID: referenceScarcityBriefType, SchemaID: referenceScarcityBriefType, Role: RolePresentation, AllowedVisibilities: public, MediaType: "text/markdown", PayloadProfile: PayloadExactBytes, ValidatorID: referenceRelationValidatorID(referenceScarcityBriefType), BindingValidatorID: referenceRelationBindingValidatorID, ParentRules: []ParentRule{parentRule(EdgeRenders, relation.ScarcityPublicEvidenceSchemaVersion, 1, 1)}},
	}
	return types
}

func referenceRelationPayloadValidators() map[string]PayloadValidator {
	validators := map[string]PayloadValidator{
		referenceRelationValidatorID(mutation.CorpusDevelopmentPlanSchemaVersion): func(payload []byte) error {
			_, err := mutation.DecodeCorpusDevelopmentPlan(bytes.NewReader(payload))
			return err
		},
		referenceRelationValidatorID(mutation.ConstructRepairEvidenceSchemaVersion): func(payload []byte) error {
			_, err := mutation.DecodeConstructRepairEvidence(bytes.NewReader(payload))
			return err
		},
		referenceRelationValidatorID(mutation.ConstructChallengeSchemaVersion): func(payload []byte) error {
			_, err := mutation.DecodeConstructChallengeEvidence(bytes.NewReader(payload))
			return err
		},
		referenceRelationValidatorID(mutation.CorpusDevelopmentPlanSchemaVersionV3): func(payload []byte) error {
			_, err := mutation.DecodeCorpusDevelopmentPlan(bytes.NewReader(payload))
			return err
		},
		referenceRelationValidatorID(relation.PlanSchemaVersionV3): func(payload []byte) error {
			_, err := relation.DecodePlanV3(bytes.NewReader(payload))
			return err
		},
		referenceRelationValidatorID(relation.ScarcityPublicEvidenceSchemaVersion): func(payload []byte) error {
			_, err := relation.DecodeScarcityPublicEvidence(bytes.NewReader(payload))
			return err
		},
		referenceRelationValidatorID(relation.OwnerInspectionPublicAttestationSchemaVersion): func(payload []byte) error {
			_, err := relation.DecodeOwnerInspectionPublicAttestation(bytes.NewReader(payload))
			return err
		},
		referenceRelationValidatorID(referencePrivateCommitmentType): func(payload []byte) error {
			var commitment privateOmissionCommitment
			if err := decodeReferenceJSON(payload, &commitment); err != nil {
				return err
			}
			return commitment.Validate()
		},
		referenceRelationValidatorID(referencePrivateRelationInventoryType): validateOpaquePrivateJSON,
		referenceRelationValidatorID(referenceScarcityBriefType): func(payload []byte) error {
			if len(payload) == 0 || !bytes.HasSuffix(payload, []byte("\n")) {
				return errors.New("scarcity brief must be non-empty newline-terminated Markdown")
			}
			return nil
		},
	}
	for typeID, schema := range map[string]string{
		mutation.CorpusDevelopmentAuditSchemaVersion:   mutation.CorpusDevelopmentAuditSchemaVersion,
		mutation.CorpusReleaseSchemaVersion:            mutation.CorpusReleaseSchemaVersion,
		mutation.CorpusDevelopmentAuditSchemaVersionV3: mutation.CorpusDevelopmentAuditSchemaVersionV3,
		mutation.CorpusReleaseSchemaVersionV3:          mutation.CorpusReleaseSchemaVersionV3,
		relation.PrimarySampleSchemaVersionV3:          relation.PrimarySampleSchemaVersionV3,
		relation.ScarcitySentinelSchemaVersionV3:       relation.ScarcitySentinelSchemaVersionV3,
	} {
		typeID, schema := typeID, schema
		validators[referenceRelationValidatorID(typeID)] = func(payload []byte) error {
			return validateReferenceJSONIdentity(payload, schema)
		}
	}
	return validators
}

func referenceRelationBindingValidators() map[string]BindingValidator {
	return map[string]BindingValidator{referenceRelationBindingValidatorID: validateReferenceRelationBindings}
}

func addReferenceRelationEvidence(repositoryRoot string, registry *Registry, records *[]ComponentRecord, payloads map[string][]byte) ([]ComponentRecord, *ComponentRecord, error) {
	planV2, err := addReferenceFile(repositoryRoot, registry, "method.plan-v2", mutation.CorpusDevelopmentPlanSchemaVersion, VisibilityPublic, "eval/governance/controlled-corruption-v2-plan.json", nil, records, payloads)
	if err != nil {
		return nil, nil, err
	}
	repairRaw, err := readReferenceFile(repositoryRoot, "eval/governance/construct-repair-evidence-v1.json")
	if err != nil {
		return nil, nil, err
	}
	repairEvidence, err := mutation.DecodeConstructRepairEvidence(bytes.NewReader(repairRaw))
	if err != nil {
		return nil, nil, err
	}
	repair, err := addReferencePayload(registry, ComponentInput{
		Name: "method.construct-repair", TypeID: mutation.ConstructRepairEvidenceSchemaVersion, Visibility: VisibilityPublic,
		Payload: repairRaw, Parents: []ParentRef{
			internalParentRef(EdgeGovernedBy, planV2),
			omittedReference(EdgeRedacts, repairEvidence.Corpus.AuditDigest, mutation.CorpusDevelopmentAuditSchemaVersion, RoleDerivation, VisibilityRestricted, "restricted-v2-corpus-audit"),
			omittedReference(EdgeRedacts, repairEvidence.Corpus.ReleaseDigest, mutation.CorpusReleaseSchemaVersion, RoleDerivation, VisibilityRestricted, "restricted-v2-corpus-release"),
		},
	}, records, payloads)
	if err != nil {
		return nil, nil, err
	}
	challenge, err := addReferenceFile(repositoryRoot, registry, "method.construct-firewall-challenge", mutation.ConstructChallengeSchemaVersion, VisibilityPublic, "eval/governance/construct-firewall-challenge-v1.json", nil, records, payloads)
	if err != nil {
		return nil, nil, err
	}

	planV3, err := addReferenceFile(repositoryRoot, registry, "corruption.plan-v3", mutation.CorpusDevelopmentPlanSchemaVersionV3, VisibilityPublic, "eval/governance/controlled-corruption-v3-plan.json", nil, records, payloads)
	if err != nil {
		return nil, nil, err
	}
	auditV3, err := addReferenceFile(repositoryRoot, registry, "corruption.natural-audit-v3", mutation.CorpusDevelopmentAuditSchemaVersionV3, VisibilityPublic, "eval/governance/controlled-corruption-v3-natural-audit.json", []ParentRef{internalParentRef(EdgeGovernedBy, planV3)}, records, payloads)
	if err != nil {
		return nil, nil, err
	}
	releaseV3, err := addReferenceFile(repositoryRoot, registry, "corruption.release-v3", mutation.CorpusReleaseSchemaVersionV3, VisibilityPublic, "eval/governance/controlled-corruption-v3-release.json", []ParentRef{
		internalParentRef(EdgeGovernedBy, planV3), internalParentRef(EdgeDerivedFrom, auditV3),
	}, records, payloads)
	if err != nil {
		return nil, nil, err
	}
	relationPlan, err := addReferenceFile(repositoryRoot, registry, "relation.plan-v3", relation.PlanSchemaVersionV3, VisibilityPublic, "eval/governance/relation-audit-plan-v3.json", []ParentRef{
		internalParentRef(EdgeDerivedFrom, planV3), internalParentRef(EdgeDerivedFrom, auditV3), internalParentRef(EdgeDerivedFrom, releaseV3),
	}, records, payloads)
	if err != nil {
		return nil, nil, err
	}
	primary, err := addReferenceFile(repositoryRoot, registry, "relation.primary-v3", relation.PrimarySampleSchemaVersionV3, VisibilityPublic, "eval/governance/relation-primary-sample-v3.json", []ParentRef{
		internalParentRef(EdgeGovernedBy, relationPlan), internalParentRef(EdgeDerivedFrom, planV3),
		internalParentRef(EdgeDerivedFrom, auditV3), internalParentRef(EdgeDerivedFrom, releaseV3),
	}, records, payloads)
	if err != nil {
		return nil, nil, err
	}
	sentinel, err := addReferenceFile(repositoryRoot, registry, "relation.scarcity-sentinel-v3", relation.ScarcitySentinelSchemaVersionV3, VisibilityPublic, "eval/governance/relation-scarcity-sentinel-v3.json", []ParentRef{
		internalParentRef(EdgeGovernedBy, relationPlan), internalParentRef(EdgeDerivedFrom, primary),
		internalParentRef(EdgeDerivedFrom, planV3), internalParentRef(EdgeDerivedFrom, auditV3), internalParentRef(EdgeDerivedFrom, releaseV3),
	}, records, payloads)
	if err != nil {
		return nil, nil, err
	}
	scarcity, err := addReferenceFile(repositoryRoot, registry, "relation.scarcity-negative-evidence", relation.ScarcityPublicEvidenceSchemaVersion, VisibilityPublic, "eval/results/relation-scarcity-negative-evidence.json", []ParentRef{
		internalParentRef(EdgeDerivedFrom, planV3), internalParentRef(EdgeDerivedFrom, auditV3), internalParentRef(EdgeDerivedFrom, releaseV3),
		internalParentRef(EdgeDerivedFrom, relationPlan), internalParentRef(EdgeDerivedFrom, primary), internalParentRef(EdgeDerivedFrom, sentinel),
	}, records, payloads)
	if err != nil {
		return nil, nil, err
	}

	attestationRaw, err := readReferenceFile(repositoryRoot, "eval/results/relation-owner-inspection-attestation.json")
	if err != nil {
		return nil, nil, err
	}
	attestationValue, err := relation.DecodeOwnerInspectionPublicAttestation(bytes.NewReader(attestationRaw))
	if err != nil {
		return nil, nil, err
	}
	commitmentValue, err := sealPrivateOmissionCommitment(referencePrivateRelationInventoryType, attestationValue.PackageInventoryDigest, referenceOmissionClass)
	if err != nil {
		return nil, nil, err
	}
	commitmentRaw, err := protocol.CanonicalMarshal(commitmentValue)
	if err != nil {
		return nil, nil, err
	}
	commitment, err := addReferencePayload(registry, ComponentInput{
		Name: "relation.owner-private-chain-commitment", TypeID: referencePrivateCommitmentType, Visibility: VisibilityPublic,
		Payload: commitmentRaw, Parents: []ParentRef{omittedReference(EdgeCommitsTo, attestationValue.PackageInventoryDigest, referencePrivateRelationInventoryType, RoleObservation, VisibilityPrivate, referenceOmissionClass)},
	}, records, payloads)
	if err != nil {
		return nil, nil, err
	}
	attestation, err := addReferencePayload(registry, ComponentInput{
		Name: "relation.owner-inspection-attestation", TypeID: relation.OwnerInspectionPublicAttestationSchemaVersion,
		Visibility: VisibilityPublic, Payload: attestationRaw, Parents: []ParentRef{internalParentRef(EdgeAttests, commitment)},
	}, records, payloads)
	if err != nil {
		return nil, nil, err
	}
	brief, err := addReferenceFile(repositoryRoot, registry, "relation.scarcity-brief", referenceScarcityBriefType, VisibilityPublic, "eval/results/relation-scarcity-negative-evidence.md", []ParentRef{internalParentRef(EdgeRenders, scarcity)}, records, payloads)
	if err != nil {
		return nil, nil, err
	}
	return []ComponentRecord{repair, challenge, scarcity, attestation}, &brief, nil
}

func validateReferenceRelationBindings(context BindingContext) error {
	switch context.Component.TypeID {
	case mutation.ConstructRepairEvidenceSchemaVersion:
		return validateConstructRepairBindings(context)
	case mutation.CorpusDevelopmentAuditSchemaVersionV3:
		plan, err := mutationPlanParent(context, mutation.CorpusDevelopmentPlanSchemaVersionV3)
		if err != nil {
			return err
		}
		var audit mutation.CorpusDevelopmentAuditV3
		if err := decodeReferenceJSON(context.Payload, &audit); err != nil {
			return err
		}
		return audit.Validate(plan)
	case mutation.CorpusReleaseSchemaVersionV3:
		plan, err := mutationPlanParent(context, mutation.CorpusDevelopmentPlanSchemaVersionV3)
		if err != nil {
			return err
		}
		audit, err := mutationAuditV3Parent(context)
		if err != nil {
			return err
		}
		var release mutation.CorpusReleaseV3
		if err := decodeReferenceJSON(context.Payload, &release); err != nil {
			return err
		}
		return release.Validate(plan, audit)
	case relation.PlanSchemaVersionV3:
		plan, err := decodeRelationPlan(context.Payload)
		if err != nil {
			return err
		}
		return validateRelationCorpusParents(context, plan.SourceCorpusPlanDigest, plan.SourceConstructAuditDigest, plan.SourceCorpusDigest, plan.SourceMutationProgramDigest)
	case relation.PrimarySampleSchemaVersionV3:
		plan, err := relationPlanParent(context)
		if err != nil {
			return err
		}
		var sample relation.PrimarySampleV3
		if err := decodeReferenceJSON(context.Payload, &sample); err != nil {
			return err
		}
		if err := sample.Validate(plan); err != nil {
			return err
		}
		return validateRelationCorpusParents(context, sample.SourceCorpusPlanDigest, sample.SourceConstructAuditDigest, sample.SourceCorpusDigest, sample.SourceMutationProgramDigest)
	case relation.ScarcitySentinelSchemaVersionV3:
		plan, err := relationPlanParent(context)
		if err != nil {
			return err
		}
		primary, err := relationPrimaryParent(context)
		if err != nil {
			return err
		}
		var sentinel relation.ScarcitySentinelV3
		if err := decodeReferenceJSON(context.Payload, &sentinel); err != nil {
			return err
		}
		if err := sentinel.Validate(plan, primary); err != nil {
			return err
		}
		return validateRelationCorpusParents(context, sentinel.SourceCorpusPlanDigest, sentinel.SourceConstructAuditDigest, sentinel.SourceCorpusDigest, sentinel.SourceMutationProgramDigest)
	case relation.ScarcityPublicEvidenceSchemaVersion:
		return validateScarcityEvidenceBindings(context)
	case referencePrivateCommitmentType:
		return validatePrivateCommitmentBindings(context)
	case relation.OwnerInspectionPublicAttestationSchemaVersion:
		return validateOwnerAttestationBindings(context)
	case referenceScarcityBriefType:
		return validateScarcityBriefBinding(context)
	default:
		return fmt.Errorf("unsupported reference relation binding type %q", context.Component.TypeID)
	}
}

func validateConstructRepairBindings(context BindingContext) error {
	evidence, err := mutation.DecodeConstructRepairEvidence(bytes.NewReader(context.Payload))
	if err != nil {
		return err
	}
	plan, err := mutationPlanParent(context, mutation.CorpusDevelopmentPlanSchemaVersion)
	if err != nil {
		return err
	}
	if evidence.Corpus.PlanDigest != plan.Digest || evidence.Corpus.MutationProgramDigest != plan.MutationProgramDigest {
		return errors.New("construct-repair evidence differs from its capsule plan")
	}
	audit, err := uniqueReferenceParent(context, mutation.CorpusDevelopmentAuditSchemaVersion)
	if err != nil {
		return err
	}
	release, err := uniqueReferenceParent(context, mutation.CorpusReleaseSchemaVersion)
	if err != nil {
		return err
	}
	if audit.Reference.Resolution != ParentOmitted || release.Reference.Resolution != ParentOmitted ||
		audit.Reference.ComponentID != evidence.Corpus.AuditDigest || release.Reference.ComponentID != evidence.Corpus.ReleaseDigest {
		return errors.New("construct-repair evidence omitted parents differ from its corpus bindings")
	}
	return nil
}

func validateRelationCorpusParents(context BindingContext, planDigest, auditDigest, releaseDigest, programDigest string) error {
	plan, err := mutationPlanParent(context, mutation.CorpusDevelopmentPlanSchemaVersionV3)
	if err != nil {
		return err
	}
	audit, err := mutationAuditV3Parent(context)
	if err != nil {
		return err
	}
	release, err := mutationReleaseV3Parent(context)
	if err != nil {
		return err
	}
	if planDigest != plan.Digest || auditDigest != audit.Digest || releaseDigest != release.Digest || programDigest != plan.MutationProgramDigest {
		return errors.New("relation artifact differs from its corruption-corpus capsule parents")
	}
	return nil
}

func validateScarcityEvidenceBindings(context BindingContext) error {
	plan, err := relationPlanParent(context)
	if err != nil {
		return err
	}
	primary, err := relationPrimaryParent(context)
	if err != nil {
		return err
	}
	sentinelParent, err := uniqueReferenceParent(context, relation.ScarcitySentinelSchemaVersionV3)
	if err != nil {
		return err
	}
	var sentinel relation.ScarcitySentinelV3
	if err := decodeReferenceJSON(sentinelParent.Payload, &sentinel); err != nil {
		return err
	}
	corpusPlan, err := mutationPlanParent(context, mutation.CorpusDevelopmentPlanSchemaVersionV3)
	if err != nil {
		return err
	}
	audit, err := mutationAuditV3Parent(context)
	if err != nil {
		return err
	}
	release, err := mutationReleaseV3Parent(context)
	if err != nil {
		return err
	}
	evidence, err := relation.DecodeScarcityPublicEvidence(bytes.NewReader(context.Payload))
	if err != nil {
		return err
	}
	return relation.VerifyScarcityPublicEvidenceDocument(evidence, plan, primary, sentinel, corpusPlan, audit, release)
}

func validatePrivateCommitmentBindings(context BindingContext) error {
	var commitment privateOmissionCommitment
	if err := decodeReferenceJSON(context.Payload, &commitment); err != nil {
		return err
	}
	if err := commitment.Validate(); err != nil {
		return err
	}
	parent, err := uniqueReferenceParent(context, referencePrivateRelationInventoryType)
	if err != nil {
		return err
	}
	if parent.Reference.Resolution != ParentOmitted || parent.Reference.ComponentID != commitment.SubjectDigest ||
		parent.Reference.OmissionClass != commitment.OmissionClass || commitment.SubjectType != parent.Reference.TypeID {
		return errors.New("private omission commitment differs from its declared capsule parent")
	}
	return nil
}

func validateOwnerAttestationBindings(context BindingContext) error {
	attestation, err := relation.DecodeOwnerInspectionPublicAttestation(bytes.NewReader(context.Payload))
	if err != nil {
		return err
	}
	parent, err := uniqueReferenceParent(context, referencePrivateCommitmentType)
	if err != nil {
		return err
	}
	var commitment privateOmissionCommitment
	if err := decodeReferenceJSON(parent.Payload, &commitment); err != nil {
		return err
	}
	if attestation.PackageInventoryDigest != commitment.SubjectDigest || commitment.OmissionClass != referenceOmissionClass {
		return errors.New("owner-inspection attestation differs from its private-chain commitment")
	}
	return nil
}

func validateScarcityBriefBinding(context BindingContext) error {
	parent, err := uniqueReferenceParent(context, relation.ScarcityPublicEvidenceSchemaVersion)
	if err != nil {
		return err
	}
	evidence, err := relation.DecodeScarcityPublicEvidence(bytes.NewReader(parent.Payload))
	if err != nil {
		return err
	}
	rendered, err := relation.RenderScarcityPublicBriefMarkdown(evidence)
	if err != nil {
		return err
	}
	if !bytes.Equal(context.Payload, []byte(rendered)) {
		return errors.New("scarcity Markdown differs from its deterministic evidence renderer")
	}
	return nil
}

func mutationPlanParent(context BindingContext, typeID string) (mutation.CorpusDevelopmentPlan, error) {
	parent, err := uniqueReferenceParent(context, typeID)
	if err != nil {
		return mutation.CorpusDevelopmentPlan{}, err
	}
	return mutation.DecodeCorpusDevelopmentPlan(bytes.NewReader(parent.Payload))
}

func mutationAuditV3Parent(context BindingContext) (mutation.CorpusDevelopmentAuditV3, error) {
	parent, err := uniqueReferenceParent(context, mutation.CorpusDevelopmentAuditSchemaVersionV3)
	if err != nil {
		return mutation.CorpusDevelopmentAuditV3{}, err
	}
	var audit mutation.CorpusDevelopmentAuditV3
	if err := decodeReferenceJSON(parent.Payload, &audit); err != nil {
		return mutation.CorpusDevelopmentAuditV3{}, err
	}
	return audit, nil
}

func mutationReleaseV3Parent(context BindingContext) (mutation.CorpusReleaseV3, error) {
	parent, err := uniqueReferenceParent(context, mutation.CorpusReleaseSchemaVersionV3)
	if err != nil {
		return mutation.CorpusReleaseV3{}, err
	}
	var release mutation.CorpusReleaseV3
	if err := decodeReferenceJSON(parent.Payload, &release); err != nil {
		return mutation.CorpusReleaseV3{}, err
	}
	return release, nil
}

func relationPlanParent(context BindingContext) (relation.RelationPlanV3, error) {
	parent, err := uniqueReferenceParent(context, relation.PlanSchemaVersionV3)
	if err != nil {
		return relation.RelationPlanV3{}, err
	}
	return decodeRelationPlan(parent.Payload)
}

func decodeRelationPlan(payload []byte) (relation.RelationPlanV3, error) {
	return relation.DecodePlanV3(bytes.NewReader(payload))
}

func relationPrimaryParent(context BindingContext) (relation.PrimarySampleV3, error) {
	parent, err := uniqueReferenceParent(context, relation.PrimarySampleSchemaVersionV3)
	if err != nil {
		return relation.PrimarySampleV3{}, err
	}
	var primary relation.PrimarySampleV3
	if err := decodeReferenceJSON(parent.Payload, &primary); err != nil {
		return relation.PrimarySampleV3{}, err
	}
	return primary, nil
}

func uniqueReferenceParent(context BindingContext, typeID string) (BoundParent, error) {
	var found *BoundParent
	for index := range context.Parents {
		if context.Parents[index].Reference.TypeID != typeID {
			continue
		}
		if found != nil {
			return BoundParent{}, fmt.Errorf("capsule component has duplicate parent type %q", typeID)
		}
		found = &context.Parents[index]
	}
	if found == nil {
		return BoundParent{}, fmt.Errorf("capsule component has no parent type %q", typeID)
	}
	return *found, nil
}

func validateReferenceJSONIdentity(payload []byte, schema string) error {
	var identity struct {
		SchemaVersion string `json:"schema_version"`
		Digest        string `json:"digest"`
	}
	if err := decodeReferenceJSON(payload, &identity); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			var document map[string]json.RawMessage
			if decodeErr := decodeReferenceJSON(payload, &document); decodeErr != nil {
				return decodeErr
			}
			var version string
			var digest string
			versionErr := json.Unmarshal(document["schema_version"], &version)
			digestErr := json.Unmarshal(document["digest"], &digest)
			if versionErr != nil || digestErr != nil || version != schema || !validDigest(digest) {
				return errors.New("reference JSON identity is invalid")
			}
			return nil
		}
		return err
	}
	if identity.SchemaVersion != schema || !validDigest(identity.Digest) {
		return errors.New("reference JSON identity is invalid")
	}
	return nil
}

func validateOpaquePrivateJSON(payload []byte) error {
	var document map[string]json.RawMessage
	if err := decodeReferenceJSON(payload, &document); err != nil {
		return err
	}
	if len(document) == 0 {
		return errors.New("private relation inventory is empty")
	}
	return nil
}

func sealPrivateOmissionCommitment(subjectType, subjectDigest, omissionClass string) (privateOmissionCommitment, error) {
	commitment := privateOmissionCommitment{
		SchemaVersion: referencePrivateCommitmentType, SubjectType: subjectType, SubjectDigest: subjectDigest,
		OmissionClass: omissionClass, VerificationState: "private-chain-verified-before-public-projection", PublicReproduction: false,
	}
	digest, err := privateOmissionCommitmentDigest(commitment)
	if err != nil {
		return privateOmissionCommitment{}, err
	}
	commitment.Digest = digest
	return commitment, commitment.Validate()
}

func (commitment privateOmissionCommitment) Validate() error {
	if commitment.SchemaVersion != referencePrivateCommitmentType || commitment.SubjectType != referencePrivateRelationInventoryType ||
		!validDigest(commitment.SubjectDigest) || commitment.OmissionClass != referenceOmissionClass ||
		commitment.VerificationState != "private-chain-verified-before-public-projection" || commitment.PublicReproduction {
		return errors.New("private omission commitment identity or disclosure boundary is invalid")
	}
	digest, err := privateOmissionCommitmentDigest(commitment)
	if err != nil || digest != commitment.Digest {
		return errors.New("private omission commitment digest is invalid")
	}
	return nil
}

func privateOmissionCommitmentDigest(commitment privateOmissionCommitment) (string, error) {
	commitment.Digest = ""
	return protocol.Digest(commitment)
}

func referenceRelationValidatorID(typeID string) string {
	return "evalwitness.validator.reference." + strings.TrimPrefix(typeID, "evalwitness.")
}

func parentRuleWithResolutions(kind EdgeKind, parentType string, minimum, maximum int, resolutions ...ParentResolution) ParentRule {
	return ParentRule{Kind: kind, ParentType: parentType, Minimum: minimum, Maximum: maximum, Resolutions: resolutions}
}

func omittedReference(kind EdgeKind, componentID, typeID string, role Role, visibility Visibility, omissionClass string) ParentRef {
	return ParentRef{
		Kind: kind, ComponentID: componentID, TypeID: typeID, Role: role, Visibility: visibility,
		Resolution: ParentOmitted, OmissionClass: omissionClass,
	}
}
