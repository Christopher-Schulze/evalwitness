package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	claimledger "github.com/Christopher-Schulze/evalwitness/internal/claim"
	"github.com/Christopher-Schulze/evalwitness/internal/reliance"
)

type relianceCapsuleBuildReport struct {
	SchemaVersion    string                `json:"schema_version"`
	BaseCapsuleID    string                `json:"base_capsule_id"`
	CapsuleID        string                `json:"capsule_id"`
	ManifestDigest   string                `json:"manifest_digest"`
	RegistryDigest   string                `json:"registry_digest"`
	MapDigest        string                `json:"map_digest"`
	CapsuleDirectory string                `json:"capsule_directory"`
	ArchivePath      string                `json:"archive_path"`
	Archive          capsule.ArchiveReport `json:"archive"`
	LedgerPath       string                `json:"ledger_path"`
	LedgerDigest     string                `json:"ledger_digest"`
	ProviderCalls    int                   `json:"provider_calls"`
	NetworkRequired  bool                  `json:"network_required"`
	Offline          bool                  `json:"offline"`
}

type relianceCapsuleVerifyReport struct {
	SchemaVersion     string `json:"schema_version"`
	BaseCapsuleID     string `json:"base_capsule_id"`
	CapsuleID         string `json:"capsule_id"`
	ManifestDigest    string `json:"manifest_digest"`
	MapDigest         string `json:"map_digest"`
	LedgerDigest      string `json:"ledger_digest"`
	ProfileDigest     string `json:"profile_digest"`
	PaperDigest       string `json:"paper_digest"`
	ExplorerDigest    string `json:"explorer_digest"`
	ClaimsSupported   int    `json:"claims_supported"`
	ClaimsUnsupported int    `json:"claims_unsupported"`
	Dimensions        int    `json:"dimensions"`
	ArtifactsVerified int    `json:"artifacts_verified"`
	ProviderCalls     int    `json:"provider_calls"`
	NetworkRequired   bool   `json:"network_required"`
	Offline           bool   `json:"offline"`
}

type relianceCapsuleBuildInputs struct {
	baseCapsulePath string
	mapPath         string
	destination     string
	archivePath     string
	ledgerPath      string
}

type relianceCapsuleVerifyInputs struct {
	baseCapsulePath string
	source          string
	mapPath         string
	ledgerPath      string
	profilePath     string
	paperPath       string
	explorerPath    string
}

type relianceCapsulePackage struct {
	base      capsule.ReferencePackage
	child     capsule.ReferencePackage
	mapValue  reliance.EvidenceRelianceMap
	ledger    claimledger.Ledger
	ledgerRaw []byte
}

type relianceProjectionArtifacts struct {
	profile     reliance.RelianceProfileProjection
	profileRaw  []byte
	paper       reliance.ReliancePaperProjection
	paperRaw    []byte
	explorer    reliance.RelianceExplorerProjection
	explorerRaw []byte
}

func runRelianceCapsuleBuild(args []string) int {
	flags := flag.NewFlagSet("capsule build-reliance", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	baseCapsulePath := flags.String("base-capsule", "", "frozen public TASK 050 base capsule directory")
	mapPath := flags.String("map", "", "canonical evidence-reliance map")
	destination := flags.String("destination", "", "new reliance capsule directory")
	archivePath := flags.String("archive", "", "new deterministic reliance capsule tar.gz path")
	ledgerPath := flags.String("ledger", "", "new canonical reliance claim-ledger path")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "capsule build-reliance:", err)
		return 2
	}
	if *baseCapsulePath == "" || *mapPath == "" || *destination == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "capsule build-reliance: --base-capsule, --map, and --destination are required; positional arguments are forbidden")
		return 2
	}
	inputs := relianceCapsuleBuildInputs{
		baseCapsulePath: *baseCapsulePath, mapPath: *mapPath, destination: *destination,
		archivePath: *archivePath, ledgerPath: *ledgerPath,
	}
	report, err := buildRelianceCapsulePublication(context.Background(), inputs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule build-reliance:", err)
		return 1
	}
	return writeCanonicalCommandOutput("capsule build-reliance", report)
}

func runRelianceCapsuleVerify(args []string) int {
	flags := flag.NewFlagSet("capsule verify-reliance", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	baseCapsulePath := flags.String("base-capsule", "", "frozen public TASK 050 base capsule directory")
	source := flags.String("source", "", "reliance capsule directory")
	mapPath := flags.String("map", "", "canonical evidence-reliance map")
	ledgerPath := flags.String("ledger", "", "canonical reliance claim ledger")
	profilePath := flags.String("profile", "", "canonical reliance profile projection")
	paperPath := flags.String("paper", "", "canonical reliance paper projection")
	explorerPath := flags.String("explorer", "", "canonical reliance explorer projection")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "capsule verify-reliance:", err)
		return 2
	}
	inputs := relianceCapsuleVerifyInputs{
		baseCapsulePath: *baseCapsulePath, source: *source, mapPath: *mapPath, ledgerPath: *ledgerPath,
		profilePath: *profilePath, paperPath: *paperPath, explorerPath: *explorerPath,
	}
	if !completeRelianceCapsuleVerifyInputs(inputs) || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "capsule verify-reliance: --base-capsule, --source, --map, --ledger, --profile, --paper, and --explorer are required; positional arguments are forbidden")
		return 2
	}
	report, err := verifyRelianceCapsulePublication(context.Background(), inputs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule verify-reliance:", err)
		return 1
	}
	return writeCanonicalCommandOutput("capsule verify-reliance", report)
}

func buildRelianceCapsulePublication(
	ctx context.Context,
	inputs relianceCapsuleBuildInputs,
) (relianceCapsuleBuildReport, error) {
	if ctx == nil || inputs.baseCapsulePath == "" || inputs.mapPath == "" || inputs.destination == "" {
		return relianceCapsuleBuildReport{}, errors.New("reliance capsule build requires context, frozen base capsule, map, and destination")
	}
	inputs.archivePath, inputs.ledgerPath = resolveRelianceCapsuleBuildOutputs(
		inputs.destination, inputs.archivePath, inputs.ledgerPath,
	)
	if err := requireNewCommandTargets([]string{inputs.destination, inputs.archivePath, inputs.ledgerPath}); err != nil {
		return relianceCapsuleBuildReport{}, err
	}
	pack, err := buildRelianceCapsulePackage(ctx, inputs.baseCapsulePath, inputs.mapPath)
	if err != nil {
		return relianceCapsuleBuildReport{}, err
	}
	archive, err := publishCapsuleArtifacts(ctx, inputs.destination, inputs.archivePath,
		pack.child.Registry, pack.child.Manifest, pack.child.Payloads,
		capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic},
		[]capsuleSidecar{{Destination: inputs.ledgerPath, Payload: pack.ledgerRaw}})
	if err != nil {
		return relianceCapsuleBuildReport{}, err
	}
	return relianceCapsuleBuildReport{
		SchemaVersion: "evalwitness.reliance-capsule-build-report.v1", BaseCapsuleID: pack.base.Manifest.CapsuleID,
		CapsuleID: pack.child.Manifest.CapsuleID, ManifestDigest: pack.child.Manifest.ManifestDigest,
		RegistryDigest: pack.child.Registry.Digest(), MapDigest: pack.mapValue.Digest,
		CapsuleDirectory: inputs.destination, ArchivePath: inputs.archivePath, Archive: archive,
		LedgerPath: inputs.ledgerPath, LedgerDigest: pack.ledger.Digest,
		ProviderCalls: 0, NetworkRequired: false, Offline: true,
	}, nil
}

func verifyRelianceCapsulePublication(
	ctx context.Context,
	inputs relianceCapsuleVerifyInputs,
) (relianceCapsuleVerifyReport, error) {
	if ctx == nil || !completeRelianceCapsuleVerifyInputs(inputs) {
		return relianceCapsuleVerifyReport{}, errors.New("reliance capsule verification requires context and every artifact path")
	}
	pack, claimReport, err := loadRelianceCapsulePublication(ctx, inputs)
	if err != nil {
		return relianceCapsuleVerifyReport{}, err
	}
	artifacts, err := buildRelianceProjectionArtifacts(ctx, pack)
	if err != nil {
		return relianceCapsuleVerifyReport{}, err
	}
	if err := verifyRelianceProjectionArtifacts(inputs, artifacts); err != nil {
		return relianceCapsuleVerifyReport{}, err
	}
	return relianceCapsuleVerifyReport{
		SchemaVersion: "evalwitness.reliance-capsule-verify-report.v1", BaseCapsuleID: pack.base.Manifest.CapsuleID,
		CapsuleID: pack.child.Manifest.CapsuleID, ManifestDigest: pack.child.Manifest.ManifestDigest,
		MapDigest: pack.mapValue.Digest, LedgerDigest: pack.ledger.Digest,
		ProfileDigest: artifacts.profile.Digest, PaperDigest: artifacts.paper.Digest,
		ExplorerDigest:    artifacts.explorer.Digest,
		ClaimsSupported:   claimReport.StatusCounts[string(claimledger.StatusSupported)],
		ClaimsUnsupported: claimReport.StatusCounts[string(claimledger.StatusUnsupported)],
		Dimensions:        len(artifacts.profile.Dimensions), ArtifactsVerified: 5,
		ProviderCalls: 0, NetworkRequired: false, Offline: true,
	}, nil
}

func buildRelianceCapsulePackage(
	ctx context.Context,
	baseCapsulePath string,
	mapPath string,
) (relianceCapsulePackage, error) {
	mapValue, _, err := readRelianceMapArtifact(mapPath)
	if err != nil {
		return relianceCapsulePackage{}, err
	}
	base, err := loadRelianceBaseCapsule(ctx, baseCapsulePath)
	if err != nil {
		return relianceCapsulePackage{}, fmt.Errorf("load frozen reliance base capsule: %w", err)
	}
	child, err := reliance.BuildEvidenceRelianceCapsule(base, mapValue)
	if err != nil {
		return relianceCapsulePackage{}, fmt.Errorf("build reliance capsule: %w", err)
	}
	ledger, err := reliance.BuildEvidenceRelianceLedger(ctx, base, child)
	if err != nil {
		return relianceCapsulePackage{}, fmt.Errorf("build reliance claim ledger: %w", err)
	}
	ledgerRaw, err := claimledger.EncodeLedger(ledger)
	if err != nil {
		return relianceCapsulePackage{}, fmt.Errorf("encode reliance claim ledger: %w", err)
	}
	return relianceCapsulePackage{base, child, mapValue, ledger, ledgerRaw}, nil
}

func loadRelianceCapsulePublication(
	ctx context.Context,
	inputs relianceCapsuleVerifyInputs,
) (relianceCapsulePackage, claimledger.VerificationReport, error) {
	base, err := loadRelianceBaseCapsule(ctx, inputs.baseCapsulePath)
	if err != nil {
		return relianceCapsulePackage{}, claimledger.VerificationReport{}, fmt.Errorf("load frozen reliance base capsule: %w", err)
	}
	child, err := reliance.LoadEvidenceRelianceCapsule(ctx, inputs.source, base)
	if err != nil {
		return relianceCapsulePackage{}, claimledger.VerificationReport{}, fmt.Errorf("load reliance capsule: %w", err)
	}
	mapValue, _, err := readRelianceMapArtifact(inputs.mapPath)
	if err != nil {
		return relianceCapsulePackage{}, claimledger.VerificationReport{}, err
	}
	if err := verifyRelianceCapsuleMapIdentity(base, child, mapValue); err != nil {
		return relianceCapsulePackage{}, claimledger.VerificationReport{}, err
	}
	ledger, ledgerRaw, err := readRelianceLedgerArtifact(inputs.ledgerPath)
	if err != nil {
		return relianceCapsulePackage{}, claimledger.VerificationReport{}, err
	}
	pack := relianceCapsulePackage{base, child, mapValue, ledger, ledgerRaw}
	report, err := verifyRelianceLedgerArtifact(ctx, pack)
	return pack, report, err
}

func buildRelianceProjectionArtifacts(
	ctx context.Context,
	pack relianceCapsulePackage,
) (relianceProjectionArtifacts, error) {
	profile, err := reliance.BuildRelianceProfileProjection(ctx, pack.base, pack.child, pack.ledger)
	if err != nil {
		return relianceProjectionArtifacts{}, err
	}
	paper, err := reliance.BuildReliancePaperProjection(ctx, pack.base, pack.child, pack.ledger)
	if err != nil {
		return relianceProjectionArtifacts{}, err
	}
	explorer, err := reliance.BuildRelianceExplorerProjection(ctx, pack.base, pack.child, pack.ledger)
	if err != nil {
		return relianceProjectionArtifacts{}, err
	}
	profileRaw, err := json.Marshal(profile)
	if err != nil {
		return relianceProjectionArtifacts{}, err
	}
	paperRaw, err := json.Marshal(paper)
	if err != nil {
		return relianceProjectionArtifacts{}, err
	}
	explorerRaw, err := json.Marshal(explorer)
	if err != nil {
		return relianceProjectionArtifacts{}, err
	}
	return relianceProjectionArtifacts{profile, profileRaw, paper, paperRaw, explorer, explorerRaw}, nil
}

func verifyRelianceProjectionArtifacts(
	inputs relianceCapsuleVerifyInputs,
	artifacts relianceProjectionArtifacts,
) error {
	for _, artifact := range []struct {
		path string
		raw  []byte
	}{
		{inputs.profilePath, artifacts.profileRaw},
		{inputs.paperPath, artifacts.paperRaw},
		{inputs.explorerPath, artifacts.explorerRaw},
	} {
		actual, err := readBoundedCommandFile(artifact.path)
		if err != nil {
			return err
		}
		if !bytes.Equal(actual, artifact.raw) {
			return fmt.Errorf("reliance projection %q differs from deterministic capsule recomputation", artifact.path)
		}
	}
	return nil
}

func verifyRelianceCapsuleMapIdentity(
	base capsule.ReferencePackage,
	child capsule.ReferencePackage,
	value reliance.EvidenceRelianceMap,
) error {
	expected, err := reliance.BuildEvidenceRelianceCapsule(base, value)
	if err != nil {
		return err
	}
	if expected.Manifest.CapsuleID != child.Manifest.CapsuleID ||
		expected.Manifest.ManifestDigest != child.Manifest.ManifestDigest ||
		expected.Registry.Digest() != child.Registry.Digest() {
		return errors.New("reliance capsule differs from the declared canonical map")
	}
	return nil
}

func verifyRelianceLedgerArtifact(
	ctx context.Context,
	pack relianceCapsulePackage,
) (claimledger.VerificationReport, error) {
	expected, err := reliance.BuildEvidenceRelianceLedger(ctx, pack.base, pack.child)
	if err != nil {
		return claimledger.VerificationReport{}, err
	}
	expectedRaw, err := claimledger.EncodeLedger(expected)
	if err != nil || !bytes.Equal(expectedRaw, pack.ledgerRaw) {
		return claimledger.VerificationReport{}, errors.Join(err, errors.New("reliance claim ledger differs from deterministic capsule recomputation"))
	}
	return claimledger.VerifyLedger(ctx, pack.child.Registry, pack.child.Manifest, pack.child.Payloads, pack.ledger)
}

func readRelianceMapArtifact(path string) (reliance.EvidenceRelianceMap, []byte, error) {
	raw, err := readBoundedCommandFile(path)
	if err != nil {
		return reliance.EvidenceRelianceMap{}, nil, err
	}
	value, err := reliance.DecodeEvidenceRelianceMap(raw)
	if err != nil {
		return reliance.EvidenceRelianceMap{}, nil, err
	}
	return value, raw, nil
}

func readRelianceLedgerArtifact(path string) (claimledger.Ledger, []byte, error) {
	raw, err := readBoundedCommandFile(path)
	if err != nil {
		return claimledger.Ledger{}, nil, err
	}
	value, err := claimledger.DecodeLedger(raw)
	if err != nil {
		return claimledger.Ledger{}, nil, err
	}
	return value, raw, nil
}

func loadRelianceBaseCapsule(ctx context.Context, path string) (capsule.ReferencePackage, error) {
	registry, err := capsule.ReferenceRegistry()
	if err != nil {
		return capsule.ReferencePackage{}, err
	}
	manifest, payloads, err := capsule.LoadDirectory(ctx, path, registry,
		capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic})
	if err != nil {
		return capsule.ReferencePackage{}, err
	}
	return capsule.ReferencePackage{Registry: registry, Manifest: manifest, Payloads: payloads}, nil
}

func resolveRelianceCapsuleBuildOutputs(destination, archivePath, ledgerPath string) (string, string) {
	if archivePath == "" {
		archivePath = destination + ".tar.gz"
	}
	if ledgerPath == "" {
		ledgerPath = destination + ".claims.json"
	}
	return archivePath, ledgerPath
}

func completeRelianceCapsuleVerifyInputs(inputs relianceCapsuleVerifyInputs) bool {
	return inputs.baseCapsulePath != "" && inputs.source != "" && inputs.mapPath != "" && inputs.ledgerPath != "" &&
		inputs.profilePath != "" && inputs.paperPath != "" && inputs.explorerPath != ""
}
