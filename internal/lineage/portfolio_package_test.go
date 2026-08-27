package lineage

import "testing"

func TestDevelopmentPortfolioPackageIsClaimBoundedAndComplete(t *testing.T) {
	bom, err := BuildVerificationLineageOfflineBOM("../..")
	if err != nil {
		t.Fatal(err)
	}
	card, err := BuildVerificationLineageDevelopmentDatasetCard("../..")
	if err != nil {
		t.Fatal(err)
	}
	limitations, err := BuildVerificationLineageLimitationsLedger("../..")
	if err != nil {
		t.Fatal(err)
	}
	release, err := BuildVerificationLineageDevelopmentRelease("../..")
	if err != nil {
		t.Fatal(err)
	}
	if release.AuditDigest != bom.AuditDigest || release.DatasetCardDigest != card.Header.Digest || release.LimitationsDigest != limitations.Digest {
		t.Fatal("development release detached from its canonical BOM, dataset card, or limitations")
	}
	if len(release.Files) != 20 || card.Counts[3] != (DatasetCardCount{Name: "empirical_task_groups", Value: 0}) || len(limitations.Limitations) != 7 {
		t.Fatal("development package counts drifted")
	}
	if release.ProviderCallsRequired != 0 || !release.PublicProjection || !release.RestrictedMaterialExcluded {
		t.Fatal("development release crossed its public provider-free boundary")
	}
}

func TestDevelopmentPortfolioPackageRejectsClaimAndManifestTampering(t *testing.T) {
	limitations, err := BuildVerificationLineageLimitationsLedger("../..")
	if err != nil {
		t.Fatal(err)
	}
	limitations.Limitations[1].Status = "measured"
	if err := limitations.Validate(); err == nil {
		t.Fatal("limitations accepted a claim-changing mutation")
	}
	release, err := BuildVerificationLineageDevelopmentRelease("../..")
	if err != nil {
		t.Fatal(err)
	}
	release.Files[0].Digest = digestBytes([]byte("tampered"))
	if err := release.Validate(); err == nil {
		t.Fatal("release accepted a file-manifest mutation")
	}
}
