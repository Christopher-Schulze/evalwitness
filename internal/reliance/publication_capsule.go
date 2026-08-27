package reliance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	RelianceCapsuleRegistryID     = "evalwitness.reliance-registry.v1"
	EvidenceRelianceComponentName = "reliance.evidence-map"
	relianceMapValidatorID        = "evalwitness.validator.evidence-reliance-map.v1"
	maximumRelianceMapBytes       = 64 << 20
)

func BuildEvidenceRelianceCapsule(
	base capsule.ReferencePackage,
	value EvidenceRelianceMap,
) (capsule.ReferencePackage, error) {
	if base.Registry == nil {
		return capsule.ReferencePackage{}, errors.New("evidence reliance capsule requires a base reference package")
	}
	if _, err := capsule.VerifyPackage(context.Background(), base.Registry, base.Manifest, base.Payloads,
		capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic}); err != nil {
		return capsule.ReferencePackage{}, fmt.Errorf("verify evidence reliance base capsule: %w", err)
	}
	registry, err := relianceCapsuleRegistry(base.Registry)
	if err != nil {
		return capsule.ReferencePackage{}, err
	}
	parents, err := relianceCapsuleParents(base)
	if err != nil {
		return capsule.ReferencePackage{}, err
	}
	record, normalized, err := buildEvidenceRelianceComponent(registry, parents, value)
	if err != nil {
		return capsule.ReferencePackage{}, err
	}
	manifest, err := capsule.BuildManifest(registry, capsule.ManifestInput{
		StudyID: "task-065-evidence-reliance", CellID: "local-mechanism-map-v1",
		ParentCapsules:  []capsule.CapsuleRef{{Relation: "extends", CapsuleID: base.Manifest.CapsuleID}},
		ScientificRoots: []string{record.ComponentID}, Components: []capsule.ComponentRecord{record},
	})
	if err != nil {
		return capsule.ReferencePackage{}, err
	}
	return capsule.ReferencePackage{
		Registry: registry, Manifest: manifest, Payloads: map[string][]byte{record.Payload.Digest: normalized},
	}, nil
}

func buildEvidenceRelianceComponent(
	registry *capsule.Registry,
	parents []capsule.ParentRef,
	value EvidenceRelianceMap,
) (capsule.ComponentRecord, []byte, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return capsule.ComponentRecord{}, nil, err
	}
	return capsule.BuildComponent(registry, capsule.ComponentInput{
		Name: EvidenceRelianceComponentName, TypeID: EvidenceRelianceMapSchemaVersion,
		Visibility: capsule.VisibilityPublic, Payload: payload, Parents: parents,
	})
}

func LoadEvidenceRelianceCapsule(
	ctx context.Context,
	root string,
	base capsule.ReferencePackage,
) (capsule.ReferencePackage, error) {
	if ctx == nil || root == "" || base.Registry == nil {
		return capsule.ReferencePackage{}, errors.New("load evidence reliance capsule requires context, root, and base registry")
	}
	registry, err := relianceCapsuleRegistry(base.Registry)
	if err != nil {
		return capsule.ReferencePackage{}, err
	}
	manifest, payloads, err := capsule.LoadDirectory(ctx, root, registry,
		capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic})
	if err != nil {
		return capsule.ReferencePackage{}, err
	}
	child := capsule.ReferencePackage{Registry: registry, Manifest: manifest, Payloads: payloads}
	if _, err := verifyEvidenceReliancePackageFamily(ctx, base, child); err != nil {
		return capsule.ReferencePackage{}, err
	}
	return child, nil
}

func verifyEvidenceReliancePackageFamily(
	ctx context.Context,
	base capsule.ReferencePackage,
	child capsule.ReferencePackage,
) (EvidenceRelianceMap, error) {
	if ctx == nil {
		return EvidenceRelianceMap{}, errors.New("evidence reliance capsule verification requires context")
	}
	if _, err := capsule.VerifyPackageFamily(ctx, referencePackageAsCapsule(child),
		[]capsule.Package{referencePackageAsCapsule(base)},
		capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic}); err != nil {
		return EvidenceRelianceMap{}, err
	}
	record, err := uniqueRelianceCapsuleComponent(child.Manifest)
	if err != nil {
		return EvidenceRelianceMap{}, err
	}
	return DecodeEvidenceRelianceMap(child.Payloads[record.Payload.Digest])
}

func uniqueRelianceCapsuleComponent(manifest capsule.Manifest) (capsule.ComponentRecord, error) {
	var result capsule.ComponentRecord
	found := 0
	for _, record := range manifest.Components {
		if record.Name == EvidenceRelianceComponentName && record.TypeID == EvidenceRelianceMapSchemaVersion {
			result, found = record, found+1
		}
	}
	if found != 1 {
		return capsule.ComponentRecord{}, errors.New("evidence reliance capsule lacks one canonical map component")
	}
	return result, nil
}

func referencePackageAsCapsule(value capsule.ReferencePackage) capsule.Package {
	return capsule.Package(value)
}

func relianceCapsuleRegistry(base *capsule.Registry) (*capsule.Registry, error) {
	if base == nil {
		return nil, errors.New("evidence reliance registry requires a base registry")
	}
	typeDefinition := capsule.ComponentType{
		TypeID: EvidenceRelianceMapSchemaVersion, SchemaID: EvidenceRelianceMapSchemaVersion,
		Role: capsule.RoleDerivation, AllowedVisibilities: []capsule.Visibility{capsule.VisibilityPublic},
		MediaType: "application/json", PayloadProfile: capsule.PayloadExactBytes,
		ValidatorID: relianceMapValidatorID, ParentRules: relianceCapsuleParentRules(),
	}
	document, err := capsule.SealRegistry(RelianceCapsuleRegistryID, base.Digest(), []capsule.ComponentType{typeDefinition})
	if err != nil {
		return nil, err
	}
	return capsule.ExtendRegistry(base, document, map[string]capsule.PayloadValidator{
		relianceMapValidatorID: validateRelianceMapPayload,
	})
}

func relianceCapsuleParentRules() []capsule.ParentRule {
	return []capsule.ParentRule{
		{
			Kind: capsule.EdgeDerivedFrom, ParentType: capsule.LegacyClaimFactsSchemaVersion,
			Minimum: 1, Maximum: 1, Resolutions: []capsule.ParentResolution{capsule.ParentExternal},
		},
		{
			Kind: capsule.EdgeDerivedFrom, ParentType: protocolkit.RunSchema,
			Minimum: 1, Maximum: 1, Resolutions: []capsule.ParentResolution{capsule.ParentExternal},
		},
	}
}

func relianceCapsuleParents(base capsule.ReferencePackage) ([]capsule.ParentRef, error) {
	types := []string{capsule.LegacyClaimFactsSchemaVersion, protocolkit.RunSchema}
	result := make([]capsule.ParentRef, len(types))
	for index, typeID := range types {
		record, err := uniqueRelianceCapsuleParent(base.Manifest, typeID)
		if err != nil {
			return nil, err
		}
		result[index] = capsule.ParentRef{
			Kind: capsule.EdgeDerivedFrom, ComponentID: record.ComponentID, TypeID: record.TypeID,
			Role: record.Role, Visibility: record.Visibility, Resolution: capsule.ParentExternal,
			CapsuleID: base.Manifest.CapsuleID,
		}
	}
	return result, nil
}

func uniqueRelianceCapsuleParent(manifest capsule.Manifest, typeID string) (capsule.ComponentRecord, error) {
	var result capsule.ComponentRecord
	found := 0
	for _, record := range manifest.Components {
		if record.TypeID == typeID {
			result, found = record, found+1
		}
	}
	if found != 1 {
		return capsule.ComponentRecord{}, fmt.Errorf("evidence reliance base capsule has %d components of type %q", found, typeID)
	}
	return result, nil
}

func validateRelianceMapPayload(payload []byte) error {
	_, err := DecodeEvidenceRelianceMap(payload)
	return err
}

func DecodeEvidenceRelianceMap(payload []byte) (EvidenceRelianceMap, error) {
	if len(payload) == 0 || len(payload) > maximumRelianceMapBytes {
		return EvidenceRelianceMap{}, errors.New("evidence reliance map payload is empty or exceeds its size limit")
	}
	var value EvidenceRelianceMap
	if err := decodeExactRelianceMap(payload, &value); err != nil {
		return EvidenceRelianceMap{}, err
	}
	if err := value.Validate(); err != nil {
		return EvidenceRelianceMap{}, err
	}
	expected, err := json.Marshal(value)
	if err != nil || !bytes.Equal(payload, expected) {
		return EvidenceRelianceMap{}, errors.New("evidence reliance map payload is not exact canonical reliance JSON")
	}
	return value, nil
}

func decodeExactRelianceMap(payload []byte, value *EvidenceRelianceMap) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("decode evidence reliance map: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("evidence reliance map contains trailing JSON")
	}
	return nil
}
