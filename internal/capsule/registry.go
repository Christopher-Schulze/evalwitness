package capsule

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

type PayloadValidator func([]byte) error

type BoundParent struct {
	Reference ParentRef
	Record    ComponentRecord
	Payload   []byte
}

type BindingContext struct {
	Component ComponentRecord
	Payload   []byte
	Parents   []BoundParent
}

type BindingValidator func(BindingContext) error

type Registry struct {
	document   RegistryDocument
	types      map[string]ComponentType
	validators map[string]PayloadValidator
	bindings   map[string]BindingValidator
}

func SealRegistry(registryID, baseDigest string, types []ComponentType) (RegistryDocument, error) {
	document := RegistryDocument{
		SchemaVersion: RegistrySchemaVersion, CanonicalPolicy: CanonicalPolicy,
		RegistryID: registryID, BaseRegistryDigest: baseDigest, Types: cloneComponentTypes(types),
	}
	slices.SortFunc(document.Types, func(left, right ComponentType) int {
		return strings.Compare(left.TypeID, right.TypeID)
	})
	for index := range document.Types {
		normalizeComponentType(&document.Types[index])
	}
	digest, err := registryDigest(document)
	if err != nil {
		return RegistryDocument{}, err
	}
	document.Digest = digest
	if err := document.Validate(); err != nil {
		return RegistryDocument{}, err
	}
	return document, nil
}

func NewRegistry(document RegistryDocument, validators map[string]PayloadValidator) (*Registry, error) {
	return NewRegistryWithBindings(document, validators, nil)
}

func NewRegistryWithBindings(document RegistryDocument, validators map[string]PayloadValidator, bindings map[string]BindingValidator) (*Registry, error) {
	if err := document.Validate(); err != nil {
		return nil, err
	}
	registry := &Registry{
		document: document, types: make(map[string]ComponentType, len(document.Types)),
		validators: make(map[string]PayloadValidator, len(validators)), bindings: make(map[string]BindingValidator, len(bindings)),
	}
	for id, validator := range validators {
		if !validIdentifier(id) || validator == nil {
			return nil, fmt.Errorf("invalid capsule validator %q", id)
		}
		registry.validators[id] = validator
	}
	for id, validator := range bindings {
		if !validIdentifier(id) || validator == nil {
			return nil, fmt.Errorf("invalid capsule binding validator %q", id)
		}
		registry.bindings[id] = validator
	}
	for _, descriptor := range document.Types {
		if registry.validators[descriptor.ValidatorID] == nil {
			return nil, fmt.Errorf("capsule type %q requires unavailable validator %q", descriptor.TypeID, descriptor.ValidatorID)
		}
		if descriptor.BindingValidatorID != "" && registry.bindings[descriptor.BindingValidatorID] == nil {
			return nil, fmt.Errorf("capsule type %q requires unavailable binding validator %q", descriptor.TypeID, descriptor.BindingValidatorID)
		}
		registry.types[descriptor.TypeID] = descriptor
	}
	for _, descriptor := range document.Types {
		for _, rule := range descriptor.ParentRules {
			parent, found := registry.types[rule.ParentType]
			if !found {
				return nil, fmt.Errorf("capsule type %q references unregistered parent type %q", descriptor.TypeID, rule.ParentType)
			}
			if !roleEdgeAllowed(
				ComponentRecord{TypeID: descriptor.TypeID, Role: descriptor.Role},
				ComponentRecord{TypeID: parent.TypeID, Role: parent.Role}, rule.Kind,
			) {
				return nil, fmt.Errorf("capsule type %q declares role-inverting edge %q to %q", descriptor.TypeID, rule.Kind, rule.ParentType)
			}
			if slices.Contains(rule.Resolutions, ParentOmitted) && rule.Kind != EdgeRedacts && rule.Kind != EdgeCommitsTo {
				return nil, fmt.Errorf("capsule type %q permits omission on non-projection edge %q", descriptor.TypeID, rule.Kind)
			}
		}
	}
	return registry, nil
}

func ExtendRegistry(base *Registry, extension RegistryDocument, validators map[string]PayloadValidator) (*Registry, error) {
	return ExtendRegistryWithBindings(base, extension, validators, nil)
}

func ExtendRegistryWithBindings(base *Registry, extension RegistryDocument, validators map[string]PayloadValidator, bindings map[string]BindingValidator) (*Registry, error) {
	if base == nil {
		return nil, errors.New("capsule registry extension requires a base registry")
	}
	if err := extension.Validate(); err != nil {
		return nil, fmt.Errorf("validate capsule registry extension: %w", err)
	}
	if extension.BaseRegistryDigest != base.document.Digest {
		return nil, errors.New("capsule registry extension does not bind the base registry")
	}
	types := cloneComponentTypes(base.document.Types)
	for _, descriptor := range extension.Types {
		if _, exists := base.types[descriptor.TypeID]; exists {
			return nil, fmt.Errorf("capsule registry extension redefines %q", descriptor.TypeID)
		}
		types = append(types, descriptor)
	}
	mergedValidators := make(map[string]PayloadValidator, len(base.validators)+len(validators))
	for id, validator := range base.validators {
		mergedValidators[id] = validator
	}
	for id, validator := range validators {
		if _, exists := mergedValidators[id]; exists {
			return nil, fmt.Errorf("capsule registry extension replaces validator %q", id)
		}
		mergedValidators[id] = validator
	}
	mergedBindings := make(map[string]BindingValidator, len(base.bindings)+len(bindings))
	for id, validator := range base.bindings {
		mergedBindings[id] = validator
	}
	for id, validator := range bindings {
		if _, exists := mergedBindings[id]; exists {
			return nil, fmt.Errorf("capsule registry extension replaces binding validator %q", id)
		}
		mergedBindings[id] = validator
	}
	document, err := SealRegistry(extension.RegistryID, base.document.Digest, types)
	if err != nil {
		return nil, err
	}
	return NewRegistryWithBindings(document, mergedValidators, mergedBindings)
}

func (d RegistryDocument) Validate() error {
	if d.SchemaVersion != RegistrySchemaVersion || d.CanonicalPolicy != CanonicalPolicy ||
		!validIdentifier(d.RegistryID) || !validOptionalDigest(d.BaseRegistryDigest) || !validDigest(d.Digest) || len(d.Types) == 0 {
		return errors.New("capsule registry identity is invalid")
	}
	previous := ""
	for _, descriptor := range d.Types {
		if err := validateComponentType(descriptor); err != nil {
			return err
		}
		if descriptor.TypeID <= previous {
			return errors.New("capsule registry types must be unique and sorted")
		}
		previous = descriptor.TypeID
	}
	digest, err := registryDigest(d)
	if err != nil || digest != d.Digest {
		return errors.New("capsule registry digest is invalid")
	}
	return nil
}

func (r *Registry) Document() RegistryDocument {
	if r == nil {
		return RegistryDocument{}
	}
	copy := r.document
	copy.Types = cloneComponentTypes(r.document.Types)
	return copy
}

func (r *Registry) Digest() string {
	if r == nil {
		return ""
	}
	return r.document.Digest
}

func (r *Registry) Lookup(typeID string) (ComponentType, bool) {
	if r == nil {
		return ComponentType{}, false
	}
	descriptor, found := r.types[typeID]
	return descriptor, found
}

func (r *Registry) ValidatePayload(typeID string, payload []byte) error {
	descriptor, found := r.Lookup(typeID)
	if !found {
		return fmt.Errorf("unknown capsule component type %q", typeID)
	}
	validator := r.validators[descriptor.ValidatorID]
	if validator == nil {
		return fmt.Errorf("capsule validator %q is unavailable", descriptor.ValidatorID)
	}
	if err := validator(slices.Clone(payload)); err != nil {
		return fmt.Errorf("validate capsule type %q: %w", typeID, err)
	}
	return nil
}

func (r *Registry) ValidateBindings(context BindingContext) error {
	descriptor, found := r.Lookup(context.Component.TypeID)
	if !found {
		return fmt.Errorf("unknown capsule component type %q", context.Component.TypeID)
	}
	if descriptor.BindingValidatorID == "" {
		return nil
	}
	validator := r.bindings[descriptor.BindingValidatorID]
	if validator == nil {
		return fmt.Errorf("capsule binding validator %q is unavailable", descriptor.BindingValidatorID)
	}
	if err := validator(cloneBindingContext(context)); err != nil {
		return fmt.Errorf("validate capsule bindings for %q: %w", context.Component.Name, err)
	}
	return nil
}

func cloneBindingContext(context BindingContext) BindingContext {
	cloned := BindingContext{
		Component: cloneComponentRecord(context.Component),
		Payload:   slices.Clone(context.Payload),
		Parents:   make([]BoundParent, len(context.Parents)),
	}
	for index, parent := range context.Parents {
		cloned.Parents[index] = BoundParent{
			Reference: parent.Reference,
			Record:    cloneComponentRecord(parent.Record),
			Payload:   slices.Clone(parent.Payload),
		}
	}
	return cloned
}

func validateComponentType(descriptor ComponentType) error {
	if !validIdentifier(descriptor.TypeID) || !validSchemaIdentity(descriptor.SchemaID) || !descriptor.Role.Valid() ||
		!validMediaType(descriptor.MediaType) || !descriptor.PayloadProfile.Valid() || !validIdentifier(descriptor.ValidatorID) ||
		descriptor.BindingValidatorID != "" && !validIdentifier(descriptor.BindingValidatorID) {
		return fmt.Errorf("capsule component type %q is invalid", descriptor.TypeID)
	}
	if err := validateVisibilities(descriptor.AllowedVisibilities); err != nil {
		return fmt.Errorf("capsule component type %q: %w", descriptor.TypeID, err)
	}
	previous := ""
	for _, rule := range descriptor.ParentRules {
		key := string(rule.Kind) + "\x00" + rule.ParentType
		if err := validateParentRule(rule); err != nil {
			return fmt.Errorf("capsule component type %q: %w", descriptor.TypeID, err)
		}
		if key <= previous {
			return fmt.Errorf("capsule component type %q parent rules must be unique and sorted", descriptor.TypeID)
		}
		previous = key
	}
	return nil
}

func validateParentRule(rule ParentRule) error {
	if !rule.Kind.Valid() || !validIdentifier(rule.ParentType) || rule.Minimum < 0 ||
		rule.Maximum < -1 || rule.Maximum >= 0 && rule.Maximum < rule.Minimum || len(rule.Resolutions) == 0 {
		return errors.New("parent rule is invalid")
	}
	previous := ""
	for _, resolution := range rule.Resolutions {
		if !resolution.Valid() || string(resolution) <= previous {
			return errors.New("parent rule resolutions must be valid, unique, and sorted")
		}
		previous = string(resolution)
	}
	return nil
}

func validateVisibilities(values []Visibility) error {
	if len(values) == 0 {
		return errors.New("at least one visibility is required")
	}
	previous := -1
	for _, value := range values {
		if !value.Valid() || value.rank() <= previous {
			return errors.New("visibilities must be valid, unique, and ordered public, restricted, private")
		}
		previous = value.rank()
	}
	return nil
}

func registryDigest(document RegistryDocument) (string, error) {
	copy := document
	copy.Digest = ""
	return protocol.Digest(copy)
}

func normalizeComponentType(descriptor *ComponentType) {
	slices.SortFunc(descriptor.AllowedVisibilities, func(left, right Visibility) int { return left.rank() - right.rank() })
	slices.SortFunc(descriptor.ParentRules, func(left, right ParentRule) int {
		return strings.Compare(string(left.Kind)+"\x00"+left.ParentType, string(right.Kind)+"\x00"+right.ParentType)
	})
	for index := range descriptor.ParentRules {
		slices.Sort(descriptor.ParentRules[index].Resolutions)
	}
}

func cloneComponentTypes(values []ComponentType) []ComponentType {
	cloned := make([]ComponentType, len(values))
	for index, value := range values {
		cloned[index] = value
		cloned[index].AllowedVisibilities = slices.Clone(value.AllowedVisibilities)
		cloned[index].ParentRules = slices.Clone(value.ParentRules)
		for rule := range cloned[index].ParentRules {
			cloned[index].ParentRules[rule].Resolutions = slices.Clone(value.ParentRules[rule].Resolutions)
		}
	}
	return cloned
}

func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}

func validOptionalDigest(value string) bool {
	return value == "" || validDigest(value)
}

func validIdentifier(value string) bool {
	if value == "" || len(value) > 256 || value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || strings.ContainsRune("._:/-", character) {
			continue
		}
		return false
	}
	return true
}

func validSchemaIdentity(value string) bool {
	return validIdentifier(value) && strings.Contains(value, ".")
}

func validMediaType(value string) bool {
	return value != "" && len(value) <= 128 && value == strings.TrimSpace(value) && strings.Contains(value, "/")
}
