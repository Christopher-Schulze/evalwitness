package replay

import (
	"testing"
)

func TestResponseBundleChallengeGuards(t *testing.T) {
	capturePath := buildResponseBundleCapture(t)
	sources := map[string]string{"primary": capturePath}
	policy, evidence := responseBundlePolicyForTest(t, sources)
	redistributionEvidence := responseBundleRedistributionEvidenceForTest()
	build, err := BuildResponseBundle(policy, evidence, redistributionEvidence, sources, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	alternate := buildResponseBundleCaptureWithPrompt(t, "other request corpus")
	suite, err := ChallengeResponseBundle(build, policy, evidence, redistributionEvidence, alternate)
	if err != nil {
		t.Fatal(err)
	}
	if len(suite.Receipts) != 7 {
		t.Fatalf("receipt count = %d", len(suite.Receipts))
	}
	for _, receipt := range suite.Receipts {
		if !receipt.Failed {
			t.Fatalf("challenge %s did not trip its guard: %+v", receipt.ChallengeID, receipt)
		}
		if receipt.SourcePayloadSHA256 == "" || receipt.Digest == "" {
			t.Fatalf("challenge %s missing identities: %+v", receipt.ChallengeID, receipt)
		}
	}
}
