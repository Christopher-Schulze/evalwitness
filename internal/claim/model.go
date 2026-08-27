package claim

import (
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	LedgerSchemaVersion           = "evalwitness.claim-ledger.v1"
	ChallengeReceiptSchemaVersion = "evalwitness.claim-challenge-receipt.v1"
	ProjectionSchemaVersion       = "evalwitness.claim-projection.v1"
	VerifierVersion               = "evalwitness.claim-verifier.v1"
)

type Status string

const (
	StatusSupported   Status = "supported"
	StatusExploratory Status = "exploratory"
	StatusUnsupported Status = "unsupported"
	StatusSuperseded  Status = "superseded"
	StatusWithdrawn   Status = "withdrawn"
)

func (status Status) Valid() bool {
	switch status {
	case StatusSupported, StatusExploratory, StatusUnsupported, StatusSuperseded, StatusWithdrawn:
		return true
	default:
		return false
	}
}

func (status Status) Assertable() bool {
	return status == StatusSupported || status == StatusExploratory
}

type EvidenceLevel string

const (
	EvidenceE0 EvidenceLevel = "E0"
	EvidenceE1 EvidenceLevel = "E1"
	EvidenceE2 EvidenceLevel = "E2"
	EvidenceE3 EvidenceLevel = "E3"
	EvidenceE4 EvidenceLevel = "E4"
	EvidenceE5 EvidenceLevel = "E5"
	EvidenceE6 EvidenceLevel = "E6"
)

func (level EvidenceLevel) Valid() bool {
	return level.rank() >= 0
}

func (level EvidenceLevel) rank() int {
	switch level {
	case EvidenceE0:
		return 0
	case EvidenceE1:
		return 1
	case EvidenceE2:
		return 2
	case EvidenceE3:
		return 3
	case EvidenceE4:
		return 4
	case EvidenceE5:
		return 5
	case EvidenceE6:
		return 6
	default:
		return -1
	}
}

type Operation string

const (
	OperationComponentExists Operation = "component_exists"
	OperationPointerEquals   Operation = "pointer_equals"
	OperationCount           Operation = "count"
	OperationSum             Operation = "sum"
	OperationRatio           Operation = "ratio"
	OperationDifference      Operation = "difference"
	OperationAllEqual        Operation = "all_equal"
)

func (operation Operation) Valid() bool {
	switch operation {
	case OperationComponentExists, OperationPointerEquals, OperationCount, OperationSum,
		OperationRatio, OperationDifference, OperationAllEqual:
		return true
	default:
		return false
	}
}

type ValueKind string

const (
	ValueBoolean ValueKind = "boolean"
	ValueString  ValueKind = "string"
	ValueNumber  ValueKind = "number"
	ValueNull    ValueKind = "null"
)

type ExactValue struct {
	Kind  ValueKind `json:"kind"`
	Value string    `json:"value"`
}

func BooleanValue(value bool) ExactValue {
	if value {
		return ExactValue{Kind: ValueBoolean, Value: "true"}
	}
	return ExactValue{Kind: ValueBoolean, Value: "false"}
}

func StringValue(value string) ExactValue {
	return ExactValue{Kind: ValueString, Value: value}
}

func NumberValue(value string) ExactValue {
	return ExactValue{Kind: ValueNumber, Value: value}
}

func NullValue() ExactValue {
	return ExactValue{Kind: ValueNull, Value: ""}
}

func (value ExactValue) Validate() error {
	switch value.Kind {
	case ValueBoolean:
		if value.Value != "true" && value.Value != "false" {
			return errors.New("claim boolean value is invalid")
		}
	case ValueString:
		if !validText(value.Value) {
			return errors.New("claim string value is invalid")
		}
	case ValueNumber:
		rational, ok := new(big.Rat).SetString(value.Value)
		if !ok || rational.RatString() != value.Value {
			return errors.New("claim number must be a canonical exact rational")
		}
	case ValueNull:
		if value.Value != "" {
			return errors.New("claim null value must be empty")
		}
	default:
		return errors.New("claim value kind is invalid")
	}
	return nil
}

func (value ExactValue) Equal(other ExactValue) bool {
	return value == other
}

type Operand struct {
	Component string `json:"component"`
	TypeID    string `json:"type_id"`
	Pointer   string `json:"pointer"`
}

func (operand Operand) Validate() error {
	if !validToken(operand.Component) || !validToken(operand.TypeID) || !validJSONPointer(operand.Pointer) {
		return errors.New("claim expression operand is invalid")
	}
	return nil
}

type Expression struct {
	Operation Operation  `json:"operation"`
	Operands  []Operand  `json:"operands"`
	Expected  ExactValue `json:"expected"`
}

func (expression Expression) Validate() error {
	if !expression.Operation.Valid() || expression.Operands == nil || len(expression.Operands) == 0 {
		return errors.New("claim expression identity is invalid")
	}
	if err := expression.Expected.Validate(); err != nil {
		return err
	}
	for _, operand := range expression.Operands {
		if err := operand.Validate(); err != nil {
			return err
		}
	}
	switch expression.Operation {
	case OperationComponentExists, OperationPointerEquals, OperationCount:
		if len(expression.Operands) != 1 {
			return errors.New("claim unary expression requires one operand")
		}
	case OperationRatio, OperationDifference:
		if len(expression.Operands) != 2 || expression.Expected.Kind != ValueNumber {
			return errors.New("claim binary numeric expression requires two operands and a numeric expectation")
		}
	case OperationSum:
		if expression.Expected.Kind != ValueNumber {
			return errors.New("claim sum requires a numeric expectation")
		}
	case OperationAllEqual:
		if len(expression.Operands) < 2 || expression.Expected.Kind != ValueBoolean {
			return errors.New("claim equality expression requires at least two operands and a boolean expectation")
		}
	}
	if expression.Operation == OperationComponentExists && expression.Expected.Kind != ValueBoolean {
		return errors.New("claim component existence requires a boolean expectation")
	}
	if expression.Operation == OperationCount && expression.Expected.Kind != ValueNumber {
		return errors.New("claim count requires a numeric expectation")
	}
	return nil
}

type Scope struct {
	Routes      []string `json:"routes"`
	Models      []string `json:"models"`
	Domains     []string `json:"domains"`
	Tasks       []string `json:"tasks"`
	Policies    []string `json:"policies"`
	TimeBounds  []string `json:"time_bounds"`
	Entrypoints []string `json:"entrypoints"`
}

func (scope Scope) Validate() error {
	groups := [][]string{scope.Routes, scope.Models, scope.Domains, scope.Tasks, scope.Policies, scope.TimeBounds, scope.Entrypoints}
	nonEmpty := false
	for _, group := range groups {
		if group == nil || !validSortedText(group) {
			return errors.New("claim scope fields must be non-null, unique, and sorted")
		}
		nonEmpty = nonEmpty || len(group) > 0
	}
	if !nonEmpty {
		return errors.New("claim scope must contain at least one bound")
	}
	return nil
}

type Uncertainty struct {
	Status          string `json:"status"`
	Method          string `json:"method"`
	ConfidenceLevel string `json:"confidence_level"`
	ClusterUnit     string `json:"cluster_unit"`
}

func (uncertainty Uncertainty) Validate() error {
	switch uncertainty.Status {
	case "not_applicable", "descriptive_only", "unavailable_legacy", "reported":
	default:
		return errors.New("claim uncertainty status is invalid")
	}
	if !validText(uncertainty.Method) || !validText(uncertainty.ClusterUnit) {
		return errors.New("claim uncertainty metadata is invalid")
	}
	if uncertainty.ConfidenceLevel != "" {
		rational, ok := new(big.Rat).SetString(uncertainty.ConfidenceLevel)
		if !ok || rational.RatString() != uncertainty.ConfidenceLevel || rational.Sign() <= 0 || rational.Cmp(big.NewRat(1, 1)) > 0 {
			return errors.New("claim confidence level is invalid")
		}
	}
	if uncertainty.Status == "reported" && uncertainty.ConfidenceLevel == "" {
		return errors.New("reported claim uncertainty requires a confidence level")
	}
	return nil
}

type Multiplicity struct {
	Status string `json:"status"`
	Family string `json:"family"`
	Method string `json:"method"`
}

func (multiplicity Multiplicity) Validate() error {
	switch multiplicity.Status {
	case "not_applicable", "descriptive_only", "unadjusted", "adjusted":
	default:
		return errors.New("claim multiplicity status is invalid")
	}
	if !validText(multiplicity.Family) || !validText(multiplicity.Method) {
		return errors.New("claim multiplicity metadata is invalid")
	}
	return nil
}

type EvidenceCeiling struct {
	MaximumLevel        EvidenceLevel `json:"maximum_level"`
	StrongestStatement  string        `json:"strongest_statement"`
	RequiredParentTypes []string      `json:"required_parent_types"`
	RequiredCaveats     []string      `json:"required_caveats"`
}

func (ceiling EvidenceCeiling) Validate() error {
	if !ceiling.MaximumLevel.Valid() || !validText(ceiling.StrongestStatement) ||
		ceiling.RequiredParentTypes == nil || !validSortedTokens(ceiling.RequiredParentTypes) ||
		ceiling.RequiredCaveats == nil || !validSortedText(ceiling.RequiredCaveats) {
		return errors.New("claim evidence ceiling is invalid")
	}
	return nil
}

type Generation struct {
	Lane     string `json:"lane"`
	Evidence string `json:"evidence"`
	Current  string `json:"current"`
}

func (generation Generation) Validate() error {
	if !validToken(generation.Lane) || !validToken(generation.Evidence) || !validToken(generation.Current) {
		return errors.New("claim generation is invalid")
	}
	return nil
}

type AttestationState string

const (
	AttestationNotRequired AttestationState = "not_required"
	AttestationCurrent     AttestationState = "current"
	AttestationStale       AttestationState = "stale"
	AttestationUnavailable AttestationState = "unavailable"
)

func (state AttestationState) Valid() bool {
	return state == AttestationNotRequired || state == AttestationCurrent || state == AttestationStale || state == AttestationUnavailable
}

type ChallengeClass string

const (
	ChallengeCaveatRemoval        ChallengeClass = "caveat-removal"
	ChallengeDenominatorDeletion  ChallengeClass = "denominator-deletion"
	ChallengeDigestSubstitution   ChallengeClass = "digest-substitution"
	ChallengeParentRemoval        ChallengeClass = "parent-removal"
	ChallengeScopeWidening        ChallengeClass = "scope-widening"
	ChallengeStaleAttestation     ChallengeClass = "stale-attestation"
	ChallengeSupersededGeneration ChallengeClass = "superseded-generation"
	ChallengeVisibilityLeak       ChallengeClass = "visibility-leak"
)

func ChallengeClasses() []ChallengeClass {
	return []ChallengeClass{
		ChallengeCaveatRemoval,
		ChallengeDenominatorDeletion,
		ChallengeDigestSubstitution,
		ChallengeParentRemoval,
		ChallengeScopeWidening,
		ChallengeStaleAttestation,
		ChallengeSupersededGeneration,
		ChallengeVisibilityLeak,
	}
}

type ChallengeApplicability string

const (
	ChallengeApplied      ChallengeApplicability = "applied"
	ChallengeInapplicable ChallengeApplicability = "inapplicable"
)

type ChallengeSpec struct {
	ChallengeID        string                 `json:"challenge_id"`
	Class              ChallengeClass         `json:"class"`
	Applicability      ChallengeApplicability `json:"applicability"`
	InapplicableReason string                 `json:"inapplicable_reason"`
	Mutation           string                 `json:"mutation"`
	ExpectedGuard      Guard                  `json:"expected_guard"`
}

func (challenge ChallengeSpec) Validate(claimID string) error {
	if challenge.ChallengeID != strings.ToLower(claimID)+"."+string(challenge.Class) || !slices.Contains(ChallengeClasses(), challenge.Class) {
		return errors.New("claim challenge identity is invalid")
	}
	switch challenge.Applicability {
	case ChallengeApplied:
		if !validText(challenge.Mutation) || !challenge.ExpectedGuard.Valid() || challenge.InapplicableReason != "" || expectedGuard(challenge.Class) != challenge.ExpectedGuard {
			return errors.New("applied claim challenge is invalid")
		}
	case ChallengeInapplicable:
		if !validInapplicableReason(challenge.InapplicableReason) || challenge.Mutation != "" || challenge.ExpectedGuard != "" {
			return errors.New("inapplicable claim challenge is invalid")
		}
	default:
		return errors.New("claim challenge applicability is invalid")
	}
	return nil
}

type HistoryEntry struct {
	Revision     int    `json:"revision"`
	PriorStatus  Status `json:"prior_status"`
	Reason       string `json:"reason"`
	SupersededBy string `json:"superseded_by"`
}

func (entry HistoryEntry) Validate() error {
	if entry.Revision < 1 || !entry.PriorStatus.Valid() || !validText(entry.Reason) || entry.SupersededBy != "" && !validClaimID(entry.SupersededBy) {
		return errors.New("claim history entry is invalid")
	}
	return nil
}

type Claim struct {
	ClaimID            string           `json:"claim_id"`
	TextTemplate       string           `json:"text_template"`
	Status             Status           `json:"status"`
	EvidenceLevel      EvidenceLevel    `json:"evidence_level"`
	Scope              Scope            `json:"scope"`
	Expression         Expression       `json:"expression"`
	Uncertainty        Uncertainty      `json:"uncertainty"`
	Multiplicity       Multiplicity     `json:"multiplicity"`
	CapsuleIDs         []string         `json:"capsule_ids"`
	EvidenceComponents []string         `json:"evidence_components"`
	Caveats            []string         `json:"caveats"`
	EvidenceCeiling    EvidenceCeiling  `json:"evidence_ceiling"`
	Attestation        AttestationState `json:"attestation"`
	Generation         Generation       `json:"generation"`
	GuardingTest       string           `json:"guarding_test"`
	Challenges         []ChallengeSpec  `json:"challenges"`
	History            []HistoryEntry   `json:"history"`
}

func (claim Claim) Validate() error {
	if !validClaimID(claim.ClaimID) || !validText(claim.TextTemplate) || !claim.Status.Valid() || !claim.EvidenceLevel.Valid() ||
		claim.CapsuleIDs == nil || !validSortedDigests(claim.CapsuleIDs) || claim.EvidenceComponents == nil ||
		len(claim.EvidenceComponents) == 0 || !validSortedTokens(claim.EvidenceComponents) || claim.Caveats == nil ||
		!validSortedText(claim.Caveats) || !claim.Attestation.Valid() || !validText(claim.GuardingTest) || claim.History == nil {
		return fmt.Errorf("claim %q identity or closed fields are invalid", claim.ClaimID)
	}
	if err := claim.Scope.Validate(); err != nil {
		return fmt.Errorf("claim %q: %w", claim.ClaimID, err)
	}
	if err := claim.Expression.Validate(); err != nil {
		return fmt.Errorf("claim %q: %w", claim.ClaimID, err)
	}
	if err := claim.Uncertainty.Validate(); err != nil {
		return fmt.Errorf("claim %q: %w", claim.ClaimID, err)
	}
	if err := claim.Multiplicity.Validate(); err != nil {
		return fmt.Errorf("claim %q: %w", claim.ClaimID, err)
	}
	if err := claim.EvidenceCeiling.Validate(); err != nil {
		return fmt.Errorf("claim %q: %w", claim.ClaimID, err)
	}
	if claim.EvidenceCeiling.StrongestStatement != claim.TextTemplate {
		return fmt.Errorf("claim %q text exceeds or differs from its evidence ceiling", claim.ClaimID)
	}
	if err := claim.Generation.Validate(); err != nil {
		return fmt.Errorf("claim %q: %w", claim.ClaimID, err)
	}
	if claim.Status.Assertable() && claim.EvidenceLevel == EvidenceE0 {
		return fmt.Errorf("claim %q cannot be assertable without evidence", claim.ClaimID)
	}
	if claim.Status.Assertable() && claim.Expression.Operation == OperationComponentExists {
		return fmt.Errorf("claim %q cannot be assertable from component presence alone", claim.ClaimID)
	}
	if claim.Status.Assertable() && claim.Generation.Evidence != claim.Generation.Current {
		return fmt.Errorf("claim %q assertable evidence generation is not current", claim.ClaimID)
	}
	if claim.Status == StatusSuperseded && claim.Generation.Evidence == claim.Generation.Current {
		return fmt.Errorf("claim %q superseded evidence generation is still marked current", claim.ClaimID)
	}
	if claim.Status.Assertable() && claim.Attestation != AttestationNotRequired && claim.Attestation != AttestationCurrent {
		return fmt.Errorf("claim %q assertable status lacks a usable attestation state", claim.ClaimID)
	}
	if claim.EvidenceLevel.rank() > claim.EvidenceCeiling.MaximumLevel.rank() {
		return fmt.Errorf("claim %q exceeds its evidence ceiling", claim.ClaimID)
	}
	for _, operand := range claim.Expression.Operands {
		if !slices.Contains(claim.EvidenceComponents, operand.Component) {
			return fmt.Errorf("claim %q expression uses undeclared evidence component %q", claim.ClaimID, operand.Component)
		}
	}
	for _, required := range claim.EvidenceCeiling.RequiredCaveats {
		if !slices.Contains(claim.Caveats, required) {
			return fmt.Errorf("claim %q omits required caveat %q", claim.ClaimID, required)
		}
	}
	if len(claim.Challenges) != len(ChallengeClasses()) {
		return fmt.Errorf("claim %q does not declare every challenge class", claim.ClaimID)
	}
	applied := 0
	for index, class := range ChallengeClasses() {
		challenge := claim.Challenges[index]
		if challenge.Class != class {
			return fmt.Errorf("claim %q challenges are not in canonical class order", claim.ClaimID)
		}
		if err := challenge.Validate(claim.ClaimID); err != nil {
			return fmt.Errorf("claim %q: %w", claim.ClaimID, err)
		}
		applicable, reason := challengeApplicability(claim, class)
		if applicable && challenge.Applicability != ChallengeApplied {
			return fmt.Errorf("claim %q marks applicable challenge %q as inapplicable", claim.ClaimID, class)
		}
		if !applicable && (challenge.Applicability != ChallengeInapplicable || challenge.InapplicableReason != reason) {
			return fmt.Errorf("claim %q has an invalid inapplicable declaration for challenge %q", claim.ClaimID, class)
		}
		if challenge.Applicability == ChallengeApplied {
			applied++
		}
	}
	if claim.Status.Assertable() && applied == 0 {
		return fmt.Errorf("claim %q has no executable challenge", claim.ClaimID)
	}
	previousRevision := 0
	for _, entry := range claim.History {
		if err := entry.Validate(); err != nil || entry.Revision <= previousRevision {
			return fmt.Errorf("claim %q history is invalid or unsorted", claim.ClaimID)
		}
		previousRevision = entry.Revision
	}
	if (claim.Status == StatusSuperseded || claim.Status == StatusWithdrawn) && len(claim.History) == 0 {
		return fmt.Errorf("claim %q requires lifecycle history", claim.ClaimID)
	}
	return nil
}

type Ledger struct {
	SchemaVersion   string  `json:"schema_version"`
	CanonicalPolicy string  `json:"canonical_policy"`
	CapsuleID       string  `json:"capsule_id"`
	ManifestDigest  string  `json:"manifest_digest"`
	Claims          []Claim `json:"claims"`
	Digest          string  `json:"digest"`
}

func SealLedger(manifest capsule.Manifest, claims []Claim) (Ledger, error) {
	ledger := Ledger{
		SchemaVersion: LedgerSchemaVersion, CanonicalPolicy: capsule.CanonicalPolicy,
		CapsuleID: manifest.CapsuleID, ManifestDigest: manifest.ManifestDigest,
		Claims: cloneClaims(claims),
	}
	slices.SortFunc(ledger.Claims, func(left, right Claim) int { return strings.Compare(left.ClaimID, right.ClaimID) })
	for index := range ledger.Claims {
		ledger.Claims[index].CapsuleIDs = []string{manifest.CapsuleID}
		normalizeClaim(&ledger.Claims[index])
	}
	digest, err := ledgerDigest(ledger)
	if err != nil {
		return Ledger{}, err
	}
	ledger.Digest = digest
	if err := ledger.Validate(); err != nil {
		return Ledger{}, err
	}
	return ledger, nil
}

func (ledger Ledger) Validate() error {
	if ledger.SchemaVersion != LedgerSchemaVersion || ledger.CanonicalPolicy != capsule.CanonicalPolicy ||
		!validDigest(ledger.CapsuleID) || !validDigest(ledger.ManifestDigest) || !validDigest(ledger.Digest) || len(ledger.Claims) == 0 {
		return errors.New("claim ledger identity is invalid")
	}
	previous := ""
	for _, claim := range ledger.Claims {
		if claim.ClaimID <= previous {
			return errors.New("claim ledger entries must be unique and sorted")
		}
		if err := claim.Validate(); err != nil {
			return err
		}
		if !slices.Equal(claim.CapsuleIDs, []string{ledger.CapsuleID}) {
			return fmt.Errorf("claim %q does not bind exactly the ledger capsule", claim.ClaimID)
		}
		previous = claim.ClaimID
	}
	digest, err := ledgerDigest(ledger)
	if err != nil || digest != ledger.Digest {
		return errors.New("claim ledger digest is invalid")
	}
	return nil
}

func ledgerDigest(ledger Ledger) (string, error) {
	ledger.Digest = ""
	return protocol.Digest(ledger)
}

func normalizeClaim(claim *Claim) {
	slices.Sort(claim.CapsuleIDs)
	slices.Sort(claim.EvidenceComponents)
	slices.Sort(claim.Caveats)
	slices.Sort(claim.EvidenceCeiling.RequiredParentTypes)
	slices.Sort(claim.EvidenceCeiling.RequiredCaveats)
	slices.SortFunc(claim.Challenges, func(left, right ChallengeSpec) int { return strings.Compare(string(left.Class), string(right.Class)) })
	slices.SortFunc(claim.History, func(left, right HistoryEntry) int { return left.Revision - right.Revision })
	normalizeScope(&claim.Scope)
}

func normalizeScope(scope *Scope) {
	slices.Sort(scope.Routes)
	slices.Sort(scope.Models)
	slices.Sort(scope.Domains)
	slices.Sort(scope.Tasks)
	slices.Sort(scope.Policies)
	slices.Sort(scope.TimeBounds)
	slices.Sort(scope.Entrypoints)
}

func cloneClaims(claims []Claim) []Claim {
	cloned := make([]Claim, len(claims))
	for index, claim := range claims {
		cloned[index] = claim
		cloned[index].CapsuleIDs = slices.Clone(claim.CapsuleIDs)
		cloned[index].EvidenceComponents = slices.Clone(claim.EvidenceComponents)
		cloned[index].Caveats = slices.Clone(claim.Caveats)
		cloned[index].EvidenceCeiling.RequiredParentTypes = slices.Clone(claim.EvidenceCeiling.RequiredParentTypes)
		cloned[index].EvidenceCeiling.RequiredCaveats = slices.Clone(claim.EvidenceCeiling.RequiredCaveats)
		cloned[index].Challenges = slices.Clone(claim.Challenges)
		cloned[index].History = slices.Clone(claim.History)
		cloned[index].Scope.Routes = slices.Clone(claim.Scope.Routes)
		cloned[index].Scope.Models = slices.Clone(claim.Scope.Models)
		cloned[index].Scope.Domains = slices.Clone(claim.Scope.Domains)
		cloned[index].Scope.Tasks = slices.Clone(claim.Scope.Tasks)
		cloned[index].Scope.Policies = slices.Clone(claim.Scope.Policies)
		cloned[index].Scope.TimeBounds = slices.Clone(claim.Scope.TimeBounds)
		cloned[index].Scope.Entrypoints = slices.Clone(claim.Scope.Entrypoints)
		cloned[index].Expression.Operands = slices.Clone(claim.Expression.Operands)
	}
	return cloned
}

func validClaimID(value string) bool {
	if len(value) != 7 || !strings.HasPrefix(value, "CLM-") {
		return false
	}
	for _, character := range value[4:] {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
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

func validText(value string) bool {
	return value != "" && len(value) <= 4096 && value == strings.TrimSpace(value)
}

func validToken(value string) bool {
	if value == "" || len(value) > 256 || value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || strings.ContainsRune("._:/-", character) {
			continue
		}
		return false
	}
	return true
}

func validJSONPointer(pointer string) bool {
	if pointer == "" {
		return true
	}
	if !strings.HasPrefix(pointer, "/") || len(pointer) > 2048 {
		return false
	}
	for index := 0; index < len(pointer); index++ {
		if pointer[index] != '~' {
			continue
		}
		if index+1 >= len(pointer) || pointer[index+1] != '0' && pointer[index+1] != '1' {
			return false
		}
		index++
	}
	return true
}

func validSortedText(values []string) bool {
	previous := ""
	for _, value := range values {
		if !validText(value) || value <= previous {
			return false
		}
		previous = value
	}
	return true
}

func validSortedTokens(values []string) bool {
	previous := ""
	for _, value := range values {
		if !validToken(value) || value <= previous {
			return false
		}
		previous = value
	}
	return true
}

func validSortedDigests(values []string) bool {
	previous := ""
	for _, value := range values {
		if !validDigest(value) || value <= previous {
			return false
		}
		previous = value
	}
	return len(values) > 0
}

func validInapplicableReason(reason string) bool {
	switch reason {
	case "attestation_not_required", "denominator_not_used", "generation_not_applicable", "no_required_caveat", "visibility_projection_not_used":
		return true
	default:
		return false
	}
}

func challengeApplicability(claim Claim, class ChallengeClass) (bool, string) {
	switch class {
	case ChallengeCaveatRemoval:
		return len(claim.EvidenceCeiling.RequiredCaveats) > 0, "no_required_caveat"
	case ChallengeDenominatorDeletion:
		return claim.Expression.Operation == OperationRatio, "denominator_not_used"
	case ChallengeStaleAttestation:
		if !claim.Status.Assertable() {
			return false, "generation_not_applicable"
		}
		return claim.Attestation == AttestationCurrent, "attestation_not_required"
	case ChallengeSupersededGeneration:
		return claim.Status.Assertable(), "generation_not_applicable"
	case ChallengeDigestSubstitution, ChallengeParentRemoval, ChallengeScopeWidening, ChallengeVisibilityLeak:
		return true, ""
	default:
		return false, "generation_not_applicable"
	}
}
