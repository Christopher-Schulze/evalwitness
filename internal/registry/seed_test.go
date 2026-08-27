package registry

import (
	"testing"
)

func TestLoadSeedCatalogIsContrastingDevelopmentOnly(t *testing.T) {
	catalog, err := LoadSeedCatalog("../../eval/governance/registry-seed-catalog-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) != 2 {
		t.Fatalf("seed entries = %d, want 2", len(catalog))
	}
	if catalog[0].RequestContract == catalog[1].RequestContract && catalog[0].EndpointKind == catalog[1].EndpointKind {
		t.Fatal("seed entries share one contract cell")
	}
	report, err := RefreshCatalog(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if report.Current != 2 || report.Rejected != 0 {
		t.Fatalf("seed refresh = %+v", report)
	}
	matrix, err := RenderContractMatrix(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if len(matrix.Cells) != 2 {
		t.Fatalf("matrix cells = %d, want 2", len(matrix.Cells))
	}
}
