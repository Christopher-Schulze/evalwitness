package capsule

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	CalibrationExtensionSchemaVersion      = "evalwitness.extension.calibration-conformance.v1"
	VerifierRelationExtensionSchemaVersion = "evalwitness.extension.verifier-relation-conformance.v1"
	OutcomeExtensionSchemaVersion          = "evalwitness.extension.outcome-conformance.v1"
	ProfileExtensionSchemaVersion          = "evalwitness.extension.profile-conformance.v1"
	RelianceExtensionSchemaVersion         = "evalwitness.extension.reliance-conformance.v1"
	ExtensionConformanceRegistryID         = "evalwitness.reference-extension-conformance.v1"
)

//go:embed testdata/extensions/*.json
var extensionFixtureFiles embed.FS

type ExtensionConformanceFixture struct {
	SchemaVersion         string   `json:"schema_version"`
	ExtensionID           string   `json:"extension_id"`
	OwnerTask             string   `json:"owner_task"`
	Status                string   `json:"status"`
	ProviderCallsRequired int      `json:"provider_calls_required"`
	HumanActionsRequired  int      `json:"human_actions_required"`
	RequiredFields        []string `json:"required_fields"`
	ProhibitedClaims      []string `json:"prohibited_claims"`
}

type extensionFixtureDefinition struct {
	Path        string
	Name        string
	TypeID      string
	ExtensionID string
	OwnerTask   string
	ParentTypes []string
}

func BuildExtensionConformancePackage(base ReferencePackage) (ReferencePackage, error) {
	if base.Registry == nil {
		return ReferencePackage{}, errors.New("extension conformance requires a base reference package")
	}
	if _, err := VerifyPackage(
		context.Background(), base.Registry, base.Manifest, base.Payloads,
		VerificationOptions{MaximumVisibility: VisibilityPublic},
	); err != nil {
		return ReferencePackage{}, fmt.Errorf("verify extension base package: %w", err)
	}
	registry, err := extensionConformanceRegistry(base.Registry)
	if err != nil {
		return ReferencePackage{}, err
	}
	records := make([]ComponentRecord, 0, len(extensionFixtureDefinitions()))
	payloads := make(map[string][]byte, len(extensionFixtureDefinitions()))
	roots := make([]string, 0, len(extensionFixtureDefinitions()))
	for _, definition := range extensionFixtureDefinitions() {
		raw, err := extensionFixtureFiles.ReadFile(definition.Path)
		if err != nil {
			return ReferencePackage{}, err
		}
		parents := make([]ParentRef, 0, len(definition.ParentTypes))
		for _, parentType := range definition.ParentTypes {
			parent, err := componentByType(base.Manifest.Components, parentType)
			if err != nil {
				return ReferencePackage{}, err
			}
			parents = append(parents, externalParentRef(EdgeDerivedFrom, parent, base.Manifest.CapsuleID))
		}
		record, normalized, err := BuildComponent(registry, ComponentInput{
			Name: definition.Name, TypeID: definition.TypeID, Visibility: VisibilityPublic,
			Payload: raw, Parents: parents,
		})
		if err != nil {
			return ReferencePackage{}, err
		}
		records = append(records, record)
		payloads[record.Payload.Digest] = normalized
		roots = append(roots, record.ComponentID)
	}
	manifest, err := BuildManifest(registry, ManifestInput{
		StudyID: "task-050-extension-conformance", CellID: "public-extension-fixtures-v1",
		ParentCapsules:  []CapsuleRef{{Relation: "extends", CapsuleID: base.Manifest.CapsuleID}},
		ScientificRoots: roots, PresentationRoots: []string{}, Components: records,
	})
	if err != nil {
		return ReferencePackage{}, err
	}
	return ReferencePackage{Registry: registry, Manifest: manifest, Payloads: payloads}, nil
}

func extensionConformanceRegistry(base *Registry) (*Registry, error) {
	definitions := extensionFixtureDefinitions()
	types := make([]ComponentType, 0, len(definitions))
	validators := make(map[string]PayloadValidator, len(definitions))
	for _, definition := range definitions {
		rules := make([]ParentRule, 0, len(definition.ParentTypes))
		for _, parentType := range definition.ParentTypes {
			rules = append(rules, ParentRule{
				Kind: EdgeDerivedFrom, ParentType: parentType, Minimum: 1, Maximum: 1,
				Resolutions: []ParentResolution{ParentExternal},
			})
		}
		validatorID := extensionValidatorID(definition.TypeID)
		types = append(types, ComponentType{
			TypeID: definition.TypeID, SchemaID: definition.TypeID, Role: RoleDerivation,
			AllowedVisibilities: []Visibility{VisibilityPublic, VisibilityRestricted},
			MediaType:           "application/json", PayloadProfile: PayloadCanonicalJSON,
			ValidatorID: validatorID, ParentRules: rules,
		})
		definition := definition
		validators[validatorID] = func(payload []byte) error {
			var fixture ExtensionConformanceFixture
			if err := protocolkit.DecodeStrict(payload, &fixture); err != nil {
				return err
			}
			return fixture.validate(definition)
		}
	}
	document, err := SealRegistry(ExtensionConformanceRegistryID, base.Digest(), types)
	if err != nil {
		return nil, err
	}
	return ExtendRegistry(base, document, validators)
}

func (fixture ExtensionConformanceFixture) validate(definition extensionFixtureDefinition) error {
	if fixture.SchemaVersion != definition.TypeID || fixture.ExtensionID != definition.ExtensionID ||
		fixture.OwnerTask != definition.OwnerTask || fixture.Status != "conformance_fixture" ||
		fixture.ProviderCallsRequired != 0 || fixture.HumanActionsRequired != 0 ||
		len(fixture.RequiredFields) < 3 || len(fixture.ProhibitedClaims) < 3 ||
		!validSortedIdentifiers(fixture.RequiredFields) || !validSortedIdentifiers(fixture.ProhibitedClaims) {
		return errors.New("extension conformance fixture identity or boundary is invalid")
	}
	return nil
}

func extensionFixtureDefinitions() []extensionFixtureDefinition {
	return []extensionFixtureDefinition{
		{
			Path: "testdata/extensions/calibration.json", Name: "extension.calibration",
			TypeID: CalibrationExtensionSchemaVersion, ExtensionID: "task-048-calibration", OwnerTask: "TASK-048",
			ParentTypes: []string{LegacyClaimFactsSchemaVersion},
		},
		{
			Path: "testdata/extensions/outcome.json", Name: "extension.outcome",
			TypeID: OutcomeExtensionSchemaVersion, ExtensionID: "task-057-outcome", OwnerTask: "TASK-057",
			ParentTypes: []string{StudyProvenanceSchemaVersion},
		},
		{
			Path: "testdata/extensions/profile.json", Name: "extension.profile",
			TypeID: ProfileExtensionSchemaVersion, ExtensionID: "task-058-profile", OwnerTask: "TASK-058",
			ParentTypes: []string{ReferenceIndexSchemaVersion},
		},
		{
			Path: "testdata/extensions/reliance.json", Name: "extension.reliance",
			TypeID: RelianceExtensionSchemaVersion, ExtensionID: "task-065-reliance", OwnerTask: "TASK-065",
			ParentTypes: []string{LegacyClaimFactsSchemaVersion, protocolkit.RunSchema},
		},
		{
			Path: "testdata/extensions/verifier-relation.json", Name: "extension.verifier-relation",
			TypeID: VerifierRelationExtensionSchemaVersion, ExtensionID: "task-056-verifier-relation", OwnerTask: "TASK-056",
			ParentTypes: []string{mutation.ConstructRepairEvidenceSchemaVersion},
		},
	}
}

func externalParentRef(kind EdgeKind, record ComponentRecord, capsuleID string) ParentRef {
	return ParentRef{
		Kind: kind, ComponentID: record.ComponentID, TypeID: record.TypeID,
		Role: record.Role, Visibility: record.Visibility, Resolution: ParentExternal, CapsuleID: capsuleID,
	}
}

func extensionValidatorID(typeID string) string {
	return "evalwitness.validator.extension." + strings.TrimPrefix(typeID, "evalwitness.extension.")
}

func validSortedIdentifiers(values []string) bool {
	return slices.IsSorted(values) && slices.IndexFunc(values, func(value string) bool { return !validIdentifier(value) }) < 0 &&
		len(slices.Compact(slices.Clone(values))) == len(values)
}
