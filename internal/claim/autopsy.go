package claim

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/internal/lineage"
	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/profile"
	"github.com/Christopher-Schulze/evalwitness/internal/relation"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const AutopsySchemaVersion = "evalwitness.claim-autopsy.v1"

type AutopsyEvidenceRef struct {
	Name           string `json:"name"`
	TypeID         string `json:"type_id"`
	ComponentID    string `json:"component_id"`
	PayloadSHA256  string `json:"payload_sha256"`
	ArtifactDigest string `json:"artifact_digest"`
}

type AutopsyCount struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

type MethodGeneration struct {
	Generation         string               `json:"generation"`
	State              string               `json:"state"`
	Evidence           []AutopsyEvidenceRef `json:"evidence"`
	FrozenDenominators []AutopsyCount       `json:"frozen_denominators"`
	Observations       []string             `json:"observations"`
	NonClaims          []string             `json:"non_claims"`
	GuardingTests      []string             `json:"guarding_tests"`
}

type MethodTransition struct {
	From                 string   `json:"from"`
	To                   string   `json:"to"`
	Reason               string   `json:"reason"`
	EvidenceComponentIDs []string `json:"evidence_component_ids"`
	GuardingTest         string   `json:"guarding_test"`
}

type MethodIntegrityLane struct {
	Generations []MethodGeneration `json:"generations"`
	Transitions []MethodTransition `json:"transitions"`
	Current     string             `json:"current"`
	Boundary    string             `json:"boundary"`
}

type ClaimTransportLayer struct {
	Layer               string   `json:"layer"`
	ObjectDigest        string   `json:"object_digest"`
	RecordIDs           []string `json:"record_ids"`
	RequiredFields      []string `json:"required_fields"`
	DecisiveChannels    []string `json:"decisive_channels"`
	StructuredPresence  bool     `json:"structured_presence"`
	SemanticSufficiency bool     `json:"semantic_sufficiency"`
}

type ClaimTransportLane struct {
	ClaimID                       string                 `json:"claim_id"`
	CandidateID                   string                 `json:"candidate_id"`
	Evidence                      []AutopsyEvidenceRef   `json:"evidence"`
	Layers                        []ClaimTransportLayer  `json:"layers"`
	FailableProperty              string                 `json:"failable_property"`
	Executable                    string                 `json:"executable"`
	Operands                      []string               `json:"operands"`
	ObservableFailureCondition    string                 `json:"observable_failure_condition"`
	RequestFingerprint            string                 `json:"request_fingerprint"`
	CanonicalRequestLineageDigest string                 `json:"canonical_request_lineage_digest"`
	TerminalState                 lineage.TerminalState  `json:"terminal_state"`
	Freshness                     lineage.FreshnessState `json:"freshness"`
	Accepted                      bool                   `json:"accepted"`
	AuditDenominators             []AutopsyCount         `json:"audit_denominators"`
	ClaimBoundary                 []string               `json:"claim_boundary"`
	KnownLimitations              []string               `json:"known_limitations"`
	OutOfScopeUses                []string               `json:"out_of_scope_uses"`
	ProviderCalls                 int                    `json:"provider_calls"`
	ReleaseVerified               bool                   `json:"release_verified"`
}

type Autopsy struct {
	SchemaVersion       string              `json:"schema_version"`
	CapsuleID           string              `json:"capsule_id"`
	ManifestDigest      string              `json:"manifest_digest"`
	LedgerDigest        string              `json:"ledger_digest"`
	MethodIntegrity     MethodIntegrityLane `json:"method_integrity"`
	ClaimTransport      ClaimTransportLane  `json:"claim_transport"`
	CurrentClaimIDs     []string            `json:"current_claim_ids"`
	HistoricalClaimIDs  []string            `json:"historical_claim_ids"`
	DownstreamConsumers []string            `json:"downstream_consumers"`
	LifecycleInvariant  string              `json:"lifecycle_invariant"`
	Digest              string              `json:"digest"`
}

func BuildAutopsy(ctx context.Context, registry *capsule.Registry, manifest capsule.Manifest, payloads map[string][]byte, ledger Ledger) (Autopsy, error) {
	if _, err := VerifyLedger(ctx, registry, manifest, payloads, ledger); err != nil {
		return Autopsy{}, err
	}
	return buildAutopsy(manifest, payloads, ledger)
}

func buildAutopsy(manifest capsule.Manifest, payloads map[string][]byte, ledger Ledger) (Autopsy, error) {
	repairRef, repairRaw, err := autopsyComponent(manifest, payloads, mutation.ConstructRepairEvidenceSchemaVersion)
	if err != nil {
		return Autopsy{}, err
	}
	repair, err := mutation.DecodeConstructRepairEvidence(bytes.NewReader(repairRaw))
	if err != nil {
		return Autopsy{}, err
	}
	challengeRef, challengeRaw, err := autopsyComponent(manifest, payloads, mutation.ConstructChallengeSchemaVersion)
	if err != nil {
		return Autopsy{}, err
	}
	challenge, err := mutation.DecodeConstructChallengeEvidence(bytes.NewReader(challengeRaw))
	if err != nil {
		return Autopsy{}, err
	}
	auditRef, auditRaw, err := autopsyComponent(manifest, payloads, mutation.CorpusDevelopmentAuditSchemaVersionV3)
	if err != nil {
		return Autopsy{}, err
	}
	var audit mutation.CorpusDevelopmentAuditV3
	if err := protocol.DecodeStrict(auditRaw, &audit); err != nil {
		return Autopsy{}, err
	}
	releaseRef, releaseRaw, err := autopsyComponent(manifest, payloads, mutation.CorpusReleaseSchemaVersionV3)
	if err != nil {
		return Autopsy{}, err
	}
	var release mutation.CorpusReleaseV3
	if err := protocol.DecodeStrict(releaseRaw, &release); err != nil {
		return Autopsy{}, err
	}
	scarcityRef, scarcityRaw, err := autopsyComponent(manifest, payloads, relation.ScarcityPublicEvidenceSchemaVersion)
	if err != nil {
		return Autopsy{}, err
	}
	scarcity, err := relation.DecodeScarcityPublicEvidence(bytes.NewReader(scarcityRaw))
	if err != nil {
		return Autopsy{}, err
	}
	method, err := buildMethodIntegrityLane(
		autopsyEvidence(repairRef, repair.Digest), repair,
		autopsyEvidence(challengeRef, challenge.Digest), challenge,
		autopsyEvidence(auditRef, audit.Digest), audit,
		autopsyEvidence(releaseRef, release.Digest), release,
		autopsyEvidence(scarcityRef, scarcity.Digest), scarcity,
	)
	if err != nil {
		return Autopsy{}, err
	}
	transport, err := buildClaimTransportLane(manifest, payloads)
	if err != nil {
		return Autopsy{}, err
	}
	current, historical := ledgerLifecycleIDs(ledger)
	autopsy := Autopsy{
		SchemaVersion: AutopsySchemaVersion, CapsuleID: manifest.CapsuleID,
		ManifestDigest: manifest.ManifestDigest, LedgerDigest: ledger.Digest,
		MethodIntegrity: method, ClaimTransport: transport,
		CurrentClaimIDs: current, HistoricalClaimIDs: historical,
		DownstreamConsumers: []string{"TASK-037", "TASK-058", "TASK-059", "TASK-060", "TASK-062", "TASK-063", "TASK-064"},
		LifecycleInvariant:  "only claims resolving to admitted current evidence generations may enter current projections; historical claims remain inspectable and non-assertable",
	}
	autopsy.Digest, err = autopsyDigest(autopsy)
	if err != nil {
		return Autopsy{}, err
	}
	if err := autopsy.Validate(); err != nil {
		return Autopsy{}, err
	}
	// Wire profile lineage (TASK 058) into the autopsy's real flow.
	// Fail-closed: ProfileStepsView errors propagate. Gate StepsFromAutopsyView
	// on the 3-generation lane expected by the frozen v1→v2→v3 ladder so other
	// autopsy shapes (e.g. synthetic tests) don't hard-fail on a 058 nicety.
	view, err := ProfileStepsView(autopsy)
	if err != nil {
		return Autopsy{}, fmt.Errorf("profile view: %w", err)
	}
	if len(view.MethodIntegrity.Generations) == 3 {
		if _, err := profile.StepsFromAutopsyView(view); err != nil {
			return Autopsy{}, fmt.Errorf("profile lineage: %w", err)
		}
	}
	return autopsy, nil
}

func (autopsy Autopsy) Validate() error {
	if autopsy.SchemaVersion != AutopsySchemaVersion || !validDigest(autopsy.CapsuleID) ||
		!validDigest(autopsy.ManifestDigest) || !validDigest(autopsy.LedgerDigest) || !validDigest(autopsy.Digest) ||
		!slices.Equal(autopsy.DownstreamConsumers, []string{"TASK-037", "TASK-058", "TASK-059", "TASK-060", "TASK-062", "TASK-063", "TASK-064"}) ||
		autopsy.LifecycleInvariant != "only claims resolving to admitted current evidence generations may enter current projections; historical claims remain inspectable and non-assertable" ||
		!validSortedClaimIDs(autopsy.CurrentClaimIDs) || !validSortedClaimIDs(autopsy.HistoricalClaimIDs) {
		return errors.New("claim autopsy identity or lifecycle boundary is invalid")
	}
	if err := validateMethodIntegrityLane(autopsy.MethodIntegrity); err != nil {
		return err
	}
	if err := validateClaimTransportLane(autopsy.ClaimTransport); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(autopsy.CurrentClaimIDs)+len(autopsy.HistoricalClaimIDs))
	for _, id := range append(slices.Clone(autopsy.CurrentClaimIDs), autopsy.HistoricalClaimIDs...) {
		if _, duplicate := seen[id]; duplicate {
			return errors.New("claim autopsy repeats a claim across lifecycle lanes")
		}
		seen[id] = struct{}{}
	}
	digest, err := autopsyDigest(autopsy)
	if err != nil || digest != autopsy.Digest {
		return errors.New("claim autopsy digest is invalid")
	}
	return nil
}

func VerifyAutopsy(ctx context.Context, registry *capsule.Registry, manifest capsule.Manifest, payloads map[string][]byte, ledger Ledger, actual Autopsy) error {
	if err := actual.Validate(); err != nil {
		return err
	}
	expected, err := BuildAutopsy(ctx, registry, manifest, payloads, ledger)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(actual, expected) {
		return errors.New("claim autopsy differs from deterministic capsule and ledger recomputation")
	}
	return nil
}

func buildMethodIntegrityLane(
	repairRef AutopsyEvidenceRef,
	repair mutation.ConstructRepairEvidence,
	challengeRef AutopsyEvidenceRef,
	challenge mutation.ConstructChallengeEvidence,
	auditRef AutopsyEvidenceRef,
	audit mutation.CorpusDevelopmentAuditV3,
	releaseRef AutopsyEvidenceRef,
	release mutation.CorpusReleaseV3,
	scarcityRef AutopsyEvidenceRef,
	scarcity relation.ScarcityPublicEvidence,
) (MethodIntegrityLane, error) {
	nonClaims := append(slices.Clone(repair.ClaimBoundary.UnsupportedClaims), challenge.ClaimBoundary.UnsupportedClaims...)
	for _, item := range scarcity.Claims {
		if item.Status != relation.ScarcityPublicClaimSupported {
			nonClaims = append(nonClaims, item.Claim)
		}
	}
	nonClaims = sortedUnique(nonClaims)
	scarcitySupported := ""
	for _, item := range scarcity.Claims {
		if item.Status == relation.ScarcityPublicClaimSupported {
			scarcitySupported = item.Claim + ": " + item.Evidence
		}
	}
	if scarcitySupported == "" {
		return MethodIntegrityLane{}, errors.New("claim autopsy cannot locate the supported scarcity observation")
	}
	generations := []MethodGeneration{
		{
			Generation: "v1", State: "falsified", Evidence: []AutopsyEvidenceRef{repairRef},
			FrozenDenominators: []AutopsyCount{{Name: "corrected_rejections", Value: repair.Summary.CorrectedRejected}, {Name: "fixtures", Value: repair.Summary.Fixtures}, {Name: "legacy_false_acceptances", Value: repair.Summary.LegacyAccepted}},
			Observations:       []string{repair.ClaimBoundary.SupportedClaim}, NonClaims: sortedUnique(repair.ClaimBoundary.UnsupportedClaims),
			GuardingTests: []string{"TestConstructRepairCasesReproduceLegacyAcceptanceAndCorrectedRejection", "TestConstructRepairEvidenceCodecSchemaAndTamperRejection"},
		},
		{
			Generation: "v2", State: "superseded", Evidence: []AutopsyEvidenceRef{repairRef, challengeRef},
			FrozenDenominators: []AutopsyCount{
				{Name: "challenge_cases", Value: challenge.Summary.Cases}, {Name: "positive_controls", Value: challenge.Summary.PositiveControls},
				{Name: "shared_guards", Value: challenge.Summary.SharedGuards}, {Name: "v2_false_acceptances", Value: challenge.Summary.V2FalseAcceptances},
				{Name: "v3_repaired_negatives", Value: challenge.Summary.V3RepairedNegatives},
			},
			Observations: []string{challenge.ClaimBoundary.SupportedClaim}, NonClaims: sortedUnique(challenge.ClaimBoundary.UnsupportedClaims),
			GuardingTests: []string{"TestConstructChallengeCodecSchemaAndTamperRejection", "TestConstructChallengeReproducesV2FalsificationAndV3Repair"},
		},
		{
			Generation: "v3", State: "admitted_development", Evidence: []AutopsyEvidenceRef{challengeRef, auditRef, releaseRef, scarcityRef},
			FrozenDenominators: []AutopsyCount{
				{Name: "inferential_core_cases", Value: scarcity.InferentialCore.Cases}, {Name: "natural_applied_attempts", Value: audit.AppliedAttempts},
				{Name: "natural_rejected_attempts", Value: audit.RejectedAttempts}, {Name: "natural_selected_cases", Value: audit.SelectedCases},
				{Name: "natural_source_tasks", Value: audit.SourceTasks}, {Name: "natural_sources", Value: len(audit.Sources)},
				{Name: "natural_total_attempts", Value: audit.TotalAttempts}, {Name: "release_cases", Value: release.SelectedCases},
				{Name: "scarcity_admitted", Value: scarcity.Availability.Admitted}, {Name: "scarcity_attempted", Value: scarcity.Availability.Attempted},
				{Name: "scarcity_rejected", Value: scarcity.Availability.Rejected}, {Name: "scarcity_shortfall", Value: scarcity.Availability.Shortfall},
				{Name: "scarcity_target", Value: scarcity.Availability.Target}, {Name: "scarcity_test_role", Value: scarcity.StudyRoles.Test},
			},
			Observations: []string{challenge.ClaimBoundary.SupportedClaim, scarcitySupported}, NonClaims: nonClaims,
			GuardingTests: []string{"TestRelationV3PrimaryFamilySetExcludesOnlyScarcitySentinel", "TestScarcityPublicEvidenceCodecSchemaAndParentVerification", "TestScarcityPublicBriefRejectsCrossBoundCorpus"},
		},
	}
	transitions := []MethodTransition{
		{
			From: "v1", To: "v2", Reason: "three frozen v1 acceptances are rejected by v2 under closed reasons",
			EvidenceComponentIDs: []string{repairRef.ComponentID}, GuardingTest: "TestConstructRepairCasesReproduceLegacyAcceptanceAndCorrectedRejection",
		},
		{
			From: "v2", To: "v3", Reason: "five frozen v2 false acceptances are rejected by v3 while six positive controls and three shared guards are preserved",
			EvidenceComponentIDs: []string{challengeRef.ComponentID}, GuardingTest: "TestConstructChallengeReproducesV2FalsificationAndV3Repair",
		},
	}
	return MethodIntegrityLane{
		Generations: generations, Transitions: transitions, Current: "v3",
		Boundary: "v3 is admitted only as provider-free development evidence; the 280-case inferential core and 3-of-40 scarcity result remain separate and zero test-role scarcity cases forbid held-out validity claims",
	}, nil
}

func buildClaimTransportLane(manifest capsule.Manifest, payloads map[string][]byte) (ClaimTransportLane, error) {
	candidateRef, candidateRaw, err := autopsyComponent(manifest, payloads, lineage.CandidateSchemaVersion)
	if err != nil {
		return ClaimTransportLane{}, err
	}
	var candidate lineage.LineageCandidate
	if err := protocol.DecodeStrict(candidateRaw, &candidate); err != nil {
		return ClaimTransportLane{}, err
	}
	if err := candidate.Validate(); err != nil {
		return ClaimTransportLane{}, err
	}
	assessmentRef, assessmentRaw, err := autopsyComponent(manifest, payloads, lineage.AssessmentSchemaVersion)
	if err != nil {
		return ClaimTransportLane{}, err
	}
	var assessment lineage.LineageAssessment
	if err := protocol.DecodeStrict(assessmentRaw, &assessment); err != nil {
		return ClaimTransportLane{}, err
	}
	if err := assessment.Validate(); err != nil {
		return ClaimTransportLane{}, err
	}
	bomRef, bomRaw, err := autopsyComponent(manifest, payloads, lineage.BOMSchemaVersion)
	if err != nil {
		return ClaimTransportLane{}, err
	}
	bom, err := lineage.DecodeOfflineBOM(bytes.NewReader(bomRaw))
	if err != nil {
		return ClaimTransportLane{}, err
	}
	auditRef, auditRaw, err := autopsyComponent(manifest, payloads, lineage.AuditSchemaVersion)
	if err != nil {
		return ClaimTransportLane{}, err
	}
	audit, err := lineage.DecodeOfflineAudit(bytes.NewReader(auditRaw))
	if err != nil {
		return ClaimTransportLane{}, err
	}
	cardRef, cardRaw, err := autopsyComponent(manifest, payloads, lineage.DatasetCardSchemaVersion)
	if err != nil {
		return ClaimTransportLane{}, err
	}
	card, err := lineage.DecodeDevelopmentDatasetCard(bytes.NewReader(cardRaw))
	if err != nil {
		return ClaimTransportLane{}, err
	}
	releaseRef, releaseRaw, err := autopsyComponent(manifest, payloads, lineage.ReleaseSchemaVersion)
	if err != nil {
		return ClaimTransportLane{}, err
	}
	release, err := lineage.DecodeDevelopmentRelease(bytes.NewReader(releaseRaw))
	if err != nil {
		return ClaimTransportLane{}, err
	}
	layers := make([]ClaimTransportLayer, len(candidate.Layers))
	for index, layer := range candidate.Layers {
		layers[index] = ClaimTransportLayer{
			Layer: layer.Layer, ObjectDigest: layer.ObjectDigest, RecordIDs: slices.Clone(layer.RecordIDs),
			RequiredFields: slices.Clone(layer.RequiredFields), DecisiveChannels: slices.Clone(layer.DecisiveChannels),
			StructuredPresence: layer.StructuredPresence, SemanticSufficiency: layer.SemanticSufficiency,
		}
	}
	return ClaimTransportLane{
		ClaimID: candidate.ClaimID, CandidateID: candidate.CandidateID,
		Evidence: []AutopsyEvidenceRef{
			autopsyEvidence(candidateRef, candidate.Header.Digest), autopsyEvidence(assessmentRef, assessment.Header.Digest),
			autopsyEvidence(bomRef, bom.Header.Digest), autopsyEvidence(auditRef, audit.Header.Digest),
			autopsyEvidence(cardRef, card.Header.Digest), autopsyEvidence(releaseRef, release.Header.Digest),
		},
		Layers: layers, FailableProperty: bom.Evidence.FailableProperty, Executable: bom.Evidence.Executable,
		Operands: slices.Clone(bom.Evidence.Operands), ObservableFailureCondition: bom.Evidence.ObservableFailureCondition,
		RequestFingerprint: candidate.RequestFingerprint, CanonicalRequestLineageDigest: candidate.CanonicalRequestLineageDigest,
		TerminalState: assessment.TerminalState, Freshness: assessment.Freshness.State, Accepted: bom.Accepted,
		AuditDenominators: []AutopsyCount{
			{Name: "considered_task_groups", Value: audit.ConsideredTaskGroups}, {Name: "excluded_task_groups", Value: audit.ExcludedTaskGroups},
			{Name: "included_task_groups", Value: audit.IncludedTaskGroups}, {Name: "unresolved_task_groups", Value: audit.UnresolvedTaskGroups},
		},
		ClaimBoundary: slices.Clone(audit.ClaimBoundary), KnownLimitations: slices.Clone(card.KnownLimitations),
		OutOfScopeUses: slices.Clone(card.OutOfScopeUses), ProviderCalls: release.ProviderCallsRequired,
		ReleaseVerified: release.AllFilesVerified,
	}, nil
}

func validateMethodIntegrityLane(lane MethodIntegrityLane) error {
	if lane.Current != "v3" || lane.Boundary != "v3 is admitted only as provider-free development evidence; the 280-case inferential core and 3-of-40 scarcity result remain separate and zero test-role scarcity cases forbid held-out validity claims" ||
		len(lane.Generations) != 3 || len(lane.Transitions) != 2 {
		return errors.New("claim autopsy method-integrity lane identity is invalid")
	}
	expectedGenerations := []struct{ id, state string }{{"v1", "falsified"}, {"v2", "superseded"}, {"v3", "admitted_development"}}
	for index, generation := range lane.Generations {
		expected := expectedGenerations[index]
		if generation.Generation != expected.id || generation.State != expected.state || len(generation.Evidence) == 0 ||
			!validAutopsyCounts(generation.FrozenDenominators) || len(generation.Observations) == 0 ||
			generation.NonClaims == nil || !slices.IsSorted(generation.NonClaims) || len(generation.GuardingTests) == 0 {
			return fmt.Errorf("claim autopsy generation %q is invalid", generation.Generation)
		}
		for _, evidence := range generation.Evidence {
			if err := evidence.Validate(); err != nil {
				return err
			}
		}
	}
	for index, transition := range lane.Transitions {
		if transition.From != expectedGenerations[index].id || transition.To != expectedGenerations[index+1].id || transition.Reason == "" ||
			len(transition.EvidenceComponentIDs) != 1 || !validDigest(transition.EvidenceComponentIDs[0]) || transition.GuardingTest == "" {
			return errors.New("claim autopsy method transition is invalid")
		}
	}
	return nil
}

func validateClaimTransportLane(lane ClaimTransportLane) error {
	expectedLayers := []string{"runtime_witness", "native_export", "canonical_graph", "retained_bundle", "verifier_request"}
	if lane.ClaimID == "" || lane.CandidateID == "" || len(lane.Evidence) != 6 || len(lane.Layers) != len(expectedLayers) ||
		lane.FailableProperty == "" || lane.Executable == "" || lane.ObservableFailureCondition == "" ||
		!validDigest(lane.RequestFingerprint) || !validDigest(lane.CanonicalRequestLineageDigest) ||
		lane.RequestFingerprint == lane.CanonicalRequestLineageDigest || !lane.Accepted || lane.Freshness != lineage.FreshnessCurrent ||
		!validAutopsyCounts(lane.AuditDenominators) || lane.ClaimBoundary == nil || lane.KnownLimitations == nil || lane.OutOfScopeUses == nil ||
		lane.ProviderCalls != 0 || !lane.ReleaseVerified {
		return errors.New("claim autopsy transport lane identity or evidence boundary is invalid")
	}
	for _, evidence := range lane.Evidence {
		if err := evidence.Validate(); err != nil {
			return err
		}
	}
	for index, layer := range lane.Layers {
		if layer.Layer != expectedLayers[index] || !validDigest(layer.ObjectDigest) || len(layer.RecordIDs) == 0 ||
			len(layer.RequiredFields) == 0 || len(layer.DecisiveChannels) == 0 || !layer.StructuredPresence || !layer.SemanticSufficiency {
			return errors.New("claim autopsy transport layer is incomplete or out of order")
		}
	}
	return nil
}

func (evidence AutopsyEvidenceRef) Validate() error {
	if evidence.Name == "" || evidence.TypeID == "" || !validDigest(evidence.ComponentID) ||
		!validDigest(evidence.PayloadSHA256) || !validDigest(evidence.ArtifactDigest) {
		return errors.New("claim autopsy evidence reference is invalid")
	}
	return nil
}

func autopsyComponent(manifest capsule.Manifest, payloads map[string][]byte, typeID string) (capsule.ComponentRecord, []byte, error) {
	var found *capsule.ComponentRecord
	for index := range manifest.Components {
		if manifest.Components[index].TypeID != typeID {
			continue
		}
		if found != nil {
			return capsule.ComponentRecord{}, nil, fmt.Errorf("claim autopsy requires one %q component", typeID)
		}
		copy := manifest.Components[index]
		found = &copy
	}
	if found == nil {
		return capsule.ComponentRecord{}, nil, fmt.Errorf("claim autopsy is missing %q evidence", typeID)
	}
	raw, ok := payloads[found.Payload.Digest]
	if !ok {
		return capsule.ComponentRecord{}, nil, fmt.Errorf("claim autopsy payload %q is missing", found.Payload.Digest)
	}
	return *found, slices.Clone(raw), nil
}

func autopsyEvidence(record capsule.ComponentRecord, artifactDigest string) AutopsyEvidenceRef {
	return AutopsyEvidenceRef{
		Name: record.Name, TypeID: record.TypeID, ComponentID: record.ComponentID,
		PayloadSHA256: record.Payload.Digest, ArtifactDigest: artifactDigest,
	}
}

func ledgerLifecycleIDs(ledger Ledger) ([]string, []string) {
	current := make([]string, 0, len(ledger.Claims))
	historical := make([]string, 0, len(ledger.Claims))
	for _, item := range ledger.Claims {
		if item.Status.Assertable() {
			current = append(current, item.ClaimID)
		} else {
			historical = append(historical, item.ClaimID)
		}
	}
	return current, historical
}

func validAutopsyCounts(values []AutopsyCount) bool {
	if len(values) == 0 {
		return false
	}
	previous := ""
	for _, value := range values {
		if value.Name == "" || value.Name <= previous || value.Value < 0 {
			return false
		}
		previous = value.Name
	}
	return true
}

func validSortedClaimIDs(values []string) bool {
	if values == nil {
		return false
	}
	previous := ""
	for _, value := range values {
		if !validClaimID(value) || value <= previous {
			return false
		}
		previous = value
	}
	return true
}

func sortedUnique(values []string) []string {
	result := slices.Clone(values)
	sort.Strings(result)
	return slices.Compact(result)
}

func autopsyDigest(autopsy Autopsy) (string, error) {
	autopsy.Digest = ""
	return protocol.Digest(autopsy)
}
