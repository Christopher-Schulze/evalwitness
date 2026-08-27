package capsule

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

type ComponentInput struct {
	Name       string
	TypeID     string
	Visibility Visibility
	Payload    []byte
	Parents    []ParentRef
}

func BuildComponent(registry *Registry, input ComponentInput) (ComponentRecord, []byte, error) {
	if registry == nil {
		return ComponentRecord{}, nil, errors.New("capsule registry is required")
	}
	descriptor, found := registry.Lookup(input.TypeID)
	if !found {
		return ComponentRecord{}, nil, fmt.Errorf("unknown capsule component type %q", input.TypeID)
	}
	payload, err := normalizePayload(descriptor.PayloadProfile, input.Payload)
	if err != nil {
		return ComponentRecord{}, nil, err
	}
	if err := registry.ValidatePayload(input.TypeID, payload); err != nil {
		return ComponentRecord{}, nil, err
	}
	record := componentRecord(descriptor, input, payload)
	normalizeParentRefs(record.Parents)
	record.ComponentID, err = componentDigest(record)
	if err != nil {
		return ComponentRecord{}, nil, err
	}
	if err := ValidateComponentRecord(registry, record); err != nil {
		return ComponentRecord{}, nil, err
	}
	return record, payload, nil
}

func ValidateComponentRecord(registry *Registry, record ComponentRecord) error {
	if registry == nil {
		return errors.New("capsule registry is required")
	}
	descriptor, found := registry.Lookup(record.TypeID)
	if !found {
		return fmt.Errorf("unknown capsule component type %q", record.TypeID)
	}
	if !validIdentifier(record.Name) || !validDigest(record.ComponentID) || record.SchemaID != descriptor.SchemaID ||
		record.Role != descriptor.Role || record.MediaType != descriptor.MediaType || record.Payload.Canonicalization != descriptor.PayloadProfile {
		return fmt.Errorf("capsule component %q does not match its registered type", record.Name)
	}
	if !slices.Contains(descriptor.AllowedVisibilities, record.Visibility) {
		return fmt.Errorf("capsule component %q uses forbidden visibility %q", record.Name, record.Visibility)
	}
	if err := validatePayloadIdentity(record.Payload); err != nil {
		return fmt.Errorf("capsule component %q: %w", record.Name, err)
	}
	if err := validateParentRefs(descriptor, record.Parents); err != nil {
		return fmt.Errorf("capsule component %q: %w", record.Name, err)
	}
	digest, err := componentDigest(record)
	if err != nil || digest != record.ComponentID {
		return fmt.Errorf("capsule component %q identity digest is invalid", record.Name)
	}
	return nil
}

func componentRecord(descriptor ComponentType, input ComponentInput, payload []byte) ComponentRecord {
	return ComponentRecord{
		Name: input.Name, TypeID: descriptor.TypeID, SchemaID: descriptor.SchemaID,
		Role: descriptor.Role, Visibility: input.Visibility, MediaType: descriptor.MediaType,
		Payload: PayloadIdentity{
			Algorithm: DigestAlgorithm, Digest: protocol.DigestBytes(payload), Bytes: int64(len(payload)),
			Canonicalization: descriptor.PayloadProfile,
		},
		Parents: slices.Clone(input.Parents),
	}
}

func normalizePayload(profile PayloadProfile, payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return nil, errors.New("capsule payload must not be empty")
	}
	switch profile {
	case PayloadExactBytes:
		return slices.Clone(payload), nil
	case PayloadCanonicalJSON:
		canonical, err := protocol.CanonicalizeJSON(payload)
		if err != nil {
			return nil, fmt.Errorf("canonicalize capsule payload: %w", err)
		}
		return canonical, nil
	default:
		return nil, fmt.Errorf("unknown capsule payload profile %q", profile)
	}
}

func validatePayloadIdentity(payload PayloadIdentity) error {
	if payload.Algorithm != DigestAlgorithm || !validDigest(payload.Digest) || payload.Bytes < 1 || !payload.Canonicalization.Valid() {
		return errors.New("payload identity is invalid")
	}
	return nil
}

func validateParentRefs(descriptor ComponentType, parents []ParentRef) error {
	previous := ""
	for _, parent := range parents {
		key := parentRefKey(parent)
		if err := validateParentRef(parent); err != nil {
			return err
		}
		if key <= previous {
			return errors.New("parent references must be unique and sorted")
		}
		previous = key
	}
	for _, rule := range descriptor.ParentRules {
		count := countMatchingParents(parents, rule)
		if count < rule.Minimum || rule.Maximum >= 0 && count > rule.Maximum {
			return fmt.Errorf("parent rule %q -> %q count %d is outside [%d,%d]", rule.Kind, rule.ParentType, count, rule.Minimum, rule.Maximum)
		}
	}
	for _, parent := range parents {
		if !slices.ContainsFunc(descriptor.ParentRules, func(rule ParentRule) bool {
			return parent.Kind == rule.Kind && parent.TypeID == rule.ParentType && slices.Contains(rule.Resolutions, parent.Resolution)
		}) {
			return fmt.Errorf("parent %q is not allowed by the component type", parent.ComponentID)
		}
	}
	return nil
}

func validateParentRef(parent ParentRef) error {
	if !parent.Kind.Valid() || !validDigest(parent.ComponentID) || !validIdentifier(parent.TypeID) ||
		!parent.Role.Valid() || !parent.Visibility.Valid() || !parent.Resolution.Valid() {
		return errors.New("parent reference is invalid")
	}
	switch parent.Resolution {
	case ParentInternal:
		if parent.CapsuleID != "" || parent.OmissionClass != "" {
			return errors.New("internal parent contains external or omission metadata")
		}
	case ParentExternal:
		if !validDigest(parent.CapsuleID) || parent.OmissionClass != "" {
			return errors.New("external parent must bind one capsule and no omission class")
		}
	case ParentOmitted:
		if !validIdentifier(parent.OmissionClass) || !validOptionalDigest(parent.CapsuleID) {
			return errors.New("omitted parent must declare an omission class")
		}
	}
	return nil
}

func countMatchingParents(parents []ParentRef, rule ParentRule) int {
	count := 0
	for _, parent := range parents {
		if parent.Kind == rule.Kind && parent.TypeID == rule.ParentType && slices.Contains(rule.Resolutions, parent.Resolution) {
			count++
		}
	}
	return count
}

func componentDigest(record ComponentRecord) (string, error) {
	copy := record
	copy.ComponentID = ""
	return protocol.Digest(copy)
}

func normalizeParentRefs(parents []ParentRef) {
	slices.SortFunc(parents, func(left, right ParentRef) int {
		return strings.Compare(parentRefKey(left), parentRefKey(right))
	})
}

func parentRefKey(parent ParentRef) string {
	return string(parent.Kind) + "\x00" + parent.TypeID + "\x00" + parent.ComponentID
}
