package stress

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
)

const (
	developmentChallengeID       = "candidate-order-one-minimal-portable-challenge"
	developmentChallengeStatus   = "self_contained_mechanism_challenge"
	developmentChallengeMaxBytes = 1 << 20
	developmentCaseStudyDigestV1 = "b4a7adb23505a88ee0ab79c73957bce5fbd15539a0e5e687590719d3ec4ee84b"
	developmentChallengeClaim    = "the self-contained verifier reproduces the canonical provider-free candidate-order violation, complete reduction trace, and two-line one-minimal witness from the embedded licensed fixture bytes without repository access"
)

type DevelopmentChallengeFixture struct {
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	Bytes      int    `json:"bytes"`
	ContentHex string `json:"content_hex"`
}

type DevelopmentChallenge struct {
	SchemaVersion     string                          `json:"schema_version"`
	CanonicalPolicy   string                          `json:"canonical_policy"`
	ChallengeID       string                          `json:"challenge_id"`
	Status            string                          `json:"status"`
	Fixtures          []DevelopmentChallengeFixture   `json:"fixtures"`
	FixtureSetDigest  string                          `json:"fixture_set_digest"`
	Expected          DevelopmentChallengeExpectation `json:"expected"`
	EmpiricalUnits    int                             `json:"empirical_units"`
	ProviderCalls     int                             `json:"provider_calls"`
	NetworkRequired   bool                            `json:"network_required"`
	SupportedClaim    string                          `json:"supported_claim"`
	UnsupportedClaims []string                        `json:"unsupported_claims"`
	Digest            string                          `json:"digest"`
}

type DevelopmentChallengeExpectation struct {
	CaseDigest             string   `json:"case_digest"`
	Outcome                Outcome  `json:"outcome"`
	OriginalLineUnits      int      `json:"original_line_units"`
	FinalLineUnits         int      `json:"final_line_units"`
	ReductionAttempts      int      `json:"reduction_attempts"`
	AcceptedReductions     int      `json:"accepted_reductions"`
	FinalRejectionAttempts int      `json:"final_rejection_attempts"`
	FinalWitnessUnitIDs    []string `json:"final_witness_unit_ids"`
	ClaimBoundary          string   `json:"claim_boundary"`
	ProviderCalls          int      `json:"provider_calls"`
	NetworkRequired        bool     `json:"network_required"`
}

type DevelopmentChallengeReceiptVerification struct {
	Valid           bool   `json:"valid"`
	ReceiptDigest   string `json:"receipt_digest"`
	ChallengeDigest string `json:"challenge_digest"`
	GuardCount      int    `json:"guard_count"`
	GuardsPassed    int    `json:"guards_passed"`
	EmpiricalUnits  int    `json:"empirical_units"`
	ProviderCalls   int    `json:"provider_calls"`
	NetworkRequired bool   `json:"network_required"`
}

type DevelopmentChallengeReceipt struct {
	SchemaVersion          string                             `json:"schema_version"`
	CanonicalPolicy        string                             `json:"canonical_policy"`
	ChallengeDigest        string                             `json:"challenge_digest"`
	ExpectedCaseDigest     string                             `json:"expected_case_digest"`
	ReproducedCaseDigest   string                             `json:"reproduced_case_digest"`
	FixtureSetDigest       string                             `json:"fixture_set_digest"`
	FixturesVerified       int                                `json:"fixtures_verified"`
	OriginalLineUnits      int                                `json:"original_line_units"`
	FinalLineUnits         int                                `json:"final_line_units"`
	ReductionAttempts      int                                `json:"reduction_attempts"`
	AcceptedReductions     int                                `json:"accepted_reductions"`
	FinalRejectionAttempts int                                `json:"final_rejection_attempts"`
	ViolationReproduced    bool                               `json:"violation_reproduced"`
	OneMinimalReproduced   bool                               `json:"one_minimal_reproduced"`
	RepositoryRequired     bool                               `json:"repository_required"`
	EmpiricalUnits         int                                `json:"empirical_units"`
	ProviderCalls          int                                `json:"provider_calls"`
	NetworkRequired        bool                               `json:"network_required"`
	ClaimBoundaryPreserved bool                               `json:"claim_boundary_preserved"`
	Guards                 []DevelopmentChallengeGuardReceipt `json:"guards"`
	GuardCount             int                                `json:"guard_count"`
	GuardsPassed           int                                `json:"guards_passed"`
	Valid                  bool                               `json:"valid"`
	Digest                 string                             `json:"digest"`
}

type DevelopmentChallengeGuardReceipt struct {
	GuardID       string `json:"guard_id"`
	Category      string `json:"category"`
	Mutation      string `json:"mutation"`
	ExpectedGuard string `json:"expected_guard"`
	ObservedGuard string `json:"observed_guard"`
	Passed        bool   `json:"passed"`
}

var developmentChallengeUnsupportedClaims = []string{
	"global minimum counterexample",
	"held-out confirmation",
	"independent implementation replication",
	"model, provider, or verifier reliability",
	"population generalization",
}

func BuildDevelopmentChallenge(repositoryRoot string) (DevelopmentChallenge, error) {
	fixtures, err := readDevelopmentFixtures(repositoryRoot)
	if err != nil {
		return DevelopmentChallenge{}, err
	}
	value, err := buildDevelopmentChallenge(fixtures)
	if err != nil {
		return DevelopmentChallenge{}, err
	}
	return value, value.Validate()
}

func VerifyDevelopmentChallenge(value DevelopmentChallenge) (DevelopmentChallengeReceipt, error) {
	if err := value.Validate(); err != nil {
		return DevelopmentChallengeReceipt{}, err
	}
	fixtures, err := value.embeddedFixtures()
	if err != nil {
		return DevelopmentChallengeReceipt{}, err
	}
	reproduced, err := buildDevelopmentCaseStudy(fixtures)
	if err != nil {
		return DevelopmentChallengeReceipt{}, fmt.Errorf("reproduce stress development challenge: %w", err)
	}
	if err := value.Expected.validateReproduced(reproduced); err != nil {
		return DevelopmentChallengeReceipt{}, err
	}
	lastAccepted := -1
	for index, step := range reproduced.Counterexample.Steps {
		if step.Decision == ReductionAccepted {
			lastAccepted = index
		}
	}
	receipt := DevelopmentChallengeReceipt{
		SchemaVersion: DevelopmentChallengeReceiptSchemaVersion, CanonicalPolicy: CanonicalPolicy, ChallengeDigest: value.Digest,
		ExpectedCaseDigest: value.Expected.CaseDigest, ReproducedCaseDigest: reproduced.Digest,
		FixtureSetDigest: value.FixtureSetDigest, FixturesVerified: len(value.Fixtures),
		OriginalLineUnits: reproduced.OriginalLineUnits, FinalLineUnits: reproduced.FinalLineUnits,
		ReductionAttempts: len(reproduced.Counterexample.Steps), AcceptedReductions: reproduced.AcceptedReductions,
		FinalRejectionAttempts: len(reproduced.Counterexample.Steps) - lastAccepted - 1,
		ViolationReproduced:    reproduced.Observation.Outcome == OutcomeViolated,
		OneMinimalReproduced:   reproduced.Counterexample.Minimality == ReductionOneMinimal,
		RepositoryRequired:     false, EmpiricalUnits: 0, ProviderCalls: 0, NetworkRequired: false,
		ClaimBoundaryPreserved: reproduced.ClaimBoundary == developmentClaimBoundary,
		Valid:                  true,
	}
	receipt.Guards, err = runDevelopmentChallengeGuardSuite(value)
	if err != nil {
		return DevelopmentChallengeReceipt{}, err
	}
	receipt.GuardCount = len(receipt.Guards)
	for _, guard := range receipt.Guards {
		if guard.Passed {
			receipt.GuardsPassed++
		}
	}
	receipt.Digest, err = developmentChallengeReceiptDigest(receipt)
	if err != nil {
		return DevelopmentChallengeReceipt{}, err
	}
	if err := receipt.ValidateAgainst(value); err != nil {
		return DevelopmentChallengeReceipt{}, err
	}
	return receipt, nil
}

func (value DevelopmentChallenge) Validate() error {
	if value.SchemaVersion != DevelopmentChallengeSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		value.ChallengeID != developmentChallengeID || value.Status != developmentChallengeStatus ||
		value.EmpiricalUnits != 0 || value.ProviderCalls != 0 || value.NetworkRequired || value.SupportedClaim != developmentChallengeClaim ||
		!slices.Equal(value.UnsupportedClaims, developmentChallengeUnsupportedClaims) || !validDigest(value.FixtureSetDigest) ||
		!validDigest(value.Digest) || len(value.Fixtures) != 4 {
		return errors.New("stress development challenge identity, execution, or claim boundary is invalid")
	}
	if err := value.Expected.Validate(); err != nil {
		return err
	}
	fixtures, err := value.embeddedFixtures()
	if err != nil {
		return err
	}
	refs := developmentFixtureRefs(fixtures)
	fixtureSetDigest, err := digestDocument(refs)
	if err != nil || fixtureSetDigest != value.FixtureSetDigest {
		return errors.New("stress development challenge embedded fixture set differs from its expected case")
	}
	expected, err := developmentChallengeDigest(value)
	if err != nil || expected != value.Digest {
		return errors.New("stress development challenge digest is invalid")
	}
	return nil
}

func (value DevelopmentChallengeReceipt) ValidateAgainst(challenge DevelopmentChallenge) error {
	if err := challenge.Validate(); err != nil {
		return err
	}
	if err := value.Validate(); err != nil {
		return err
	}
	if value.ChallengeDigest != challenge.Digest || value.ExpectedCaseDigest != challenge.Expected.CaseDigest ||
		value.ReproducedCaseDigest != challenge.Expected.CaseDigest || value.FixtureSetDigest != challenge.FixtureSetDigest ||
		value.FixturesVerified != len(challenge.Fixtures) || value.ReductionAttempts != challenge.Expected.ReductionAttempts ||
		value.AcceptedReductions != challenge.Expected.AcceptedReductions {
		return errors.New("stress development challenge receipt does not prove the complete portable reproduction")
	}
	return nil
}

func (value DevelopmentChallengeExpectation) Validate() error {
	if value.CaseDigest != developmentCaseStudyDigestV1 || value.Outcome != OutcomeViolated ||
		value.OriginalLineUnits != developmentOriginalLineUnits || value.FinalLineUnits != developmentFinalLineUnits ||
		value.ReductionAttempts != 53 || value.AcceptedReductions != developmentOriginalLineUnits-developmentFinalLineUnits ||
		value.FinalRejectionAttempts != developmentFinalLineUnits ||
		!slices.Equal(value.FinalWitnessUnitIDs, []string{"trajectory-a-line-013", "trajectory-b-line-013"}) ||
		value.ClaimBoundary != developmentClaimBoundary || value.ProviderCalls != 0 || value.NetworkRequired {
		return errors.New("stress development challenge expectation is invalid")
	}
	return nil
}

func (value DevelopmentChallengeExpectation) validateReproduced(reproduced DevelopmentCaseStudy) error {
	if err := value.Validate(); err != nil {
		return err
	}
	lastAccepted := -1
	for index, step := range reproduced.Counterexample.Steps {
		if step.Decision == ReductionAccepted {
			lastAccepted = index
		}
	}
	witnessIDs := make([]string, len(reproduced.FinalWitness))
	for index, witness := range reproduced.FinalWitness {
		witnessIDs[index] = witness.UnitID
	}
	if reproduced.Digest != value.CaseDigest || reproduced.Observation.Outcome != value.Outcome ||
		reproduced.OriginalLineUnits != value.OriginalLineUnits || reproduced.FinalLineUnits != value.FinalLineUnits ||
		len(reproduced.Counterexample.Steps) != value.ReductionAttempts || reproduced.AcceptedReductions != value.AcceptedReductions ||
		len(reproduced.Counterexample.Steps)-lastAccepted-1 != value.FinalRejectionAttempts ||
		!slices.Equal(witnessIDs, value.FinalWitnessUnitIDs) || reproduced.ClaimBoundary != value.ClaimBoundary ||
		reproduced.ProviderCalls != value.ProviderCalls || reproduced.NetworkRequired != value.NetworkRequired {
		return errors.New("stress development challenge does not reproduce its expectation from embedded bytes")
	}
	return nil
}

func (value DevelopmentChallengeReceipt) Validate() error {
	if value.SchemaVersion != DevelopmentChallengeReceiptSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		!validDigest(value.ChallengeDigest) || !validDigest(value.ExpectedCaseDigest) ||
		value.ReproducedCaseDigest != value.ExpectedCaseDigest || !validDigest(value.FixtureSetDigest) || value.FixturesVerified != 4 ||
		value.OriginalLineUnits != developmentOriginalLineUnits || value.FinalLineUnits != developmentFinalLineUnits ||
		value.ReductionAttempts != 53 || value.AcceptedReductions != developmentOriginalLineUnits-developmentFinalLineUnits ||
		value.FinalRejectionAttempts != developmentFinalLineUnits || !value.ViolationReproduced || !value.OneMinimalReproduced ||
		value.RepositoryRequired || value.EmpiricalUnits != 0 || value.ProviderCalls != 0 || value.NetworkRequired || !value.ClaimBoundaryPreserved || !value.Valid ||
		value.GuardCount != len(developmentChallengeGuardDefinitions) || value.GuardsPassed != value.GuardCount ||
		!reflect.DeepEqual(value.Guards, expectedDevelopmentChallengeGuardReceipts()) || !validDigest(value.Digest) {
		return errors.New("stress development challenge receipt identity or portable-reproduction evidence is invalid")
	}
	expected, err := developmentChallengeReceiptDigest(value)
	if err != nil || expected != value.Digest {
		return errors.New("stress development challenge receipt digest is invalid")
	}
	return nil
}

func VerifyDevelopmentChallengeReceipt(challenge DevelopmentChallenge, receipt DevelopmentChallengeReceipt) (DevelopmentChallengeReceiptVerification, error) {
	if err := receipt.ValidateAgainst(challenge); err != nil {
		return DevelopmentChallengeReceiptVerification{}, err
	}
	return DevelopmentChallengeReceiptVerification{
		Valid: true, ReceiptDigest: receipt.Digest, ChallengeDigest: challenge.Digest,
		GuardCount: receipt.GuardCount, GuardsPassed: receipt.GuardsPassed,
		EmpiricalUnits: receipt.EmpiricalUnits, ProviderCalls: receipt.ProviderCalls, NetworkRequired: receipt.NetworkRequired,
	}, nil
}

type developmentChallengeGuardDefinition struct {
	id, category, mutation, guard string
}

var developmentChallengeGuardDefinitions = []developmentChallengeGuardDefinition{
	{"portable_reproduction", "positive_control", "rebuild the complete case from embedded bytes", "portable_reproduction_passed"},
	{"fixture_byte_tamper", "adversarial_guard", "change one embedded trajectory byte and reseal the outer digest", "fixture_digest_mismatch"},
	{"expected_case_substitution", "adversarial_guard", "replace the expected case digest and reseal the outer digest", "expected_case_invalid"},
	{"fixture_inventory_reorder", "adversarial_guard", "swap the two embedded trajectories and reseal the outer digest", "fixture_inventory_invalid"},
	{"unknown_json_property", "adversarial_guard", "inject one unknown root property", "unknown_json_rejected"},
	{"trailing_json_document", "adversarial_guard", "append a second JSON document", "trailing_json_rejected"},
	{"challenge_digest_substitution", "adversarial_guard", "replace the challenge digest", "challenge_digest_invalid"},
}

func runDevelopmentChallengeGuardSuite(value DevelopmentChallenge) ([]DevelopmentChallengeGuardReceipt, error) {
	receipts := newDevelopmentChallengeGuardReceipts()
	observe := func(index int, err error, fragment string) error {
		if err == nil || !bytes.Contains([]byte(err.Error()), []byte(fragment)) {
			return fmt.Errorf("stress development challenge guard %q reached %v, want %q", receipts[index].GuardID, err, fragment)
		}
		receipts[index].ObservedGuard = receipts[index].ExpectedGuard
		receipts[index].Passed = true
		return nil
	}

	fixtures, err := value.embeddedFixtures()
	if err != nil {
		return nil, err
	}
	reproduced, err := buildDevelopmentCaseStudy(fixtures)
	if err != nil || value.Expected.validateReproduced(reproduced) != nil {
		return nil, errors.New("stress development challenge positive control did not reproduce")
	}
	receipts[0].ObservedGuard, receipts[0].Passed = receipts[0].ExpectedGuard, true

	fixtureTamper := value
	fixtureTamper.Fixtures = slices.Clone(value.Fixtures)
	raw, err := hex.DecodeString(fixtureTamper.Fixtures[1].ContentHex)
	if err != nil || len(raw) == 0 {
		return nil, errors.New("stress development challenge fixture cannot be tampered for its guard")
	}
	raw[0] ^= 1
	fixtureTamper.Fixtures[1].ContentHex = hex.EncodeToString(raw)
	fixtureTamper.Digest, err = developmentChallengeDigest(fixtureTamper)
	if err != nil {
		return nil, err
	}
	if err := observe(1, fixtureTamper.Validate(), "fixture bytes do not match their digest"); err != nil {
		return nil, err
	}

	caseTamper := value
	caseTamper.Expected.CaseDigest = digestBytes([]byte("fabricated-development-case"))
	caseTamper.Digest, err = developmentChallengeDigest(caseTamper)
	if err != nil {
		return nil, err
	}
	caseFixtures, err := caseTamper.embeddedFixtures()
	if err != nil {
		return nil, err
	}
	caseReproduced, err := buildDevelopmentCaseStudy(caseFixtures)
	if err != nil {
		return nil, err
	}
	if err := observe(2, caseTamper.Expected.validateReproduced(caseReproduced), "expectation is invalid"); err != nil {
		return nil, err
	}

	inventoryTamper := value
	inventoryTamper.Fixtures = slices.Clone(value.Fixtures)
	inventoryTamper.Fixtures[1], inventoryTamper.Fixtures[2] = inventoryTamper.Fixtures[2], inventoryTamper.Fixtures[1]
	inventoryTamper.Digest, err = developmentChallengeDigest(inventoryTamper)
	if err != nil {
		return nil, err
	}
	if err := observe(3, inventoryTamper.Validate(), "fixture identity is invalid"); err != nil {
		return nil, err
	}

	encoded, err := EncodeIndented(value)
	if err != nil {
		return nil, err
	}
	unknown := bytes.Replace(encoded, []byte("{"), []byte(`{"unknown":true,`), 1)
	if err := observe(4, decodeDevelopmentChallengeOnly(unknown), "unknown field"); err != nil {
		return nil, err
	}
	if err := observe(5, decodeDevelopmentChallengeOnly(append(slices.Clone(encoded), []byte("\n{}")...)), "trailing JSON"); err != nil {
		return nil, err
	}

	digestTamper := value
	digestTamper.Digest = digestBytes([]byte("foreign-development-challenge"))
	if err := observe(6, digestTamper.Validate(), "challenge digest is invalid"); err != nil {
		return nil, err
	}
	return receipts, nil
}

func newDevelopmentChallengeGuardReceipts() []DevelopmentChallengeGuardReceipt {
	result := make([]DevelopmentChallengeGuardReceipt, len(developmentChallengeGuardDefinitions))
	for index, definition := range developmentChallengeGuardDefinitions {
		result[index] = DevelopmentChallengeGuardReceipt{
			GuardID: definition.id, Category: definition.category, Mutation: definition.mutation,
			ExpectedGuard: definition.guard,
		}
	}
	return result
}

func expectedDevelopmentChallengeGuardReceipts() []DevelopmentChallengeGuardReceipt {
	result := newDevelopmentChallengeGuardReceipts()
	for index := range result {
		result[index].ObservedGuard = result[index].ExpectedGuard
		result[index].Passed = true
	}
	return result
}

func decodeDevelopmentChallengeOnly(raw []byte) error {
	_, err := DecodeDevelopmentChallenge(bytes.NewReader(raw))
	return err
}

func DecodeDevelopmentChallenge(reader io.Reader) (DevelopmentChallenge, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, developmentChallengeMaxBytes+1))
	if err != nil {
		return DevelopmentChallenge{}, fmt.Errorf("read stress development challenge: %w", err)
	}
	if len(raw) > developmentChallengeMaxBytes {
		return DevelopmentChallenge{}, errors.New("stress development challenge exceeds the maximum document size")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var value DevelopmentChallenge
	if err := decoder.Decode(&value); err != nil {
		return DevelopmentChallenge{}, fmt.Errorf("decode stress development challenge: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return DevelopmentChallenge{}, errors.New("stress development challenge has trailing JSON")
	}
	return value, value.Validate()
}

func DecodeDevelopmentChallengeReceipt(reader io.Reader) (DevelopmentChallengeReceipt, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, developmentChallengeMaxBytes+1))
	if err != nil {
		return DevelopmentChallengeReceipt{}, fmt.Errorf("read stress development challenge receipt: %w", err)
	}
	if len(raw) > developmentChallengeMaxBytes {
		return DevelopmentChallengeReceipt{}, errors.New("stress development challenge receipt exceeds the maximum document size")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var value DevelopmentChallengeReceipt
	if err := decoder.Decode(&value); err != nil {
		return DevelopmentChallengeReceipt{}, fmt.Errorf("decode stress development challenge receipt: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return DevelopmentChallengeReceipt{}, errors.New("stress development challenge receipt has trailing JSON")
	}
	return value, value.Validate()
}

func buildDevelopmentChallenge(fixtures developmentFixtures) (DevelopmentChallenge, error) {
	caseStudy, err := buildDevelopmentCaseStudy(fixtures)
	if err != nil {
		return DevelopmentChallenge{}, err
	}
	value := DevelopmentChallenge{
		SchemaVersion: DevelopmentChallengeSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		ChallengeID: developmentChallengeID, Status: developmentChallengeStatus,
		Fixtures: []DevelopmentChallengeFixture{
			developmentChallengeFixture(fixtures.task), developmentChallengeFixture(fixtures.trajectoryA),
			developmentChallengeFixture(fixtures.trajectoryB), developmentChallengeFixture(fixtures.license),
		},
		FixtureSetDigest:  caseStudy.FixtureSetDigest,
		Expected:          developmentChallengeExpectation(caseStudy),
		EmpiricalUnits:    0,
		SupportedClaim:    developmentChallengeClaim,
		UnsupportedClaims: slices.Clone(developmentChallengeUnsupportedClaims),
	}
	value.Digest, err = developmentChallengeDigest(value)
	if err != nil {
		return DevelopmentChallenge{}, err
	}
	return value, nil
}

func developmentChallengeExpectation(value DevelopmentCaseStudy) DevelopmentChallengeExpectation {
	lastAccepted := -1
	for index, step := range value.Counterexample.Steps {
		if step.Decision == ReductionAccepted {
			lastAccepted = index
		}
	}
	witnessIDs := make([]string, len(value.FinalWitness))
	for index, witness := range value.FinalWitness {
		witnessIDs[index] = witness.UnitID
	}
	return DevelopmentChallengeExpectation{
		CaseDigest: value.Digest, Outcome: value.Observation.Outcome,
		OriginalLineUnits: value.OriginalLineUnits, FinalLineUnits: value.FinalLineUnits,
		ReductionAttempts: len(value.Counterexample.Steps), AcceptedReductions: value.AcceptedReductions,
		FinalRejectionAttempts: len(value.Counterexample.Steps) - lastAccepted - 1,
		FinalWitnessUnitIDs:    witnessIDs, ClaimBoundary: value.ClaimBoundary,
		ProviderCalls: value.ProviderCalls, NetworkRequired: value.NetworkRequired,
	}
}

func developmentChallengeFixture(fixture developmentFixture) DevelopmentChallengeFixture {
	return DevelopmentChallengeFixture{
		Path: fixture.ref.Path, SHA256: fixture.ref.SHA256, Bytes: fixture.ref.Bytes,
		ContentHex: hex.EncodeToString(fixture.raw),
	}
}

func (value DevelopmentChallenge) embeddedFixtures() (developmentFixtures, error) {
	if len(value.Fixtures) != 4 {
		return developmentFixtures{}, errors.New("stress development challenge fixture inventory is incomplete")
	}
	wantPaths := []string{developmentTaskPath, developmentTrajectoryAPath, developmentTrajectoryBPath, developmentLicensePath}
	decoded := make([]developmentFixture, len(value.Fixtures))
	for index, fixture := range value.Fixtures {
		if fixture.Path != wantPaths[index] || !validDigest(fixture.SHA256) || fixture.Bytes <= 0 || fixture.Bytes > developmentFixtureMaxBytes {
			return developmentFixtures{}, errors.New("stress development challenge fixture identity is invalid")
		}
		raw, err := hex.DecodeString(fixture.ContentHex)
		if err != nil || len(raw) != fixture.Bytes {
			return developmentFixtures{}, errors.New("stress development challenge fixture encoding or size is invalid")
		}
		digest := sha256.Sum256(raw)
		if hex.EncodeToString(digest[:]) != fixture.SHA256 {
			return developmentFixtures{}, errors.New("stress development challenge fixture bytes do not match their digest")
		}
		decoded[index] = developmentFixture{ref: DevelopmentFixtureRef{Path: fixture.Path, SHA256: fixture.SHA256, Bytes: fixture.Bytes}, raw: raw}
	}
	if !bytes.HasPrefix(decoded[3].raw, []byte("MIT License\n")) {
		return developmentFixtures{}, errors.New("stress development challenge does not embed the declared MIT license")
	}
	return developmentFixtures{task: decoded[0], trajectoryA: decoded[1], trajectoryB: decoded[2], license: decoded[3]}, nil
}

func developmentFixtureRefs(fixtures developmentFixtures) []DevelopmentFixtureRef {
	return []DevelopmentFixtureRef{fixtures.task.ref, fixtures.trajectoryA.ref, fixtures.trajectoryB.ref, fixtures.license.ref}
}

func developmentChallengeDigest(value DevelopmentChallenge) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}

func developmentChallengeReceiptDigest(value DevelopmentChallengeReceipt) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}

func digestBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}
