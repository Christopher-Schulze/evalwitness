package stress

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/baseline"
	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

const (
	developmentCaseStudyID       = "first-listed-candidate-order-one-minimal"
	developmentCaseStudyDataRole = "adapter_development"
	developmentCaseStudyStatus   = "mechanism_demonstration"
	developmentCaseID            = "development-candidate-order-control"
	developmentTaskPath          = "scripts/tests/sample-task.txt"
	developmentTrajectoryAPath   = "scripts/tests/sample-traj-a.txt"
	developmentTrajectoryBPath   = "scripts/tests/sample-traj-b.txt"
	developmentLicensePath       = "LICENSE"
	developmentLicenseSPDX       = "MIT"
	developmentControlArmID      = "zero-cost-first-listed"
	developmentFixtureMaxBytes   = 1 << 20
	developmentOriginalLineUnits = 32
	developmentFinalLineUnits    = 2
	developmentClaimBoundary     = "provider-free development mechanism evidence only; zero empirical units; no verifier, provider, population, held-out, or global-minimum claim"
)

var developmentAllowedClaims = []string{
	"checked_in_fixture_reproduction",
	"deterministic_candidate_order_negative_control",
	"shared_one_minimal_reducer_mechanism",
}

var developmentForbiddenClaims = []string{
	"global_minimum",
	"held_out_confirmation",
	"model_or_provider_comparison",
	"population_generalization",
	"verifier_reliability_estimate",
}

type DevelopmentFixtureRef struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
}

type DevelopmentWitnessLine struct {
	UnitID      string `json:"unit_id"`
	FixturePath string `json:"fixture_path"`
	Line        int    `json:"line"`
	Content     string `json:"content"`
}

type DevelopmentControlObservation struct {
	ControlArmID         string           `json:"control_arm_id"`
	OriginalOrder        []string         `json:"original_order"`
	TransformedOrder     []string         `json:"transformed_order"`
	OriginalSelected     string           `json:"original_selected"`
	TransformedSelected  string           `json:"transformed_selected"`
	Constraint           ConstraintResult `json:"constraint"`
	Outcome              Outcome          `json:"outcome"`
	CompletedRepetitions int              `json:"completed_repetitions"`
	ProviderCalls        int              `json:"provider_calls"`
	NetworkRequired      bool             `json:"network_required"`
	Digest               string           `json:"digest"`
}

type DevelopmentCaseStudy struct {
	SchemaVersion      string                        `json:"schema_version"`
	CanonicalPolicy    string                        `json:"canonical_policy"`
	CaseStudyID        string                        `json:"case_study_id"`
	DataRole           string                        `json:"data_role"`
	Status             string                        `json:"status"`
	Task               DevelopmentFixtureRef         `json:"task"`
	TaskRequirement    string                        `json:"task_requirement"`
	Trajectories       []DevelopmentFixtureRef       `json:"trajectories"`
	License            DevelopmentFixtureRef         `json:"license"`
	LicenseSPDX        string                        `json:"license_spdx"`
	FixtureSetDigest   string                        `json:"fixture_set_digest"`
	Relation           Relation                      `json:"relation"`
	Observation        DevelopmentControlObservation `json:"observation"`
	Counterexample     Counterexample                `json:"counterexample"`
	FinalWitness       []DevelopmentWitnessLine      `json:"final_witness"`
	OriginalLineUnits  int                           `json:"original_line_units"`
	FinalLineUnits     int                           `json:"final_line_units"`
	AcceptedReductions int                           `json:"accepted_reductions"`
	ReductionPercent   float64                       `json:"reduction_percent"`
	EmpiricalUnits     int                           `json:"empirical_units"`
	ProviderCalls      int                           `json:"provider_calls"`
	NetworkRequired    bool                          `json:"network_required"`
	ClaimBoundary      string                        `json:"claim_boundary"`
	AllowedClaims      []string                      `json:"allowed_claims"`
	ForbiddenClaims    []string                      `json:"forbidden_claims"`
	Digest             string                        `json:"digest"`
}

func BuildDevelopmentCaseStudy(repositoryRoot string) (DevelopmentCaseStudy, error) {
	fixtures, err := readDevelopmentFixtures(repositoryRoot)
	if err != nil {
		return DevelopmentCaseStudy{}, err
	}
	return buildDevelopmentCaseStudy(fixtures)
}

func buildDevelopmentCaseStudy(fixtures developmentFixtures) (DevelopmentCaseStudy, error) {
	relation, err := sealV3CatalogRelation(mutation.FamilyCandidateOrderReversal, EstimandSensitivity, []preprocess.SourceFormat{preprocess.SourcePlainText})
	if err != nil {
		return DevelopmentCaseStudy{}, err
	}
	input, err := newDevelopmentCaseInput(fixtures.trajectoryA.raw, fixtures.trajectoryB.raw)
	if err != nil {
		return DevelopmentCaseStudy{}, err
	}
	privacyDigest, err := digestDocument(struct {
		LicenseSPDX string `json:"license_spdx"`
		LicenseSHA  string `json:"license_sha256"`
	}{developmentLicenseSPDX, fixtures.license.ref.SHA256})
	if err != nil {
		return DevelopmentCaseStudy{}, err
	}
	oracle := developmentCaseOracle{
		relation: relation, privacyPolicyDigest: privacyDigest,
		allowedLines: input.lineIndex(), requiredUnits: input.decisiveUnits(),
	}
	observation, err := buildDevelopmentControlObservation(relation, input)
	if err != nil {
		return DevelopmentCaseStudy{}, err
	}
	maximumEvaluations := len(input.Units()) * (len(input.Units()) + 1) / 2
	counterexample, err := ReduceCounterexample(context.Background(), ReductionRequest{
		RelationDigest: relation.Digest, SourceResultDigest: observation.Digest, CaseID: developmentCaseID,
		PrivacyPolicyDigest: privacyDigest, PublicReleaseAllowed: true,
		Input: input, Oracle: oracle, MaximumEvaluations: maximumEvaluations,
	})
	if err != nil {
		return DevelopmentCaseStudy{}, err
	}
	witness, err := developmentFinalWitness(counterexample.FinalUnits, input.lineIndex())
	if err != nil {
		return DevelopmentCaseStudy{}, err
	}
	fixtureSetDigest, err := digestDocument([]DevelopmentFixtureRef{fixtures.task.ref, fixtures.trajectoryA.ref, fixtures.trajectoryB.ref, fixtures.license.ref})
	if err != nil {
		return DevelopmentCaseStudy{}, err
	}
	originalUnits, finalUnits := len(counterexample.OriginalUnits), len(counterexample.FinalUnits)
	value := DevelopmentCaseStudy{
		SchemaVersion: DevelopmentCaseStudySchemaVersion, CanonicalPolicy: CanonicalPolicy,
		CaseStudyID: developmentCaseStudyID, DataRole: developmentCaseStudyDataRole, Status: developmentCaseStudyStatus,
		Task: fixtures.task.ref, TaskRequirement: strings.TrimSpace(string(fixtures.task.raw)),
		Trajectories: []DevelopmentFixtureRef{fixtures.trajectoryA.ref, fixtures.trajectoryB.ref},
		License:      fixtures.license.ref, LicenseSPDX: developmentLicenseSPDX, FixtureSetDigest: fixtureSetDigest,
		Relation: relation, Observation: observation, Counterexample: counterexample, FinalWitness: witness,
		OriginalLineUnits: originalUnits, FinalLineUnits: finalUnits, AcceptedReductions: counterexample.AcceptedReductions,
		ReductionPercent: 100 * float64(originalUnits-finalUnits) / float64(originalUnits),
		EmpiricalUnits:   0, ProviderCalls: 0, NetworkRequired: false, ClaimBoundary: developmentClaimBoundary,
		AllowedClaims: append([]string(nil), developmentAllowedClaims...), ForbiddenClaims: append([]string(nil), developmentForbiddenClaims...),
	}
	value.Digest, err = developmentCaseStudyDigest(value)
	if err != nil {
		return DevelopmentCaseStudy{}, err
	}
	if err := value.Validate(); err != nil {
		return DevelopmentCaseStudy{}, err
	}
	return value, nil
}

func (value DevelopmentCaseStudy) Validate() error {
	if value.SchemaVersion != DevelopmentCaseStudySchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		value.CaseStudyID != developmentCaseStudyID || value.DataRole != developmentCaseStudyDataRole || value.Status != developmentCaseStudyStatus ||
		strings.TrimSpace(value.TaskRequirement) == "" || value.TaskRequirement != strings.TrimSpace(value.TaskRequirement) ||
		value.LicenseSPDX != developmentLicenseSPDX || value.ClaimBoundary != developmentClaimBoundary || value.EmpiricalUnits != 0 ||
		value.ProviderCalls != 0 || value.NetworkRequired || !validDigest(value.FixtureSetDigest) || !validDigest(value.Digest) {
		return errors.New("stress development case study identity, execution boundary, or claim boundary is invalid")
	}
	if err := validateDevelopmentFixtureRefs(value); err != nil {
		return err
	}
	expectedRelation, err := sealV3CatalogRelation(mutation.FamilyCandidateOrderReversal, EstimandSensitivity, []preprocess.SourceFormat{preprocess.SourcePlainText})
	if err != nil || !reflect.DeepEqual(value.Relation, expectedRelation) {
		return errors.New("stress development case study relation is not the canonical candidate-order sensitivity contract")
	}
	if err := value.Observation.ValidateAgainst(value.Relation); err != nil {
		return err
	}
	if err := value.Counterexample.Validate(); err != nil {
		return err
	}
	if value.Counterexample.RelationDigest != value.Relation.Digest || value.Counterexample.SourceResultDigest != value.Observation.Digest ||
		value.Counterexample.CaseID != developmentCaseID || !value.Counterexample.PublicReleaseAllowed ||
		value.OriginalLineUnits != len(value.Counterexample.OriginalUnits) || value.FinalLineUnits != len(value.Counterexample.FinalUnits) ||
		value.AcceptedReductions != value.Counterexample.AcceptedReductions || value.OriginalLineUnits != developmentOriginalLineUnits ||
		value.FinalLineUnits != developmentFinalLineUnits || value.AcceptedReductions != developmentOriginalLineUnits-developmentFinalLineUnits ||
		value.ReductionPercent != 100*float64(value.OriginalLineUnits-value.FinalLineUnits)/float64(value.OriginalLineUnits) {
		return errors.New("stress development case study reduction summary or counterexample binding is invalid")
	}
	if err := validateDevelopmentWitness(value.FinalWitness, value.Counterexample.FinalUnits); err != nil {
		return err
	}
	if !slices.Equal(value.AllowedClaims, developmentAllowedClaims) || !slices.Equal(value.ForbiddenClaims, developmentForbiddenClaims) {
		return errors.New("stress development case study claim surface is invalid")
	}
	expected, err := developmentCaseStudyDigest(value)
	if err != nil || value.Digest != expected {
		return errors.New("stress development case study digest is invalid")
	}
	return nil
}

func (value DevelopmentCaseStudy) ValidateAgainstRepository(repositoryRoot string) error {
	if err := value.Validate(); err != nil {
		return err
	}
	want, err := BuildDevelopmentCaseStudy(repositoryRoot)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, want) {
		return errors.New("stress development case study differs from the repository fixtures or canonical reduction")
	}
	return nil
}

func (value DevelopmentControlObservation) ValidateAgainst(relation Relation) error {
	if value.ControlArmID != developmentControlArmID || !slices.Equal(value.OriginalOrder, []string{"trajectory-a", "trajectory-b"}) ||
		!slices.Equal(value.TransformedOrder, []string{"trajectory-b", "trajectory-a"}) || value.OriginalSelected != "trajectory-a" ||
		value.TransformedSelected != "trajectory-b" || value.Outcome != OutcomeViolated || value.CompletedRepetitions != relation.Repeat.MaximumRepetitions ||
		value.ProviderCalls != 0 || value.NetworkRequired || value.Constraint.Status != ConstraintViolated || !validDigest(value.Digest) {
		return errors.New("stress development control observation does not prove the first-listed order violation")
	}
	want, err := buildDevelopmentControlObservationFromSelections(relation, value.OriginalSelected, value.TransformedSelected)
	if err != nil || !reflect.DeepEqual(value, want) {
		return errors.New("stress development control observation does not reproduce from the canonical control")
	}
	return nil
}

type developmentFixture struct {
	ref DevelopmentFixtureRef
	raw []byte
}

type developmentFixtures struct {
	task        developmentFixture
	trajectoryA developmentFixture
	trajectoryB developmentFixture
	license     developmentFixture
}

func readDevelopmentFixtures(repositoryRoot string) (fixtures developmentFixtures, returnErr error) {
	root, err := os.OpenRoot(repositoryRoot)
	if err != nil {
		return developmentFixtures{}, fmt.Errorf("open stress development fixture root: %w", err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("close stress development fixture root: %w", closeErr)
		}
	}()
	read := func(path string) (developmentFixture, error) {
		file, openErr := root.Open(path)
		if openErr != nil {
			return developmentFixture{}, fmt.Errorf("open stress development fixture %q: %w", path, openErr)
		}
		info, statErr := file.Stat()
		if statErr != nil {
			_ = file.Close()
			return developmentFixture{}, fmt.Errorf("stat stress development fixture %q: %w", path, statErr)
		}
		if !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > developmentFixtureMaxBytes {
			_ = file.Close()
			return developmentFixture{}, fmt.Errorf("stress development fixture %q is not one bounded regular file", path)
		}
		raw, readErr := io.ReadAll(io.LimitReader(file, developmentFixtureMaxBytes+1))
		closeErr := file.Close()
		if readErr != nil || closeErr != nil {
			return developmentFixture{}, fmt.Errorf("read stress development fixture %q: %w", path, errors.Join(readErr, closeErr))
		}
		if len(raw) == 0 || len(raw) > developmentFixtureMaxBytes || int64(len(raw)) != info.Size() {
			return developmentFixture{}, fmt.Errorf("stress development fixture %q changed size while it was read", path)
		}
		digest := sha256.Sum256(raw)
		return developmentFixture{ref: DevelopmentFixtureRef{Path: path, SHA256: hex.EncodeToString(digest[:]), Bytes: len(raw)}, raw: raw}, nil
	}
	task, err := read(developmentTaskPath)
	if err != nil {
		return developmentFixtures{}, err
	}
	trajectoryA, err := read(developmentTrajectoryAPath)
	if err != nil {
		return developmentFixtures{}, err
	}
	trajectoryB, err := read(developmentTrajectoryBPath)
	if err != nil {
		return developmentFixtures{}, err
	}
	license, err := read(developmentLicensePath)
	if err != nil {
		return developmentFixtures{}, err
	}
	if !strings.HasPrefix(string(license.raw), "MIT License\n") {
		return developmentFixtures{}, errors.New("stress development fixture license is not the repository MIT license")
	}
	return developmentFixtures{task: task, trajectoryA: trajectoryA, trajectoryB: trajectoryB, license: license}, nil
}

type developmentCaseLine struct {
	UnitID      string `json:"unit_id"`
	Trajectory  string `json:"trajectory"`
	FixturePath string `json:"fixture_path"`
	Line        int    `json:"line"`
	Content     string `json:"content"`
}

type developmentCaseInput struct {
	Lines []developmentCaseLine `json:"lines"`
}

func newDevelopmentCaseInput(trajectoryA, trajectoryB []byte) (developmentCaseInput, error) {
	lines := append(developmentLines("a", developmentTrajectoryAPath, trajectoryA), developmentLines("b", developmentTrajectoryBPath, trajectoryB)...)
	input := developmentCaseInput{Lines: lines}
	if len(input.Lines) == 0 || len(input.decisiveUnits()) != 2 {
		return developmentCaseInput{}, errors.New("stress development fixtures do not contain both decisive output lines")
	}
	return input, nil
}

func developmentLines(trajectory, path string, raw []byte) []developmentCaseLine {
	values := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	result := make([]developmentCaseLine, len(values))
	for index, content := range values {
		result[index] = developmentCaseLine{
			UnitID: fmt.Sprintf("trajectory-%s-line-%03d", trajectory, index+1), Trajectory: trajectory,
			FixturePath: path, Line: index + 1, Content: content,
		}
	}
	return result
}

func (input developmentCaseInput) CanonicalBytes() ([]byte, error) {
	return encodeCanonical(input)
}

func (input developmentCaseInput) Units() []ReductionUnit {
	result := make([]ReductionUnit, len(input.Lines))
	for index, line := range input.Lines {
		result[index] = ReductionUnit{Kind: "fixture_line", ID: line.UnitID}
	}
	return result
}

func (input developmentCaseInput) Remove(removed ReductionUnit) (ReducibleInput, error) {
	if removed.Kind != "fixture_line" {
		return nil, errors.New("stress development reducer accepts only fixture-line units")
	}
	result := developmentCaseInput{Lines: make([]developmentCaseLine, 0, len(input.Lines)-1)}
	found := false
	for _, line := range input.Lines {
		if line.UnitID == removed.ID {
			found = true
			continue
		}
		result.Lines = append(result.Lines, line)
	}
	if !found {
		return nil, errors.New("stress development reduction unit is not retained")
	}
	return result, nil
}

func (input developmentCaseInput) lineIndex() map[string]developmentCaseLine {
	result := make(map[string]developmentCaseLine, len(input.Lines))
	for _, line := range input.Lines {
		result[line.UnitID] = line
	}
	return result
}

func (input developmentCaseInput) decisiveUnits() []ReductionUnit {
	result := make([]ReductionUnit, 0, 2)
	for _, line := range input.Lines {
		if line.Trajectory == "a" && line.Content == "5" || line.Trajectory == "b" && line.Content == "-1" {
			result = append(result, ReductionUnit{Kind: "fixture_line", ID: line.UnitID})
		}
	}
	sort.Slice(result, func(left, right int) bool { return compareReductionUnits(result[left], result[right]) < 0 })
	return result
}

type developmentCaseOracle struct {
	relation            Relation
	privacyPolicyDigest string
	allowedLines        map[string]developmentCaseLine
	requiredUnits       []ReductionUnit
}

func (oracle developmentCaseOracle) Evaluate(_ context.Context, candidate ReducibleInput) (ReductionObservation, error) {
	input, ok := candidate.(developmentCaseInput)
	if !ok {
		return ReductionObservation{}, errors.New("stress development oracle received another input type")
	}
	privacyRevalidated := true
	for _, line := range input.Lines {
		allowed, exists := oracle.allowedLines[line.UnitID]
		if !exists || allowed != line {
			privacyRevalidated = false
			break
		}
	}
	retained := make(map[ReductionUnit]struct{}, len(input.Lines))
	for _, unit := range input.Units() {
		retained[unit] = struct{}{}
	}
	relationRevalidated := true
	for _, required := range oracle.requiredUnits {
		if _, exists := retained[required]; !exists {
			relationRevalidated = false
		}
	}
	originalSelected, transformedSelected := developmentSelections(input)
	constraint, err := EvaluateConstraint(oracle.relation.Constraints[0], nil, nil, originalSelected, transformedSelected)
	if err != nil {
		return ReductionObservation{}, err
	}
	inputDigest, err := reducibleInputDigest(input)
	if err != nil {
		return ReductionObservation{}, err
	}
	relationProof, err := digestDocument(struct {
		RelationDigest      string `json:"relation_digest"`
		InputDigest         string `json:"input_digest"`
		OriginalSelected    string `json:"original_selected"`
		TransformedSelected string `json:"transformed_selected"`
	}{oracle.relation.Digest, inputDigest, originalSelected, transformedSelected})
	if err != nil {
		return ReductionObservation{}, err
	}
	unitIDs := make([]string, 0, len(input.Lines))
	for _, line := range input.Lines {
		unitIDs = append(unitIDs, line.UnitID)
	}
	privacyProof, err := digestDocument(struct {
		PolicyDigest string   `json:"policy_digest"`
		UnitIDs      []string `json:"unit_ids"`
	}{oracle.privacyPolicyDigest, unitIDs})
	if err != nil {
		return ReductionObservation{}, err
	}
	replayResult, err := digestDocument(struct {
		ControlArmID         string           `json:"control_arm_id"`
		Constraint           ConstraintResult `json:"constraint"`
		CompletedRepetitions int              `json:"completed_repetitions"`
	}{developmentControlArmID, constraint, oracle.relation.Repeat.MaximumRepetitions})
	if err != nil {
		return ReductionObservation{}, err
	}
	return SealReductionObservation(ReductionObservation{
		RelationDigest: oracle.relation.Digest, PrivacyPolicyDigest: oracle.privacyPolicyDigest,
		RelationRevalidated: relationRevalidated, PrivacyRevalidated: privacyRevalidated,
		ViolationPreserved:  relationRevalidated && privacyRevalidated && constraint.Status == ConstraintViolated,
		RelationProofDigest: relationProof, PrivacyProofDigest: privacyProof, ReplayResultDigest: replayResult,
	})
}

func buildDevelopmentControlObservation(relation Relation, input developmentCaseInput) (DevelopmentControlObservation, error) {
	originalSelected, transformedSelected := developmentSelections(input)
	return buildDevelopmentControlObservationFromSelections(relation, originalSelected, transformedSelected)
}

func buildDevelopmentControlObservationFromSelections(relation Relation, originalSelected, transformedSelected string) (DevelopmentControlObservation, error) {
	constraint, err := EvaluateConstraint(relation.Constraints[0], nil, nil, originalSelected, transformedSelected)
	if err != nil {
		return DevelopmentControlObservation{}, err
	}
	value := DevelopmentControlObservation{
		ControlArmID: developmentControlArmID, OriginalOrder: []string{"trajectory-a", "trajectory-b"},
		TransformedOrder: []string{"trajectory-b", "trajectory-a"}, OriginalSelected: originalSelected,
		TransformedSelected: transformedSelected, Constraint: constraint, Outcome: outcomeForConstraintResults([]ConstraintResult{constraint}),
		CompletedRepetitions: relation.Repeat.MaximumRepetitions, ProviderCalls: 0, NetworkRequired: false,
	}
	value.Digest, err = developmentControlObservationDigest(value)
	if err != nil {
		return DevelopmentControlObservation{}, err
	}
	return value, nil
}

func developmentSelections(input developmentCaseInput) (string, string) {
	features := map[string]baseline.Candidate{"trajectory-a": {}, "trajectory-b": {}}
	for _, line := range input.Lines {
		candidate := features["trajectory-"+line.Trajectory]
		candidate.Steps++
		candidate.TraceBytes += len(line.Content)
		candidate.ErrorWords += baseline.CountErrorWords(line.Content)
		features["trajectory-"+line.Trajectory] = candidate
	}
	original := []string{"trajectory-a", "trajectory-b"}
	transformed := []string{"trajectory-b", "trajectory-a"}
	originalIndex := baseline.FirstListed.Pick([]baseline.Candidate{features[original[0]], features[original[1]]})
	transformedIndex := baseline.FirstListed.Pick([]baseline.Candidate{features[transformed[0]], features[transformed[1]]})
	return original[originalIndex], transformed[transformedIndex]
}

func developmentFinalWitness(units []ReductionUnit, lines map[string]developmentCaseLine) ([]DevelopmentWitnessLine, error) {
	result := make([]DevelopmentWitnessLine, len(units))
	for index, unit := range units {
		line, exists := lines[unit.ID]
		if !exists || unit.Kind != "fixture_line" {
			return nil, errors.New("stress development counterexample final unit is not one fixture line")
		}
		result[index] = DevelopmentWitnessLine{UnitID: unit.ID, FixturePath: line.FixturePath, Line: line.Line, Content: line.Content}
	}
	return result, nil
}

func validateDevelopmentFixtureRefs(value DevelopmentCaseStudy) error {
	wantPaths := []string{developmentTaskPath, developmentTrajectoryAPath, developmentTrajectoryBPath, developmentLicensePath}
	refs := []DevelopmentFixtureRef{value.Task}
	refs = append(refs, value.Trajectories...)
	refs = append(refs, value.License)
	if len(refs) != len(wantPaths) {
		return errors.New("stress development case study fixture inventory is incomplete")
	}
	for index, ref := range refs {
		if ref.Path != wantPaths[index] || !validDigest(ref.SHA256) || ref.Bytes <= 0 || ref.Bytes > developmentFixtureMaxBytes {
			return errors.New("stress development case study fixture identity is invalid")
		}
	}
	digest, err := digestDocument(refs)
	if err != nil || digest != value.FixtureSetDigest {
		return errors.New("stress development case study fixture-set digest is invalid")
	}
	return nil
}

func validateDevelopmentWitness(witness []DevelopmentWitnessLine, units []ReductionUnit) error {
	if len(witness) != len(units) || len(witness) != 2 {
		return errors.New("stress development case study must reduce to exactly two decisive fixture lines")
	}
	for index, line := range witness {
		if line.UnitID != units[index].ID || line.Line <= 0 || strings.TrimSpace(line.FixturePath) == "" ||
			line.UnitID == "trajectory-a-line-013" && (line.FixturePath != developmentTrajectoryAPath || line.Line != 13 || line.Content != "5") ||
			line.UnitID == "trajectory-b-line-013" && (line.FixturePath != developmentTrajectoryBPath || line.Line != 13 || line.Content != "-1") {
			return errors.New("stress development case study final witness does not match the decisive fixture lines")
		}
	}
	if witness[0].UnitID != "trajectory-a-line-013" || witness[1].UnitID != "trajectory-b-line-013" {
		return errors.New("stress development case study final witness contains unexpected units")
	}
	return nil
}

func developmentCaseStudyDigest(value DevelopmentCaseStudy) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}

func developmentControlObservationDigest(value DevelopmentControlObservation) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}

func encodeCanonical(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode stress canonical document: %w", err)
	}
	return encoded, nil
}
