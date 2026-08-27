package capsule

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	InTotoStatementType     = "https://in-toto.io/Statement/v1"
	CapsulePredicateType    = "https://evalwitness.dev/attestation/capsule/v1"
	CapsulePredicateVersion = "evalwitness.in-toto-capsule-predicate.v1"
)

type InTotoSubject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

type CapsulePredicate struct {
	SchemaVersion          string         `json:"schema_version"`
	CapsuleID              string         `json:"capsule_id"`
	ManifestDigest         string         `json:"manifest_digest"`
	RegistryDigest         string         `json:"registry_digest"`
	StudyID                string         `json:"study_id"`
	CellID                 string         `json:"cell_id"`
	ScientificRoots        []string       `json:"scientific_roots"`
	ScientificComponents   int            `json:"scientific_components"`
	PresentationComponents int            `json:"presentation_components"`
	VisibilityCounts       map[string]int `json:"visibility_counts"`
}

type InTotoStatement struct {
	Type          string           `json:"_type"`
	Subject       []InTotoSubject  `json:"subject"`
	PredicateType string           `json:"predicateType"`
	Predicate     CapsulePredicate `json:"predicate"`
}

func BuildStatement(manifest Manifest, registry *Registry) (InTotoStatement, error) {
	if err := manifest.Validate(registry); err != nil {
		return InTotoStatement{}, err
	}
	report := verificationReport(manifest)
	statement := InTotoStatement{
		Type: InTotoStatementType,
		Subject: []InTotoSubject{{
			Name:   scientificSubjectName(manifest.CapsuleID),
			Digest: map[string]string{DigestAlgorithm: manifest.CapsuleID},
		}},
		PredicateType: CapsulePredicateType,
		Predicate: CapsulePredicate{
			SchemaVersion: CapsulePredicateVersion, CapsuleID: manifest.CapsuleID,
			ManifestDigest: manifest.ManifestDigest, RegistryDigest: manifest.RegistryDigest,
			StudyID: manifest.StudyID, CellID: manifest.CellID,
			ScientificRoots: slices.Clone(manifest.ScientificRoots), ScientificComponents: report.ScientificComponents,
			PresentationComponents: report.PresentationComponents, VisibilityCounts: report.VisibilityCounts,
		},
	}
	if err := statement.Validate(manifest); err != nil {
		return InTotoStatement{}, err
	}
	return statement, nil
}

func (s InTotoStatement) Validate(manifest Manifest) error {
	if s.Type != InTotoStatementType || s.PredicateType != CapsulePredicateType || len(s.Subject) != 1 {
		return errors.New("in-toto capsule statement identity is invalid")
	}
	subject := s.Subject[0]
	if subject.Name != scientificSubjectName(manifest.CapsuleID) || len(subject.Digest) != 1 || subject.Digest[DigestAlgorithm] != manifest.CapsuleID {
		return errors.New("in-toto capsule subject does not bind scientific identity")
	}
	expected := verificationReport(manifest)
	predicate := s.Predicate
	if predicate.SchemaVersion != CapsulePredicateVersion || predicate.CapsuleID != manifest.CapsuleID ||
		predicate.ManifestDigest != manifest.ManifestDigest || predicate.RegistryDigest != manifest.RegistryDigest ||
		predicate.StudyID != manifest.StudyID || predicate.CellID != manifest.CellID ||
		!slices.Equal(predicate.ScientificRoots, manifest.ScientificRoots) ||
		predicate.ScientificComponents != expected.ScientificComponents || predicate.PresentationComponents != expected.PresentationComponents ||
		!equalCounts(predicate.VisibilityCounts, expected.VisibilityCounts) {
		return errors.New("in-toto capsule predicate does not match the manifest")
	}
	return nil
}

func EncodeStatement(statement InTotoStatement) ([]byte, error) {
	return protocol.CanonicalMarshal(statement)
}

func DecodeStatement(raw []byte, manifest Manifest) (InTotoStatement, error) {
	var statement InTotoStatement
	if err := protocol.DecodeStrict(raw, &statement); err != nil {
		return InTotoStatement{}, fmt.Errorf("decode in-toto capsule statement: %w", err)
	}
	canonical, err := EncodeStatement(statement)
	if err != nil || string(canonical) != string(raw) {
		return InTotoStatement{}, errors.New("in-toto capsule statement is not canonical JSON")
	}
	if err := statement.Validate(manifest); err != nil {
		return InTotoStatement{}, err
	}
	return statement, nil
}

func scientificSubjectName(capsuleID string) string {
	return "evalwitness-scientific-capsule/" + strings.ToLower(capsuleID)
}

func equalCounts(left, right map[string]int) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}
