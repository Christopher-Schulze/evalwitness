package mutation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

func (manifest Manifest) Validate() error {
	relationContract, evidenceBoundary, requiresFirewall, supported := contractVersionsForProgram(manifest.Program.Version)
	if manifest.SchemaVersion != ManifestSchemaVersion || manifest.CanonicalPolicy != CanonicalPolicy || !supported || manifest.RelationContractVersion != relationContract {
		return errors.New("mutation schema, canonical policy, or relation contract is unsupported")
	}
	if missing(manifest.CorpusVersion, manifest.MutationID, manifest.SplitGroupID) {
		return errors.New("mutation corpus version, identity, and split group are required")
	}
	definition, exists := DefinitionFor(manifest.Program.Family)
	if !exists || manifest.Program.Operator != operatorForProgram(manifest.Program.Version, definition) || manifest.Class != definition.Class {
		return errors.New("mutation program, family, operator, or intervention class is invalid")
	}
	if requiresFirewall {
		if !validDigest(manifest.ConstructFirewallDigest) {
			return errors.New("v2 mutation manifest requires a construct firewall digest")
		}
	} else if manifest.ConstructFirewallDigest != "" {
		return errors.New("v1 mutation manifest cannot bind a construct firewall")
	}
	if manifest.ExpectedRelation != definition.Relation && manifest.ExpectedRelation != RelationAmbiguous {
		return errors.New("expected relation differs from the registered mutation family")
	}
	if err := validateSource(manifest.Source); err != nil {
		return err
	}
	if err := validateSourceShape(manifest.Source, definition.PairLevel, manifest.Privacy.PublicReleaseAllowed); err != nil {
		return err
	}
	if manifest.Source.TrajectoryDigest != manifest.OriginalTrajectoryDigest || !validDigest(manifest.MutatedTrajectoryDigest) {
		return errors.New("mutation trajectory identities are inconsistent or invalid")
	}
	if manifest.OriginalTrajectoryDigest == manifest.MutatedTrajectoryDigest && !definition.PairLevel {
		return errors.New("trajectory mutation did not change the canonical trajectory identity")
	}
	if err := validateProgram(manifest.Program); err != nil {
		return err
	}
	if err := validateAffected(manifest.Affected, definition.PairLevel); err != nil {
		return err
	}
	if err := validateValidator(manifest.Validator, definition); err != nil {
		return err
	}
	if err := validateOutcomeProof(manifest.OutcomeProof, manifest.Validator, definition, manifest.ExpectedRelation); err != nil {
		return err
	}
	if err := validatePreservation(manifest.Preservation, evidenceBoundary); err != nil {
		return err
	}
	if err := manifest.Witness.Validate(); err != nil {
		return err
	}
	if manifest.Witness.ValidatorID != manifest.Validator.ID || manifest.Witness.ValidatorVersion != manifest.Validator.Version || manifest.Witness.Relation != manifest.ExpectedRelation {
		return errors.New("mutation witness does not bind the validator or expected relation")
	}
	if manifest.ExpectedRelation == RelationAmbiguous && manifest.Witness.LabelState != LabelAmbiguous {
		return errors.New("ambiguous relation requires an ambiguous witness")
	}
	if definition.RequiresGoldProof && manifest.ExpectedRelation != RelationAmbiguous && manifest.Witness.LabelState != LabelProven {
		return errors.New("semantic-quality mutation requires a proven validator witness")
	}
	if err := validateLicense(manifest.License, manifest.Privacy); err != nil {
		return err
	}
	if err := validateReview(manifest.Review, manifest.Witness.LabelState); err != nil {
		return err
	}
	expected, err := manifest.digest()
	if err != nil {
		return err
	}
	if manifest.Digest != expected || manifest.MutationID != "mutation-"+expected {
		return errors.New("mutation identity does not match canonical content")
	}
	return nil
}

func validateSource(source SourceRef) error {
	if missing(source.TaskID, source.RepositoryID, source.SourceFamily, string(source.SourceFormat), source.SourceLocation,
		source.SourceRevision, source.Outcome.Kind, source.Outcome.Value) {
		return errors.New("mutation source identity or outcome is incomplete")
	}
	if !validDigest(source.SourceDigest) || !validDigest(source.TrajectoryDigest) || !validDigest(source.Outcome.WitnessDigest) {
		return errors.New("mutation source or outcome witness digest is invalid")
	}
	if !validSourceFormat(source.SourceFormat) {
		return errors.New("mutation source format is unsupported")
	}
	for _, digest := range source.PairedTrajectoryDigests {
		if !validDigest(digest) {
			return errors.New("paired mutation source digest is invalid")
		}
	}
	return nil
}

func validateSourceShape(source SourceRef, pairLevel, publicRelease bool) error {
	if pairLevel {
		if len(source.PairedTrajectoryDigests) != 2 || source.PairedTrajectoryDigests[0] == source.PairedTrajectoryDigests[1] {
			return errors.New("pair-level mutation requires two distinct ordered trajectory digests")
		}
		pairDigest, err := digestJSON(source.PairedTrajectoryDigests)
		if err != nil {
			return err
		}
		if source.TrajectoryDigest != pairDigest {
			return errors.New("pair-level source digest does not bind the ordered trajectory pair")
		}
	} else if len(source.PairedTrajectoryDigests) != 0 {
		return errors.New("trajectory-local mutation cannot declare paired trajectory digests")
	}
	if publicRelease && unsafePublicLocation(source.SourceLocation) {
		return errors.New("public mutation source location must be a repository-relative reference")
	}
	return nil
}

func validateProgram(program Program) error {
	if strings.TrimSpace(program.Seed) == "" {
		return errors.New("mutation seed is required")
	}
	if err := validateUniqueNamedValues("mutation parameters", program.Parameters); err != nil {
		return err
	}
	if !slices.IsSortedFunc(program.Parameters, func(left, right NamedValue) int {
		return strings.Compare(left.Name, right.Name)
	}) {
		return errors.New("mutation parameters must be sorted by name")
	}
	return nil
}

func validateAffected(affected AffectedSurface, pairLevel bool) error {
	if pairLevel {
		if len(affected.EventIDs) != 0 || len(affected.FieldPaths) != 0 || len(affected.FileAliases) != 0 {
			return errors.New("pair-level mutation cannot claim trajectory-local affected fields")
		}
		return nil
	}
	if len(affected.EventIDs) == 0 {
		return errors.New("trajectory mutation requires at least one affected event")
	}
	if err := validateUniqueSortedStrings("affected event IDs", affected.EventIDs); err != nil {
		return err
	}
	paths := make([]string, len(affected.FieldPaths))
	for index, path := range affected.FieldPaths {
		paths[index] = string(path)
		remainder := strings.TrimPrefix(paths[index], "/events/")
		eventID, _, found := strings.Cut(remainder, "/")
		if remainder == paths[index] || !found {
			return fmt.Errorf("affected field path %q is not event-addressed", path)
		}
		if !slices.Contains(affected.EventIDs, eventID) {
			return fmt.Errorf("affected field path %q does not address a declared event", path)
		}
	}
	if err := validateUniqueSortedStrings("affected field paths", paths); err != nil {
		return err
	}
	return validateUniqueSortedStrings("affected file aliases", affected.FileAliases)
}

func validateValidator(validator ValidatorSpec, definition Definition) error {
	if missing(validator.ID, validator.Version) || !validDigest(validator.ContractDigest) || validator.TimeoutMillis <= 0 || validator.TimeoutMillis > 300_000 || validator.MaximumOutputBytes <= 0 || validator.MaximumOutputBytes > 16*1024*1024 {
		return errors.New("validator identity, contract, timeout, or output bound is invalid")
	}
	if !slices.Contains([]ValidationKind{ValidationFormal, ValidationHermetic, ValidationPreservation}, validator.Kind) {
		return errors.New("validator kind is invalid")
	}
	if definition.RequiresGoldProof && validator.Kind != ValidationFormal && validator.Kind != ValidationHermetic {
		return errors.New("semantic-quality mutation requires a formal or hermetic validator")
	}
	return nil
}

func validateOutcomeProof(proof *OutcomeProof, validator ValidatorSpec, definition Definition, relation Relation) error {
	if !definition.RequiresGoldProof || relation == RelationAmbiguous {
		if proof != nil {
			return errors.New("outcome proof is only valid for non-ambiguous semantic-quality gold labels")
		}
		return nil
	}
	if proof == nil || !slices.Contains([]ValidationKind{ValidationFormal, ValidationHermetic}, proof.Mechanism) ||
		!validDigest(proof.ContractDigest) || !validDigest(proof.WitnessDigest) || !proof.IndependentOfTrace ||
		!proof.OriginalPassed || proof.MutatedPassed {
		return errors.New("semantic-quality gold label requires independent pass-to-fail outcome proof")
	}
	if proof.Mechanism != validator.Kind || proof.ContractDigest != validator.ContractDigest {
		return errors.New("semantic-quality outcome proof does not bind the declared validator contract")
	}
	return nil
}

func validatePreservation(record PreservationRecord, expectedBoundary string) error {
	if record.BoundaryVersion != expectedBoundary {
		return errors.New("evidence preservation boundary is unsupported")
	}
	for _, digest := range []string{record.QualityProjectionBefore, record.QualityProjectionAfter, record.EvidenceProjectionBefore,
		record.EvidenceProjectionAfter, record.CausalGraphBefore, record.CausalGraphAfter} {
		if !validDigest(digest) {
			return errors.New("preservation projection digest is invalid")
		}
	}
	if err := validateUniqueSortedStrings("changed evidence groups", record.ChangedGroups); err != nil {
		return err
	}
	if err := validateUniqueSortedStrings("preserved evidence groups", record.PreservedGroups); err != nil {
		return err
	}
	return validateUniqueSortedStrings("ambiguity reasons", record.AmbiguityReasons)
}

func (witness Witness) Validate() error {
	if witness.SchemaVersion != WitnessSchemaVersion || missing(witness.ValidatorID, witness.ValidatorVersion) || !validRelation(witness.Relation) || !slices.Contains([]LabelState{LabelProven, LabelAmbiguous, LabelInvalid}, witness.LabelState) || len(witness.Checks) == 0 {
		return errors.New("mutation witness identity, relation, label, or checks are invalid")
	}
	seen := make(map[string]struct{}, len(witness.Checks))
	allPassed := true
	for index, check := range witness.Checks {
		if missing(check.Name, check.Expected, check.Observed) {
			return fmt.Errorf("mutation witness check %d is incomplete", index)
		}
		if _, duplicate := seen[check.Name]; duplicate {
			return fmt.Errorf("duplicate mutation witness check %q", check.Name)
		}
		seen[check.Name] = struct{}{}
		allPassed = allPassed && check.Passed
	}
	if witness.LabelState == LabelProven && !allPassed {
		return errors.New("proven witness contains a failed check")
	}
	expected, err := witness.digest()
	if err != nil {
		return err
	}
	if witness.Digest != expected {
		return errors.New("mutation witness digest is invalid")
	}
	return nil
}

func validateLicense(license LicenseMetadata, privacy PrivacyMetadata) error {
	if missing(license.SPDX, license.SourceURL, license.SourceRevision, license.Redistribution, license.Attribution,
		privacy.Classification) || !validDigest(privacy.RedactionPolicyDigest) {
		return errors.New("mutation license or privacy metadata is incomplete")
	}
	if !slices.Contains([]string{"permitted", "reference_only", "private_only"}, license.Redistribution) {
		return errors.New("mutation redistribution policy is invalid")
	}
	if !slices.Contains([]string{"public", "private", "restricted"}, privacy.Classification) {
		return errors.New("mutation privacy classification is invalid")
	}
	if privacy.PublicReleaseAllowed && privacy.Classification != "public" {
		return errors.New("only public-classified mutation metadata can be released")
	}
	if privacy.PublicReleaseAllowed && license.Redistribution == "private_only" {
		return errors.New("private-only source cannot authorize a public mutation artifact")
	}
	if privacy.PublicReleaseAllowed && filepath.IsAbs(license.SourceURL) {
		return errors.New("public mutation license metadata cannot contain a local source path")
	}
	return nil
}

func validateReview(review ReviewState, label LabelState) error {
	if missing(review.SamplingStratum) || !validDigest(review.BlindPacketDigest) {
		return errors.New("mutation blind-review stratum or packet digest is invalid")
	}
	if label == LabelAmbiguous && !review.Required {
		return errors.New("ambiguous mutation requires blind review")
	}
	return nil
}

func SealManifest(manifest Manifest) (Manifest, error) {
	manifest.Program.Version = MutationProgramVersionV1
	return sealManifestForContract(manifest, RelationContractVersionV1)
}

func SealManifestV2(manifest Manifest) (Manifest, error) {
	manifest.Program.Version = MutationProgramVersionV2
	return sealManifestForContract(manifest, RelationContractVersionV2)
}

func SealManifestV3(manifest Manifest) (Manifest, error) {
	manifest.Program.Version = MutationProgramVersionV3
	return sealManifestForContract(manifest, RelationContractVersionV3)
}

func sealManifestForContract(manifest Manifest, relationContract string) (Manifest, error) {
	manifest.SchemaVersion = ManifestSchemaVersion
	manifest.CanonicalPolicy = CanonicalPolicy
	manifest.RelationContractVersion = relationContract
	manifest.Digest = ""
	manifest.MutationID = ""
	digest, err := manifest.digest()
	if err != nil {
		return Manifest{}, err
	}
	manifest.Digest = digest
	manifest.MutationID = "mutation-" + digest
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func contractVersionsForProgram(programVersion string) (string, string, bool, bool) {
	switch programVersion {
	case MutationProgramVersionV1:
		return RelationContractVersionV1, EvidenceBoundaryVersionV1, false, true
	case MutationProgramVersionV2:
		return RelationContractVersionV2, EvidenceBoundaryVersionV2, true, true
	case MutationProgramVersionV3:
		return RelationContractVersionV3, EvidenceBoundaryVersionV3, true, true
	default:
		return "", "", false, false
	}
}

func operatorForProgram(programVersion string, definition Definition) string {
	if programVersion != MutationProgramVersionV2 && programVersion != MutationProgramVersionV3 {
		return definition.Operator
	}
	switch definition.Family {
	case FamilyTestEvidenceOmitted:
		if programVersion == MutationProgramVersionV3 {
			return "remove_directly_invoked_verification_evidence"
		}
		return "remove_verified_execution_evidence"
	case FamilyNeutralFormatting:
		if programVersion == MutationProgramVersionV3 {
			return "wrap_assistant_prose_preserving_tokens"
		}
		return "wrap_natural_prose_preserving_tokens"
	case FamilyCausalIndependentReorder:
		return "swap_transaction_independent_events"
	default:
		return definition.Operator
	}
}

func SealWitness(witness Witness) (Witness, error) {
	witness.SchemaVersion = WitnessSchemaVersion
	witness.Digest = ""
	digest, err := witness.digest()
	if err != nil {
		return Witness{}, err
	}
	witness.Digest = digest
	if err := witness.Validate(); err != nil {
		return Witness{}, err
	}
	return witness, nil
}

func (manifest Manifest) digest() (string, error) {
	manifest.Digest = ""
	manifest.MutationID = ""
	return digestJSON(manifest)
}

func (witness Witness) digest() (string, error) {
	witness.Digest = ""
	return digestJSON(witness)
}

func digestJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func validRelation(relation Relation) bool {
	return slices.Contains([]Relation{RelationOriginalBetter, RelationQualityEqual, RelationQualityEqualEvidenceLow, RelationVerifiedOutcomeWins, RelationNoControlEffect, RelationAmbiguous}, relation)
}

func validSourceFormat(format preprocess.SourceFormat) bool {
	return slices.Contains([]preprocess.SourceFormat{
		preprocess.SourcePlainText,
		preprocess.SourceClaudeCode,
		preprocess.SourceCodexRollout,
		preprocess.SourceOpenCode,
		preprocess.SourceTerminalBench,
		preprocess.SourceSWEbench,
		preprocess.SourceOTLPJSON,
		preprocess.SourceAgentTrace,
	}, format)
}

func unsafePublicLocation(location string) bool {
	if filepath.IsAbs(location) || strings.HasPrefix(location, "~") || strings.Contains(location, "\\") {
		return true
	}
	for _, segment := range strings.Split(location, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func validDigest(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validateUniqueSortedStrings(name string, values []string) error {
	if !sort.StringsAreSorted(values) {
		return fmt.Errorf("%s must be sorted", name)
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return fmt.Errorf("%s contains an empty value", name)
		}
		if _, duplicate := seen[value]; duplicate {
			return fmt.Errorf("%s contains duplicate %q", name, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateUniqueNamedValues(name string, values []NamedValue) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if missing(value.Name, value.Value) {
			return fmt.Errorf("%s contains an incomplete value", name)
		}
		if _, duplicate := seen[value.Name]; duplicate {
			return fmt.Errorf("%s contains duplicate name %q", name, value.Name)
		}
		seen[value.Name] = struct{}{}
	}
	return nil
}

func missing(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return true
		}
	}
	return false
}
