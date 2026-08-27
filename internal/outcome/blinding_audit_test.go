package outcome

import (
	"math"
	"testing"
)

func TestSemanticBlindingAuditMeasuresPostLabelSourceInference(t *testing.T) {
	fixture := newReviewWorkflowFixture(t)
	leftAssignment := mustPrimaryAssignment(t, fixture, fixture.leftReviewer, fixture.leftReport, 1, "2026-08-09T14:00:00Z", '1')
	rightAssignment := mustPrimaryAssignment(t, fixture, fixture.rightReviewer, fixture.rightReport, 2, "2026-08-09T14:00:00Z", '2')
	leftBatch := mustReviewBatch(t, leftAssignment, allReviewStates(fixture.bundle, StateSolved), "2026-08-09T14:30:00Z", "2026-08-09T15:00:00Z")
	rightBatch := mustReviewBatch(t, rightAssignment, allReviewStates(fixture.bundle, StateSolved), "2026-08-09T14:30:00Z", "2026-08-09T15:00:00Z")
	protocol, err := BuildBlindingProtocol(fixture.bundle, fixture.mappings, "2026-08-09T13:30:00Z")
	if err != nil {
		t.Fatal(err)
	}
	leftProbes := mustProbeBatch(t, protocol, fixture.mappings, leftAssignment, leftBatch)
	rightDrafts := variedProbeDrafts(fixture, rightAssignment)
	rightProbes, err := BuildBlindingProbeBatch(protocol, rightAssignment, rightBatch, rightDrafts, "2026-08-09T15:45:00Z")
	if err != nil {
		t.Fatal(err)
	}
	reveal, err := BuildMappingReveal(
		fixture.bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, nil, nil, fixture.mappings,
		reviewAssignmentSeeds(leftAssignment, rightAssignment, nil, '1', '2', 0), "2026-08-09T16:00:00Z", "study-owner",
	)
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := BuildBlindingAnalysis(fixture.bundle, reveal, fixture.mappings, leftProbes, rightProbes, "2026-08-09T16:01:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Observations != 6 || analysis.Attempts != 5 || analysis.Correct != 4 || analysis.RecognizedTasks != 1 {
		t.Fatalf("blinding counts = %#v", analysis)
	}
	assertClose(t, "accuracy", analysis.ConditionGuessAccuracy, 2.0/3.0)
	assertClose(t, "coverage", analysis.GuessCoverage, 5.0/6.0)
	assertClose(t, "selective accuracy", analysis.SelectiveAccuracy, 0.8)
	assertClose(t, "expected chance", analysis.ExpectedChanceAccuracy, 4.0/9.0)
	assertClose(t, "cohen kappa", analysis.CohenKappa, 0.4)
	assertClose(t, "mean confidence", analysis.MeanConfidence, 0.8)
	assertClose(t, "guess brier", analysis.GuessBrierScore, 0.124)
	if !analysis.SelectiveAccuracyDefined || !analysis.CohenKappaDefined || !analysis.ConfidenceMetricsDefined || analysis.AccuracyInterval.Upper <= analysis.AccuracyInterval.Lower {
		t.Fatalf("blinding uncertainty or defined states = %#v", analysis)
	}
	rubricAmbiguity := mustRubricAmbiguity(t, fixture, leftAssignment, leftBatch, rightAssignment, rightBatch)
	result, err := BuildAdjudicationLedger(
		fixture.bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, nil, nil, reveal, rubricAmbiguity, analysis,
		"2026-08-09T16:05:00Z", "third independent reviewer or unresolved", 10_000, "blinding-audit-bootstrap-v1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Ledger.BlindingAnalysisDigest != analysis.Digest {
		t.Fatal("terminal adjudication ledger did not bind the semantic blinding analysis")
	}
}

func TestSemanticBlindingAuditFailsClosed(t *testing.T) {
	fixture := newReviewWorkflowFixture(t)
	leftAssignment := mustPrimaryAssignment(t, fixture, fixture.leftReviewer, fixture.leftReport, 1, "2026-08-09T14:00:00Z", '1')
	rightAssignment := mustPrimaryAssignment(t, fixture, fixture.rightReviewer, fixture.rightReport, 2, "2026-08-09T14:00:00Z", '2')
	leftBatch := mustReviewBatch(t, leftAssignment, allReviewStates(fixture.bundle, StateSolved), "2026-08-09T14:30:00Z", "2026-08-09T15:00:00Z")
	rightBatch := mustReviewBatch(t, rightAssignment, allReviewStates(fixture.bundle, StateSolved), "2026-08-09T14:30:00Z", "2026-08-09T15:00:00Z")
	protocol, err := BuildBlindingProtocol(fixture.bundle, fixture.mappings, "2026-08-09T13:30:00Z")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("one possible condition", func(t *testing.T) {
		mappings := append([]PrivateMapping(nil), fixture.mappings...)
		for index := range mappings {
			mappings[index].Condition = "only-condition"
			mappings[index], err = SealPrivateMapping(mappings[index])
			if err != nil {
				t.Fatal(err)
			}
		}
		if _, buildErr := BuildBlindingProtocol(fixture.bundle, mappings, "2026-08-09T13:30:00Z"); buildErr == nil {
			t.Fatal("accepted a blinding protocol with no meaningful source inference choice")
		}
	})

	t.Run("before label commitment", func(t *testing.T) {
		drafts := correctProbeDrafts(fixture, leftAssignment, "2026-08-09T14:59:59Z")
		if _, buildErr := BuildBlindingProbeBatch(protocol, leftAssignment, leftBatch, drafts, "2026-08-09T15:10:00Z"); buildErr == nil {
			t.Fatal("accepted a source-condition probe before outcome labels were committed")
		}
	})

	t.Run("incomplete probes", func(t *testing.T) {
		drafts := correctProbeDrafts(fixture, leftAssignment, "2026-08-09T15:10:00Z")
		if _, buildErr := BuildBlindingProbeBatch(protocol, leftAssignment, leftBatch, drafts[:len(drafts)-1], "2026-08-09T15:20:00Z"); buildErr == nil {
			t.Fatal("accepted an incomplete blinding-probe commitment")
		}
	})

	leftProbes := mustProbeBatch(t, protocol, fixture.mappings, leftAssignment, leftBatch)
	rightProbes := mustProbeBatch(t, protocol, fixture.mappings, rightAssignment, rightBatch)
	reveal, err := BuildMappingReveal(
		fixture.bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, nil, nil, fixture.mappings,
		reviewAssignmentSeeds(leftAssignment, rightAssignment, nil, '1', '2', 0), "2026-08-09T16:00:00Z", "study-owner",
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("all unknown remains defined as abstention, not perfect calibration", func(t *testing.T) {
		unknownDrafts := func(assignment ReviewAssignment) []BlindingProbeDraft {
			drafts := make([]BlindingProbeDraft, 0, len(assignment.PacketIDs))
			for _, packetID := range assignment.PacketIDs {
				drafts = append(drafts, BlindingProbeDraft{PacketID: packetID, ConditionGuess: UnknownCondition, RecognitionBasis: RecognitionNone, SubmittedAt: "2026-08-09T15:40:00Z"})
			}
			return drafts
		}
		unknownLeft, buildErr := BuildBlindingProbeBatch(protocol, leftAssignment, leftBatch, unknownDrafts(leftAssignment), "2026-08-09T15:45:00Z")
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		unknownRight, buildErr := BuildBlindingProbeBatch(protocol, rightAssignment, rightBatch, unknownDrafts(rightAssignment), "2026-08-09T15:45:00Z")
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		analysis, analysisErr := BuildBlindingAnalysis(fixture.bundle, reveal, fixture.mappings, unknownLeft, unknownRight, "2026-08-09T16:01:00Z")
		if analysisErr != nil {
			t.Fatal(analysisErr)
		}
		if analysis.Attempts != 0 || analysis.Correct != 0 || analysis.GuessCoverage != 0 || analysis.SelectiveAccuracyDefined || analysis.CohenKappaDefined || analysis.ConfidenceMetricsDefined {
			t.Fatalf("all-unknown analysis = %#v", analysis)
		}
	})

	t.Run("probe commitment after reveal", func(t *testing.T) {
		lateDrafts := correctProbeDrafts(fixture, rightAssignment, "2026-08-09T16:01:00Z")
		late, buildErr := BuildBlindingProbeBatch(protocol, rightAssignment, rightBatch, lateDrafts, "2026-08-09T16:02:00Z")
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		if _, analysisErr := BuildBlindingAnalysis(fixture.bundle, reveal, fixture.mappings, leftProbes, late, "2026-08-09T16:03:00Z"); analysisErr == nil {
			t.Fatal("accepted blinding probes committed after the private mapping reveal")
		}
	})

	t.Run("tampered statistics", func(t *testing.T) {
		analysis, buildErr := BuildBlindingAnalysis(fixture.bundle, reveal, fixture.mappings, leftProbes, rightProbes, "2026-08-09T16:01:00Z")
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		analysis.ConditionGuessAccuracy = 0
		if _, sealErr := SealBlindingAnalysis(analysis); sealErr == nil {
			t.Fatal("accepted blinding statistics inconsistent with item-level evidence")
		}
	})
}

func variedProbeDrafts(fixture reviewWorkflowFixture, assignment ReviewAssignment) []BlindingProbeDraft {
	truth := mappingConditions(fixture.mappings)
	drafts := make([]BlindingProbeDraft, 0, len(assignment.PacketIDs))
	wrongUsed := false
	for _, packetID := range assignment.PacketIDs {
		draft := BlindingProbeDraft{PacketID: packetID, ConditionGuess: truth[packetID], Confidence: 0.9, RecognitionBasis: RecognitionNone, SubmittedAt: "2026-08-09T15:40:00Z"}
		if draft.ConditionGuess == "workflow-fixture-alpha" {
			draft.ConditionGuess, draft.Confidence = UnknownCondition, 0
		} else if !wrongUsed {
			draft.ConditionGuess = "workflow-fixture-alpha"
			draft.Confidence, draft.RecognizedTask, draft.RecognitionBasis = 0.7, true, RecognitionTaskText
			wrongUsed = true
		}
		drafts = append(drafts, draft)
	}
	return drafts
}

func correctProbeDrafts(fixture reviewWorkflowFixture, assignment ReviewAssignment, submittedAt string) []BlindingProbeDraft {
	truth := mappingConditions(fixture.mappings)
	drafts := make([]BlindingProbeDraft, 0, len(assignment.PacketIDs))
	for _, packetID := range assignment.PacketIDs {
		drafts = append(drafts, BlindingProbeDraft{PacketID: packetID, ConditionGuess: truth[packetID], Confidence: 0.8, RecognitionBasis: RecognitionNone, SubmittedAt: submittedAt})
	}
	return drafts
}

func mappingConditions(mappings []PrivateMapping) map[string]string {
	values := make(map[string]string, len(mappings))
	for _, mapping := range mappings {
		values[mapping.PacketID] = mapping.Condition
	}
	return values
}

func assertClose(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("%s = %.16f, want %.16f", name, got, want)
	}
}
