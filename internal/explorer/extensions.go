package explorer

import (
	"errors"
	"slices"
	"sort"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/internal/reliance"
)

type extensionDefinition struct {
	id          string
	owner       string
	required    []string
	unavailable Availability
}

func extensionDefinitions() []extensionDefinition {
	return []extensionDefinition{
		{"task-048-calibration", "TASK-048", []string{capsule.CalibrationExtensionSchemaVersion}, AvailabilityNotMeasured},
		{"task-056-verifier-relation", "TASK-056", []string{capsule.VerifierRelationExtensionSchemaVersion}, AvailabilityNotMeasured},
		{"task-057-outcome", "TASK-057", []string{capsule.OutcomeExtensionSchemaVersion}, AvailabilityNotRun},
		{"task-058-profile", "TASK-058", []string{capsule.ProfileExtensionSchemaVersion}, AvailabilityNotMeasured},
		{"task-065-reliance", "TASK-065", []string{reliance.EvidenceRelianceMapSchemaVersion}, AvailabilityNotMeasured},
	}
}

func buildExtensionViews(manifest capsule.Manifest) []ExtensionView {
	componentsByType := indexExtensionComponents(manifest)
	definitions := extensionDefinitions()
	views := make([]ExtensionView, len(definitions))
	for index, definition := range definitions {
		components, missing := extensionEvidence(definition.required, componentsByType)
		availability := AvailabilityAvailable
		if len(missing) != 0 {
			availability = definition.unavailable
		}
		views[index] = ExtensionView{
			ExtensionID: definition.id, OwnerTask: definition.owner,
			RequiredTypes: slices.Clone(definition.required), Components: components,
			MissingTypes: missing, Availability: availability,
		}
	}
	return views
}

func extensionEvidence(required []string, indexed map[string][]string) ([]ExtensionComponentIdentity, []string) {
	components := []ExtensionComponentIdentity{}
	missing := []string{}
	for _, typeID := range required {
		componentIDs := indexed[typeID]
		if len(componentIDs) == 0 {
			missing = append(missing, typeID)
			continue
		}
		for _, componentID := range componentIDs {
			components = append(components, ExtensionComponentIdentity{TypeID: typeID, ComponentID: componentID})
		}
	}
	return components, missing
}

func indexExtensionComponents(manifest capsule.Manifest) map[string][]string {
	indexed := make(map[string][]string)
	for _, component := range manifest.Components {
		indexed[component.TypeID] = append(indexed[component.TypeID], component.ComponentID)
	}
	for typeID := range indexed {
		sort.Strings(indexed[typeID])
	}
	return indexed
}

func validateExtensionViews(views []ExtensionView) error {
	definitions := extensionDefinitions()
	if len(views) != len(definitions) {
		return errors.New("evidence explorer extension registry is incomplete")
	}
	for index, definition := range definitions {
		if err := validateExtensionView(views[index], definition); err != nil {
			return err
		}
	}
	return nil
}

func validateExtensionView(view ExtensionView, definition extensionDefinition) error {
	if view.ExtensionID != definition.id || view.OwnerTask != definition.owner ||
		!slices.Equal(view.RequiredTypes, definition.required) || !view.Availability.Valid() ||
		validateSortedUniqueAllowEmpty("extension missing types", view.MissingTypes) != nil {
		return errors.New("evidence explorer extension registry is invalid")
	}
	presentTypes, err := validateExtensionComponents(view.Components, definition.required)
	if err != nil {
		return err
	}
	expectedMissing := missingExtensionTypes(definition.required, presentTypes)
	if !slices.Equal(view.MissingTypes, expectedMissing) {
		return errors.New("evidence explorer extension missing types differ from component evidence")
	}
	if len(expectedMissing) == 0 {
		if view.Availability != AvailabilityAvailable || len(view.Components) == 0 {
			return errors.New("evidence explorer available extension lacks evidence")
		}
		return nil
	}
	if view.Availability != definition.unavailable {
		return errors.New("evidence explorer unavailable extension differs from its contract")
	}
	return nil
}

func validateExtensionComponents(components []ExtensionComponentIdentity, required []string) (map[string]struct{}, error) {
	presentTypes := make(map[string]struct{})
	previous := ExtensionComponentIdentity{}
	for index, component := range components {
		if !slices.Contains(required, component.TypeID) || !validDigest(component.ComponentID) ||
			(index > 0 && (component.TypeID < previous.TypeID ||
				(component.TypeID == previous.TypeID && component.ComponentID <= previous.ComponentID))) {
			return nil, errors.New("evidence explorer extension component identity is invalid or unordered")
		}
		presentTypes[component.TypeID] = struct{}{}
		previous = component
	}
	return presentTypes, nil
}

func missingExtensionTypes(required []string, present map[string]struct{}) []string {
	missing := []string{}
	for _, typeID := range required {
		if _, ok := present[typeID]; !ok {
			missing = append(missing, typeID)
		}
	}
	return missing
}
