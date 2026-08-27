package capsule

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

type ManifestInput struct {
	StudyID           string
	CellID            string
	ParentCapsules    []CapsuleRef
	ScientificRoots   []string
	PresentationRoots []string
	Components        []ComponentRecord
}

type scientificManifest struct {
	SchemaVersion   string            `json:"schema_version"`
	CanonicalPolicy string            `json:"canonical_policy"`
	RegistryDigest  string            `json:"registry_digest"`
	StudyID         string            `json:"study_id"`
	CellID          string            `json:"cell_id"`
	ParentCapsules  []CapsuleRef      `json:"parent_capsules"`
	ScientificRoots []string          `json:"scientific_roots"`
	Components      []ComponentRecord `json:"components"`
}

func BuildManifest(registry *Registry, input ManifestInput) (Manifest, error) {
	if registry == nil {
		return Manifest{}, errors.New("capsule registry is required")
	}
	manifest := Manifest{
		SchemaVersion: ManifestSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		RegistryDigest: registry.Digest(), StudyID: input.StudyID, CellID: input.CellID,
		ParentCapsules: slices.Clone(input.ParentCapsules), ScientificRoots: slices.Clone(input.ScientificRoots),
		PresentationRoots: slices.Clone(input.PresentationRoots), Components: cloneComponentRecords(input.Components),
	}
	normalizeManifest(&manifest)
	if err := validateManifestStructure(registry, manifest, false); err != nil {
		return Manifest{}, err
	}
	var err error
	manifest.CapsuleID, err = scientificDigest(manifest)
	if err != nil {
		return Manifest{}, err
	}
	manifest.ManifestDigest, err = manifestDigest(manifest)
	if err != nil {
		return Manifest{}, err
	}
	if err := manifest.Validate(registry); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func (m Manifest) Validate(registry *Registry) error {
	if err := validateManifestStructure(registry, m, true); err != nil {
		return err
	}
	capsuleID, err := scientificDigest(m)
	if err != nil || capsuleID != m.CapsuleID {
		return errors.New("capsule scientific identity is invalid")
	}
	digest, err := manifestDigest(m)
	if err != nil || digest != m.ManifestDigest {
		return errors.New("capsule manifest digest is invalid")
	}
	return nil
}

func validateManifestStructure(registry *Registry, manifest Manifest, requireIdentity bool) error {
	if registry == nil || manifest.SchemaVersion != ManifestSchemaVersion || manifest.CanonicalPolicy != CanonicalPolicy ||
		manifest.RegistryDigest != registry.Digest() || !validIdentifier(manifest.StudyID) || !validIdentifier(manifest.CellID) {
		return errors.New("capsule manifest identity is invalid")
	}
	if requireIdentity && (!validDigest(manifest.CapsuleID) || !validDigest(manifest.ManifestDigest)) {
		return errors.New("capsule manifest digests are missing")
	}
	if !requireIdentity && (manifest.CapsuleID != "" || manifest.ManifestDigest != "") {
		return errors.New("unsealed capsule manifest contains identity digests")
	}
	if len(manifest.Components) == 0 || len(manifest.ScientificRoots) == 0 {
		return errors.New("capsule requires components and a scientific root")
	}
	if err := validateCapsuleRefs(manifest.ParentCapsules); err != nil {
		return err
	}
	components, err := validateComponentSet(registry, manifest.Components)
	if err != nil {
		return err
	}
	if err := validateRoots(manifest, components); err != nil {
		return err
	}
	return validateGraph(registry, manifest, components)
}

func validateComponentSet(registry *Registry, records []ComponentRecord) (map[string]ComponentRecord, error) {
	components := make(map[string]ComponentRecord, len(records))
	names := make(map[string]struct{}, len(records))
	previous := ""
	for _, record := range records {
		if record.ComponentID <= previous {
			return nil, errors.New("capsule components must be unique and sorted by component ID")
		}
		if _, duplicate := names[record.Name]; duplicate {
			return nil, fmt.Errorf("duplicate capsule component name %q", record.Name)
		}
		if err := ValidateComponentRecord(registry, record); err != nil {
			return nil, err
		}
		previous = record.ComponentID
		components[record.ComponentID] = record
		names[record.Name] = struct{}{}
	}
	return components, nil
}

func validateRoots(manifest Manifest, components map[string]ComponentRecord) error {
	if err := validateRootSet("scientific", manifest.ScientificRoots, components, false); err != nil {
		return err
	}
	if err := validateRootSet("presentation", manifest.PresentationRoots, components, true); err != nil {
		return err
	}
	return nil
}

func validateRootSet(name string, roots []string, components map[string]ComponentRecord, presentation bool) error {
	previous := ""
	for _, root := range roots {
		record, found := components[root]
		if !found || !validDigest(root) || root <= previous || (record.Role == RolePresentation) != presentation {
			return fmt.Errorf("capsule %s roots are invalid", name)
		}
		previous = root
	}
	if presentation && len(roots) == 0 {
		return nil
	}
	return nil
}

func validateGraph(registry *Registry, manifest Manifest, components map[string]ComponentRecord) error {
	parentCapsules := make(map[string]struct{}, len(manifest.ParentCapsules))
	for _, parent := range manifest.ParentCapsules {
		parentCapsules[parent.CapsuleID] = struct{}{}
	}
	for _, child := range manifest.Components {
		for _, parent := range child.Parents {
			if parent.Resolution != ParentInternal && parent.CapsuleID != "" {
				if _, declared := parentCapsules[parent.CapsuleID]; !declared {
					return fmt.Errorf("capsule component %q references undeclared parent capsule %q", child.Name, parent.CapsuleID)
				}
			}
			resolved, err := resolveParent(registry, parent, components)
			if err != nil {
				return fmt.Errorf("capsule component %q: %w", child.Name, err)
			}
			if err := validateEdge(child, resolved, parent); err != nil {
				return fmt.Errorf("capsule component %q: %w", child.Name, err)
			}
		}
	}
	if err := rejectCycles(manifest.Components, components); err != nil {
		return err
	}
	return rejectUnreachable(manifest, components)
}

func resolveParent(registry *Registry, parent ParentRef, components map[string]ComponentRecord) (ComponentRecord, error) {
	if _, known := registry.Lookup(parent.TypeID); !known {
		return ComponentRecord{}, fmt.Errorf("parent type %q is not registered", parent.TypeID)
	}
	if parent.Resolution != ParentInternal {
		return ComponentRecord{ComponentID: parent.ComponentID, TypeID: parent.TypeID, Role: parent.Role, Visibility: parent.Visibility}, nil
	}
	resolved, found := components[parent.ComponentID]
	if !found {
		return ComponentRecord{}, fmt.Errorf("internal parent %q is missing", parent.ComponentID)
	}
	if resolved.TypeID != parent.TypeID || resolved.Role != parent.Role || resolved.Visibility != parent.Visibility {
		return ComponentRecord{}, fmt.Errorf("internal parent %q metadata does not match", parent.ComponentID)
	}
	return resolved, nil
}

func validateEdge(child, parent ComponentRecord, reference ParentRef) error {
	if !roleEdgeAllowed(child, parent, reference.Kind) {
		return fmt.Errorf("edge %q inverts or violates provenance roles", reference.Kind)
	}
	privacyProjection := reference.Kind == EdgeRedacts || reference.Kind == EdgeCommitsTo
	if privacyProjection {
		if parent.Visibility.rank() <= child.Visibility.rank() {
			return fmt.Errorf("edge %q does not cross from a more private parent", reference.Kind)
		}
	} else if child.Visibility.rank() < parent.Visibility.rank() {
		return fmt.Errorf("edge %q leaks a more private parent", reference.Kind)
	}
	if reference.Resolution == ParentOmitted && !privacyProjection {
		return fmt.Errorf("edge %q cannot omit its parent", reference.Kind)
	}
	if reference.Kind == EdgeSupersedes && child.TypeID != parent.TypeID {
		return errors.New("supersession must preserve component type")
	}
	return nil
}

func roleEdgeAllowed(child, parent ComponentRecord, kind EdgeKind) bool {
	switch kind {
	case EdgeDerivedFrom:
		return slices.Contains([]Role{RoleDerivation, RoleGovernance}, child.Role) && parent.Role != RolePresentation
	case EdgeObservedFrom:
		return child.Role == RoleObservation && parent.Role == RoleObservation
	case EdgeGovernedBy:
		return child.Role != RoleGovernance && child.Role != RolePresentation && parent.Role == RoleGovernance
	case EdgeAttests:
		return child.Role == RoleAttestation && slices.Contains([]Role{RoleObservation, RoleDerivation, RoleCommitment}, parent.Role)
	case EdgeCommitsTo:
		return child.Role == RoleCommitment && parent.Role != RolePresentation
	case EdgeRedacts:
		return slices.Contains([]Role{RoleDerivation, RoleCommitment}, child.Role) &&
			slices.Contains([]Role{RoleObservation, RoleDerivation, RoleAttestation}, parent.Role)
	case EdgeRenders:
		return child.Role == RolePresentation && parent.Role != RolePresentation
	case EdgeSupersedes:
		return child.Role == parent.Role
	default:
		return false
	}
}

func rejectCycles(records []ComponentRecord, components map[string]ComponentRecord) error {
	state := make(map[string]uint8, len(records))
	var visit func(string) error
	visit = func(componentID string) error {
		if state[componentID] == 1 {
			return fmt.Errorf("capsule provenance graph contains a cycle at %q", componentID)
		}
		if state[componentID] == 2 {
			return nil
		}
		state[componentID] = 1
		for _, parent := range components[componentID].Parents {
			if parent.Resolution == ParentInternal {
				if err := visit(parent.ComponentID); err != nil {
					return err
				}
			}
		}
		state[componentID] = 2
		return nil
	}
	for _, record := range records {
		if err := visit(record.ComponentID); err != nil {
			return err
		}
	}
	return nil
}

func rejectUnreachable(manifest Manifest, components map[string]ComponentRecord) error {
	reachable := make(map[string]struct{}, len(components))
	stack := append(slices.Clone(manifest.ScientificRoots), manifest.PresentationRoots...)
	for len(stack) > 0 {
		last := len(stack) - 1
		componentID := stack[last]
		stack = stack[:last]
		if _, seen := reachable[componentID]; seen {
			continue
		}
		reachable[componentID] = struct{}{}
		for _, parent := range components[componentID].Parents {
			if parent.Resolution == ParentInternal {
				stack = append(stack, parent.ComponentID)
			}
		}
	}
	if len(reachable) != len(components) {
		return errors.New("capsule contains a component unreachable from every declared root")
	}
	return nil
}

func validateCapsuleRefs(references []CapsuleRef) error {
	previous := ""
	for _, reference := range references {
		key := reference.Relation + "\x00" + reference.CapsuleID
		if !validIdentifier(reference.Relation) || !validDigest(reference.CapsuleID) || key <= previous {
			return errors.New("parent capsules must be valid, unique, and sorted")
		}
		previous = key
	}
	return nil
}

func scientificDigest(manifest Manifest) (string, error) {
	identity := scientificManifest{
		SchemaVersion: manifest.SchemaVersion, CanonicalPolicy: manifest.CanonicalPolicy,
		RegistryDigest: manifest.RegistryDigest, StudyID: manifest.StudyID, CellID: manifest.CellID,
		ParentCapsules: slices.Clone(manifest.ParentCapsules), ScientificRoots: slices.Clone(manifest.ScientificRoots),
	}
	for _, record := range manifest.Components {
		if record.Role != RolePresentation {
			identity.Components = append(identity.Components, cloneComponentRecord(record))
		}
	}
	return protocol.Digest(identity)
}

func manifestDigest(manifest Manifest) (string, error) {
	copy := manifest
	copy.ManifestDigest = ""
	return protocol.Digest(copy)
}

func normalizeManifest(manifest *Manifest) {
	slices.SortFunc(manifest.ParentCapsules, func(left, right CapsuleRef) int {
		return strings.Compare(left.Relation+"\x00"+left.CapsuleID, right.Relation+"\x00"+right.CapsuleID)
	})
	slices.Sort(manifest.ScientificRoots)
	slices.Sort(manifest.PresentationRoots)
	slices.SortFunc(manifest.Components, func(left, right ComponentRecord) int {
		return strings.Compare(left.ComponentID, right.ComponentID)
	})
}

func cloneComponentRecords(values []ComponentRecord) []ComponentRecord {
	cloned := make([]ComponentRecord, len(values))
	for index, value := range values {
		cloned[index] = cloneComponentRecord(value)
	}
	return cloned
}

func cloneComponentRecord(value ComponentRecord) ComponentRecord {
	value.Parents = slices.Clone(value.Parents)
	return value
}
