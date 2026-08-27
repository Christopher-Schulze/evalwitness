package capsule

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/lineage"
	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/relation"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	ReferenceIndexSchemaVersion = "evalwitness.reference-evidence-index.v1"
	referenceIndexValidatorID   = "evalwitness.validator.reference-evidence-index.v1"
	referenceIndexBindingID     = "evalwitness.validator.reference-evidence-index-bindings.v1"
)

type ReferenceIndexEntry struct {
	Name        string `json:"name"`
	ComponentID string `json:"component_id"`
	TypeID      string `json:"type_id"`
}

type ReferenceIndex struct {
	SchemaVersion string                `json:"schema_version"`
	Entries       []ReferenceIndexEntry `json:"entries"`
	Digest        string                `json:"digest"`
}

func referenceIndexTypes() []ComponentType {
	return []ComponentType{{
		TypeID: ReferenceIndexSchemaVersion, SchemaID: ReferenceIndexSchemaVersion, Role: RoleDerivation,
		AllowedVisibilities: []Visibility{VisibilityPublic}, MediaType: "application/json",
		PayloadProfile: PayloadCanonicalJSON, ValidatorID: referenceIndexValidatorID, BindingValidatorID: referenceIndexBindingID,
		ParentRules: []ParentRule{
			parentRule(EdgeDerivedFrom, AnalysisProvenanceSchemaVersion, 1, 1),
			parentRule(EdgeDerivedFrom, BuildProvenanceSchemaVersion, 1, 1),
			parentRule(EdgeDerivedFrom, ClockProvenanceSchemaVersion, 1, 1),
			parentRule(EdgeDerivedFrom, DatasetProvenanceSchemaVersion, 1, 1),
			parentRule(EdgeDerivedFrom, LegacyClaimFactsSchemaVersion, 1, 1),
			parentRule(EdgeDerivedFrom, protocol.RunSchema, 1, 1),
			parentRule(EdgeDerivedFrom, RouteProvenanceSchemaVersion, 1, 1),
			parentRule(EdgeDerivedFrom, StudyProvenanceSchemaVersion, 1, 1),
			parentRule(EdgeDerivedFrom, lineage.ReleaseSchemaVersion, 1, 1),
			parentRule(EdgeDerivedFrom, preprocess.CanonicalTrajectorySchema, 1, 1),
			parentRule(EdgeDerivedFrom, mutation.ConstructRepairEvidenceSchemaVersion, 1, 1),
			parentRule(EdgeDerivedFrom, mutation.ConstructChallengeSchemaVersion, 1, 1),
			parentRule(EdgeDerivedFrom, relation.ScarcityPublicEvidenceSchemaVersion, 1, 1),
			parentRule(EdgeDerivedFrom, relation.OwnerInspectionPublicAttestationSchemaVersion, 1, 1),
		},
	}}
}

func referenceIndexPayloadValidators() map[string]PayloadValidator {
	return map[string]PayloadValidator{referenceIndexValidatorID: func(payload []byte) error {
		var index ReferenceIndex
		if err := decodeReferenceJSON(payload, &index); err != nil {
			return err
		}
		return index.Validate()
	}}
}

func referenceIndexBindingValidators() map[string]BindingValidator {
	return map[string]BindingValidator{referenceIndexBindingID: validateReferenceIndexBindings}
}

func buildReferenceIndex(roots []ComponentRecord) (ReferenceIndex, error) {
	index := ReferenceIndex{SchemaVersion: ReferenceIndexSchemaVersion, Entries: make([]ReferenceIndexEntry, 0, len(roots))}
	for _, root := range roots {
		index.Entries = append(index.Entries, ReferenceIndexEntry{Name: root.Name, ComponentID: root.ComponentID, TypeID: root.TypeID})
	}
	slices.SortFunc(index.Entries, func(left, right ReferenceIndexEntry) int { return strings.Compare(left.Name, right.Name) })
	digest, err := referenceIndexDigest(index)
	if err != nil {
		return ReferenceIndex{}, err
	}
	index.Digest = digest
	return index, index.Validate()
}

func (index ReferenceIndex) Validate() error {
	if index.SchemaVersion != ReferenceIndexSchemaVersion || !validDigest(index.Digest) || len(index.Entries) == 0 {
		return errors.New("reference evidence index identity is invalid")
	}
	previous := ""
	seenComponents := make(map[string]struct{}, len(index.Entries))
	for _, entry := range index.Entries {
		if !validIdentifier(entry.Name) || !validDigest(entry.ComponentID) || !validIdentifier(entry.TypeID) || entry.Name <= previous {
			return errors.New("reference evidence index entries are invalid, duplicated, or unsorted")
		}
		if _, duplicate := seenComponents[entry.ComponentID]; duplicate {
			return errors.New("reference evidence index repeats a component")
		}
		seenComponents[entry.ComponentID] = struct{}{}
		previous = entry.Name
	}
	digest, err := referenceIndexDigest(index)
	if err != nil || digest != index.Digest {
		return errors.New("reference evidence index digest is invalid")
	}
	return nil
}

func validateReferenceIndexBindings(context BindingContext) error {
	var index ReferenceIndex
	if err := decodeReferenceJSON(context.Payload, &index); err != nil {
		return err
	}
	if err := index.Validate(); err != nil {
		return err
	}
	if len(index.Entries) != len(context.Parents) {
		return errors.New("reference evidence index and capsule graph have different root counts")
	}
	parents := make(map[string]BoundParent, len(context.Parents))
	for _, parent := range context.Parents {
		if parent.Reference.Resolution != ParentInternal {
			return errors.New("reference evidence index requires internal roots")
		}
		parents[parent.Record.ComponentID] = parent
	}
	for _, entry := range index.Entries {
		parent, found := parents[entry.ComponentID]
		if !found || entry.Name != parent.Record.Name || entry.TypeID != parent.Record.TypeID {
			return fmt.Errorf("reference evidence root %q differs from its capsule parent", entry.Name)
		}
	}
	return nil
}

func referenceIndexDigest(index ReferenceIndex) (string, error) {
	index.Digest = ""
	return protocol.Digest(index)
}
