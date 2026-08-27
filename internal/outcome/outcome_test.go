package outcome

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

func TestOutcomeEvidenceSourcesRemainSeparateAndRevisionIsAppendOnly(t *testing.T) {
	benchmark := mustEvidence(t, Evidence{ID: "benchmark", Kind: EvidenceBenchmarkReward, State: StateSolved, ArtifactDigest: digestText("reward"), ObservedAt: "2026-08-09T08:00:00Z", Limitation: "benchmark reward may be incomplete", ParentDigests: []string{}})
	rerun := mustEvidence(t, Evidence{ID: "rerun", Kind: EvidenceIndependentRun, State: StateUnsolved, ArtifactDigest: digestText("run"), ValidatorID: "pinned-tests.v1", ObservedAt: "2026-08-09T09:00:00Z", Independent: true, Limitation: "tests do not prove full task intent", ParentDigests: []string{benchmark.Digest}})
	record, err := SealRecord(Record{
		TaskAlias: "task-a", Revision: 1, Evidence: []Evidence{benchmark, rerun}, Resolution: StateUnsolved,
		ResolutionBasis: []string{"rerun"}, Limitations: []string{"task intent not fully executable"}, AuthorID: "review-lead", RevisionReason: "initial triangulation",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(record.Evidence) != 2 || record.Evidence[0].State == record.Evidence[1].State {
		t.Fatal("contradictory benchmark and rerun evidence was overwritten")
	}
	revision, err := BuildRevision(record, record.Evidence, StateIndeterminate, []string{"benchmark", "rerun"}, []string{"validated contradiction remains unresolved"}, "review-lead", "preserve disagreement")
	if err != nil {
		t.Fatal(err)
	}
	if revision.ParentDigest != record.Digest || revision.Revision != 2 || revision.Digest == record.Digest {
		t.Fatal("outcome revision did not preserve immutable parent lineage")
	}
}

func TestBlindPacketLeakageFailsClosed(t *testing.T) {
	request := BlindBuildRequest{
		SchemaVersion: BlindBuildSchemaVersion, PlanDigest: digestText("plan"), TaskAlias: "task-opaque",
		Evidence:        []PacketEvidence{{Slot: "A", Kind: "trajectory", Content: "redacted evidence", ContentDigest: digestText("redacted evidence"), License: "MIT", Limitation: "excerpt"}},
		RubricQuestions: []string{"Is the available evidence sufficient?"}, PrivacyClass: "public", PublicReleasable: true,
		SourceCaseDigest: digestText("case"), Condition: "presentation-invariance", ExpectedRelation: "quality_equal",
		SlotMappings: []SlotMapping{{Slot: "A", SourceDigest: digestText("source-a")}}, BlindingKeyID: "key-v1",
		ForbiddenValues: []string{"deepseek", "original_better"},
	}
	key := []byte("0123456789abcdef0123456789abcdef")
	packet, mapping, err := BuildBlindedPacketFromRequest(request, key)
	if err != nil {
		t.Fatal(err)
	}
	if mapping.PacketID != packet.PacketID || mapping.SourceTaskAlias != request.TaskAlias || mapping.Condition == "" || !validOpaqueTaskAlias(packet.TaskAlias) ||
		!validOpaqueSlot(packet.Evidence[0].Slot) || mapping.SlotMappings[0].Slot != packet.Evidence[0].Slot {
		t.Fatal("private mapping did not bind the opaque packet")
	}
	encoded, err := EncodeIndented(packet)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{request.TaskAlias, "presentation-invariance", "quality_equal", "key-v1", digestText("case"), "\"slot\": \"A\""} {
		if strings.Contains(string(encoded), private) {
			t.Fatalf("public packet leaked private value %q", private)
		}
	}
	tampered := packet
	tampered.Evidence[0].Content = "DeepSeek selected candidate A"
	tampered.Evidence[0].ContentDigest = digestText(tampered.Evidence[0].Content)
	tampered, err = SealBlindPacket(tampered, packet.PacketID)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidatePacketLeakage(tampered, []string{"deepseek"}); err == nil {
		t.Fatal("blind packet accepted evaluator identity leakage")
	}
	if _, _, err := BuildBlindedPacketFromRequest(request, []byte("short")); err == nil {
		t.Fatal("blind packet accepted an undersized HMAC key")
	}
	secondRequest := request
	secondRequest.SourceCaseDigest = digestText("case-two")
	second, _, err := BuildBlindedPacketFromRequest(secondRequest, key)
	if err != nil || second.PacketID == packet.PacketID || second.TaskAlias == packet.TaskAlias || second.Evidence[0].Slot == packet.Evidence[0].Slot {
		t.Fatal("HMAC identities did not change across source cases")
	}
	badSlots := request
	badSlots.SlotMappings = []SlotMapping{{Slot: "B", SourceDigest: digestText("source-a")}}
	if _, _, err := BuildBlindedPacketFromRequest(badSlots, key); err == nil {
		t.Fatal("blind packet accepted mismatched public and private slots")
	}
}

func TestResolutionRequiresIndependentTieBreak(t *testing.T) {
	packetID := "packet-" + digestText("packet-a")
	left := mustLabel(t, packetID, "reviewer-1", 1, StateSolved)
	right := mustLabel(t, packetID, "reviewer-2", 2, StateUnsolved)
	resolution, err := ResolveLabels([]Label{left, right}, nil, "2026-08-09T10:00:00Z", "third reviewer or unresolved")
	if err != nil {
		t.Fatal(err)
	}
	if resolution.State != StateIndeterminate || resolution.AgreementState != "unresolved_disagreement" {
		t.Fatalf("unresolved disagreement = %#v", resolution)
	}
	tie := mustLabel(t, packetID, "reviewer-3", 3, StateSolved)
	resolution, err = ResolveLabels([]Label{left, right}, &tie, "2026-08-09T10:00:00Z", "third reviewer or unresolved")
	if err != nil || resolution.State != StateSolved || resolution.TieBreakLabelDigest != tie.Digest {
		t.Fatalf("independent tie break failed: resolution=%#v err=%v", resolution, err)
	}
	tie.AdjudicatorAlias = left.AdjudicatorAlias
	tie, err = SealLabel(tie)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveLabels([]Label{left, right}, &tie, "2026-08-09T10:00:00Z", "third reviewer or unresolved"); err == nil {
		t.Fatal("resolution accepted a primary reviewer as tie breaker")
	}
	duplicateSlot := mustLabel(t, packetID, "reviewer-2", 1, StateUnsolved)
	if _, err := ResolveLabels([]Label{left, duplicateSlot}, nil, "2026-08-09T10:00:00Z", "third reviewer or unresolved"); err == nil {
		t.Fatal("resolution accepted two primary labels for the same reviewer slot")
	}
}

func TestAgreementReportsRawKappaPrevalenceAndClusterInterval(t *testing.T) {
	pairs := []AgreementPair{
		{PacketID: "packet-" + digestText("p1"), TaskGroupID: "t1", Left: StateSolved, Right: StateSolved},
		{PacketID: "packet-" + digestText("p2"), TaskGroupID: "t2", Left: StateSolved, Right: StateUnsolved},
		{PacketID: "packet-" + digestText("p3"), TaskGroupID: "t3", Left: StateUnsolved, Right: StateUnsolved},
		{PacketID: "packet-" + digestText("p4"), TaskGroupID: "t4", Left: StateUnsolved, Right: StateSolved},
	}
	report, err := ComputeAgreement(pairs, 10_000, "frozen-bootstrap-seed")
	if err != nil {
		t.Fatal(err)
	}
	if report.RawAgreement != 0.5 || report.CohenKappa != 0 || !report.CohenKappaDefined || report.DefinedKappaReplicates == 0 || len(report.LabelPrevalence) != 2 {
		t.Fatalf("agreement report = %#v", report)
	}
	identical := []AgreementPair{
		{PacketID: "packet-" + digestText("p1"), TaskGroupID: "t1", Left: StateSolved, Right: StateSolved},
		{PacketID: "packet-" + digestText("p2"), TaskGroupID: "t2", Left: StateSolved, Right: StateSolved},
	}
	pathological, err := ComputeAgreement(identical, 10_000, "prevalence-pathology")
	if err != nil || pathological.CohenKappaDefined {
		t.Fatalf("prevalence-degenerate kappa was not explicit: report=%#v err=%v", pathological, err)
	}
}

func TestOutcomePreservationRejectsChangedOrIndeterminateState(t *testing.T) {
	source := minimalRecord(t, "task", StateSolved, "source")
	same := minimalRecord(t, "task", StateSolved, "same")
	preservation, err := EvaluatePreservation(source, same, "independent rerun")
	if err != nil || !preservation.Admissible {
		t.Fatalf("matching decisive outcomes were not admissible: %#v %v", preservation, err)
	}
	changed := minimalRecord(t, "task", StateUnsolved, "changed")
	preservation, err = EvaluatePreservation(source, changed, "independent rerun")
	if err != nil || preservation.Admissible || !slices.Contains(preservation.InadmissibilityReasons, "outcome_state_changed") {
		t.Fatalf("changed outcome was admitted: %#v %v", preservation, err)
	}
}

func TestIndependentOutcomeExecutionSeparatesTaskFailureFromInfrastructureFailure(t *testing.T) {
	if mode := os.Getenv("EVALWITNESS_OUTCOME_EXECUTION_HELPER"); mode != "" {
		switch mode {
		case "pass":
			fmt.Print("tests passed")
			os.Exit(0)
		case "fail":
			fmt.Print("tests failed")
			os.Exit(3)
		case "output-limit":
			fmt.Print(strings.Repeat("x", 1024))
			os.Exit(0)
		case "timeout":
			time.Sleep(2 * time.Second)
			os.Exit(0)
		case "cancel":
			time.Sleep(2 * time.Second)
			os.Exit(0)
		default:
			os.Exit(4)
		}
	}
	if runtime.GOOS == "windows" {
		t.Skip("process-tree timeout semantics are covered by platform-specific mutation harness tests")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		mode          string
		timeoutMillis int
		maximumOutput int
		cancel        bool
		want          State
		wantFailure   mutation.HermeticFailure
	}{
		{mode: "pass", timeoutMillis: 10_000, maximumOutput: 4_096, want: StateSolved},
		{mode: "fail", timeoutMillis: 10_000, maximumOutput: 4_096, want: StateUnsolved},
		{mode: "output-limit", timeoutMillis: 10_000, maximumOutput: 16, want: StateEnvironmentFail, wantFailure: mutation.HermeticFailureOutputLimit},
		{mode: "timeout", timeoutMillis: 50, maximumOutput: 4_096, want: StateEnvironmentFail, wantFailure: mutation.HermeticFailureTimeout},
		{mode: "cancel", timeoutMillis: 10_000, maximumOutput: 4_096, cancel: true, want: StateEnvironmentFail, wantFailure: mutation.HermeticFailureExecution},
	} {
		t.Run(fixture.mode, func(t *testing.T) {
			validator := mutation.TrustedValidator{
				Spec:       mutation.ValidatorSpec{ID: "outcome-fixture-" + fixture.mode, Version: "v1", Kind: mutation.ValidationHermetic, TimeoutMillis: fixture.timeoutMillis, MaximumOutputBytes: fixture.maximumOutput},
				Executable: executable, Arguments: []string{"-test.run=TestIndependentOutcomeExecutionSeparatesTaskFailureFromInfrastructureFailure"},
				Environment: mutation.SortedEnvironment(map[string]string{"EVALWITNESS_OUTCOME_EXECUTION_HELPER": fixture.mode}), PassingExitCode: 0,
			}
			validator.Spec.ContractDigest, err = mutation.TrustedValidatorContractDigest(validator)
			if err != nil {
				t.Fatal(err)
			}
			registry, err := mutation.NewHermeticRegistry([]mutation.TrustedValidator{validator})
			if err != nil {
				t.Fatal(err)
			}
			executionContext := context.Background()
			if fixture.cancel {
				cancelledContext, cancel := context.WithCancel(executionContext)
				cancel()
				executionContext = cancelledContext
			}
			log, err := RunIndependentOutcome(executionContext, registry, "task-opaque", validator.Spec, mutation.TaskEnvironment{
				ID: "fixture", Revision: "sha256:fixture", Root: t.TempDir(), Disposable: true, NetworkDisabled: true,
			}, "2026-08-09T12:00:00Z", "fixture validates only process outcome")
			if err != nil {
				t.Fatal(err)
			}
			if log.Outcome != fixture.want || log.Failure != fixture.wantFailure {
				t.Fatalf("outcome execution = %#v", log)
			}
			evidence, err := EvidenceFromExecution(log, "independent-rerun", false)
			if err != nil || evidence.State != fixture.want || evidence.Kind != EvidenceIndependentRun {
				t.Fatalf("execution evidence = %#v err=%v", evidence, err)
			}
		})
	}
}

func TestOutcomeCodecsAndSchemasAreStrict(t *testing.T) {
	plan := validPlan(t)
	encoded, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodePlan(strings.NewReader(string(encoded))); err != nil {
		t.Fatal(err)
	}
	unknown := strings.TrimSuffix(string(encoded), "}") + `,"unknown":true}`
	if _, err := DecodePlan(strings.NewReader(unknown)); err == nil {
		t.Fatal("outcome codec accepted unknown field")
	}
	for _, document := range []string{
		"plan", "record", "record-draft", "evidence", "evidence-draft", "blind-build-request", "blind-packet", "private-mapping", "label", "label-draft", "resolution", "agreement",
		"preservation", "sample-commitment", "pilot-sample-v1", "pilot-readiness-v1", "pilot-sample", "pilot-readiness", "pilot-source-binding", "pilot-private-materials", "pilot-inspection", "natural-inventory-request", "natural-inventory", "executable-log", "qualification-set", "qualification-report",
		"review-bundle", "reviewer-record", "review-assignment", "label-batch", "mapping-reveal", "adjudication-ledger", "reviewer-handbook", "reviewer-kit",
		"blinding-protocol", "blinding-probe", "blinding-probe-batch", "blinding-analysis", "rubric-ambiguity-analysis", "source-audit",
	} {
		schema, err := Schema(document)
		if err != nil || schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
			t.Fatalf("outcome schema %q: %v", document, err)
		}
	}
	readinessSchema, err := Schema("pilot-readiness")
	if err != nil || readinessSchema["$id"] != "https://evalwitness.dev/schemas/outcome-pilot-readiness.v3.json" {
		t.Fatalf("outcome pilot readiness schema did not preserve its v3 identity: %#v err=%v", readinessSchema["$id"], err)
	}
}

func TestQualificationSetScoresOutcomeAndReasonCodes(t *testing.T) {
	set, err := DefaultQualificationSet()
	if err != nil {
		t.Fatal(err)
	}
	labels := make([]Label, 0, len(set.Cases))
	for _, item := range set.Cases {
		label, err := SealLabel(Label{
			PacketID: item.Packet.PacketID, AdjudicatorAlias: "reviewer-qualified", ReviewerSlot: 1,
			PrimaryOutcome: item.ExpectedOutcome, TaskSatisfaction: RatingUnclear, TechnicalCorrectness: RatingUnclear,
			VerificationQuality: RatingUnclear, HarmfulSideEffects: RatingNotApplicable, EvidenceSufficiency: RatingUnclear,
			ReasonCodes: append([]ReasonCode(nil), item.RequiredReasonCodes...), SubmittedAt: "2026-08-09T13:00:00Z",
			RubricVersion: set.RubricVersion, QualificationDigest: set.Digest, ConflictsOfInterest: []string{},
		})
		if err != nil {
			t.Fatal(err)
		}
		labels = append(labels, label)
	}
	report, err := ScoreQualification(set, labels)
	if err != nil || !report.Qualified || report.Score != 1 || report.PassedCases != len(set.Cases) {
		t.Fatalf("qualification report = %#v err=%v", report, err)
	}
	labels[0].PrimaryOutcome = StateUnsolved
	labels[0], err = SealLabel(labels[0])
	if err != nil {
		t.Fatal(err)
	}
	report, err = ScoreQualification(set, labels)
	if err != nil || !report.Qualified || report.Score != 0.8 {
		t.Fatalf("qualification threshold report = %#v err=%v", report, err)
	}
	labels[1].ReasonCodes = []ReasonCode{ReasonEvidenceInsufficient}
	labels[1], err = SealLabel(labels[1])
	if err != nil {
		t.Fatal(err)
	}
	report, err = ScoreQualification(set, labels)
	if err != nil || report.Qualified || report.Score != 0.6 {
		t.Fatalf("qualification reason-code failure = %#v err=%v", report, err)
	}
}

func TestStudyResolutionRequiresPassingReviewerQualification(t *testing.T) {
	set, err := DefaultQualificationSet()
	if err != nil {
		t.Fatal(err)
	}
	qualificationLabels := make([]Label, 0, len(set.Cases))
	for _, item := range set.Cases {
		qualificationLabels = append(qualificationLabels, mustQualificationLabel(t, set, item, "reviewer-1"))
	}
	report, err := ScoreQualification(set, qualificationLabels)
	if err != nil {
		t.Fatal(err)
	}
	packetID := "packet-" + digestText("qualified-study-packet")
	left := mustLabel(t, packetID, "reviewer-1", 1, StateSolved)
	left.QualificationDigest = report.Digest
	left, err = SealLabel(left)
	if err != nil {
		t.Fatal(err)
	}
	right := mustLabel(t, packetID, "reviewer-2", 2, StateSolved)
	if _, err := ResolveQualifiedLabels([]Label{left, right}, []QualificationReport{report, report}, nil, nil, "2026-08-09T14:00:00Z", "qualified primaries"); err == nil {
		t.Fatal("qualified resolution accepted a report belonging to another reviewer")
	}
	reviewerTwoLabels := make([]Label, 0, len(set.Cases))
	for _, item := range set.Cases {
		reviewerTwoLabels = append(reviewerTwoLabels, mustQualificationLabel(t, set, item, "reviewer-2"))
	}
	reviewerTwoReport, err := ScoreQualification(set, reviewerTwoLabels)
	if err != nil {
		t.Fatal(err)
	}
	right.QualificationDigest = reviewerTwoReport.Digest
	right, err = SealLabel(right)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveQualifiedLabels([]Label{left, right}, []QualificationReport{report, reviewerTwoReport}, nil, nil, "2026-08-09T14:00:00Z", "qualified primaries"); err != nil {
		t.Fatal(err)
	}
}

func TestGovernedOutcomePlanMatchesExecutableDefault(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "..", "eval", "governance", "outcome-adjudication-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close outcome adjudication plan: %v", err)
		}
	}()
	governed, err := DecodePlan(file)
	if err != nil {
		t.Fatal(err)
	}
	executable, err := DefaultPlan()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(governed, executable) {
		t.Fatal("governed outcome adjudication plan drifted from executable default")
	}
}

func TestGovernedMutationSampleCommitmentIsSealed(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "..", "eval", "governance", "outcome-mutation-sample-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close outcome mutation sample: %v", err)
		}
	}()
	commitment, err := DecodeSampleCommitment(file)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := DefaultPlan()
	if err != nil {
		t.Fatal(err)
	}
	if commitment.SelectedCases != 31 || len(commitment.FamilyCounts) != 8 || commitment.PlanDigest != plan.Digest {
		t.Fatalf("governed outcome mutation sample is incomplete: %#v", commitment)
	}
}

func TestGovernedNaturalInventoryMatchesDevelopmentArtifactsWithoutTaskReuse(t *testing.T) {
	requestFile, err := os.Open(filepath.Join("..", "..", "eval", "governance", "outcome-natural-inventory-request-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	request, err := DecodeNaturalInventoryRequest(requestFile)
	_ = requestFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	governedRequest, err := DefaultNaturalInventoryRequest()
	if err != nil || !reflect.DeepEqual(request, governedRequest) {
		t.Fatalf("governed natural request drifted from executable default: %v", err)
	}
	inventoryFile, err := os.Open(filepath.Join("..", "..", "eval", "governance", "outcome-natural-inventory-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	governed, err := DecodeNaturalInventory(inventoryFile)
	_ = inventoryFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := DefaultPlan()
	if err != nil {
		t.Fatal(err)
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(filepath.Join(workingDirectory, "..", "..")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(workingDirectory) })
	generated, err := BuildNaturalInventory(plan, request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(governed, generated) || generated.Status != "incomplete" || generated.SelectedCases != 36 || len(generated.Shortfalls) != 2 {
		t.Fatalf("governed natural inventory drifted: %#v", generated)
	}
	seen := make(map[string]struct{}, len(generated.Selections))
	for _, selection := range generated.Selections {
		if _, duplicate := seen[selection.TaskGroupDigest]; duplicate {
			t.Fatal("natural inventory reused a task group across strata")
		}
		seen[selection.TaskGroupDigest] = struct{}{}
	}
	tampered := generated
	tampered.Selections = append([]NaturalSelection(nil), generated.Selections...)
	tampered.Selections[1].TaskGroupDigest = tampered.Selections[0].TaskGroupDigest
	identities := make([]string, len(tampered.Selections))
	for index, selection := range tampered.Selections {
		identities[index] = selection.Stratum + "\x00" + selection.CaseDigest
	}
	tampered.SelectionDigest = digestText(joinWithNUL(identities))
	if _, err := SealNaturalInventory(tampered); err == nil {
		t.Fatal("natural inventory accepted task-group reuse after resealing")
	}
}

func TestGovernedQualificationSetMatchesExecutableDefault(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "..", "eval", "governance", "outcome-qualification-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close outcome qualification set: %v", err)
		}
	}()
	governed, err := DecodeQualificationSet(file)
	if err != nil {
		t.Fatal(err)
	}
	executable, err := DefaultQualificationSet()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(governed, executable) {
		t.Fatal("governed outcome qualification set drifted from executable default")
	}
}

func FuzzOutcomeStrictPlanDecoder(f *testing.F) {
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"schema_version":"unknown"}`))
	f.Add([]byte("[]\n{}"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<20 {
			t.Skip()
		}
		_, _ = DecodePlan(strings.NewReader(string(raw)))
		_, _ = DecodeRecord(strings.NewReader(string(raw)))
		_, _ = DecodeBlindPacket(strings.NewReader(string(raw)))
		_, _ = DecodeLabel(strings.NewReader(string(raw)))
	})
}

func validPlan(t *testing.T) Plan {
	t.Helper()
	plan, err := SealPlan(Plan{
		ProtocolVersion: "evalwitness.outcome-adjudication.v1", SourceCorpusDigest: digestText("corpus"), MutationSampleSize: 31, NaturalSampleSize: 24,
		RequiredStrata:   []string{"abstention", "baseline_disagreement", "high_confidence_error", "provider_failure", "random_control", "verifier_correct", "verifier_judge_disagreement", "verifier_wrong"},
		MutationFamilies: []string{"candidate_order_reversal", "causally_independent_event_reorder", "falsified_test_evidence", "incomplete_tool_output", "irrelevant_verbosity", "neutral_formatting", "omitted_test_evidence", "untrusted_score_tag_injection"},
		SamplingRule:     "frozen hash within stratum", ReplacementRule: "next eligible hash in same stratum", PrimaryAdjudicators: 2, TieBreakAdjudicators: 1,
		BlindingRule: "HMAC-rekeyed packets omit condition and evaluator outputs", RubricVersion: "evalwitness.outcome-rubric.v1",
		AgreementMetrics: []string{"cohen_kappa", "label_prevalence", "raw_agreement"}, BootstrapIterations: 10_000, BootstrapSeed: "outcome-bootstrap-v1",
		ConflictRule: "third independent reviewer or unresolved", OutcomeResolutionRule: "executable contradiction outranks asserted reward only when validator is valid",
		SensitivityAnalysis: "original and adjudicated labels reported", PublicPacketPolicy: "licensed redacted evidence only", PrivateMappingPolicy: "owner-only HMAC mapping",
		RecruitmentRequiresConsent: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func mustEvidence(t *testing.T, evidence Evidence) Evidence {
	t.Helper()
	sealed, err := SealEvidence(evidence)
	if err != nil {
		t.Fatal(err)
	}
	return sealed
}

func mustLabel(t *testing.T, packetID, reviewer string, round int, state State) Label {
	t.Helper()
	label, err := SealLabel(Label{
		PacketID: packetID, AdjudicatorAlias: reviewer, ReviewerSlot: round, PrimaryOutcome: state,
		TaskSatisfaction: RatingSufficient, TechnicalCorrectness: RatingSufficient, VerificationQuality: RatingSufficient,
		HarmfulSideEffects: RatingNotApplicable, EvidenceSufficiency: RatingSufficient, ReasonCodes: []ReasonCode{ReasonEvidenceConsistent},
		SubmittedAt: "2026-08-09T09:30:00Z", RubricVersion: "evalwitness.outcome-rubric.v1", QualificationDigest: digestText(reviewer), ConflictsOfInterest: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return label
}

func mustQualificationLabel(t *testing.T, set QualificationSet, item QualificationCase, reviewer string) Label {
	t.Helper()
	label, err := SealLabel(Label{
		PacketID: item.Packet.PacketID, AdjudicatorAlias: reviewer, ReviewerSlot: 1, PrimaryOutcome: item.ExpectedOutcome,
		TaskSatisfaction: RatingUnclear, TechnicalCorrectness: RatingUnclear, VerificationQuality: RatingUnclear,
		HarmfulSideEffects: RatingNotApplicable, EvidenceSufficiency: RatingUnclear,
		ReasonCodes: append([]ReasonCode(nil), item.RequiredReasonCodes...), SubmittedAt: "2026-08-09T13:00:00Z",
		RubricVersion: set.RubricVersion, QualificationDigest: set.Digest, ConflictsOfInterest: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return label
}

func minimalRecord(t *testing.T, task string, state State, suffix string) Record {
	t.Helper()
	evidence := mustEvidence(t, Evidence{ID: "evidence-" + suffix, Kind: EvidenceIndependentRun, State: state, ArtifactDigest: digestText(suffix), ValidatorID: "fixture", ObservedAt: "2026-08-09T09:00:00Z", Independent: true, Limitation: "fixture", ParentDigests: []string{}})
	record, err := SealRecord(Record{TaskAlias: task, Revision: 1, Evidence: []Evidence{evidence}, Resolution: state, ResolutionBasis: []string{evidence.ID}, Limitations: []string{}, AuthorID: "fixture", RevisionReason: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	return record
}
