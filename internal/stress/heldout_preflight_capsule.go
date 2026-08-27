package stress

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
)

const (
	HeldOutPreflightCapsuleRegistryID = "evalwitness.stress-held-out-preflight-capsule-registry.v1"
	heldOutPreflightCapsuleValidator  = "evalwitness.validator.stress-held-out-preflight-capsule.v1"
	heldOutPreflightCapsuleBinding    = "evalwitness.validator.stress-held-out-preflight-capsule-bindings.v1"
)

func BuildHeldOutPreflightCapsule(
	ctx context.Context,
	privateRelation capsule.Package,
	admission HeldOutAdmissionPlan,
	execution HeldOutExecutionBatchBinding,
	evidence HeldOutPreflightEvidence,
) (capsule.Package, HeldOutPreflightCustody, error) {
	custody, err := BuildHeldOutPreflightCustody(ctx, privateRelation, admission, execution, evidence)
	if err != nil {
		return capsule.Package{}, HeldOutPreflightCustody{}, err
	}
	proofRecord, err := heldOutPrivateRelationProofComponent(privateRelation.Manifest)
	if err != nil {
		return capsule.Package{}, HeldOutPreflightCustody{}, err
	}
	registry, err := heldOutPreflightCapsuleRegistry(privateRelation.Registry)
	if err != nil {
		return capsule.Package{}, HeldOutPreflightCustody{}, err
	}
	records := make([]capsule.ComponentRecord, 0, 4)
	payloads := make(map[string][]byte, 4)
	privateProof := heldOutExternalCapsuleParent(capsule.EdgeDerivedFrom, proofRecord, privateRelation.Manifest.CapsuleID)
	admissionRecord, err := addHeldOutPreflightCapsuleComponent(
		registry, "stress.held-out-admission", HeldOutAdmissionPlanSchemaVersion, admission,
		[]capsule.ParentRef{privateProof}, &records, payloads,
	)
	if err != nil {
		return capsule.Package{}, HeldOutPreflightCustody{}, err
	}
	executionRecord, err := addHeldOutPreflightCapsuleComponent(
		registry, "stress.held-out-execution-binding", HeldOutExecutionBatchBindingSchemaVersion, execution,
		[]capsule.ParentRef{heldOutInternalCapsuleParent(capsule.EdgeDerivedFrom, admissionRecord)}, &records, payloads,
	)
	if err != nil {
		return capsule.Package{}, HeldOutPreflightCustody{}, err
	}
	evidenceRecord, err := addHeldOutPreflightCapsuleComponent(
		registry, "stress.held-out-preflight-evidence", HeldOutPreflightEvidenceSchemaVersion, evidence,
		[]capsule.ParentRef{heldOutInternalCapsuleParent(capsule.EdgeDerivedFrom, executionRecord)}, &records, payloads,
	)
	if err != nil {
		return capsule.Package{}, HeldOutPreflightCustody{}, err
	}
	custodyRecord, err := addHeldOutPreflightCapsuleComponent(
		registry, "stress.held-out-preflight-custody", HeldOutPreflightCustodySchemaVersion, custody,
		[]capsule.ParentRef{
			heldOutInternalCapsuleParent(capsule.EdgeDerivedFrom, admissionRecord),
			heldOutInternalCapsuleParent(capsule.EdgeDerivedFrom, executionRecord),
			heldOutInternalCapsuleParent(capsule.EdgeDerivedFrom, evidenceRecord),
			privateProof,
		}, &records, payloads,
	)
	if err != nil {
		return capsule.Package{}, HeldOutPreflightCustody{}, err
	}
	manifest, err := capsule.BuildManifest(registry, capsule.ManifestInput{
		StudyID: evidence.StudyRecord.Study.StudyID, CellID: "held-out-preflight",
		ParentCapsules:  []capsule.CapsuleRef{{Relation: "extends", CapsuleID: privateRelation.Manifest.CapsuleID}},
		ScientificRoots: []string{custodyRecord.ComponentID}, Components: records,
	})
	if err != nil {
		return capsule.Package{}, HeldOutPreflightCustody{}, err
	}
	value := capsule.Package{Registry: registry, Manifest: manifest, Payloads: payloads}
	if _, err := VerifyHeldOutPreflightCapsule(ctx, value, privateRelation, admission, execution, evidence); err != nil {
		return capsule.Package{}, HeldOutPreflightCustody{}, err
	}
	return value, custody, nil
}

func VerifyHeldOutPreflightCapsule(
	ctx context.Context,
	value capsule.Package,
	privateRelation capsule.Package,
	admission HeldOutAdmissionPlan,
	execution HeldOutExecutionBatchBinding,
	evidence HeldOutPreflightEvidence,
) (HeldOutPreflightCustody, error) {
	if ctx == nil || value.Registry == nil || privateRelation.Registry == nil {
		return HeldOutPreflightCustody{}, errors.New("stress held-out preflight capsule verification requires context and both registries")
	}
	expectedRegistry, err := heldOutPreflightCapsuleRegistry(privateRelation.Registry)
	if err != nil {
		return HeldOutPreflightCustody{}, err
	}
	if value.Registry.Digest() != expectedRegistry.Digest() || value.Registry.Document().RegistryID != HeldOutPreflightCapsuleRegistryID {
		return HeldOutPreflightCustody{}, errors.New("stress held-out preflight capsule uses a foreign registry")
	}
	if _, err := capsule.VerifyPackageFamily(
		ctx, value, []capsule.Package{privateRelation}, capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPrivate},
	); err != nil {
		return HeldOutPreflightCustody{}, fmt.Errorf("verify stress held-out preflight capsule family: %w", err)
	}
	if len(value.Manifest.Components) != 4 || len(value.Manifest.ScientificRoots) != 1 || len(value.Manifest.ParentCapsules) != 1 ||
		value.Manifest.ParentCapsules[0].CapsuleID != privateRelation.Manifest.CapsuleID {
		return HeldOutPreflightCustody{}, errors.New("stress held-out preflight capsule inventory or private parent set is invalid")
	}
	decodedAdmission, err := decodeHeldOutPreflightCapsuleComponent[HeldOutAdmissionPlan](value, HeldOutAdmissionPlanSchemaVersion)
	if err != nil {
		return HeldOutPreflightCustody{}, err
	}
	decodedExecution, err := decodeHeldOutPreflightCapsuleComponent[HeldOutExecutionBatchBinding](value, HeldOutExecutionBatchBindingSchemaVersion)
	if err != nil {
		return HeldOutPreflightCustody{}, err
	}
	decodedEvidence, err := decodeHeldOutPreflightCapsuleComponent[HeldOutPreflightEvidence](value, HeldOutPreflightEvidenceSchemaVersion)
	if err != nil {
		return HeldOutPreflightCustody{}, err
	}
	decodedCustody, err := decodeHeldOutPreflightCapsuleComponent[HeldOutPreflightCustody](value, HeldOutPreflightCustodySchemaVersion)
	if err != nil {
		return HeldOutPreflightCustody{}, err
	}
	if !reflect.DeepEqual(decodedAdmission, admission) || !reflect.DeepEqual(decodedExecution, execution) || !reflect.DeepEqual(decodedEvidence, evidence) {
		return HeldOutPreflightCustody{}, errors.New("stress held-out preflight capsule substituted an admission, execution, or preflight evidence parent")
	}
	verifiedAt, err := parseHeldOutPreflightTime(evidence.VerifiedAt)
	if err != nil {
		return HeldOutPreflightCustody{}, err
	}
	if err := decodedCustody.ValidateAgainst(admission, execution, evidence, privateRelation, verifiedAt); err != nil {
		return HeldOutPreflightCustody{}, err
	}
	root, exists := heldOutCapsuleComponentByType(value.Manifest.Components, HeldOutPreflightCustodySchemaVersion)
	if !exists || !slices.Contains(value.Manifest.ScientificRoots, root.ComponentID) {
		return HeldOutPreflightCustody{}, errors.New("stress held-out preflight custody is not the capsule scientific root")
	}
	if _, err := verifyHeldOutPrivateRelationParent(ctx, privateRelation, admission); err != nil {
		return HeldOutPreflightCustody{}, err
	}
	return decodedCustody, nil
}

func heldOutPreflightCapsuleRegistry(base *capsule.Registry) (*capsule.Registry, error) {
	if base == nil {
		return nil, errors.New("stress held-out preflight capsule requires the private relation base registry")
	}
	private := []capsule.Visibility{capsule.VisibilityPrivate}
	types := []capsule.ComponentType{
		{
			TypeID: HeldOutAdmissionPlanSchemaVersion, SchemaID: HeldOutAdmissionPlanSchemaVersion,
			Role: capsule.RoleDerivation, AllowedVisibilities: private, MediaType: "application/json",
			PayloadProfile: capsule.PayloadExactBytes, ValidatorID: heldOutPreflightCapsuleValidator,
			BindingValidatorID: heldOutPreflightCapsuleBinding,
			ParentRules: []capsule.ParentRule{{
				Kind: capsule.EdgeDerivedFrom, ParentType: capsule.PrivateRelationProofSchemaVersion, Minimum: 1, Maximum: 1,
				Resolutions: []capsule.ParentResolution{capsule.ParentExternal},
			}},
		},
		{
			TypeID: HeldOutExecutionBatchBindingSchemaVersion, SchemaID: HeldOutExecutionBatchBindingSchemaVersion,
			Role: capsule.RoleDerivation, AllowedVisibilities: private, MediaType: "application/json",
			PayloadProfile: capsule.PayloadExactBytes, ValidatorID: heldOutPreflightCapsuleValidator,
			BindingValidatorID: heldOutPreflightCapsuleBinding,
			ParentRules: []capsule.ParentRule{{
				Kind: capsule.EdgeDerivedFrom, ParentType: HeldOutAdmissionPlanSchemaVersion, Minimum: 1, Maximum: 1,
				Resolutions: []capsule.ParentResolution{capsule.ParentInternal},
			}},
		},
		{
			TypeID: HeldOutPreflightEvidenceSchemaVersion, SchemaID: HeldOutPreflightEvidenceSchemaVersion,
			Role: capsule.RoleDerivation, AllowedVisibilities: private, MediaType: "application/json",
			PayloadProfile: capsule.PayloadExactBytes, ValidatorID: heldOutPreflightCapsuleValidator,
			BindingValidatorID: heldOutPreflightCapsuleBinding,
			ParentRules: []capsule.ParentRule{{
				Kind: capsule.EdgeDerivedFrom, ParentType: HeldOutExecutionBatchBindingSchemaVersion, Minimum: 1, Maximum: 1,
				Resolutions: []capsule.ParentResolution{capsule.ParentInternal},
			}},
		},
		{
			TypeID: HeldOutPreflightCustodySchemaVersion, SchemaID: HeldOutPreflightCustodySchemaVersion,
			Role: capsule.RoleDerivation, AllowedVisibilities: private, MediaType: "application/json",
			PayloadProfile: capsule.PayloadExactBytes, ValidatorID: heldOutPreflightCapsuleValidator,
			BindingValidatorID: heldOutPreflightCapsuleBinding,
			ParentRules: []capsule.ParentRule{
				{Kind: capsule.EdgeDerivedFrom, ParentType: HeldOutAdmissionPlanSchemaVersion, Minimum: 1, Maximum: 1, Resolutions: []capsule.ParentResolution{capsule.ParentInternal}},
				{Kind: capsule.EdgeDerivedFrom, ParentType: HeldOutExecutionBatchBindingSchemaVersion, Minimum: 1, Maximum: 1, Resolutions: []capsule.ParentResolution{capsule.ParentInternal}},
				{Kind: capsule.EdgeDerivedFrom, ParentType: HeldOutPreflightEvidenceSchemaVersion, Minimum: 1, Maximum: 1, Resolutions: []capsule.ParentResolution{capsule.ParentInternal}},
				{Kind: capsule.EdgeDerivedFrom, ParentType: capsule.PrivateRelationProofSchemaVersion, Minimum: 1, Maximum: 1, Resolutions: []capsule.ParentResolution{capsule.ParentExternal}},
			},
		},
	}
	document, err := capsule.SealRegistry(HeldOutPreflightCapsuleRegistryID, base.Digest(), types)
	if err != nil {
		return nil, err
	}
	return capsule.ExtendRegistryWithBindings(
		base, document,
		map[string]capsule.PayloadValidator{heldOutPreflightCapsuleValidator: validateHeldOutPreflightCapsulePayload},
		map[string]capsule.BindingValidator{heldOutPreflightCapsuleBinding: validateHeldOutPreflightCapsuleBindings},
	)
}

func validateHeldOutPreflightCapsulePayload(raw []byte) error {
	var identity struct {
		SchemaVersion string `json:"schema_version"`
	}
	if err := json.Unmarshal(raw, &identity); err != nil {
		return err
	}
	switch identity.SchemaVersion {
	case HeldOutAdmissionPlanSchemaVersion:
		var value HeldOutAdmissionPlan
		if err := decodeHeldOutCapsuleJSON(raw, &value); err != nil {
			return err
		}
		return value.Validate()
	case HeldOutExecutionBatchBindingSchemaVersion:
		var value HeldOutExecutionBatchBinding
		if err := decodeHeldOutCapsuleJSON(raw, &value); err != nil {
			return err
		}
		return value.Validate()
	case HeldOutPreflightEvidenceSchemaVersion:
		var value HeldOutPreflightEvidence
		if err := decodeHeldOutCapsuleJSON(raw, &value); err != nil {
			return err
		}
		return value.Validate()
	case HeldOutPreflightCustodySchemaVersion:
		var value HeldOutPreflightCustody
		if err := decodeHeldOutCapsuleJSON(raw, &value); err != nil {
			return err
		}
		return value.Validate()
	default:
		return fmt.Errorf("unsupported stress held-out preflight capsule payload %q", identity.SchemaVersion)
	}
}

func validateHeldOutPreflightCapsuleBindings(value capsule.BindingContext) error {
	switch value.Component.TypeID {
	case HeldOutAdmissionPlanSchemaVersion:
		return nil
	case HeldOutExecutionBatchBindingSchemaVersion:
		var execution HeldOutExecutionBatchBinding
		var admission HeldOutAdmissionPlan
		if err := decodeHeldOutCapsuleJSON(value.Payload, &execution); err != nil {
			return err
		}
		if err := decodeHeldOutBoundParent(value.Parents, HeldOutAdmissionPlanSchemaVersion, &admission); err != nil {
			return err
		}
		if execution.AdmissionPlanDigest != admission.Digest || execution.CampaignDigest != admission.CampaignDigest {
			return errors.New("stress held-out execution capsule component differs from its admission parent")
		}
	case HeldOutPreflightEvidenceSchemaVersion:
		var evidence HeldOutPreflightEvidence
		var execution HeldOutExecutionBatchBinding
		if err := decodeHeldOutCapsuleJSON(value.Payload, &evidence); err != nil {
			return err
		}
		if err := decodeHeldOutBoundParent(value.Parents, HeldOutExecutionBatchBindingSchemaVersion, &execution); err != nil {
			return err
		}
		if evidence.StudyRecord.Study.ManifestDigest != execution.StudyManifestDigest {
			return errors.New("stress held-out preflight evidence capsule component differs from its execution parent")
		}
	case HeldOutPreflightCustodySchemaVersion:
		var custody HeldOutPreflightCustody
		var admission HeldOutAdmissionPlan
		var execution HeldOutExecutionBatchBinding
		var evidence HeldOutPreflightEvidence
		if err := decodeHeldOutCapsuleJSON(value.Payload, &custody); err != nil {
			return err
		}
		if err := decodeHeldOutBoundParent(value.Parents, HeldOutAdmissionPlanSchemaVersion, &admission); err != nil {
			return err
		}
		if err := decodeHeldOutBoundParent(value.Parents, HeldOutExecutionBatchBindingSchemaVersion, &execution); err != nil {
			return err
		}
		if err := decodeHeldOutBoundParent(value.Parents, HeldOutPreflightEvidenceSchemaVersion, &evidence); err != nil {
			return err
		}
		if custody.AdmissionPlanDigest != admission.Digest || custody.ExecutionBatchBindingDigest != execution.Digest ||
			custody.PreflightEvidenceDigest != evidence.Digest || custody.StudyManifestDigest != evidence.StudyRecord.Study.ManifestDigest {
			return errors.New("stress held-out preflight custody capsule component differs from its internal parents")
		}
	default:
		return fmt.Errorf("unsupported stress held-out preflight capsule binding %q", value.Component.TypeID)
	}
	return nil
}

func addHeldOutPreflightCapsuleComponent(
	registry *capsule.Registry,
	name string,
	typeID string,
	value any,
	parents []capsule.ParentRef,
	records *[]capsule.ComponentRecord,
	payloads map[string][]byte,
) (capsule.ComponentRecord, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return capsule.ComponentRecord{}, err
	}
	record, normalized, err := capsule.BuildComponent(registry, capsule.ComponentInput{
		Name: name, TypeID: typeID, Visibility: capsule.VisibilityPrivate, Payload: raw, Parents: parents,
	})
	if err != nil {
		return capsule.ComponentRecord{}, err
	}
	*records = append(*records, record)
	payloads[record.Payload.Digest] = normalized
	return record, nil
}

func decodeHeldOutPreflightCapsuleComponent[T any](value capsule.Package, typeID string) (T, error) {
	var zero T
	record, exists := heldOutCapsuleComponentByType(value.Manifest.Components, typeID)
	if !exists {
		return zero, fmt.Errorf("stress held-out preflight capsule lacks component type %q", typeID)
	}
	raw, exists := value.Payloads[record.Payload.Digest]
	if !exists {
		return zero, fmt.Errorf("stress held-out preflight capsule lacks payload for %q", typeID)
	}
	var decoded T
	if err := decodeHeldOutCapsuleJSON(raw, &decoded); err != nil {
		return zero, err
	}
	return decoded, nil
}

func decodeHeldOutBoundParent(parents []capsule.BoundParent, typeID string, target any) error {
	for _, parent := range parents {
		if parent.Record.TypeID != typeID || parent.Reference.Resolution != capsule.ParentInternal {
			continue
		}
		return decodeHeldOutCapsuleJSON(parent.Payload, target)
	}
	return fmt.Errorf("stress held-out preflight capsule lacks internal parent type %q", typeID)
}

func decodeHeldOutCapsuleJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("stress held-out capsule payload has trailing JSON")
	}
	return nil
}

func heldOutCapsuleComponentByType(values []capsule.ComponentRecord, typeID string) (capsule.ComponentRecord, bool) {
	var result capsule.ComponentRecord
	found := false
	for _, value := range values {
		if value.TypeID != typeID {
			continue
		}
		if found {
			return capsule.ComponentRecord{}, false
		}
		result, found = value, true
	}
	return result, found
}

func heldOutInternalCapsuleParent(kind capsule.EdgeKind, value capsule.ComponentRecord) capsule.ParentRef {
	return capsule.ParentRef{
		Kind: kind, ComponentID: value.ComponentID, TypeID: value.TypeID,
		Role: value.Role, Visibility: value.Visibility, Resolution: capsule.ParentInternal,
	}
}

func heldOutExternalCapsuleParent(kind capsule.EdgeKind, value capsule.ComponentRecord, capsuleID string) capsule.ParentRef {
	return capsule.ParentRef{
		Kind: kind, ComponentID: value.ComponentID, TypeID: value.TypeID,
		Role: value.Role, Visibility: value.Visibility, Resolution: capsule.ParentExternal, CapsuleID: capsuleID,
	}
}

func parseHeldOutPreflightTime(value string) (timeValue time.Time, err error) {
	timeValue, err = time.Parse(time.RFC3339, value)
	if err != nil || timeValue.IsZero() || value != timeValue.UTC().Format(time.RFC3339) {
		return time.Time{}, errors.New("stress held-out preflight time is invalid")
	}
	return timeValue, nil
}
