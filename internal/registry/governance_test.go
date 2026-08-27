package registry

import "testing"

func TestValidateGovernanceActionRejectsHistoryRewrite(t *testing.T) {
	err := ValidateGovernanceAction(GovernanceAction{
		Action: GovernanceActionDispute, EntryID: "e1", Actor: "maintainer", Reason: "stale route",
		NextStatus: EvidenceStatusDisputed, RewriteHistory: true,
	})
	if err == nil {
		t.Fatal("history rewrite accepted")
	}
	if err := ValidateGovernanceAction(GovernanceAction{
		Action: GovernanceActionCorrection, EntryID: "e1", Actor: "maintainer", Reason: "typo in license",
		NextStatus: EvidenceStatusIndependentlyReproduced,
	}); err == nil {
		t.Fatal("correction promotion accepted")
	}
	if err := ValidateGovernanceAction(GovernanceAction{
		Action: GovernanceActionWithdrawal, EntryID: "e1", Actor: "contributor", Reason: "takedown request",
		NextStatus: EvidenceStatusWithdrawn,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateSchemaMigrationRejectsRewrite(t *testing.T) {
	if err := ValidateSchemaMigration(SchemaMigration{FromSchema: 1, ToSchema: 2, Rewrites: true}); err == nil {
		t.Fatal("rewriting migration accepted")
	}
	if err := ValidateSchemaMigration(SchemaMigration{FromSchema: 1, ToSchema: 2}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateContributorRejectsUnknownMode(t *testing.T) {
	if err := ValidateContributor(ContributorRecord{ContributorID: "c1", Mode: "verified_provider"}); err == nil {
		t.Fatal("provider-certified contributor accepted")
	}
	if err := ValidateContributor(ContributorRecord{ContributorID: "c1", Mode: ContributorModeAnonymous}); err != nil {
		t.Fatal(err)
	}
}
