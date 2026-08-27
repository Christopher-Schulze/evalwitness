package capsule

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/Christopher-Schulze/evalwitness/internal/lineage"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const ReferenceRegistryID = "evalwitness.reference-registry.v1"

type ReferencePackage struct {
	Registry *Registry
	Manifest Manifest
	Payloads map[string][]byte
}

func BuildReferencePackage(repositoryRoot string) (ReferencePackage, error) {
	registry, err := ReferenceRegistry()
	if err != nil {
		return ReferencePackage{}, err
	}
	lineageComponents, err := lineage.BuildVerificationLineageReferenceComponents(repositoryRoot)
	if err != nil {
		return ReferencePackage{}, err
	}
	pack, err := buildLineageReferencePackage(registry, lineageComponents)
	if err != nil {
		return ReferencePackage{}, err
	}
	relationRoots, presentationRoot, err := addReferenceRelationEvidence(repositoryRoot, registry, &pack.Manifest.Components, pack.Payloads)
	if err != nil {
		return ReferencePackage{}, err
	}
	traceRoot, err := addReferenceTraceEvidence(repositoryRoot, registry, &pack.Manifest.Components, pack.Payloads)
	if err != nil {
		return ReferencePackage{}, err
	}
	protocolRoot, err := addReferenceProtocolEvidence(repositoryRoot, registry, &pack.Manifest.Components, pack.Payloads)
	if err != nil {
		return ReferencePackage{}, err
	}
	provenanceRoots, renderReceipt, err := addReferenceProvenanceEvidence(context.Background(), repositoryRoot, registry, &pack.Manifest.Components, pack.Payloads)
	if err != nil {
		return ReferencePackage{}, err
	}
	legacyRoot, err := addReferenceLegacyEvidence(repositoryRoot, registry, &pack.Manifest.Components, pack.Payloads)
	if err != nil {
		return ReferencePackage{}, err
	}
	legacyFacts, err := addReferenceLegacyClaimFacts(registry, legacyRoot, &pack.Manifest.Components, pack.Payloads)
	if err != nil {
		return ReferencePackage{}, err
	}
	lineageRoot, err := componentByType(pack.Manifest.Components, lineage.ReleaseSchemaVersion)
	if err != nil {
		return ReferencePackage{}, err
	}
	roots := append([]ComponentRecord{lineageRoot, traceRoot, protocolRoot, legacyFacts}, relationRoots...)
	roots = append(roots, provenanceRoots...)
	index, err := buildReferenceIndex(roots)
	if err != nil {
		return ReferencePackage{}, err
	}
	indexRaw, err := protocol.CanonicalMarshal(index)
	if err != nil {
		return ReferencePackage{}, err
	}
	indexParents := make([]ParentRef, 0, len(roots))
	for _, root := range roots {
		indexParents = append(indexParents, internalParentRef(EdgeDerivedFrom, root))
	}
	indexRecord, err := addReferencePayload(registry, ComponentInput{
		Name: "reference.evidence-index", TypeID: ReferenceIndexSchemaVersion, Visibility: VisibilityPublic,
		Payload: indexRaw, Parents: indexParents,
	}, &pack.Manifest.Components, pack.Payloads)
	if err != nil {
		return ReferencePackage{}, err
	}
	manifest, err := BuildManifest(registry, ManifestInput{
		StudyID: "task-050-reference", CellID: "public-reference-v1", ScientificRoots: []string{indexRecord.ComponentID},
		PresentationRoots: []string{presentationRoot.ComponentID, renderReceipt.ComponentID}, Components: pack.Manifest.Components,
	})
	if err != nil {
		return ReferencePackage{}, err
	}
	pack.Manifest = manifest
	return pack, nil
}

func ReferenceRegistry() (*Registry, error) {
	types := mergeComponentTypes(referenceIndexTypes(), lineageComponentTypes(), referenceTraceTypes(), referenceRelationTypes(), referenceProtocolTypes(), referenceProvenanceTypes(), referenceLegacyTypes(), referenceLegacyFactTypes())
	validators, err := mergePayloadValidators(referenceIndexPayloadValidators(), lineageValidators(), referenceTracePayloadValidators(), referenceRelationPayloadValidators(), referenceProtocolPayloadValidators(), referenceProvenancePayloadValidators(), referenceLegacyPayloadValidators(), referenceLegacyFactPayloadValidators())
	if err != nil {
		return nil, err
	}
	bindings, err := mergeBindingValidators(referenceIndexBindingValidators(), lineageBindingValidators(), referenceTraceBindingValidators(), referenceRelationBindingValidators(), referenceProtocolBindingValidators(), referenceProvenanceBindingValidators(), referenceLegacyBindingValidators(), referenceLegacyFactBindingValidators())
	if err != nil {
		return nil, err
	}
	document, err := SealRegistry(ReferenceRegistryID, "", types)
	if err != nil {
		return nil, err
	}
	return NewRegistryWithBindings(document, validators, bindings)
}

func buildLineageReferencePackage(registry *Registry, source lineage.VerificationLineageReferenceComponents) (ReferencePackage, error) {
	records := make([]ComponentRecord, 0, 12)
	payloads := make(map[string][]byte, 12)
	plan, err := addLineageComponent(registry, "lineage.plan", "plan", source.Plan, nil, &records, payloads)
	if err != nil {
		return ReferencePackage{}, err
	}
	sourceRecord, err := addLineageComponent(registry, "lineage.source", "source", source.Source, []ParentRef{internalParentRef(EdgeGovernedBy, plan)}, &records, payloads)
	if err != nil {
		return ReferencePackage{}, err
	}
	witness, err := addLineageComponent(registry, "lineage.witness", "execution-witness", source.Witness, []ParentRef{internalParentRef(EdgeObservedFrom, sourceRecord)}, &records, payloads)
	if err != nil {
		return ReferencePackage{}, err
	}
	candidate, err := addLineageComponent(registry, "lineage.candidate", "candidate", source.Candidate, []ParentRef{
		internalParentRef(EdgeDerivedFrom, sourceRecord), internalParentRef(EdgeDerivedFrom, witness),
	}, &records, payloads)
	if err != nil {
		return ReferencePackage{}, err
	}
	assessment, err := addLineageComponent(registry, "lineage.assessment", "assessment", source.Assessment, []ParentRef{
		internalParentRef(EdgeDerivedFrom, candidate), internalParentRef(EdgeDerivedFrom, witness),
	}, &records, payloads)
	if err != nil {
		return ReferencePackage{}, err
	}
	capabilities, err := addCapabilities(registry, source.Capabilities, plan, &records, payloads)
	if err != nil {
		return ReferencePackage{}, err
	}
	auditParents := []ParentRef{internalParentRef(EdgeGovernedBy, plan), internalParentRef(EdgeDerivedFrom, assessment)}
	auditCapabilityDigest, err := lineageParentDigest(source.Audit.Header.Parents, "capability")
	if err != nil {
		return ReferencePackage{}, err
	}
	auditCapability, err := capabilityForDigest(capabilities, auditCapabilityDigest, source.Capabilities)
	if err != nil {
		return ReferencePackage{}, err
	}
	auditParents = append(auditParents, internalParentRef(EdgeDerivedFrom, auditCapability))
	audit, err := addLineageComponent(registry, "lineage.audit", "audit", source.Audit, auditParents, &records, payloads)
	if err != nil {
		return ReferencePackage{}, err
	}
	bom, err := addLineageComponent(registry, "lineage.bom", "bom", source.BOM, []ParentRef{
		internalParentRef(EdgeDerivedFrom, sourceRecord), internalParentRef(EdgeDerivedFrom, witness),
		internalParentRef(EdgeDerivedFrom, candidate), internalParentRef(EdgeDerivedFrom, assessment),
		internalParentRef(EdgeDerivedFrom, audit),
	}, &records, payloads)
	if err != nil {
		return ReferencePackage{}, err
	}
	datasetParents := []ParentRef{internalParentRef(EdgeGovernedBy, plan), internalParentRef(EdgeDerivedFrom, audit)}
	for _, capability := range capabilities {
		datasetParents = append(datasetParents, internalParentRef(EdgeDerivedFrom, capability))
	}
	datasetCard, err := addLineageComponent(registry, "lineage.dataset-card", "dataset-card", source.DatasetCard, datasetParents, &records, payloads)
	if err != nil {
		return ReferencePackage{}, err
	}
	release, err := addLineageComponent(registry, "lineage.release", "release", source.Release, []ParentRef{
		internalParentRef(EdgeGovernedBy, plan), internalParentRef(EdgeDerivedFrom, audit),
		internalParentRef(EdgeDerivedFrom, bom), internalParentRef(EdgeDerivedFrom, datasetCard),
	}, &records, payloads)
	if err != nil {
		return ReferencePackage{}, err
	}
	manifest, err := BuildManifest(registry, ManifestInput{
		StudyID: "task-050-reference", CellID: "task-069-development-lineage",
		ScientificRoots: []string{release.ComponentID}, Components: records,
	})
	if err != nil {
		return ReferencePackage{}, err
	}
	return ReferencePackage{Registry: registry, Manifest: manifest, Payloads: payloads}, nil
}

func addCapabilities(registry *Registry, values []lineage.TraceCapabilityVector, plan ComponentRecord, records *[]ComponentRecord, payloads map[string][]byte) ([]ComponentRecord, error) {
	components := make([]ComponentRecord, 0, len(values))
	for _, value := range values {
		component, err := addLineageComponent(registry, "lineage."+value.Header.ObjectID, "capability-vector", value, []ParentRef{
			internalParentRef(EdgeGovernedBy, plan),
		}, records, payloads)
		if err != nil {
			return nil, err
		}
		components = append(components, component)
	}
	return components, nil
}

func lineageParentDigest(parents []lineage.ParentRef, relation string) (string, error) {
	for _, parent := range parents {
		if parent.Relation == relation {
			return parent.Digest, nil
		}
	}
	return "", fmt.Errorf("lineage parent relation %q is absent from the reference package", relation)
}

func capabilityForDigest(components []ComponentRecord, digest string, values []lineage.TraceCapabilityVector) (ComponentRecord, error) {
	for index, value := range values {
		if value.Header.Digest == digest && index < len(components) {
			return components[index], nil
		}
	}
	return ComponentRecord{}, errors.New("lineage audit capability parent is absent from the reference package")
}

func addLineageComponent[T any](registry *Registry, name, documentType string, value T, parents []ParentRef, records *[]ComponentRecord, payloads map[string][]byte) (ComponentRecord, error) {
	payload, err := lineage.EncodeIndented(value)
	if err != nil {
		return ComponentRecord{}, err
	}
	typeID, err := lineageTypeID(documentType)
	if err != nil {
		return ComponentRecord{}, err
	}
	record, normalized, err := BuildComponent(registry, ComponentInput{
		Name: name, TypeID: typeID, Visibility: VisibilityPublic, Payload: payload, Parents: parents,
	})
	if err != nil {
		return ComponentRecord{}, err
	}
	if existing, found := payloads[record.Payload.Digest]; found && !bytes.Equal(existing, normalized) {
		return ComponentRecord{}, errors.New("reference package contains a payload digest collision")
	}
	payloads[record.Payload.Digest] = normalized
	*records = append(*records, record)
	return record, nil
}

func lineageComponentTypes() []ComponentType {
	public := []Visibility{VisibilityPublic}
	exactJSON := func(typeID string, role Role, validator string, rules ...ParentRule) ComponentType {
		return ComponentType{
			TypeID: typeID, SchemaID: typeID, Role: role, AllowedVisibilities: public,
			MediaType: "application/json", PayloadProfile: PayloadExactBytes, ValidatorID: validator,
			BindingValidatorID: lineageBindingValidatorID, ParentRules: rules,
		}
	}
	return []ComponentType{
		exactJSON(lineage.PlanSchemaVersion, RoleGovernance, lineageValidatorID("plan")),
		exactJSON(lineage.SourceSchemaVersion, RoleObservation, lineageValidatorID("source"), parentRule(EdgeGovernedBy, lineage.PlanSchemaVersion, 1, 1)),
		exactJSON(lineage.WitnessSchemaVersion, RoleObservation, lineageValidatorID("execution-witness"), parentRule(EdgeObservedFrom, lineage.SourceSchemaVersion, 1, 1)),
		exactJSON(lineage.CandidateSchemaVersion, RoleDerivation, lineageValidatorID("candidate"), parentRule(EdgeDerivedFrom, lineage.SourceSchemaVersion, 1, 1), parentRule(EdgeDerivedFrom, lineage.WitnessSchemaVersion, 1, 1)),
		exactJSON(lineage.AssessmentSchemaVersion, RoleDerivation, lineageValidatorID("assessment"), parentRule(EdgeDerivedFrom, lineage.CandidateSchemaVersion, 1, 1), parentRule(EdgeDerivedFrom, lineage.WitnessSchemaVersion, 1, 1)),
		exactJSON(lineage.CapabilitySchemaVersion, RoleDerivation, lineageValidatorID("capability-vector"), parentRule(EdgeGovernedBy, lineage.PlanSchemaVersion, 1, 1)),
		exactJSON(lineage.AuditSchemaVersion, RoleDerivation, lineageValidatorID("audit"), parentRule(EdgeDerivedFrom, lineage.AssessmentSchemaVersion, 1, 1), parentRule(EdgeDerivedFrom, lineage.CapabilitySchemaVersion, 1, 1), parentRule(EdgeGovernedBy, lineage.PlanSchemaVersion, 1, 1)),
		exactJSON(lineage.BOMSchemaVersion, RoleDerivation, lineageValidatorID("bom"), parentRule(EdgeDerivedFrom, lineage.SourceSchemaVersion, 1, 1), parentRule(EdgeDerivedFrom, lineage.WitnessSchemaVersion, 1, 1), parentRule(EdgeDerivedFrom, lineage.CandidateSchemaVersion, 1, 1), parentRule(EdgeDerivedFrom, lineage.AssessmentSchemaVersion, 1, 1), parentRule(EdgeDerivedFrom, lineage.AuditSchemaVersion, 1, 1)),
		exactJSON(lineage.DatasetCardSchemaVersion, RoleDerivation, lineageValidatorID("dataset-card"), parentRule(EdgeDerivedFrom, lineage.AuditSchemaVersion, 1, 1), parentRule(EdgeDerivedFrom, lineage.CapabilitySchemaVersion, 3, 3), parentRule(EdgeGovernedBy, lineage.PlanSchemaVersion, 1, 1)),
		exactJSON(lineage.ReleaseSchemaVersion, RoleDerivation, lineageValidatorID("release"), parentRule(EdgeDerivedFrom, lineage.AuditSchemaVersion, 1, 1), parentRule(EdgeDerivedFrom, lineage.BOMSchemaVersion, 1, 1), parentRule(EdgeDerivedFrom, lineage.DatasetCardSchemaVersion, 1, 1), parentRule(EdgeGovernedBy, lineage.PlanSchemaVersion, 1, 1)),
	}
}

func lineageValidators() map[string]PayloadValidator {
	validators := make(map[string]PayloadValidator)
	for _, documentType := range []string{"plan", "source", "execution-witness", "candidate", "assessment", "capability-vector", "audit", "bom", "dataset-card", "release"} {
		documentType := documentType
		validators[lineageValidatorID(documentType)] = func(payload []byte) error {
			summary, err := lineage.DecodeDocument(documentType, bytes.NewReader(payload))
			if err != nil {
				return err
			}
			if !summary.Valid {
				return fmt.Errorf("lineage %s payload is not valid", documentType)
			}
			return nil
		}
	}
	return validators
}

func lineageTypeID(documentType string) (string, error) {
	switch documentType {
	case "plan":
		return lineage.PlanSchemaVersion, nil
	case "source":
		return lineage.SourceSchemaVersion, nil
	case "execution-witness":
		return lineage.WitnessSchemaVersion, nil
	case "candidate":
		return lineage.CandidateSchemaVersion, nil
	case "assessment":
		return lineage.AssessmentSchemaVersion, nil
	case "capability-vector":
		return lineage.CapabilitySchemaVersion, nil
	case "audit":
		return lineage.AuditSchemaVersion, nil
	case "bom":
		return lineage.BOMSchemaVersion, nil
	case "dataset-card":
		return lineage.DatasetCardSchemaVersion, nil
	case "release":
		return lineage.ReleaseSchemaVersion, nil
	default:
		return "", fmt.Errorf("unsupported lineage reference type %q", documentType)
	}
}

func lineageValidatorID(documentType string) string {
	return "evalwitness.validator.lineage." + documentType + ".v1"
}

func parentRule(kind EdgeKind, parentType string, minimum, maximum int) ParentRule {
	return ParentRule{
		Kind: kind, ParentType: parentType, Minimum: minimum, Maximum: maximum,
		Resolutions: []ParentResolution{ParentInternal},
	}
}

func internalParentRef(kind EdgeKind, record ComponentRecord) ParentRef {
	return ParentRef{
		Kind: kind, ComponentID: record.ComponentID, TypeID: record.TypeID, Role: record.Role,
		Visibility: record.Visibility, Resolution: ParentInternal,
	}
}

func componentByType(records []ComponentRecord, typeID string) (ComponentRecord, error) {
	var found *ComponentRecord
	for index := range records {
		if records[index].TypeID != typeID {
			continue
		}
		if found != nil {
			return ComponentRecord{}, fmt.Errorf("reference package contains multiple components of type %q", typeID)
		}
		found = &records[index]
	}
	if found == nil {
		return ComponentRecord{}, fmt.Errorf("reference package has no component of type %q", typeID)
	}
	return *found, nil
}
