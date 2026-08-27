package outcome

import (
	"encoding/json"
	"strings"
	"testing"
)

type sourceAuditFixture struct {
	review      reviewWorkflowFixture
	reveal      MappingReveal
	result      AdjudicationResult
	records     []Record
	analyzedAt  string
	bootstrapID string
}

func TestOutcomeSourceAuditReconcilesPerformanceBlindEvidence(t *testing.T) {
	fixture := newSourceAuditFixture(t)
	audit, err := BuildOutcomeSourceAudit(
		fixture.review.bundle, fixture.reveal, fixture.result.Ledger, fixture.review.mappings, fixture.records,
		fixture.result.Resolutions, fixture.analyzedAt, 10_000, fixture.bootstrapID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if audit.Packets != 3 || audit.SourceEvidenceObservations != 10 || audit.PacketsWithInternalConflict != 1 ||
		audit.PacketsWithAnyDisagreement != 1 || audit.BenchmarkComparablePackets != 2 || audit.BenchmarkChangedPackets != 1 ||
		audit.BenchmarkChangedRate != 0.5 {
		t.Fatalf("source audit summary = %#v", audit)
	}
	coverage := sourceCoverageByKind(audit.SourceCoverage)
	assertSourceCoverage(t, coverage[EvidenceClaimedTest], 3, 4, 1)
	assertSourceCoverage(t, coverage[EvidenceBenchmarkReward], 2, 2, 0)
	assertSourceCoverage(t, coverage[EvidenceIndependentRun], 3, 3, 0)
	assertSourceCoverage(t, coverage[EvidenceFormalRelation], 1, 1, 0)
	assertSourceCoverage(t, coverage[EvidenceHumanLabel], 3, 3, 0)

	benchmarkHuman := sourcePairMetric(t, audit.PairMetrics, EvidenceBenchmarkReward, EvidenceHumanLabel)
	if benchmarkHuman.ComparableCases != 2 || benchmarkHuman.Agreements != 1 || benchmarkHuman.Disagreements != 1 ||
		!benchmarkHuman.CohenKappaDefined || benchmarkHuman.CohenKappa != 0 {
		t.Fatalf("benchmark-human metric = %#v", benchmarkHuman)
	}
	independentHuman := sourcePairMetric(t, audit.PairMetrics, EvidenceIndependentRun, EvidenceHumanLabel)
	if independentHuman.ComparableCases != 3 || independentHuman.Agreements != 3 || independentHuman.Disagreements != 0 ||
		!independentHuman.CohenKappaDefined || independentHuman.CohenKappa != 1 {
		t.Fatalf("independent-human metric = %#v", independentHuman)
	}
	claimedHuman := sourcePairMetric(t, audit.PairMetrics, EvidenceClaimedTest, EvidenceHumanLabel)
	if claimedHuman.CasesWithBothSources != 3 || claimedHuman.AmbiguousCases != 1 || claimedHuman.ComparableCases != 2 ||
		claimedHuman.Agreements != 1 || claimedHuman.Disagreements != 1 {
		t.Fatalf("claimed-human metric = %#v", claimedHuman)
	}
	if len(audit.BenchmarkTransitions) != 2 {
		t.Fatalf("benchmark transitions = %#v", audit.BenchmarkTransitions)
	}
	if err := audit.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestOutcomeSourceAuditFailsClosedAtEvidenceBoundaries(t *testing.T) {
	fixture := newSourceAuditFixture(t)
	build := func(records []Record, resolutions []Resolution, analyzedAt string) error {
		_, err := BuildOutcomeSourceAudit(
			fixture.review.bundle, fixture.reveal, fixture.result.Ledger, fixture.review.mappings, records, resolutions,
			analyzedAt, 10_000, fixture.bootstrapID,
		)
		return err
	}

	t.Run("before terminal ledger", func(t *testing.T) {
		if err := build(fixture.records, fixture.result.Resolutions, fixture.result.Ledger.CompletedAt); err == nil {
			t.Fatal("accepted source audit before terminal ledger completion")
		}
	})
	t.Run("missing source record", func(t *testing.T) {
		if err := build(fixture.records[:len(fixture.records)-1], fixture.result.Resolutions, fixture.analyzedAt); err == nil {
			t.Fatal("accepted missing source record")
		}
	})
	t.Run("human evidence pre-embedded", func(t *testing.T) {
		records := append([]Record(nil), fixture.records...)
		human := mustSourceEvidence(t, "human", EvidenceHumanLabel, StateSolved)
		records[0].Evidence = append(records[0].Evidence, human)
		records[0], _ = SealRecord(records[0])
		if err := build(records, fixture.result.Resolutions, fixture.analyzedAt); err == nil {
			t.Fatal("accepted source record with pre-embedded human outcome")
		}
	})
	t.Run("resolution outside ledger", func(t *testing.T) {
		resolutions := append([]Resolution(nil), fixture.result.Resolutions...)
		resolutions[0].Rule = "tampered rule"
		resolutions[0], _ = SealResolution(resolutions[0])
		if err := build(fixture.records, resolutions, fixture.analyzedAt); err == nil {
			t.Fatal("accepted resolution outside terminal ledger")
		}
	})
	t.Run("tampered item evidence", func(t *testing.T) {
		audit, err := BuildOutcomeSourceAudit(
			fixture.review.bundle, fixture.reveal, fixture.result.Ledger, fixture.review.mappings, fixture.records,
			fixture.result.Resolutions, fixture.analyzedAt, 10_000, fixture.bootstrapID,
		)
		if err != nil {
			t.Fatal(err)
		}
		audit.Items[0].SourceEvidence[0].State = StateUnsolved
		if _, err := SealOutcomeSourceAudit(audit); err == nil {
			t.Fatal("accepted tampered item evidence")
		}
	})
	t.Run("tampered aggregate", func(t *testing.T) {
		audit, err := BuildOutcomeSourceAudit(
			fixture.review.bundle, fixture.reveal, fixture.result.Ledger, fixture.review.mappings, fixture.records,
			fixture.result.Resolutions, fixture.analyzedAt, 10_000, fixture.bootstrapID,
		)
		if err != nil {
			t.Fatal(err)
		}
		audit.PacketsWithAnyDisagreement++
		if _, err := SealOutcomeSourceAudit(audit); err == nil {
			t.Fatal("accepted aggregate inconsistent with item evidence")
		}
	})
}

func TestOutcomeSourceAuditTreatsNotAdjudicatedEvidenceAsAmbiguous(t *testing.T) {
	evidence := mustSourceEvidence(t, "claimed", EvidenceClaimedTest, StateNotAdjudicated)
	summary := summarizeEvidenceKind(EvidenceClaimedTest, []Evidence{evidence})
	if summary.StateDefined || summary.InternalConflict || summary.Observations != 1 {
		t.Fatalf("not-adjudicated source summary = %#v", summary)
	}
}

func TestRecordDraftSealsStrictOutcomeRecord(t *testing.T) {
	evidence := mustSourceEvidence(t, "independent", EvidenceIndependentRun, StateSolved)
	draft := `{"task_alias":"task-record-draft","revision":1,"evidence":[` + mustJSON(t, evidence) + `],"resolution":"solved","resolution_basis":["independent"],"limitations":[],"author_id":"fixture","revision_reason":"record draft test"}`
	decoded, err := DecodeRecordDraft(strings.NewReader(draft))
	if err != nil {
		t.Fatal(err)
	}
	record, err := SealRecordDraft(decoded)
	if err != nil || record.TaskAlias != "task-record-draft" || record.RecordID == "" {
		t.Fatalf("sealed record = %#v err=%v", record, err)
	}
	unknown := strings.TrimSuffix(draft, "}") + `,"unknown":true}`
	if _, err := DecodeRecordDraft(strings.NewReader(unknown)); err == nil {
		t.Fatal("record draft accepted an unknown field")
	}
}

func TestEvidenceDraftSealsStrictEvidence(t *testing.T) {
	draft := `{"id":"claimed","kind":"claimed_test_output","state":"solved","artifact_digest":"` + digestText("claimed-artifact") + `","observed_at":"2026-08-09T11:00:00Z","independent":false,"public":true,"limitation":"fixture","parent_digests":[]}`
	decoded, err := DecodeEvidenceDraft(strings.NewReader(draft))
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := SealEvidenceDraft(decoded)
	if err != nil || evidence.ID != "claimed" || evidence.Digest == "" {
		t.Fatalf("sealed evidence = %#v err=%v", evidence, err)
	}
	unknown := strings.TrimSuffix(draft, "}") + `,"unknown":true}`
	if _, err := DecodeEvidenceDraft(strings.NewReader(unknown)); err == nil {
		t.Fatal("evidence draft accepted an unknown field")
	}
}

func newSourceAuditFixture(t *testing.T) sourceAuditFixture {
	t.Helper()
	fixture := newReviewWorkflowFixture(t)
	leftAssignment := mustPrimaryAssignment(t, fixture, fixture.leftReviewer, fixture.leftReport, 1, "2026-08-09T14:00:00Z", '1')
	rightAssignment := mustPrimaryAssignment(t, fixture, fixture.rightReviewer, fixture.rightReport, 2, "2026-08-09T14:00:00Z", '2')
	leftStates := map[string]State{
		fixture.bundle.Items[0].Packet.PacketID: StateSolved,
		fixture.bundle.Items[1].Packet.PacketID: StateSolved,
		fixture.bundle.Items[2].Packet.PacketID: StateUnsolved,
	}
	rightStates := map[string]State{
		fixture.bundle.Items[0].Packet.PacketID: StateSolved,
		fixture.bundle.Items[1].Packet.PacketID: StateUnsolved,
		fixture.bundle.Items[2].Packet.PacketID: StateUnsolved,
	}
	leftBatch := mustReviewBatch(t, leftAssignment, leftStates, "2026-08-09T14:30:00Z", "2026-08-09T15:00:00Z")
	rightBatch := mustReviewBatch(t, rightAssignment, rightStates, "2026-08-09T14:35:00Z", "2026-08-09T15:00:00Z")
	tieAssignment, err := BuildTieBreakAssignment(
		fixture.bundle, fixture.tieReviewer, fixture.tieReport, leftAssignment, leftBatch, rightAssignment, rightBatch,
		reviewSeed('3'), "2026-08-09T15:10:00Z",
	)
	if err != nil {
		t.Fatal(err)
	}
	tieBatch := mustReviewBatch(t, tieAssignment, map[string]State{tieAssignment.PacketIDs[0]: StateSolved}, "2026-08-09T15:20:00Z", "2026-08-09T15:30:00Z")
	reveal, err := BuildMappingReveal(
		fixture.bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, &tieAssignment, &tieBatch,
		fixture.mappings, reviewAssignmentSeeds(leftAssignment, rightAssignment, &tieAssignment, '1', '2', '3'), "2026-08-09T16:00:00Z", "study-owner",
	)
	if err != nil {
		t.Fatal(err)
	}
	blinding := mustBlindingAnalysis(t, fixture, leftAssignment, leftBatch, rightAssignment, rightBatch, reveal)
	rubric := mustRubricAmbiguity(t, fixture, leftAssignment, leftBatch, rightAssignment, rightBatch)
	result, err := BuildAdjudicationLedger(
		fixture.bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, &tieAssignment, &tieBatch, reveal,
		rubric, blinding, "2026-08-09T16:05:00Z", "third independent reviewer or unresolved", 10_000, "source-audit-ledger-bootstrap-v1",
	)
	if err != nil {
		t.Fatal(err)
	}
	records := sourceAuditRecords(t, fixture, result.Resolutions)
	return sourceAuditFixture{review: fixture, reveal: reveal, result: result, records: records, analyzedAt: "2026-08-09T16:06:00Z", bootstrapID: "source-audit-bootstrap-v1"}
}

func sourceAuditRecords(t *testing.T, fixture reviewWorkflowFixture, resolutions []Resolution) []Record {
	t.Helper()
	mappingByPacket := privateMappingsByPacket(fixture.mappings)
	resolutionByPacket := resolutionsByPacket(resolutions)
	records := make([]Record, 0, len(fixture.bundle.Items))
	for index, item := range fixture.bundle.Items {
		var evidence []Evidence
		var basis string
		switch index {
		case 0:
			evidence = []Evidence{
				mustSourceEvidence(t, "benchmark", EvidenceBenchmarkReward, StateSolved),
				mustSourceEvidence(t, "claimed", EvidenceClaimedTest, StateSolved),
				mustSourceEvidence(t, "independent", EvidenceIndependentRun, StateSolved),
			}
			basis = "independent"
		case 1:
			evidence = []Evidence{
				mustSourceEvidence(t, "benchmark", EvidenceBenchmarkReward, StateUnsolved),
				mustSourceEvidence(t, "claimed", EvidenceClaimedTest, StateUnsolved),
				mustSourceEvidence(t, "formal", EvidenceFormalRelation, StateSolved),
				mustSourceEvidence(t, "independent", EvidenceIndependentRun, StateSolved),
			}
			basis = "independent"
		default:
			evidence = []Evidence{
				mustSourceEvidence(t, "claimed-a", EvidenceClaimedTest, StateSolved),
				mustSourceEvidence(t, "claimed-b", EvidenceClaimedTest, StateUnsolved),
				mustSourceEvidence(t, "independent", EvidenceIndependentRun, StateUnsolved),
			}
			basis = "independent"
		}
		mapping := mappingByPacket[item.Packet.PacketID]
		record, err := SealRecord(Record{
			TaskAlias: mapping.SourceTaskAlias, Revision: 1, Evidence: evidence, Resolution: resolutionByPacket[item.Packet.PacketID].State,
			ResolutionBasis: []string{basis}, Limitations: []string{"synthetic source-audit fixture"}, AuthorID: "fixture", RevisionReason: "source audit fixture",
		})
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	return records
}

func mustSourceEvidence(t *testing.T, id string, kind EvidenceKind, state State) Evidence {
	t.Helper()
	independent := kind == EvidenceIndependentRun || kind == EvidenceFormalRelation
	validator := ""
	if independent {
		validator = "fixture-validator"
	}
	evidence, err := SealEvidence(Evidence{
		ID: id, Kind: kind, State: state, ArtifactDigest: digestText("artifact-" + id + "-" + string(state)), ValidatorID: validator,
		ObservedAt: "2026-08-09T11:00:00Z", Independent: independent, Public: true, Limitation: "synthetic source-audit fixture", ParentDigests: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return evidence
}

func sourceCoverageByKind(values []OutcomeSourceCoverageMetric) map[EvidenceKind]OutcomeSourceCoverageMetric {
	result := make(map[EvidenceKind]OutcomeSourceCoverageMetric, len(values))
	for _, value := range values {
		result[value.Kind] = value
	}
	return result
}

func assertSourceCoverage(t *testing.T, metric OutcomeSourceCoverageMetric, packetsWithSource, observations, conflicts int) {
	t.Helper()
	if metric.PacketsWithSource != packetsWithSource || metric.Observations != observations || metric.InternalConflictPackets != conflicts {
		t.Fatalf("source coverage = %#v", metric)
	}
}

func sourcePairMetric(t *testing.T, values []OutcomeSourcePairMetric, left, right EvidenceKind) OutcomeSourcePairMetric {
	t.Helper()
	for _, value := range values {
		if value.Left == left && value.Right == right {
			return value
		}
	}
	t.Fatalf("missing source pair %s/%s", left, right)
	return OutcomeSourcePairMetric{}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
