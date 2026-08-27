package outcome

import (
	"math"
	"testing"
)

func TestRubricAmbiguityAnalysisExplainsPrimaryDisagreement(t *testing.T) {
	fixture, leftAssignment, leftBatch, rightAssignment, rightBatch := rubricAmbiguityReview(t)
	analysis, err := BuildRubricAmbiguityAnalysis(
		fixture.bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, "2026-08-09T15:05:00Z",
	)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Packets != 3 || analysis.LabelObservations != 6 || analysis.AxisComparisons != 15 || analysis.PrimaryOutcomeDisagreements != 2 || analysis.AnyAxisDisagreements != 1 {
		t.Fatalf("rubric ambiguity denominators = %#v", analysis)
	}
	assertClose(t, "primary outcome disagreement", analysis.PrimaryOutcomeDisagreementRate, 2.0/3.0)
	assertClose(t, "any-axis disagreement", analysis.AnyAxisDisagreementRate, 1.0/3.0)
	assertClose(t, "indeterminate rate", analysis.IndeterminateRate, 1.0/6.0)
	assertClose(t, "unclear rate", analysis.UnclearRate, 1.0/30.0)
	assertClose(t, "exact reason match rate", analysis.ExactReasonMatchRate, 2.0/3.0)
	assertClose(t, "zero reason overlap rate", analysis.ZeroReasonOverlapRate, 1.0/3.0)
	assertClose(t, "reason Jaccard distance", analysis.MeanReasonJaccardDistance, 1.0/3.0)
	if analysis.ExactReasonMatches != 2 || analysis.ZeroReasonOverlaps != 1 || analysis.UnclearRatings != 1 || analysis.IndeterminateLabels != 1 {
		t.Fatalf("rubric ambiguity counts = %#v", analysis)
	}
	for name, interval := range map[string]Interval{
		"indeterminate": analysis.IndeterminateInterval, "unclear": analysis.UnclearInterval,
		"exact reasons": analysis.ExactReasonMatchInterval, "zero-overlap reasons": analysis.ZeroReasonOverlapInterval,
	} {
		if interval.Lower >= interval.Upper {
			t.Fatalf("%s Wilson interval = %#v", name, interval)
		}
	}
	metricByAxis := make(map[RubricAxis]RubricAxisMetric, len(analysis.AxisMetrics))
	for _, metric := range analysis.AxisMetrics {
		metricByAxis[metric.Axis] = metric
	}
	for _, axis := range []RubricAxis{RubricAxisTechnicalCorrectness, RubricAxisVerificationQuality, RubricAxisEvidenceSufficiency} {
		if metricByAxis[axis].Disagreements != 1 {
			t.Fatalf("axis %s metric = %#v", axis, metricByAxis[axis])
		}
	}
	if metricByAxis[RubricAxisTaskSatisfaction].Disagreements != 0 || metricByAxis[RubricAxisHarmfulSideEffects].Disagreements != 0 || metricByAxis[RubricAxisVerificationQuality].UnclearRatings != 1 {
		t.Fatalf("stable-axis or unclear metric changed: %#v", analysis.AxisMetrics)
	}

	reveal := rubricAmbiguityReveal(t, fixture, leftAssignment, leftBatch, rightAssignment, rightBatch)
	blinding := mustBlindingAnalysis(t, fixture, leftAssignment, leftBatch, rightAssignment, rightBatch, reveal)
	result, err := BuildAdjudicationLedger(
		fixture.bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, nil, nil, reveal, analysis, blinding,
		"2026-08-09T16:05:00Z", "third independent reviewer or unresolved", 10_000, "rubric-ambiguity-bootstrap-v1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Ledger.RubricAmbiguityDigest != analysis.Digest {
		t.Fatal("terminal adjudication ledger did not bind prereveal rubric ambiguity evidence")
	}
}

func TestRubricAmbiguityAnalysisFailsClosed(t *testing.T) {
	fixture, leftAssignment, leftBatch, rightAssignment, rightBatch := rubricAmbiguityReview(t)

	t.Run("not after both label commitments", func(t *testing.T) {
		if _, err := BuildRubricAmbiguityAnalysis(fixture.bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, "2026-08-09T15:00:00Z"); err == nil {
			t.Fatal("accepted rubric ambiguity analysis before both primary commitments were sealed")
		}
	})

	t.Run("same review twice", func(t *testing.T) {
		if _, err := BuildRubricAmbiguityAnalysis(fixture.bundle, leftAssignment, leftBatch, leftAssignment, leftBatch, "2026-08-09T15:05:00Z"); err == nil {
			t.Fatal("accepted duplicate primary reviewer evidence")
		}
	})

	analysis, err := BuildRubricAmbiguityAnalysis(fixture.bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, "2026-08-09T15:05:00Z")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("partial item coverage", func(t *testing.T) {
		tampered := analysis
		tampered.Items = append([]RubricAmbiguityItem(nil), analysis.Items[:len(analysis.Items)-1]...)
		if _, err := SealRubricAmbiguityAnalysis(tampered); err == nil {
			t.Fatal("accepted partial rubric ambiguity item coverage")
		}
	})

	t.Run("tampered item derivation", func(t *testing.T) {
		tampered := analysis
		tampered.Items = append([]RubricAmbiguityItem(nil), analysis.Items...)
		tampered.Items[0].PrimaryOutcomeDisagreement = !tampered.Items[0].PrimaryOutcomeDisagreement
		if _, err := SealRubricAmbiguityAnalysis(tampered); err == nil {
			t.Fatal("accepted an item flag inconsistent with embedded labels")
		}
	})

	t.Run("tampered aggregate", func(t *testing.T) {
		tampered := analysis
		tampered.PrimaryOutcomeDisagreements = 0
		if _, err := SealRubricAmbiguityAnalysis(tampered); err == nil {
			t.Fatal("accepted aggregate statistics inconsistent with item evidence")
		}
	})

	reveal := rubricAmbiguityReveal(t, fixture, leftAssignment, leftBatch, rightAssignment, rightBatch)
	blinding := mustBlindingAnalysis(t, fixture, leftAssignment, leftBatch, rightAssignment, rightBatch, reveal)

	t.Run("resealed labels not in committed batches", func(t *testing.T) {
		items := append([]RubricAmbiguityItem(nil), analysis.Items...)
		left := items[0].LeftLabel
		left.TaskSatisfaction = RatingUnclear
		left, err = SealLabel(left)
		if err != nil {
			t.Fatal(err)
		}
		items[0] = buildRubricAmbiguityItem(
			ReviewItem{TaskGroupID: items[0].TaskGroupID, Packet: BlindPacket{PacketID: items[0].PacketID}}, left, items[0].RightLabel,
		)
		tampered := summarizeRubricAmbiguityItems(items)
		tampered.BundleDigest, tampered.PrimaryCommitments = analysis.BundleDigest, analysis.PrimaryCommitments
		tampered.AnalyzedAt, tampered.Limitations = analysis.AnalyzedAt, rubricAmbiguityLimitations()
		tampered, err = SealRubricAmbiguityAnalysis(tampered)
		if err != nil {
			t.Fatal(err)
		}
		if _, ledgerErr := BuildAdjudicationLedger(
			fixture.bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, nil, nil, reveal, tampered, blinding,
			"2026-08-09T16:05:00Z", "third independent reviewer or unresolved", 10_000, "rubric-ambiguity-bootstrap-v1",
		); ledgerErr == nil {
			t.Fatal("terminal ledger accepted labels outside the committed primary batches")
		}
	})

	t.Run("analysis at reveal", func(t *testing.T) {
		late, buildErr := BuildRubricAmbiguityAnalysis(fixture.bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, reveal.RevealedAt)
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		if _, ledgerErr := BuildAdjudicationLedger(
			fixture.bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, nil, nil, reveal, late, blinding,
			"2026-08-09T16:05:00Z", "third independent reviewer or unresolved", 10_000, "rubric-ambiguity-bootstrap-v1",
		); ledgerErr == nil {
			t.Fatal("terminal ledger accepted a rubric analysis that was not prereveal")
		}
	})

	t.Run("missing analysis", func(t *testing.T) {
		if _, ledgerErr := BuildAdjudicationLedger(
			fixture.bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, nil, nil, reveal, RubricAmbiguityAnalysis{}, blinding,
			"2026-08-09T16:05:00Z", "third independent reviewer or unresolved", 10_000, "rubric-ambiguity-bootstrap-v1",
		); ledgerErr == nil {
			t.Fatal("terminal ledger accepted missing rubric ambiguity evidence")
		}
	})
}

func rubricAmbiguityReview(t *testing.T) (reviewWorkflowFixture, ReviewAssignment, LabelBatch, ReviewAssignment, LabelBatch) {
	t.Helper()
	fixture := newReviewWorkflowFixture(t)
	leftAssignment := mustPrimaryAssignment(t, fixture, fixture.leftReviewer, fixture.leftReport, 1, "2026-08-09T14:00:00Z", '1')
	rightAssignment := mustPrimaryAssignment(t, fixture, fixture.rightReviewer, fixture.rightReport, 2, "2026-08-09T14:00:00Z", '2')
	leftStates := map[string]State{
		fixture.bundle.Items[0].Packet.PacketID: StateSolved,
		fixture.bundle.Items[1].Packet.PacketID: StateSolved,
		fixture.bundle.Items[2].Packet.PacketID: StateIndeterminate,
	}
	rightStates := map[string]State{
		fixture.bundle.Items[0].Packet.PacketID: StateSolved,
		fixture.bundle.Items[1].Packet.PacketID: StateUnsolved,
		fixture.bundle.Items[2].Packet.PacketID: StateSolved,
	}
	leftBatch := mustReviewBatch(t, leftAssignment, leftStates, "2026-08-09T14:30:00Z", "2026-08-09T15:00:00Z")
	rightLabels := labelsForAssignment(t, rightAssignment, rightStates, "2026-08-09T14:35:00Z")
	wanted := fixture.bundle.Items[1].Packet.PacketID
	for index := range rightLabels {
		if rightLabels[index].PacketID != wanted {
			continue
		}
		rightLabels[index].TechnicalCorrectness = RatingInsufficient
		rightLabels[index].VerificationQuality = RatingUnclear
		rightLabels[index].EvidenceSufficiency = RatingInsufficient
		rightLabels[index].ReasonCodes = []ReasonCode{ReasonEvidenceInsufficient, ReasonTechnicalDefect, ReasonVerificationIncomplete}
		sealed, err := SealLabel(rightLabels[index])
		if err != nil {
			t.Fatal(err)
		}
		rightLabels[index] = sealed
	}
	rightBatch, err := BuildLabelBatch(rightAssignment, rightLabels, "2026-08-09T15:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	return fixture, leftAssignment, leftBatch, rightAssignment, rightBatch
}

func rubricAmbiguityReveal(t *testing.T, fixture reviewWorkflowFixture, leftAssignment ReviewAssignment, leftBatch LabelBatch, rightAssignment ReviewAssignment, rightBatch LabelBatch) MappingReveal {
	t.Helper()
	reveal, err := BuildMappingReveal(
		fixture.bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, nil, nil, fixture.mappings,
		reviewAssignmentSeeds(leftAssignment, rightAssignment, nil, '1', '2', 0), "2026-08-09T16:00:00Z", "study-owner",
	)
	if err != nil {
		t.Fatal(err)
	}
	return reveal
}

func TestRubricAmbiguityWilsonIntervalIsNonDegenerate(t *testing.T) {
	interval := wilsonInterval(2, 3)
	if math.IsNaN(interval.Lower) || math.IsNaN(interval.Upper) || interval.Lower >= interval.Upper {
		t.Fatalf("Wilson interval = %#v", interval)
	}
}
