package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	claimledger "github.com/Christopher-Schulze/evalwitness/internal/claim"
	"github.com/Christopher-Schulze/evalwitness/internal/relation"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const maximumEvidenceCommandFileBytes = 128 << 20

type capsuleBuildReport struct {
	SchemaVersion    string                `json:"schema_version"`
	CapsuleID        string                `json:"capsule_id"`
	ManifestDigest   string                `json:"manifest_digest"`
	RegistryDigest   string                `json:"registry_digest"`
	CapsuleDirectory string                `json:"capsule_directory"`
	ArchivePath      string                `json:"archive_path"`
	Archive          capsule.ArchiveReport `json:"archive"`
	LedgerPath       string                `json:"ledger_path"`
	LedgerDigest     string                `json:"ledger_digest"`
	StatementPath    string                `json:"statement_path"`
	ProjectionPath   string                `json:"projection_path"`
	ProjectionDigest string                `json:"projection_digest"`
	AutopsyPath      string                `json:"autopsy_path"`
	AutopsyDigest    string                `json:"autopsy_digest"`
	ProviderCalls    int                   `json:"provider_calls"`
	Offline          bool                  `json:"offline"`
}

type capsuleVerifyReport struct {
	SchemaVersion      string                          `json:"schema_version"`
	Capsule            capsule.VerificationReport      `json:"capsule"`
	Claims             *claimledger.VerificationReport `json:"claims,omitempty"`
	StatementVerified  bool                            `json:"statement_verified"`
	ProjectionVerified bool                            `json:"projection_verified"`
	AutopsyVerified    bool                            `json:"autopsy_verified"`
	Offline            bool                            `json:"offline"`
}

type privateRelationCapsuleBuildReport struct {
	SchemaVersion    string                `json:"schema_version"`
	CapsuleID        string                `json:"capsule_id"`
	ManifestDigest   string                `json:"manifest_digest"`
	RegistryDigest   string                `json:"registry_digest"`
	CapsuleDirectory string                `json:"capsule_directory"`
	ArchivePath      string                `json:"archive_path"`
	Archive          capsule.ArchiveReport `json:"archive"`
	Components       int                   `json:"components"`
	PackageFiles     int                   `json:"package_files"`
	EventCount       int                   `json:"event_count"`
	Corrections      int                   `json:"corrections"`
	ProviderCalls    int                   `json:"provider_calls"`
	Offline          bool                  `json:"offline"`
}

type privateRelationCapsuleVerifyReport struct {
	SchemaVersion string                     `json:"schema_version"`
	Public        capsule.VerificationReport `json:"public"`
	Private       capsule.VerificationReport `json:"private"`
	FamilyBound   bool                       `json:"family_bound"`
	Offline       bool                       `json:"offline"`
}

func runCapsule(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "capsule: missing subcommand (build|verify|build-reliance|verify-reliance|build-private-relation|verify-private-relation)")
		return 2
	}
	switch args[0] {
	case "build":
		return runCapsuleBuild(args[1:])
	case "verify":
		return runCapsuleVerify(args[1:])
	case "build-reliance":
		return runRelianceCapsuleBuild(args[1:])
	case "verify-reliance":
		return runRelianceCapsuleVerify(args[1:])
	case "build-private-relation":
		return runPrivateRelationCapsuleBuild(args[1:])
	case "verify-private-relation":
		return runPrivateRelationCapsuleVerify(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "capsule: unknown subcommand %q\n", args[0])
		return 2
	}
}

func runPrivateRelationCapsuleBuild(args []string) int {
	flags := flag.NewFlagSet("capsule build-private-relation", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	repositoryRoot := flags.String("repository-root", ".", "repository root")
	packageRoot := flags.String("package-root", "", "exact owner-only package-format-v5 root")
	privateRoot := flags.String("private-root", "", "owner-only inspection journal vault")
	sessionDigest := flags.String("session", "", "completed guided inspection session digest")
	destination := flags.String("destination", "", "new private capsule directory")
	archivePath := flags.String("archive", "", "new deterministic private tar.gz path")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "capsule build-private-relation:", err)
		return 2
	}
	if *packageRoot == "" || *privateRoot == "" || *sessionDigest == "" || *destination == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "capsule build-private-relation: --package-root, --private-root, --session, and --destination are required")
		return 2
	}
	if *archivePath == "" {
		*archivePath = *destination + ".tar.gz"
	}
	if err := requireNewCommandTargets([]string{*destination, *archivePath}); err != nil {
		fmt.Fprintln(os.Stderr, "capsule build-private-relation:", err)
		return 1
	}
	loaded, root, session, events, err := loadGuidedInspectionState(*packageRoot, *privateRoot, *sessionDigest)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule build-private-relation load:", err)
		return 1
	}
	completionRaw, err := root.ReadSensitive(pilotInspectionCompletionPath(session.Digest), relation.MaximumDocumentSize)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule build-private-relation completion:", err)
		return 1
	}
	completion, err := relation.DecodePilotInspectionCompletion(bytes.NewReader(completionRaw))
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule build-private-relation completion:", err)
		return 1
	}
	inspectionRaw, err := root.ReadSensitive(filepath.Join("pilot-inspections", completion.InspectionRecordDigest+".json"), relation.MaximumDocumentSize)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule build-private-relation inspection:", err)
		return 1
	}
	record, err := relation.DecodePilotInspectionRecord(bytes.NewReader(inspectionRaw))
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule build-private-relation inspection:", err)
		return 1
	}
	sessionRaw, err := root.ReadSensitive(pilotInspectionSessionPath(session.Digest), relation.MaximumDocumentSize)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule build-private-relation session:", err)
		return 1
	}
	eventPayloads := make([][]byte, len(events))
	for index := range events {
		eventPayloads[index], err = root.ReadSensitive(pilotInspectionEventPath(session.Digest, index+1), relation.MaximumDocumentSize)
		if err != nil {
			fmt.Fprintln(os.Stderr, "capsule build-private-relation event:", err)
			return 1
		}
	}
	inventoryRaw, err := readBoundedCommandFile(filepath.Join(loaded.root, "package-inventory.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule build-private-relation inventory:", err)
		return 1
	}
	packageFiles, err := readPrivateRelationPackageFiles(loaded)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule build-private-relation package:", err)
		return 1
	}
	chain := relation.OwnerInspectionPrivateChain{
		Completion: completion, Record: record, Session: session, Events: events,
		Readiness: loaded.readiness, Bundle: loaded.bundle, Mappings: loaded.mappings,
		Plan: loaded.plan, Primary: loaded.primary, Sentinel: loaded.sentinel, Pilot: loaded.pilot,
		ScarcityMaterials: loaded.scarcityMaterials, PackageBinding: loaded.binding,
	}
	publicPack, err := capsule.BuildReferencePackage(*repositoryRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule build-private-relation public reference:", err)
		return 1
	}
	privatePack, err := capsule.BuildPrivateRelationPackage(publicPack, capsule.PrivateRelationSources{
		InventoryPayload: inventoryRaw, PackageFiles: packageFiles, SessionPayload: sessionRaw,
		EventPayloads: eventPayloads, InspectionPayload: inspectionRaw, CompletionPayload: completionRaw, Chain: chain,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule build-private-relation:", err)
		return 1
	}
	ctx := context.Background()
	archive, err := publishCapsuleArtifacts(
		ctx, *destination, *archivePath, privatePack.Registry, privatePack.Manifest, privatePack.Payloads,
		capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPrivate}, nil,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule build-private-relation publish:", err)
		return 1
	}
	report := privateRelationCapsuleBuildReport{
		SchemaVersion: "evalwitness.private-relation-capsule-build-report.v1",
		CapsuleID:     privatePack.Manifest.CapsuleID, ManifestDigest: privatePack.Manifest.ManifestDigest,
		RegistryDigest: privatePack.Registry.Digest(), CapsuleDirectory: *destination,
		ArchivePath: *archivePath, Archive: archive, Components: len(privatePack.Manifest.Components),
		PackageFiles: privatePack.Proof.PackagePayloadFiles, EventCount: privatePack.Proof.EventCount,
		Corrections: privatePack.Proof.Corrections, ProviderCalls: 0, Offline: true,
	}
	return writeCanonicalCommandOutput("capsule build-private-relation", report)
}

func runPrivateRelationCapsuleVerify(args []string) int {
	flags := flag.NewFlagSet("capsule verify-private-relation", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	publicSource := flags.String("public-source", "", "public parent capsule directory")
	privateSource := flags.String("source", "", "private relation capsule directory")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "capsule verify-private-relation:", err)
		return 2
	}
	if *publicSource == "" || *privateSource == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "capsule verify-private-relation: --public-source and --source are required")
		return 2
	}
	ctx := context.Background()
	publicRegistry, err := capsule.ReferenceRegistry()
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule verify-private-relation public registry:", err)
		return 1
	}
	privateRegistry, err := capsule.PrivateRelationRegistry(publicRegistry)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule verify-private-relation private registry:", err)
		return 1
	}
	publicOptions := capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic}
	privateOptions := capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPrivate}
	publicReport, err := capsule.VerifyDirectory(ctx, *publicSource, publicRegistry, publicOptions)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule verify-private-relation public:", err)
		return 1
	}
	publicManifest, publicPayloads, err := capsule.LoadDirectory(ctx, *publicSource, publicRegistry, publicOptions)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule verify-private-relation public:", err)
		return 1
	}
	privateReport, err := capsule.VerifyDirectory(ctx, *privateSource, privateRegistry, privateOptions)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule verify-private-relation private:", err)
		return 1
	}
	privateManifest, privatePayloads, err := capsule.LoadDirectory(ctx, *privateSource, privateRegistry, privateOptions)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule verify-private-relation private:", err)
		return 1
	}
	if _, err := capsule.VerifyPackageFamily(
		ctx,
		capsule.Package{Registry: privateRegistry, Manifest: privateManifest, Payloads: privatePayloads},
		[]capsule.Package{{Registry: publicRegistry, Manifest: publicManifest, Payloads: publicPayloads}},
		privateOptions,
	); err != nil {
		fmt.Fprintln(os.Stderr, "capsule verify-private-relation family:", err)
		return 1
	}
	return writeCanonicalCommandOutput("capsule verify-private-relation", privateRelationCapsuleVerifyReport{
		SchemaVersion: "evalwitness.private-relation-capsule-verify-report.v1",
		Public:        publicReport, Private: privateReport, FamilyBound: true, Offline: true,
	})
}

func runCapsuleBuild(args []string) int {
	flags := flag.NewFlagSet("capsule build", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	repositoryRoot := flags.String("repository-root", ".", "repository root")
	destination := flags.String("destination", "", "new capsule directory")
	archivePath := flags.String("archive", "", "new deterministic tar.gz path")
	ledgerPath := flags.String("ledger", "", "new canonical claim-ledger path")
	statementPath := flags.String("statement", "", "new canonical in-toto statement path")
	projectionPath := flags.String("projection", "", "new deterministic claim projection path")
	autopsyPath := flags.String("autopsy", "", "new deterministic claim autopsy path")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "capsule build:", err)
		return 2
	}
	if *destination == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "capsule build: --destination is required and positional arguments are forbidden")
		return 2
	}
	outputs := resolveCapsuleBuildOutputs(*destination, *archivePath, *ledgerPath, *statementPath, *projectionPath, *autopsyPath)
	if err := requireNewCommandTargets(outputs); err != nil {
		fmt.Fprintln(os.Stderr, "capsule build:", err)
		return 1
	}
	pack, err := capsule.BuildReferencePackage(*repositoryRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule build reference:", err)
		return 1
	}
	ledger, err := claimledger.DefaultLedger(pack.Manifest)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule build ledger:", err)
		return 1
	}
	projection, err := claimledger.BuildProjection(context.Background(), pack.Registry, pack.Manifest, pack.Payloads, ledger)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule build projection:", err)
		return 1
	}
	autopsy, err := claimledger.BuildAutopsy(context.Background(), pack.Registry, pack.Manifest, pack.Payloads, ledger)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule build autopsy:", err)
		return 1
	}
	statement, err := capsule.BuildStatement(pack.Manifest, pack.Registry)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule build statement:", err)
		return 1
	}
	ledgerRaw, err := claimledger.EncodeLedger(ledger)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule encode ledger:", err)
		return 1
	}
	projectionRaw, err := claimledger.EncodeProjection(projection)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule encode projection:", err)
		return 1
	}
	autopsyRaw, err := claimledger.EncodeAutopsy(autopsy)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule encode autopsy:", err)
		return 1
	}
	statementRaw, err := capsule.EncodeStatement(statement)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule encode statement:", err)
		return 1
	}
	ctx := context.Background()
	archive, err := publishCapsuleArtifacts(
		ctx, outputs[0], outputs[1], pack.Registry, pack.Manifest, pack.Payloads,
		capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic},
		[]capsuleSidecar{
			{Destination: outputs[2], Payload: ledgerRaw},
			{Destination: outputs[3], Payload: statementRaw},
			{Destination: outputs[4], Payload: projectionRaw},
			{Destination: outputs[5], Payload: autopsyRaw},
		},
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule publish:", err)
		return 1
	}
	report := capsuleBuildReport{
		SchemaVersion: "evalwitness.capsule-build-report.v1", CapsuleID: pack.Manifest.CapsuleID,
		ManifestDigest: pack.Manifest.ManifestDigest, RegistryDigest: pack.Registry.Digest(),
		CapsuleDirectory: outputs[0], ArchivePath: outputs[1], Archive: archive,
		LedgerPath: outputs[2], LedgerDigest: ledger.Digest, StatementPath: outputs[3],
		ProjectionPath: outputs[4], ProjectionDigest: projection.Digest,
		AutopsyPath: outputs[5], AutopsyDigest: autopsy.Digest,
		ProviderCalls: 0, Offline: true,
	}
	return writeCanonicalCommandOutput("capsule build", report)
}

func runCapsuleVerify(args []string) int {
	flags := flag.NewFlagSet("capsule verify", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	source := flags.String("source", "", "capsule directory")
	ledgerPath := flags.String("ledger", "", "canonical claim ledger")
	statementPath := flags.String("statement", "", "canonical in-toto statement")
	projectionPath := flags.String("projection", "", "canonical claim projection")
	autopsyPath := flags.String("autopsy", "", "canonical claim autopsy")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "capsule verify:", err)
		return 2
	}
	if *source == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "capsule verify: --source is required and positional arguments are forbidden")
		return 2
	}
	registry, err := capsule.ReferenceRegistry()
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule registry:", err)
		return 1
	}
	ctx := context.Background()
	options := capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic}
	capsuleReport, err := capsule.VerifyDirectory(ctx, *source, registry, options)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule verify:", err)
		return 1
	}
	manifest, payloads, err := capsule.LoadDirectory(ctx, *source, registry, options)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capsule load:", err)
		return 1
	}
	report := capsuleVerifyReport{SchemaVersion: "evalwitness.capsule-verify-report.v1", Capsule: capsuleReport, Offline: true}
	var ledger claimledger.Ledger
	if *ledgerPath != "" {
		ledger, err = readClaimLedger(*ledgerPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "capsule verify ledger:", err)
			return 1
		}
		claimReport, verifyErr := claimledger.VerifyLedger(ctx, registry, manifest, payloads, ledger)
		if verifyErr != nil {
			fmt.Fprintln(os.Stderr, "capsule verify claims:", verifyErr)
			return 1
		}
		report.Claims = &claimReport
	}
	if *statementPath != "" {
		raw, readErr := readBoundedCommandFile(*statementPath)
		if readErr != nil {
			fmt.Fprintln(os.Stderr, "capsule verify statement:", readErr)
			return 1
		}
		if _, decodeErr := capsule.DecodeStatement(raw, manifest); decodeErr != nil {
			fmt.Fprintln(os.Stderr, "capsule verify statement:", decodeErr)
			return 1
		}
		report.StatementVerified = true
	}
	if *projectionPath != "" {
		if *ledgerPath == "" {
			fmt.Fprintln(os.Stderr, "capsule verify projection: --projection requires --ledger")
			return 2
		}
		raw, readErr := readBoundedCommandFile(*projectionPath)
		if readErr != nil {
			fmt.Fprintln(os.Stderr, "capsule verify projection:", readErr)
			return 1
		}
		actual, decodeErr := claimledger.DecodeProjection(raw)
		if decodeErr != nil {
			fmt.Fprintln(os.Stderr, "capsule verify projection:", decodeErr)
			return 1
		}
		expected, buildErr := claimledger.BuildProjection(ctx, registry, manifest, payloads, ledger)
		if buildErr != nil {
			fmt.Fprintln(os.Stderr, "capsule verify projection:", buildErr)
			return 1
		}
		expectedRaw, encodeErr := claimledger.EncodeProjection(expected)
		if encodeErr != nil || !bytes.Equal(raw, expectedRaw) || actual.Digest != expected.Digest {
			fmt.Fprintln(os.Stderr, "capsule verify projection: projection differs from deterministic recomputation")
			return 1
		}
		report.ProjectionVerified = true
	}
	if *autopsyPath != "" {
		if *ledgerPath == "" {
			fmt.Fprintln(os.Stderr, "capsule verify autopsy: --autopsy requires --ledger")
			return 2
		}
		raw, readErr := readBoundedCommandFile(*autopsyPath)
		if readErr != nil {
			fmt.Fprintln(os.Stderr, "capsule verify autopsy:", readErr)
			return 1
		}
		actual, decodeErr := claimledger.DecodeAutopsy(raw)
		if decodeErr != nil {
			fmt.Fprintln(os.Stderr, "capsule verify autopsy:", decodeErr)
			return 1
		}
		if verifyErr := claimledger.VerifyAutopsy(ctx, registry, manifest, payloads, ledger, actual); verifyErr != nil {
			fmt.Fprintln(os.Stderr, "capsule verify autopsy:", verifyErr)
			return 1
		}
		report.AutopsyVerified = true
	}
	return writeCanonicalCommandOutput("capsule verify", report)
}

func resolveCapsuleBuildOutputs(destination, archivePath, ledgerPath, statementPath, projectionPath, autopsyPath string) []string {
	if archivePath == "" {
		archivePath = destination + ".tar.gz"
	}
	if ledgerPath == "" {
		ledgerPath = destination + ".claims.json"
	}
	if statementPath == "" {
		statementPath = destination + ".intoto.json"
	}
	if projectionPath == "" {
		projectionPath = destination + ".projection.json"
	}
	if autopsyPath == "" {
		autopsyPath = destination + ".autopsy.json"
	}
	return []string{destination, archivePath, ledgerPath, statementPath, projectionPath, autopsyPath}
}

func requireNewCommandTargets(paths []string) error {
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if _, duplicate := seen[absolute]; duplicate {
			return fmt.Errorf("output target %q is duplicated", path)
		}
		seen[absolute] = struct{}{}
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("output target %q already exists or cannot be inspected", path)
		}
	}
	return nil
}

func readPrivateRelationPackageFiles(loaded guidedPilotPackage) (map[string][]byte, error) {
	files := make(map[string][]byte, len(loaded.inventory.Files))
	for _, entry := range loaded.inventory.Files {
		absolute := filepath.Join(loaded.root, filepath.FromSlash(entry.Path))
		info, err := os.Lstat(absolute)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
			info.Mode().Perm() != safety.SensitiveFileMode || info.Size() != entry.Bytes {
			return nil, fmt.Errorf("private package file %q changed after inventory verification", entry.Path)
		}
		raw, err := os.ReadFile(absolute)
		if err != nil || int64(len(raw)) != entry.Bytes || protocol.DigestBytes(raw) != entry.SHA256 {
			return nil, fmt.Errorf("private package file %q changed during exact-byte capture", entry.Path)
		}
		files[entry.Path] = raw
	}
	return files, nil
}

func writeNewPublicCommandFile(path string, raw []byte) error {
	if len(raw) == 0 {
		return errors.New("refusing to write an empty evidence file")
	}
	policy, err := safety.CurrentPathPolicy()
	if err != nil {
		return err
	}
	validated, err := policy.ValidateMutationRoot(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(validated), safety.PublicDirectoryMode); err != nil {
		return err
	}
	file, err := os.OpenFile(validated, os.O_WRONLY|os.O_CREATE|os.O_EXCL, safety.PublicFileMode)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		_ = file.Close()
		if !committed {
			_ = os.Remove(validated)
		}
	}()
	if _, err := file.Write(raw); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	committed = true
	return nil
}

func readBoundedCommandFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 1 || info.Size() > maximumEvidenceCommandFileBytes {
		return nil, fmt.Errorf("evidence path %q is not a bounded regular file", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil || int64(len(raw)) != info.Size() {
		return nil, errors.Join(err, errors.New("evidence file read was incomplete"))
	}
	return raw, nil
}

func readClaimLedger(path string) (claimledger.Ledger, error) {
	raw, err := readBoundedCommandFile(path)
	if err != nil {
		return claimledger.Ledger{}, err
	}
	return claimledger.DecodeLedger(bytes.TrimSpace(raw))
}

func writeCanonicalCommandOutput(scope string, value any) int {
	raw, err := protocol.CanonicalMarshal(value)
	if err != nil {
		fmt.Fprintln(os.Stderr, scope+":", err)
		return 1
	}
	return writeCommandOutput(scope, append(raw, '\n'))
}
