package protocol

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

var (
	identifierPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._:-]{0,127}$`)
	namespacePattern  = regexp.MustCompile(`^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*){2,}$`)
)

func DefaultLimits() ResourceLimits {
	return ResourceLimits{
		MaxMessageBytes: 8 << 20, MaxCaseBytes: 4 << 20, MaxCasesPerRun: 10_000,
		MaxDurationMillis: 60_000, MaxExtensionsPerItem: 32,
	}
}

func SupportedVersions() []string {
	return []string{CurrentVersion, PreviousMinorVersion}
}

func PermittedClaim(level ConformanceLevel) string {
	switch level {
	case LevelSyntax:
		return "implementation exchanges and validates the named protocol version and normative syntax cases"
	case LevelDeterministicReplay:
		return "implementation reproduces the named sealed observations and evidence invariants offline"
	case LevelLiveScoreEvidence:
		return "the named route returned conformant score evidence during a separately authorized bounded observation"
	case LevelEmpiricalReliability:
		return "the named locked study measured the declared verifier failure modes on its stated population"
	case LevelIndependentReproduction:
		return "the named independent actor reproduced the named frozen artifacts or study under the declared boundary"
	default:
		return ""
	}
}

func ValidateDescriptor(descriptor EvaluatorDescriptor) error {
	if descriptor.SchemaVersion != DescriptorSchema || descriptor.ProtocolName != ProtocolName {
		return errors.New("descriptor schema or protocol name is invalid")
	}
	if !identifierPattern.MatchString(descriptor.EvaluatorID) {
		return errors.New("descriptor evaluator_id is invalid")
	}
	if descriptor.Implementation.Name == "" || descriptor.Implementation.Version == "" ||
		!validDigest(descriptor.Implementation.IdentityDigest) {
		return errors.New("descriptor implementation identity is incomplete")
	}
	if descriptor.ProtocolVersions == nil || descriptor.ExecutionModes == nil || descriptor.ConformanceLevels == nil ||
		descriptor.ScoreEvidenceVersions == nil || descriptor.DecisionVersions == nil || descriptor.Extensions == nil {
		return errors.New("descriptor omits a required array")
	}
	if err := validateUniqueStrings(descriptor.ProtocolVersions, "protocol_versions"); err != nil {
		return err
	}
	if !containsString(descriptor.ProtocolVersions, CurrentVersion) {
		return errors.New("descriptor does not support the current protocol version")
	}
	if err := validateUniqueStrings(descriptor.ExecutionModes, "execution_modes"); err != nil {
		return err
	}
	if err := validateLevels(descriptor.ConformanceLevels); err != nil {
		return err
	}
	if err := validateLimits(descriptor.Limits); err != nil {
		return err
	}
	if err := ValidateExtensions(descriptor.Extensions, descriptor.Limits.MaxExtensionsPerItem); err != nil {
		return err
	}
	return nil
}

func ValidateCaseEnvelope(auditCase AuditCase, limits ResourceLimits) error {
	if auditCase.SchemaVersion != CaseSchema || !supportedVersion(auditCase.ProtocolVersion) {
		return errors.New("audit case schema or protocol version is unsupported")
	}
	if !identifierPattern.MatchString(auditCase.CaseID) || auditCase.Description == "" {
		return errors.New("audit case identity or description is invalid")
	}
	if PermittedClaim(auditCase.Level) == "" || !validCaseKind(auditCase.Kind) {
		return errors.New("audit case conformance level or kind is invalid")
	}
	if auditCase.Invocation.SchemaVersion != InvocationSchema || auditCase.Invocation.InvocationID == "" ||
		auditCase.Invocation.Operation != auditCase.Kind {
		return errors.New("audit invocation does not match its case")
	}
	if auditCase.Invocation.TimeoutMillis <= 0 || auditCase.Invocation.TimeoutMillis > limits.MaxDurationMillis ||
		auditCase.Invocation.MaxInputBytes <= 0 || auditCase.Invocation.MaxInputBytes > limits.MaxCaseBytes {
		return errors.New("audit invocation resource limits are invalid")
	}
	if auditCase.Expected.Status != ExpectationAccepted && auditCase.Expected.Status != ExpectationRejected &&
		auditCase.Expected.Status != ExpectationUnsupported {
		return errors.New("audit case expectation is invalid")
	}
	if err := validateUniqueStrings(auditCase.RequiredCapabilities, "required_capabilities"); err != nil {
		return err
	}
	if err := ValidateExtensions(auditCase.Extensions, limits.MaxExtensionsPerItem); err != nil {
		return err
	}
	if err := validateTrajectoryRefs(auditCase.Invocation.TrajectoryRefs); err != nil {
		return err
	}
	return ValidateExtensions(auditCase.Invocation.Extensions, limits.MaxExtensionsPerItem)
}

func ValidateInvocation(invocation AuditInvocation) error {
	if invocation.SchemaVersion != InvocationSchema || invocation.InvocationID == "" || !invocation.Offline {
		return errors.New("reference invocation must be current, identified, and offline")
	}
	payloads := 0
	if invocation.CanonicalJSON != nil {
		payloads++
	}
	if invocation.RequestVector != nil {
		payloads++
	}
	if invocation.ScoreEvidence != nil {
		payloads++
	}
	if invocation.Decision != nil {
		payloads++
	}
	if invocation.Replay != nil {
		payloads++
	}
	if invocation.Attestation != nil {
		payloads++
	}
	if invocation.Operation == CaseExtension || invocation.Operation == CaseCompatibility {
		if payloads != 0 {
			return errors.New("extension or compatibility operation cannot contain a typed payload")
		}
	} else if payloads != 1 {
		return errors.New("protocol operation requires exactly one typed payload")
	}
	switch invocation.Operation {
	case CaseCanonicalEncoding:
		return validateCanonicalJSONInput(invocation.CanonicalJSON)
	case CaseRequestFingerprint:
		return validateRequestVector(invocation.RequestVector)
	case CaseScoreEvidence:
		return ValidateScoreEvidence(*invocation.ScoreEvidence)
	case CaseDecisionEvidence:
		return ValidateDecisionEvidence(*invocation.Decision)
	case CaseReplayEvidence:
		return validateReplay(*invocation.Replay)
	case CaseAttestation:
		return validateAttestation(*invocation.Attestation)
	case CaseExtension, CaseCompatibility:
		return nil
	default:
		return errors.New("unsupported protocol operation")
	}
}

func validateCanonicalJSONInput(vector *CanonicalJSONInput) error {
	if vector == nil || vector.SourceUTF8Hex == "" {
		return errors.New("canonical JSON vector is missing source bytes")
	}
	source, err := hex.DecodeString(vector.SourceUTF8Hex)
	if err != nil || len(source) == 0 {
		return errors.New("canonical JSON source bytes are invalid")
	}
	canonical, err := CanonicalizeJSON(source)
	if err != nil {
		return fmt.Errorf("canonical JSON source: %w", err)
	}
	expected, err := hex.DecodeString(vector.ExpectedCanonicalHex)
	if err != nil || len(expected) == 0 || !bytes.Equal(canonical, expected) {
		return errors.New("canonical JSON bytes do not match expected bytes")
	}
	if !validDigest(vector.ExpectedSHA256) || DigestBytes(canonical) != vector.ExpectedSHA256 {
		return errors.New("canonical JSON bytes do not match expected digest")
	}
	return nil
}

func ValidateScoreEvidence(evidence ScoreEvidence) error {
	if evidence.SchemaVersion != ScoreSchema || evidence.PolicyVersion == "" || evidence.Tag == "" {
		return errors.New("score evidence schema, policy, or tag is invalid")
	}
	if evidence.ExtractionMode != "verifier" && evidence.ExtractionMode != "judge" {
		return errors.New("score evidence extraction mode is invalid")
	}
	if evidence.ScoreSupport == nil || evidence.VisibleAlternatives == nil || evidence.Degradations == nil || evidence.RouteLimitations == nil {
		return errors.New("score evidence omits a required array")
	}
	if evidence.Extracted && evidence.AlignmentStatus != "exact" {
		return errors.New("extracted score evidence requires exact alignment")
	}
	if evidence.RequestedTopK <= 0 || evidence.ReturnedTopK <= 0 || evidence.ReturnedTopK > evidence.RequestedTopK ||
		len(evidence.VisibleAlternatives) != evidence.ReturnedTopK {
		return errors.New("score evidence top-k accounting is inconsistent")
	}
	visible, err := probability(evidence.VisibleProbabilityMass)
	if err != nil {
		return fmt.Errorf("visible probability mass: %w", err)
	}
	valid, err := probability(evidence.ValidScoreMass)
	if err != nil {
		return fmt.Errorf("valid score mass: %w", err)
	}
	unobserved, err := probability(evidence.UnobservedProbabilityMass)
	if err != nil {
		return fmt.Errorf("unobserved probability mass: %w", err)
	}
	if new(big.Rat).Add(new(big.Rat).Set(visible), unobserved).Cmp(big.NewRat(1, 1)) != 0 {
		return errors.New("visible and unobserved probability mass do not sum to one")
	}
	alternativeMass := new(big.Rat)
	alternativeProbabilities := make([]*big.Rat, len(evidence.VisibleAlternatives))
	chosen := 0
	for index, alternative := range evidence.VisibleAlternatives {
		if alternative.Rank != index+1 || alternative.Token == "" {
			return errors.New("visible alternatives are not complete rank order")
		}
		probabilityValue, err := probability(alternative.Probability)
		if err != nil {
			return fmt.Errorf("visible alternative %d: %w", index, err)
		}
		if _, err := parseCanonicalDecimal(alternative.LogprobDecimal); err != nil {
			return fmt.Errorf("visible alternative %d logprob: %w", index, err)
		}
		alternativeMass.Add(alternativeMass, probabilityValue)
		alternativeProbabilities[index] = probabilityValue
		if alternative.Chosen {
			chosen++
		}
		if alternative.CanonicalLetter == "" {
			if alternative.CanonicalValue != "" || len(alternative.CanonicalSourceRanks) != 0 {
				return errors.New("uncanonicalized alternative contains partial canonical provenance")
			}
		} else {
			if !validScoreLetter(alternative.CanonicalLetter) || alternative.CanonicalValue == "" ||
				validateSortedUniquePositiveInts(alternative.CanonicalSourceRanks) != nil {
				return errors.New("canonicalized alternative provenance is incomplete")
			}
			if _, err := probability(alternative.CanonicalValue); err != nil {
				return errors.New("canonicalized alternative value is invalid")
			}
		}
	}
	if alternativeMass.Cmp(visible) != 0 || chosen != 1 {
		return errors.New("visible alternative mass or chosen-token count is inconsistent")
	}
	supportMass := new(big.Rat)
	expectedNumerator := new(big.Rat)
	secondMoment := new(big.Rat)
	seenLetters := make(map[string]struct{}, len(evidence.ScoreSupport))
	usedRanks := make(map[int]struct{}, len(evidence.VisibleAlternatives))
	lastLetter := ""
	for index, support := range evidence.ScoreSupport {
		if !validScoreLetter(support.Letter) || support.Letter < lastLetter {
			return errors.New("score support is not in canonical letter order")
		}
		if _, duplicate := seenLetters[support.Letter]; duplicate {
			return errors.New("score support repeats a canonical letter")
		}
		seenLetters[support.Letter] = struct{}{}
		lastLetter = support.Letter
		value, err := probability(support.ValueDecimal)
		if err != nil {
			return fmt.Errorf("score support %d value: %w", index, err)
		}
		mass, err := probability(support.Probability)
		if err != nil {
			return fmt.Errorf("score support %d probability: %w", index, err)
		}
		if err := validateSortedUniquePositiveInts(support.SourceRanks); err != nil {
			return fmt.Errorf("score support %d source ranks: %w", index, err)
		}
		if len(support.SourceForms) == 0 {
			return errors.New("score support source forms are empty")
		}
		if err := validateSortedUniqueStrings(support.SourceForms, "score_support.source_forms"); err != nil {
			return err
		}
		expectedForms := make([]string, 0, len(support.SourceRanks))
		maximumSourceMass := new(big.Rat)
		for _, rank := range support.SourceRanks {
			if rank > len(evidence.VisibleAlternatives) {
				return errors.New("score support source rank exceeds returned alternatives")
			}
			if _, duplicate := usedRanks[rank]; duplicate {
				return errors.New("visible alternative is assigned to multiple score-support entries")
			}
			usedRanks[rank] = struct{}{}
			alternative := evidence.VisibleAlternatives[rank-1]
			if alternative.CanonicalLetter != support.Letter || alternative.CanonicalValue != support.ValueDecimal ||
				!slices.Equal(alternative.CanonicalSourceRanks, support.SourceRanks) {
				return errors.New("score support does not match alternative canonical provenance")
			}
			expectedForms = append(expectedForms, alternative.Token)
			if alternativeProbabilities[rank-1].Cmp(maximumSourceMass) > 0 {
				maximumSourceMass.Set(alternativeProbabilities[rank-1])
			}
		}
		sort.Strings(expectedForms)
		if !slices.Equal(expectedForms, support.SourceForms) || mass.Cmp(maximumSourceMass) != 0 {
			return errors.New("score support forms or max-form probability are inconsistent")
		}
		supportMass.Add(supportMass, mass)
		expectedNumerator.Add(expectedNumerator, new(big.Rat).Mul(value, mass))
		secondMoment.Add(secondMoment, new(big.Rat).Mul(new(big.Rat).Mul(value, value), mass))
	}
	if supportMass.Cmp(valid) != 0 || valid.Sign() == 0 {
		return errors.New("score support mass does not equal valid score mass")
	}
	expected := new(big.Rat).Quo(expectedNumerator, valid)
	variance := new(big.Rat).Sub(new(big.Rat).Quo(secondMoment, valid), new(big.Rat).Mul(expected, expected))
	declaredExpected, err := parseCanonicalDecimal(evidence.ConditionalExpectedScore)
	if err != nil || declaredExpected.Cmp(expected) != 0 {
		return errors.New("conditional expected score does not match canonical support")
	}
	declaredVariance, err := parseCanonicalDecimal(evidence.ConditionalVariance)
	if err != nil || declaredVariance.Cmp(variance) != 0 {
		return errors.New("conditional variance does not match canonical support")
	}
	if !validDigest(evidence.RequestFingerprint) || !validDigest(evidence.ResponseEvidenceDigest) {
		return errors.New("score evidence provenance digests are invalid")
	}
	if evidence.CapabilityAttestationDigest != "" && !validDigest(evidence.CapabilityAttestationDigest) {
		return errors.New("score evidence capability attestation digest is invalid")
	}
	if evidence.RequestedModel == "" {
		return errors.New("score evidence requested model is empty")
	}
	seenDegradations := make(map[string]struct{}, len(evidence.Degradations))
	for _, degradation := range evidence.Degradations {
		if !identifierPattern.MatchString(degradation.Code) {
			return errors.New("score evidence degradation code is invalid")
		}
		if _, duplicate := seenDegradations[degradation.Code]; duplicate {
			return fmt.Errorf("score evidence repeats degradation %q", degradation.Code)
		}
		seenDegradations[degradation.Code] = struct{}{}
	}
	return validateSortedUniqueStrings(evidence.RouteLimitations, "route_limitations")
}

func ValidateDecisionEvidence(decision DecisionEvidence) error {
	if decision.SchemaVersion != DecisionSchema || decision.PolicyVersion == "" {
		return errors.New("decision schema or policy is invalid")
	}
	validStates := map[string]bool{
		"selected": true, "tied": true, "abstained": true, "invalid": true,
		"budget_exhausted": true, "provider_failed": true,
	}
	if !validStates[decision.State] {
		return errors.New("decision state is invalid")
	}
	if decision.ConditionalScores == nil || decision.ScoreEvidenceDigests == nil {
		return errors.New("decision evidence omits a required array")
	}
	if decision.State == "abstained" && decision.AbstentionReason == "" ||
		decision.State != "abstained" && decision.AbstentionReason != "" {
		return errors.New("decision abstention reason is inconsistent with state")
	}
	if decision.State == "selected" && (len(decision.ConditionalScores) == 0 || decision.Winner == nil ||
		*decision.Winner < 0 || *decision.Winner >= len(decision.ConditionalScores)) {
		return errors.New("selected decision winner is invalid")
	}
	if decision.State != "selected" && decision.Winner != nil {
		return errors.New("non-selected decision cannot contain a winner")
	}
	for index, score := range decision.ConditionalScores {
		if _, err := probability(score); err != nil {
			return fmt.Errorf("conditional score %d: %w", index, err)
		}
	}
	strength, err := probability(decision.DecisionStrength)
	if err != nil {
		return fmt.Errorf("decision strength: %w", err)
	}
	if decision.State == "selected" && strength.Sign() == 0 || decision.State != "selected" && strength.Sign() != 0 {
		return errors.New("decision strength is inconsistent with terminal state")
	}
	if len(decision.ScoreEvidenceDigests) == 0 {
		return errors.New("decision has no score-evidence digests")
	}
	for _, digest := range decision.ScoreEvidenceDigests {
		if !validDigest(digest) {
			return errors.New("decision score-evidence digest is invalid")
		}
	}
	if !validDigest(decision.BudgetDigest) || !validDigest(decision.ProvenanceDigest) {
		return errors.New("decision budget or provenance digest is invalid")
	}
	return nil
}

func ValidateExtensions(extensions []Extension, maximum int) error {
	if len(extensions) > maximum {
		return errors.New("protocol item exceeds extension limit")
	}
	seen := make(map[string]struct{}, len(extensions))
	for _, extension := range extensions {
		if !namespacePattern.MatchString(extension.Namespace) || extension.Schema == "" || len(extension.Payload) == 0 {
			return errors.New("protocol extension identity or payload is invalid")
		}
		if _, duplicate := seen[extension.Namespace]; duplicate {
			return fmt.Errorf("duplicate protocol extension %q", extension.Namespace)
		}
		seen[extension.Namespace] = struct{}{}
		if _, err := CanonicalizeJSON(extension.Payload); err != nil {
			return fmt.Errorf("extension %s payload: %w", extension.Namespace, err)
		}
		if extension.Namespace == ReliabilityExtension {
			if err := validateReliabilityExtension(extension); err != nil {
				return err
			}
			continue
		}
		if extension.Namespace == ApplicationExtensionNamespace {
			if extension.Schema != ApplicationInvocationSchema && extension.Schema != ApplicationResultSchema {
				return errors.New("application extension schema is unsupported")
			}
			continue
		}
		if extension.Required {
			return fmt.Errorf("unknown required extension %q", extension.Namespace)
		}
	}
	return nil
}

func validateRequestVector(vector *RequestVectorInput) error {
	if vector == nil || !identifierPattern.MatchString(vector.VectorID) || !validDigest(vector.ExpectedSHA256) {
		return errors.New("request vector identity or digest is invalid")
	}
	canonical, err := hex.DecodeString(vector.CanonicalUTF8Hex)
	if err != nil || len(canonical) == 0 {
		return errors.New("request vector canonical bytes are invalid")
	}
	if DigestBytes(canonical) != vector.ExpectedSHA256 {
		return errors.New("request vector canonical bytes do not match expected digest")
	}
	return nil
}

func validateTrajectoryRefs(references []TrajectoryRef) error {
	for index, reference := range references {
		if strings.TrimSpace(reference.CanonicalSchema) == "" || strings.TrimSpace(reference.SourceFormat) == "" ||
			strings.TrimSpace(reference.MappingPolicyVersion) == "" {
			return fmt.Errorf("trajectory_refs[%d] identity is incomplete", index)
		}
		for field, value := range map[string]string{
			"source_digest": reference.SourceDigest, "trajectory_digest": reference.TrajectoryDigest,
			"accounting_digest": reference.AccountingDigest, "trace_envelope_digest": reference.TraceEnvelopeDigest,
			"mapping_report_digest": reference.MappingReportDigest,
		} {
			if !validDigest(value) {
				return fmt.Errorf("trajectory_refs[%d].%s is not a sha256 digest", index, field)
			}
		}
		if len(reference.Inline) > 0 {
			if _, err := CanonicalizeJSON(reference.Inline); err != nil {
				return fmt.Errorf("trajectory_refs[%d].inline: %w", index, err)
			}
		}
	}
	return nil
}

func validateReplay(replay ReplayEvidence) error {
	if !validDigest(replay.RequestFingerprint) || !validDigest(replay.ResponseDigest) || !validDigest(replay.EvidenceDigest) {
		return errors.New("replay evidence digest is invalid")
	}
	if replay.ReplayStatus != "exact" || replay.ReplayReason != "" {
		return errors.New("normative replay evidence must be exact without a fallback reason")
	}
	return nil
}

func validateAttestation(attestation AttestationEvidence) error {
	if attestation.SchemaVersion == "" || !validDigest(attestation.AttestationDigest) ||
		!validDigest(attestation.RequestFingerprint) {
		return errors.New("attestation evidence identity is invalid")
	}
	if attestation.State == "" || attestation.EvidenceCeiling == "" {
		return errors.New("attestation evidence state is empty")
	}
	if attestation.RouteLimitations == nil {
		return errors.New("attestation evidence omits route_limitations")
	}
	observed, err := time.Parse(time.RFC3339Nano, attestation.ObservedAt)
	if err != nil {
		return errors.New("attestation observed_at is not RFC3339")
	}
	expires, err := time.Parse(time.RFC3339Nano, attestation.ExpiresAt)
	if err != nil || !expires.After(observed) {
		return errors.New("attestation expiry is invalid")
	}
	return validateSortedUniqueStrings(attestation.RouteLimitations, "route_limitations")
}

func probability(value string) (*big.Rat, error) {
	rational, err := parseCanonicalDecimal(value)
	if err != nil {
		return nil, err
	}
	if rational.Sign() < 0 || rational.Cmp(big.NewRat(1, 1)) > 0 {
		return nil, fmt.Errorf("probability %q is outside [0,1]", value)
	}
	return rational, nil
}

func validDigest(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validateLimits(limits ResourceLimits) error {
	if limits.MaxMessageBytes <= 0 || limits.MaxMessageBytes > 8<<20 ||
		limits.MaxCaseBytes <= 0 || limits.MaxCaseBytes > 4<<20 ||
		limits.MaxCasesPerRun <= 0 || limits.MaxCasesPerRun > 10_000 ||
		limits.MaxDurationMillis <= 0 || limits.MaxDurationMillis > 60_000 ||
		limits.MaxExtensionsPerItem <= 0 || limits.MaxExtensionsPerItem > 32 {
		return errors.New("descriptor resource limits exceed protocol bounds")
	}
	return nil
}

func validateLevels(levels []ConformanceLevel) error {
	seen := make(map[ConformanceLevel]struct{}, len(levels))
	for _, level := range levels {
		if PermittedClaim(level) == "" {
			return fmt.Errorf("unknown conformance level %q", level)
		}
		if _, duplicate := seen[level]; duplicate {
			return fmt.Errorf("duplicate conformance level %q", level)
		}
		seen[level] = struct{}{}
	}
	return nil
}

func validateUniqueStrings(values []string, field string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			return fmt.Errorf("%s contains an empty value", field)
		}
		if _, duplicate := seen[value]; duplicate {
			return fmt.Errorf("%s contains duplicate %q", field, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateSortedUniqueStrings(values []string, field string) error {
	if err := validateUniqueStrings(values, field); err != nil {
		return err
	}
	if !sort.StringsAreSorted(values) {
		return fmt.Errorf("%s is not in canonical order", field)
	}
	return nil
}

func validateSortedUniquePositiveInts(values []int) error {
	if len(values) == 0 {
		return errors.New("rank set is empty")
	}
	previous := 0
	for _, value := range values {
		if value <= previous {
			return errors.New("ranks must be positive, unique, and sorted")
		}
		previous = value
	}
	return nil
}

func validCaseKind(kind CaseKind) bool {
	switch kind {
	case CaseCanonicalEncoding, CaseRequestFingerprint, CaseScoreEvidence,
		CaseDecisionEvidence, CaseReplayEvidence, CaseAttestation, CaseExtension,
		CaseCompatibility:
		return true
	default:
		return false
	}
}

func validScoreLetter(letter string) bool {
	return len(letter) == 1 && letter[0] >= 'A' && letter[0] <= 'T'
}

func supportedVersion(version string) bool {
	return version == CurrentVersion || version == PreviousMinorVersion
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func strictUnmarshal(raw json.RawMessage, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err != nil {
			return fmt.Errorf("decode trailing extension payload: %w", err)
		}
		return errors.New("extension payload contains trailing JSON")
	}
	return nil
}
