package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/relation"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

func TestVerifyPilotPackageInventoryRejectsTamperAndExtraPayload(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	packets := filepath.Join(root, "packets")
	if err := os.Mkdir(packets, 0o700); err != nil {
		t.Fatal(err)
	}
	payload := filepath.Join(packets, "01.json")
	if err := os.WriteFile(payload, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	inventory := relation.PilotPackageInventory{
		SchemaVersion: relation.PilotPackageInventorySchemaVersion,
		PackageFormat: relation.PilotPackageFormatV5,
		HashAlgorithm: relation.PilotPackageInventoryHashAlgorithm,
		Scope:         relation.PilotPackageInventoryScope,
		Directories:   []relation.PilotPackageInventoryDirectory{{Path: "packets", Mode: "0700"}},
		Files: []relation.PilotPackageInventoryFile{{
			Path: "packets/01.json", Bytes: 3, Mode: "0600",
			SHA256: "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		}},
		PayloadFiles: 1, PayloadBytes: 3,
		Digest: "7b6c081ec4a00f7765d55184aaa7b22460723ac351133a422044433a2d9f2aa4",
	}
	if err := inventory.Validate(); err != nil {
		t.Fatal(err)
	}
	encodedInventory, err := relation.EncodeIndented(inventory)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "package-inventory.json"), encodedInventory, 0o600); err != nil {
		t.Fatal(err)
	}
	inventorySHA := sha256.Sum256(encodedInventory)
	manifest := fmt.Sprintf(
		"%x  package-inventory.json\n%s  packets/01.json\n",
		inventorySHA, inventory.Files[0].SHA256,
	)
	if err := os.WriteFile(filepath.Join(root, "SHA256SUMS"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyPilotPackageInventory(root, inventory); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(payload, []byte("abd"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyPilotPackageInventory(root, inventory); err == nil {
		t.Fatal("package verifier accepted changed payload bytes")
	}
	if err := os.WriteFile(payload, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "undeclared.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyPilotPackageInventory(root, inventory); err == nil {
		t.Fatal("package verifier accepted an undeclared payload")
	}
}

func TestJournalStorageAllowsOneConcurrentSequenceWriterAndRejectsGap(t *testing.T) {
	policy, err := safety.CurrentPathPolicy()
	if err != nil {
		t.Fatal(err)
	}
	root, err := safety.CreateCacheRoot(policy, filepath.Join(t.TempDir(), "journal-vault"))
	if err != nil {
		t.Fatal(err)
	}
	const writers = 12
	start := make(chan struct{})
	results := make(chan error, writers)
	var group sync.WaitGroup
	for range writers {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			results <- root.PublishSensitiveExclusive("pilot-inspection-sessions/concurrent/events/000001.json", []byte("{}\n"))
		}()
	}
	close(start)
	group.Wait()
	close(results)
	successes := 0
	for result := range results {
		if result == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent sequence publication produced %d winners, want 1", successes)
	}

	sessionDigest := strings.Repeat("a", 64)
	gapPath := filepath.Join("pilot-inspection-sessions", sessionDigest, "events", "000002.json")
	if err := root.PublishSensitiveExclusive(gapPath, []byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPilotInspectionEvents(root, sessionDigest); err == nil {
		t.Fatal("journal loader accepted a missing first sequence event")
	}
}

func TestPilotInspectionGuideLocatesExactSectionsWithoutRenderingEvidence(t *testing.T) {
	root := t.TempDir()
	workbook := strings.Join([]string{
		"# Workbook", "", "## Packet 1 of 2", "", "- Packet ID: `relation-packet-a`", "restricted-a",
		"## Packet 2 of 2", "", "- Packet ID: `relation-packet-b`", "restricted-b", "## Owner completion gate", "done",
	}, "\n")
	atlas := strings.Join([]string{
		"# Atlas", "", "## Packet 1 of 2: `family`", "", "- Packet ID: `relation-packet-a`", "change-a",
		"## Packet 2 of 2: `family`", "", "- Packet ID: `relation-packet-b`", "change-b", "## Atlas completion boundary", "done",
	}, "\n")
	scarcity := strings.Join([]string{
		"# Scarcity", "", "## Frozen scarcity boundary", "frozen", "## Sentinel case 1 of 1", "", "- Case ID: `case-a`", "restricted",
		"## Owner completion gate", "boundary",
	}, "\n")
	for path, content := range map[string]string{
		pilotInspectionWorkbookPath: workbook,
		pilotInspectionAtlasPath:    atlas,
		pilotInspectionScarcityPath: scarcity,
	} {
		if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	loaded := guidedPilotPackage{
		root: root,
		inventory: relation.PilotPackageInventory{Files: []relation.PilotPackageInventoryFile{
			{Path: pilotInspectionWorkbookPath, SHA256: strings.Repeat("1", 64)},
			{Path: pilotInspectionAtlasPath, SHA256: strings.Repeat("2", 64)},
			{Path: pilotInspectionScarcityPath, SHA256: strings.Repeat("3", 64)},
		}},
	}
	core, err := pilotInspectionGuideLocations(loaded, relation.PilotInspectionTarget{
		SubjectKind: relation.PilotInspectionSubjectCorePacket, SubjectID: "relation-packet-a",
		Dimension: relation.PilotInspectionDimensionTaskContext,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(core) != 2 || core[0].Path != pilotInspectionAtlasPath || core[0].StartLine != 3 || core[0].EndLine != 6 ||
		core[1].Path != pilotInspectionWorkbookPath || core[1].StartLine != 3 || core[1].EndLine != 6 {
		t.Fatalf("unexpected core navigation: %+v", core)
	}
	sentinel, err := pilotInspectionGuideLocations(loaded, relation.PilotInspectionTarget{
		SubjectKind: relation.PilotInspectionSubjectScarcityCase, SubjectID: "case-a",
		Dimension: relation.PilotInspectionDimensionScarcityOriginalEvidence,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sentinel) != 1 || sentinel[0].StartLine != 5 || sentinel[0].EndLine != 8 {
		t.Fatalf("unexpected scarcity navigation: %+v", sentinel)
	}
	boundary, err := pilotInspectionGuideLocations(loaded, relation.PilotInspectionTarget{
		SubjectKind: relation.PilotInspectionSubjectScarcityBoundary, SubjectID: relation.PilotInspectionScarcityBoundaryID,
		Dimension: relation.PilotInspectionDimensionScarcityNonAuthorization,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(boundary) != 2 || boundary[0].StartLine != 3 || boundary[0].EndLine != 4 || boundary[1].StartLine != 9 {
		t.Fatalf("unexpected boundary navigation: %+v", boundary)
	}
}
