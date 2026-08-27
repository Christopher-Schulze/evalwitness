package registry

import "testing"

func TestRenderContractMatrixGroupsCompatibleContractsWithoutRanking(t *testing.T) {
	first := validEntry("z-last")
	second := validEntry("a-first")
	other := validEntry("other-contract")
	other.RequestContract = "evalwitness.other.prompt.v1"
	matrix, err := RenderContractMatrix([]IntakeEntry{first, other, second})
	if err != nil {
		t.Fatal(err)
	}
	if len(matrix.Cells) != 2 {
		t.Fatalf("cells = %d", len(matrix.Cells))
	}
	var grouped *ContractMatrixCell
	for i := range matrix.Cells {
		if matrix.Cells[i].RequestContract == first.RequestContract {
			grouped = &matrix.Cells[i]
			break
		}
	}
	if grouped == nil || len(grouped.Entries) != 2 {
		t.Fatalf("grouped cell = %+v cells = %+v", grouped, matrix.Cells)
	}
	if grouped.Entries[0].EntryID != "a-first" || grouped.Entries[1].EntryID != "z-last" {
		t.Fatalf("entry order = %+v", grouped.Entries)
	}
	if matrix.Digest == "" {
		t.Fatal("missing digest")
	}
}

func TestMergeContractMatrixKeepsHistoricalMembers(t *testing.T) {
	older := validEntry("old-fail")
	previous, err := RenderContractMatrix([]IntakeEntry{older})
	if err != nil {
		t.Fatal(err)
	}
	newer := validEntry("new-pass")
	merged, err := MergeContractMatrix(previous, []IntakeEntry{newer})
	if err != nil {
		t.Fatal(err)
	}
	if merged.ParentDigest != previous.Digest || len(merged.Cells) != 1 || len(merged.Cells[0].Entries) != 2 {
		t.Fatalf("merged = %+v", merged)
	}
	if merged.Cells[0].Entries[0].EntryID != "new-pass" || merged.Cells[0].Entries[1].EntryID != "old-fail" {
		t.Fatalf("history order = %+v", merged.Cells[0].Entries)
	}
}

func TestMergeContractMatrixRejectsTamperedHistory(t *testing.T) {
	previous, err := RenderContractMatrix([]IntakeEntry{validEntry("hist")})
	if err != nil {
		t.Fatal(err)
	}
	previous.Digest = "00" + previous.Digest[2:]
	if _, err := MergeContractMatrix(previous, []IntakeEntry{validEntry("next")}); err == nil {
		t.Fatal("tampered history accepted")
	}
}

func TestRenderContractMatrixRejectsDuplicateCatalog(t *testing.T) {
	entry := validEntry("dup")
	if _, err := RenderContractMatrix([]IntakeEntry{entry, entry}); err == nil {
		t.Fatal("duplicate catalog accepted")
	}
}
