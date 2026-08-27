package relation

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestScarcityPublicBriefRendersExactNegativeEvidenceWithoutRestrictedContent(t *testing.T) {
	plan := readGovernedPlanV3(t, "relation-audit-plan-v3.json")
	primary := readGovernedPrimaryV3(t, "relation-primary-sample-v3.json", plan)
	sentinel := readGovernedSentinelV3(t, "relation-scarcity-sentinel-v3.json", plan, primary)
	corpusPlan, audit, release := readGovernedCorpusV3(t)

	evidence, err := BuildScarcityPublicEvidence(plan, primary, sentinel, corpusPlan, audit, release)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := RenderScarcityPublicBriefMarkdown(evidence)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		ScarcityPublicBriefPolicy,
		ScarcityPublicEvidenceSchemaVersion,
		evidence.Digest,
		"198 attempted -> 3 admitted -> 3 selected",
		"**37-case shortfall**",
		"`unverified_evidence_role` | 195",
		"zero test-role sentinel cases",
		"Human construct agreement | not run",
		"Provider behavior | not measured; no provider was invoked",
		"Reviewer contact, packet sharing, or publication authority | not authorized",
	} {
		if !strings.Contains(rendered, required) {
			t.Fatalf("public scarcity brief omitted %q", required)
		}
	}
	for _, forbidden := range []string{"Task requirement", "Original evidence", "Transformed evidence", "Source IDs", "owner-scarcity-inspection"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("public scarcity brief exposed restricted surface %q", forbidden)
		}
	}
}

func TestScarcityPublicBriefRejectsCrossBoundCorpus(t *testing.T) {
	plan := readGovernedPlanV3(t, "relation-audit-plan-v3.json")
	primary := readGovernedPrimaryV3(t, "relation-primary-sample-v3.json", plan)
	sentinel := readGovernedSentinelV3(t, "relation-scarcity-sentinel-v3.json", plan, primary)
	corpusPlan, audit, release := readGovernedCorpusV3(t)
	plan.SourceCorpusDigest = digestText("different-release")
	resealed, err := sealRelationPlanV3(plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildScarcityPublicEvidence(resealed, primary, sentinel, corpusPlan, audit, release); err == nil {
		t.Fatal("public scarcity brief accepted a cross-bound corpus")
	}
}

func TestScarcityPublicEvidenceCodecSchemaAndParentVerification(t *testing.T) {
	plan := readGovernedPlanV3(t, "relation-audit-plan-v3.json")
	primary := readGovernedPrimaryV3(t, "relation-primary-sample-v3.json", plan)
	sentinel := readGovernedSentinelV3(t, "relation-scarcity-sentinel-v3.json", plan, primary)
	corpusPlan, audit, release := readGovernedCorpusV3(t)
	evidence, err := BuildScarcityPublicEvidence(plan, primary, sentinel, corpusPlan, audit, release)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeIndented(evidence)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeScarcityPublicEvidence(strings.NewReader(string(encoded)))
	if err != nil || decoded.Digest != evidence.Digest {
		t.Fatalf("scarcity public evidence round trip failed: %v", err)
	}
	if _, err := Schema("scarcity-public-evidence"); err != nil {
		t.Fatal(err)
	}

	tampered := evidence
	tampered.Parents = append([]ScarcityPublicParent(nil), evidence.Parents...)
	tampered.Parents[0].Digest = digestText("different-public-parent")
	tampered, err = SealScarcityPublicEvidence(tampered)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyScarcityPublicEvidenceDocument(tampered, plan, primary, sentinel, corpusPlan, audit, release); err == nil {
		t.Fatal("resealed scarcity public evidence accepted a different parent")
	}
}

func TestScarcityPublicEvidenceRejectsUnknownField(t *testing.T) {
	plan := readGovernedPlanV3(t, "relation-audit-plan-v3.json")
	primary := readGovernedPrimaryV3(t, "relation-primary-sample-v3.json", plan)
	sentinel := readGovernedSentinelV3(t, "relation-scarcity-sentinel-v3.json", plan, primary)
	corpusPlan, audit, release := readGovernedCorpusV3(t)
	evidence, err := BuildScarcityPublicEvidence(plan, primary, sentinel, corpusPlan, audit, release)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeIndented(evidence)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatal(err)
	}
	document["unexpected"] = true
	unknown, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeScarcityPublicEvidence(strings.NewReader(string(unknown))); err == nil {
		t.Fatal("scarcity public evidence accepted an unknown field")
	}
}
