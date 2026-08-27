package relation

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/study"
)

func TestFrozenRelationV3GovernanceReproducesFromControlledRelease(t *testing.T) {
	corpusPlan, audit, release := readGovernedCorpusV3(t)
	plan := readGovernedPlanV3(t, "relation-audit-plan-v3.json")
	primary := readGovernedPrimaryV3(t, "relation-primary-sample-v3.json", plan)
	sentinel := readGovernedSentinelV3(t, "relation-scarcity-sentinel-v3.json", plan, primary)
	pilot := readGovernedPilotV3(t, "relation-pilot-sample-v3.json", plan, primary, sentinel)
	amendment := readGovernedAmendmentV3(t, "relation-study-amendment-v3.json", plan, pilot, primary, sentinel)

	reproducedPlan, err := BuildPlanV3(corpusPlan, audit, release)
	if err != nil || reproducedPlan.Digest != plan.Digest {
		t.Fatalf("v3 relation plan does not reproduce: %v", err)
	}
	reproducedPrimary, err := BuildPrimarySampleV3(plan, release)
	if err != nil || reproducedPrimary.Digest != primary.Digest {
		t.Fatalf("v3 primary sample does not reproduce: %v", err)
	}
	reproducedSentinel, err := BuildScarcitySentinelV3(plan, primary, release)
	if err != nil || reproducedSentinel.Digest != sentinel.Digest {
		t.Fatalf("v3 scarcity sentinel does not reproduce: %v", err)
	}
	reproducedPilot, err := BuildPilotSampleV3(plan, primary, sentinel, release)
	if err != nil || reproducedPilot.Digest != pilot.Digest {
		t.Fatalf("v3 pilot sample does not reproduce: %v", err)
	}
	reproducedAmendment, err := BuildStudyAmendmentV3(plan, pilot, primary, sentinel, amendment.IssuedAt)
	if err != nil || reproducedAmendment.Digest != amendment.Digest {
		t.Fatalf("v3 study amendment does not reproduce: %v", err)
	}

	if plan.Digest != "6eac462cae0a5b626561d5cbea274a5c3a72c78b6cb9d50a8952be6ccbb6fa8c" ||
		primary.Digest != "6b721bcf0fb10e47923b46d92f3c14691cbd0dd98949ab0bf1dc016d8e1c1e43" ||
		sentinel.Digest != "ec720a56394249a47eb4c0f7ef618471ce14c69ff8c2c13e1e851350c23e71fb" ||
		pilot.Digest != "3f0a70209575316c78b87f6cb9e7641ff1f4a023eaa86c2fe2598ee14b3c94b4" ||
		amendment.Digest != "a9924c62cdb6cea4aa4b9310af9a31ca5ee3cb193645cb30fd2b1135aab0ed94" {
		t.Fatal("frozen v3 relation governance digest changed")
	}
	if primary.SelectedCases != 28 || primary.UniqueTaskGroups != 28 || primary.UniqueLineageClusters != 28 ||
		countFor(primary.SplitCounts, string(study.RoleCalibration)) != 14 || countFor(primary.SplitCounts, string(study.RoleTest)) != 14 {
		t.Fatal("v3 primary sample lost balance or strict task-lineage independence")
	}
	if sentinel.SelectedCases != 3 || sentinel.TestCases != 0 || sentinel.PrimaryOverlap != 0 || !sentinel.Exhaustive || sentinel.HeldOutClaimAvailable {
		t.Fatal("v3 scarcity sentinel lost its exhaustive non-held-out boundary")
	}
	if pilot.SelectedCases != 7 || pilot.UniqueTaskGroups != 7 || pilot.UniqueLineageClusters != 7 || pilot.PrimaryOverlap != 0 || pilot.ScarcitySentinelOverlap != 0 {
		t.Fatal("v3 pilot lost its seven-family disjointness contract")
	}
	if amendment.Primary.Cases != 28 || amendment.Primary.EffectiveTaskGroups != 28 || amendment.Primary.PrimaryLabels != 56 ||
		amendment.Inference.ZeroContradictionUpperBound != 0.10146573557272465 || amendment.EmpiricalStatus != EmpiricalStatusNotRun || amendment.ExternalActionStatus != ExternalActionNotAuthorized {
		t.Fatal("v3 amendment lost its recalculated design or no-action boundary")
	}
}

func TestRelationV3GovernanceRejectsSentinelAndLineageTamper(t *testing.T) {
	plan := readGovernedPlanV3(t, "relation-audit-plan-v3.json")
	primary := readGovernedPrimaryV3(t, "relation-primary-sample-v3.json", plan)
	sentinel := readGovernedSentinelV3(t, "relation-scarcity-sentinel-v3.json", plan, primary)

	tamperedSentinel := sentinel
	tamperedSentinel.HeldOutClaimAvailable = true
	tamperedSentinel.Digest = ""
	digest, err := digestJSON(tamperedSentinel)
	if err != nil {
		t.Fatal(err)
	}
	tamperedSentinel.Digest = digest
	if err := tamperedSentinel.Validate(plan, primary); err == nil {
		t.Fatal("v3 scarcity sentinel accepted a fabricated held-out claim")
	}

	tamperedPrimary := primary
	tamperedPrimary.Cases = append([]GovernedCaseReferenceV3(nil), primary.Cases...)
	tamperedPrimary.Cases[1].LineageClusterIDs = append([]string(nil), primary.Cases[0].LineageClusterIDs...)
	tamperedPrimary.Digest = ""
	digest, err = digestJSON(tamperedPrimary)
	if err != nil {
		t.Fatal(err)
	}
	tamperedPrimary.Digest = digest
	if err := tamperedPrimary.Validate(plan); err == nil {
		t.Fatal("v3 primary sample accepted a reused lineage cluster")
	}
}

func TestRelationV3GovernanceSchemasAreVersionMatched(t *testing.T) {
	tests := map[string]string{
		"blind-packet-v3":             BlindPacketSchemaVersionV3,
		"case-material-v3":            CaseMaterialSchemaVersionV3,
		"condition-probe-v3":          ConditionProbeSchemaVersionV3,
		"condition-probe-batch-v3":    ConditionProbeBatchSchemaVersionV3,
		"formal-human-comparison-v3":  FormalHumanComparisonSchemaVersionV3,
		"mapping-reveal-v3":           MappingRevealSchemaVersionV3,
		"pair-judgment-v3":            PairJudgmentSchemaVersionV3,
		"plan-v3":                     PlanSchemaVersionV3,
		"pilot-sample-v3":             PilotSampleSchemaVersionV3,
		"pilot-readiness-v3":          RelationPilotReadinessSchemaVersionV3,
		"pilot-change-receipt-v3":     PilotChangeReceiptSchemaVersionV3,
		"pilot-inspection-v3":         PilotInspectionSchemaVersionV3,
		"pilot-launch-dossier-v3":     PilotLaunchDossierSchemaVersionV3,
		"primary-sample-v3":           PrimarySampleSchemaVersionV3,
		"scarcity-sentinel-v3":        ScarcitySentinelSchemaVersionV3,
		"private-mapping-v3":          PrivateMappingSchemaVersionV3,
		"relation-resolution-v3":      RelationResolutionSchemaVersionV3,
		"qualification-answer-key-v3": QualificationKeySchemaVersionV3,
		"qualification-report-v3":     QualificationReportSchemaVersionV3,
		"qualification-set-v3":        QualificationSetSchemaVersionV3,
		"replay-receipt-v3":           ReplayReceiptSchemaVersionV3,
		"review-assignment-v3":        ReviewAssignmentSchemaVersionV3,
		"review-bundle-v3":            ReviewBundleSchemaVersionV3,
		"reviewer-handbook-v3":        ReviewerHandbookSchemaVersionV3,
		"reviewer-kit-v3":             ReviewerKitSchemaVersionV3,
		"reviewer-record-v3":          ReviewerRecordSchemaVersionV3,
		"judgment-batch-v3":           JudgmentBatchSchemaVersionV3,
		"prereveal-ambiguity-v3":      RelationAmbiguitySchemaVersionV3,
		"study-amendment-v3":          StudyAmendmentSchemaVersionV3,
		"terminal-ledger-v3":          TerminalRelationLedgerSchemaVersionV3,
		"translation-result-v3":       TranslationResultSchemaVersionV3,
	}
	if len(tests) != 31 {
		t.Fatalf("v3 relation schema surface has %d documents, want 30 protocol schemas plus one scarcity sentinel", len(tests))
	}
	for document, version := range tests {
		schema, err := Schema(document)
		if err != nil {
			t.Fatal(err)
		}
		properties := schema["properties"].(map[string]any)
		if properties["schema_version"].(JSONSchema)["const"] != version || properties["protocol_version"].(JSONSchema)["const"] != GovernanceProtocolVersionV3 {
			t.Fatalf("v3 relation schema %q is not version-matched", document)
		}
	}
	file := openGovernedRelationArtifact(t, "relation-audit-plan-v3.json")
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close governed v3 plan: %v", err)
		}
	}()
	if _, err := DecodePlan(file); err == nil {
		t.Fatal("historical relation plan decoder accepted a v3 governance plan")
	}
}

func readGovernedCorpusV3(t *testing.T) (mutation.CorpusDevelopmentPlan, mutation.CorpusDevelopmentAuditV3, mutation.CorpusReleaseV3) {
	t.Helper()
	root := filepath.Join("..", "..", "eval", "governance")
	planFile, err := os.Open(filepath.Join(root, "controlled-corruption-v3-plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := mutation.DecodeCorpusDevelopmentPlan(planFile)
	if closeErr := planFile.Close(); closeErr != nil {
		t.Errorf("close v3 plan: %v", closeErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	auditFile, err := os.Open(filepath.Join(root, "controlled-corruption-v3-natural-audit.json"))
	if err != nil {
		t.Fatal(err)
	}
	audit, err := mutation.DecodeCorpusDevelopmentAuditV3(auditFile, plan)
	if closeErr := auditFile.Close(); closeErr != nil {
		t.Errorf("close v3 audit: %v", closeErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	releaseFile, err := os.Open(filepath.Join(root, "controlled-corruption-v3-release.json"))
	if err != nil {
		t.Fatal(err)
	}
	release, err := mutation.DecodeCorpusReleaseV3(releaseFile, plan, audit)
	if closeErr := releaseFile.Close(); closeErr != nil {
		t.Errorf("close v3 release: %v", closeErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	return plan, audit, release
}

func readGovernedPlanV3(t *testing.T, name string) RelationPlanV3 {
	t.Helper()
	file := openGovernedRelationArtifact(t, name)
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close governed v3 plan: %v", err)
		}
	}()
	value, err := DecodePlanV3(file)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func readGovernedPrimaryV3(t *testing.T, name string, plan RelationPlanV3) PrimarySampleV3 {
	t.Helper()
	file := openGovernedRelationArtifact(t, name)
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close governed v3 primary sample: %v", err)
		}
	}()
	value, err := DecodePrimarySampleV3(file, plan)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func readGovernedSentinelV3(t *testing.T, name string, plan RelationPlanV3, primary PrimarySampleV3) ScarcitySentinelV3 {
	t.Helper()
	file := openGovernedRelationArtifact(t, name)
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close governed v3 sentinel: %v", err)
		}
	}()
	value, err := DecodeScarcitySentinelV3(file, plan, primary)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func readGovernedPilotV3(t *testing.T, name string, plan RelationPlanV3, primary PrimarySampleV3, sentinel ScarcitySentinelV3) PilotSampleV3 {
	t.Helper()
	file := openGovernedRelationArtifact(t, name)
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close governed v3 pilot: %v", err)
		}
	}()
	value, err := DecodePilotSampleV3(file, plan, primary, sentinel)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func readGovernedAmendmentV3(t *testing.T, name string, plan RelationPlanV3, pilot PilotSampleV3, primary PrimarySampleV3, sentinel ScarcitySentinelV3) StudyAmendmentV3 {
	t.Helper()
	file := openGovernedRelationArtifact(t, name)
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close governed v3 amendment: %v", err)
		}
	}()
	value, err := DecodeStudyAmendmentV3(file, plan, pilot, primary, sentinel)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestRelationV3PrimaryFamilySetExcludesOnlyScarcitySentinel(t *testing.T) {
	plan := readGovernedPlanV3(t, "relation-audit-plan-v3.json")
	families := make([]mutation.Family, 0, len(plan.CoreFamilies))
	for _, contract := range plan.CoreFamilies {
		families = append(families, contract.Family)
	}
	if slices.Contains(families, mutation.FamilyTestEvidenceOmitted) || len(families) != 7 {
		t.Fatal("v3 inferential core contains the scarcity sentinel")
	}
}
