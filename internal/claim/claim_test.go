package claim

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

func TestDefaultLedgerVerifiesEveryMigratedClaim(t *testing.T) {
	pack := buildClaimReferencePackage(t)
	ledger, err := DefaultLedger(pack.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	report, err := VerifyLedger(context.Background(), pack.Registry, pack.Manifest, pack.Payloads, ledger)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Valid || !report.Offline || len(report.Claims) != 34 {
		t.Fatalf("claim verification report = %+v", report)
	}
	wantCounts := map[string]int{"supported": 13, "exploratory": 2, "unsupported": 18, "superseded": 1}
	if !mapsEqual(report.StatusCounts, wantCounts) {
		t.Fatalf("claim status counts = %+v, want %+v", report.StatusCounts, wantCounts)
	}
	stale, err := Lookup(ledger, "CLM-007")
	if err != nil {
		t.Fatal(err)
	}
	if stale.Status != StatusUnsupported || stale.Attestation != AttestationStale {
		t.Fatalf("CLM-007 was not demoted at the stale-attestation boundary: %+v", stale)
	}
	counterfactual, err := Lookup(ledger, "CLM-014")
	if err != nil {
		t.Fatal(err)
	}
	if counterfactual.Status != StatusUnsupported {
		t.Fatalf("CLM-014 retained exact-response language without public response bytes: %+v", counterfactual)
	}
	projection, err := BuildProjection(context.Background(), pack.Registry, pack.Manifest, pack.Payloads, ledger)
	if err != nil {
		t.Fatal(err)
	}
	if len(projection.CurrentClaims) != 15 || len(projection.HistoricalClaims) != 19 || projection.CurrentClaims[0].ClaimID != "CLM-001" {
		t.Fatalf("claim projection lifecycle split = %+v", projection)
	}
}

func TestReferenceChallengesReachOnlyTheirDeclaredGuards(t *testing.T) {
	pack := buildClaimReferencePackage(t)
	ledger, err := DefaultLedger(pack.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	challengeIDs := []string{
		"clm-001.caveat-removal",
		"clm-011.denominator-deletion",
		"clm-001.digest-substitution",
		"clm-001.parent-removal",
		"clm-001.scope-widening",
		"clm-001.superseded-generation",
		"clm-001.visibility-leak",
	}
	sealedBefore, err := sealedPackageDigest(pack.Registry, pack.Manifest, pack.Payloads)
	if err != nil {
		t.Fatal(err)
	}
	for _, challengeID := range challengeIDs {
		claimID := "CLM-001"
		if challengeID == "clm-011.denominator-deletion" {
			claimID = "CLM-011"
		}
		receipt, err := Challenge(context.Background(), pack.Registry, pack.Manifest, pack.Payloads, ledger, claimID, challengeID)
		if err != nil {
			t.Fatalf("challenge %q: %v", challengeID, err)
		}
		if err := receipt.Validate(); err != nil {
			t.Fatalf("receipt %q: %v", challengeID, err)
		}
	}
	sealedAfter, err := sealedPackageDigest(pack.Registry, pack.Manifest, pack.Payloads)
	if err != nil {
		t.Fatal(err)
	}
	if sealedAfter != sealedBefore {
		t.Fatal("challenge harness changed the sealed source package")
	}
}

func TestReferenceChallengePackCoversEveryApplicableCurrentGuard(t *testing.T) {
	pack := buildClaimReferencePackage(t)
	ledger, err := DefaultLedger(pack.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	challenges, err := BuildChallengePack(context.Background(), pack.Registry, pack.Manifest, pack.Payloads, ledger)
	if err != nil {
		t.Fatal(err)
	}
	if len(challenges.Receipts) != 189 {
		t.Fatalf("challenge receipt count = %d, want 189", len(challenges.Receipts))
	}
	for _, class := range []ChallengeClass{
		ChallengeCaveatRemoval, ChallengeDenominatorDeletion, ChallengeDigestSubstitution,
		ChallengeParentRemoval, ChallengeScopeWidening, ChallengeSupersededGeneration, ChallengeVisibilityLeak,
	} {
		if challenges.ClassCounts[string(class)] == 0 {
			t.Fatalf("challenge pack omits applicable class %q", class)
		}
	}
	if challenges.ClassCounts[string(ChallengeStaleAttestation)] != 0 {
		t.Fatal("reference challenge pack fabricated a current attestation")
	}
}

func TestStaleAttestationGuardIsFailable(t *testing.T) {
	pack := buildClaimReferencePackage(t)
	ledger, err := DefaultLedger(pack.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	item, err := Lookup(ledger, "CLM-001")
	if err != nil {
		t.Fatal(err)
	}
	item.Attestation = AttestationCurrent
	state := initialVerificationState(item)
	state.Attestation = AttestationStale
	_, err = evaluateClaimState(item, newEvidenceSet(pack.Manifest, pack.Payloads), state)
	var guarded GuardError
	if !errors.As(err, &guarded) || guarded.Guard != GuardAttestationStale {
		t.Fatalf("stale attestation reached %v", err)
	}
}

func TestExpressionEngineUsesExactRationalsAndRFC6901(t *testing.T) {
	raw := []byte(`{"array":[1,2,3],"left":7.5,"nested":{"a/b":"value","tilde~key":2.5},"right":2.5,"same":7.5}`)
	digest := protocol.DigestBytes(raw)
	evidence := evidenceSet{
		records:  map[string]capsule.ComponentRecord{"fixture": {Name: "fixture", TypeID: "fixture.v1", Payload: capsule.PayloadIdentity{Digest: digest}}},
		payloads: map[string][]byte{digest: raw},
	}
	tests := []struct {
		name       string
		expression Expression
		want       ExactValue
	}{
		{name: "pointer escape", expression: pointer("fixture", "fixture.v1", "/nested/a~1b", StringValue("value")), want: StringValue("value")},
		{name: "count", expression: count("fixture", "fixture.v1", "/array", "3"), want: NumberValue("3")},
		{name: "sum", expression: Expression{Operation: OperationSum, Operands: []Operand{{Component: "fixture", TypeID: "fixture.v1", Pointer: "/array"}}, Expected: NumberValue("6")}, want: NumberValue("6")},
		{name: "ratio", expression: ratio("fixture", "fixture.v1", "/left", "fixture", "fixture.v1", "/right", "3"), want: NumberValue("3")},
		{name: "difference", expression: difference("fixture", "fixture.v1", "/left", "fixture", "fixture.v1", "/right", "5"), want: NumberValue("5")},
		{name: "all equal", expression: Expression{Operation: OperationAllEqual, Operands: []Operand{{Component: "fixture", TypeID: "fixture.v1", Pointer: "/left"}, {Component: "fixture", TypeID: "fixture.v1", Pointer: "/same"}}, Expected: BooleanValue(true)}, want: BooleanValue(true)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := evaluateExpression(test.expression, evidence)
			if err != nil {
				t.Fatal(err)
			}
			if !actual.Equal(test.want) {
				t.Fatalf("expression value = %+v, want %+v", actual, test.want)
			}
		})
	}
	exponent, err := exactJSONNumber("1.25e-3")
	if err != nil || exponent.RatString() != "1/800" {
		t.Fatalf("exact exponent = %v, %v", exponent, err)
	}
}

func TestLedgerRejectsMissingChallengeAndCaveat(t *testing.T) {
	pack := buildClaimReferencePackage(t)
	ledger, err := DefaultLedger(pack.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	missingChallenge := ledger
	missingChallenge.Claims = cloneClaims(ledger.Claims)
	missingChallenge.Claims[0].Challenges = slices.Clone(missingChallenge.Claims[0].Challenges[:len(missingChallenge.Claims[0].Challenges)-1])
	missingChallenge.Digest, err = ledgerDigest(missingChallenge)
	if err != nil {
		t.Fatal(err)
	}
	if err := missingChallenge.Validate(); err == nil {
		t.Fatal("ledger accepted a claim with an omitted challenge class")
	}

	missingCaveat := ledger
	missingCaveat.Claims = cloneClaims(ledger.Claims)
	missingCaveat.Claims[0].Caveats = []string{}
	missingCaveat.Digest, err = ledgerDigest(missingCaveat)
	if err != nil {
		t.Fatal(err)
	}
	if err := missingCaveat.Validate(); err == nil {
		t.Fatal("ledger accepted a claim without its required caveat")
	}
}

func TestLedgerRejectsChallengeApplicabilityAndPresenceOnlyPromotion(t *testing.T) {
	pack := buildClaimReferencePackage(t)
	ledger, err := DefaultLedger(pack.Manifest)
	if err != nil {
		t.Fatal(err)
	}

	invalidChallenge := ledger
	invalidChallenge.Claims = cloneClaims(ledger.Claims)
	item := &invalidChallenge.Claims[0]
	item.EvidenceCeiling.RequiredCaveats = []string{}
	item.Caveats = []string{}
	invalidChallenge.Digest, err = ledgerDigest(invalidChallenge)
	if err != nil {
		t.Fatal(err)
	}
	if err := invalidChallenge.Validate(); err == nil {
		t.Fatal("ledger accepted an applied caveat-removal challenge without a required caveat")
	}

	presenceOnly := ledger
	presenceOnly.Claims = cloneClaims(ledger.Claims)
	presenceOnly.Claims[0].Expression = exists(componentSourceTree, capsule.SourceTreeProvenanceSchemaVersion)
	presenceOnly.Digest, err = ledgerDigest(presenceOnly)
	if err != nil {
		t.Fatal(err)
	}
	if err := presenceOnly.Validate(); err == nil {
		t.Fatal("ledger promoted an assertable claim from component presence alone")
	}
}

var (
	claimReferencePackageOnce sync.Once
	claimReferencePackage     capsule.ReferencePackage
	claimReferencePackageErr  error
)

func buildClaimReferencePackage(t *testing.T) capsule.ReferencePackage {
	t.Helper()
	claimReferencePackageOnce.Do(func() {
		claimReferencePackage, claimReferencePackageErr = capsule.BuildReferencePackage(filepath.Join("..", ".."))
	})
	if claimReferencePackageErr != nil {
		t.Fatal(claimReferencePackageErr)
	}
	return cloneClaimReferencePackage(claimReferencePackage)
}

func cloneClaimReferencePackage(source capsule.ReferencePackage) capsule.ReferencePackage {
	cloned := source
	cloned.Manifest.ParentCapsules = slices.Clone(source.Manifest.ParentCapsules)
	cloned.Manifest.ScientificRoots = slices.Clone(source.Manifest.ScientificRoots)
	cloned.Manifest.PresentationRoots = slices.Clone(source.Manifest.PresentationRoots)
	cloned.Manifest.Components = slices.Clone(source.Manifest.Components)
	for index := range cloned.Manifest.Components {
		cloned.Manifest.Components[index].Parents = slices.Clone(source.Manifest.Components[index].Parents)
	}
	cloned.Payloads = make(map[string][]byte, len(source.Payloads))
	for digest, raw := range source.Payloads {
		cloned.Payloads[digest] = slices.Clone(raw)
	}
	return cloned
}

func mapsEqual(left, right map[string]int) bool {
	leftRaw, leftErr := json.Marshal(left)
	rightRaw, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && slices.Equal(leftRaw, rightRaw)
}
