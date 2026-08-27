package explorer

import (
	"strings"
	"testing"

	claimledger "github.com/Christopher-Schulze/evalwitness/internal/claim"
)

func TestExtensionRegistrySupportsMultiTypeContracts(t *testing.T) {
	firstDigest := strings.Repeat("1", 64)
	secondDigest := strings.Repeat("2", 64)
	required := []string{"example.alpha.v1", "example.beta.v1"}
	components, missing := extensionEvidence(required, map[string][]string{
		"example.alpha.v1": {firstDigest},
		"example.beta.v1":  {secondDigest},
	})
	definition := extensionDefinition{
		id: "example-extension", owner: "TASK-999", required: required,
		unavailable: AvailabilityNotMeasured,
	}
	view := ExtensionView{
		ExtensionID: definition.id, OwnerTask: definition.owner, RequiredTypes: required,
		Components: components, MissingTypes: missing, Availability: AvailabilityAvailable,
	}
	if err := validateExtensionView(view, definition); err != nil {
		t.Fatal(err)
	}
	view.Components = view.Components[:1]
	view.MissingTypes = []string{"example.beta.v1"}
	view.Availability = AvailabilityNotMeasured
	if err := validateExtensionView(view, definition); err != nil {
		t.Fatal(err)
	}
}

func TestSelectChallengeClaimUsesCoverageThenClaimID(t *testing.T) {
	receipts := []claimledger.ChallengeReceipt{
		{ClaimID: "CLM-002"}, {ClaimID: "CLM-001"}, {ClaimID: "CLM-002"}, {ClaimID: "CLM-001"},
	}
	claimID, count, err := selectChallengeClaim(receipts)
	if err != nil {
		t.Fatal(err)
	}
	if claimID != "CLM-001" || count != 2 {
		t.Fatalf("selected %s with %d receipts", claimID, count)
	}
}
