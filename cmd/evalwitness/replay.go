package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cachekit "github.com/Christopher-Schulze/evalwitness/internal/cache"
	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	claimledger "github.com/Christopher-Schulze/evalwitness/internal/claim"
	"github.com/Christopher-Schulze/evalwitness/internal/replay"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

func runReplay(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "replay: missing subcommand (bundle, capture-run, study, census-legacy-cache, or migrate)")
		return 2
	}
	switch args[0] {
	case "bundle":
		return runReplayBundle(args[1:])
	case "capture-run":
		return runReplayCaptureRun(args[1:])
	case "study":
		return runReplayStudy(args[1:])
	case "census-legacy-cache":
		return runReplayLegacyCacheCensus(args[1:])
	case "migrate":
		return runReplayMigrate(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "replay: unknown subcommand %q\n", args[0])
		return 2
	}
}

func runReplayLegacyCacheCensus(args []string) int {
	flags := flag.NewFlagSet("replay census-legacy-cache", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	source := flags.String("source", "", "read-only legacy cache root")
	publishedProvider := flags.String("published-provider", "", "published legacy provider namespace")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *source == "" || *publishedProvider == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "replay census-legacy-cache: --source and --published-provider are required; positional arguments are forbidden")
		return 2
	}
	census, err := cachekit.CensusLegacyCache(*source, *publishedProvider)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay census-legacy-cache:", err)
		return 1
	}
	return writeCanonicalCommandOutput("replay census-legacy-cache", census)
}

func runReplayMigrate(args []string) int {
	flags := flag.NewFlagSet("replay migrate", flag.ContinueOnError)
	source := flags.String("source", "", "legacy JSONL fixture")
	candidate := flags.String("candidate", "", "non-destructive migration candidate path")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *source == "" {
		fmt.Fprintln(os.Stderr, "replay migrate: --source is required")
		return 2
	}
	if *candidate == "" {
		*candidate = *source + ".candidate"
	}
	report, err := replay.MigrateLegacy(*source, *candidate)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay migrate:", err)
		return 1
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay migrate report:", err)
		return 1
	}
	fmt.Println(string(encoded))
	return 0
}

type responseBundleBuildReport struct {
	SchemaVersion   string                                  `json:"schema_version"`
	CapsuleID       string                                  `json:"capsule_id"`
	ManifestDigest  string                                  `json:"manifest_digest"`
	RegistryDigest  string                                  `json:"registry_digest"`
	IndexDigest     string                                  `json:"index_digest"`
	PolicyDigest    string                                  `json:"policy_digest"`
	Destination     string                                  `json:"destination"`
	ArchivePath     string                                  `json:"archive_path"`
	Archive         capsule.ArchiveReport                   `json:"archive"`
	Verification    replay.ResponseBundleVerificationReport `json:"verification"`
	ProviderCalls   int                                     `json:"provider_calls"`
	NetworkRequired bool                                    `json:"network_required"`
}

func runReplayBundle(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "replay bundle: missing subcommand (seal-policy, build, or verify)")
		return 2
	}
	switch args[0] {
	case "seal-policy":
		return runReplayBundleSealPolicy(args[1:])
	case "build":
		return runReplayBundleBuild(args[1:])
	case "verify":
		return runReplayBundleVerify(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "replay bundle: unknown subcommand %q\n", args[0])
		return 2
	}
}

func runReplayBundleSealPolicy(args []string) int {
	flags := flag.NewFlagSet("replay bundle seal-policy", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	source := flags.String("source", "", "unsealed response-bundle policy draft")
	repositoryRoot := flags.String("repository-root", ".", "Git repository root for producer provenance")
	producerBinary := flags.String("producer-binary", "", "exact bundle-producing evalwitness binary")
	redistributionEvidence := flags.String("redistribution-evidence", "", "public file proving response redistribution authorization")
	var captureFlags stringSlice
	flags.Var(&captureFlags, "capture", "capture name=exact.jsonl (repeatable)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *source == "" || *repositoryRoot == "" || *producerBinary == "" || *redistributionEvidence == "" || len(captureFlags) == 0 || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "replay bundle seal-policy: --source, --repository-root, --producer-binary, --redistribution-evidence, and at least one --capture name=path are required; positional arguments are forbidden")
		return 2
	}
	raw, err := readBoundedCommandFile(strings.TrimPrefix(*source, "@"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay bundle seal-policy:", err)
		return 1
	}
	evidence, err := replay.CollectResponseBundleProducerEvidence(context.Background(), *repositoryRoot, *producerBinary)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay bundle seal-policy producer:", err)
		return 1
	}
	sources, err := parseResponseCaptureFlags(captureFlags)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay bundle seal-policy captures:", err)
		return 2
	}
	policy, err := replay.SealResponseBundlePolicyDraft(raw, evidence, *redistributionEvidence, sources)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay bundle seal-policy:", err)
		return 1
	}
	return writeCanonicalCommandOutput("replay bundle seal-policy", policy)
}

func runReplayBundleBuild(args []string) int {
	flags := flag.NewFlagSet("replay bundle build", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	policyPath := flags.String("policy", "", "canonical response-bundle policy path")
	destination := flags.String("destination", "", "new response-bundle capsule directory")
	archivePath := flags.String("archive", "", "new deterministic response-bundle archive")
	repositoryRoot := flags.String("repository-root", ".", "Git repository root for producer provenance")
	producerBinary := flags.String("producer-binary", "", "exact bundle-producing evalwitness binary")
	redistributionEvidence := flags.String("redistribution-evidence", "", "public file proving response redistribution authorization")
	reviewedFindingsPath := flags.String("reviewed-findings", "", "manifest of reviewed false-positive findings to suppress (@file)")
	var captureFlags stringSlice
	flags.Var(&captureFlags, "capture", "capture name=exact.jsonl (repeatable)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *policyPath == "" || *destination == "" || *repositoryRoot == "" || *producerBinary == "" || *redistributionEvidence == "" || len(captureFlags) == 0 || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "replay bundle build: --policy, --destination, --repository-root, --producer-binary, --redistribution-evidence, and at least one --capture name=path are required; positional arguments are forbidden")
		return 2
	}
	if *archivePath == "" {
		*archivePath = *destination + ".tar.gz"
	}
	policyRaw, err := readBoundedCommandFile(strings.TrimPrefix(*policyPath, "@"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay bundle build policy:", err)
		return 1
	}
	policy, err := replay.DecodeResponseBundlePolicy(policyRaw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay bundle build policy:", err)
		return 1
	}
	sources, err := parseResponseCaptureFlags(captureFlags)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay bundle build captures:", err)
		return 2
	}
	evidence, err := replay.CollectResponseBundleProducerEvidence(context.Background(), *repositoryRoot, *producerBinary)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay bundle build producer:", err)
		return 1
	}
	knownSecrets := safety.SecretsFromEnvironment(os.Environ())
	reviewedFindings, err := loadReviewedResponseFindings(*reviewedFindingsPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay bundle build reviewed-findings:", err)
		return 1
	}
	build, err := replay.BuildResponseBundle(policy, evidence, *redistributionEvidence, sources, knownSecrets, reviewedFindings)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay bundle build:", err)
		return 1
	}
	ctx := context.Background()
	archive, err := publishCapsuleArtifacts(
		ctx, *destination, *archivePath, build.Package.Registry, build.Package.Manifest, build.Package.Payloads,
		capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic}, nil,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay bundle publish:", err)
		return 1
	}
	verification, err := replay.VerifyResponseBundleDirectory(ctx, *destination, knownSecrets, reviewedFindings)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay bundle verify published output:", err)
		return 1
	}
	return writeCanonicalCommandOutput("replay bundle build", responseBundleBuildReport{
		SchemaVersion: "evalwitness.response-bundle-build-report.v1",
		CapsuleID:     build.Package.Manifest.CapsuleID, ManifestDigest: build.Package.Manifest.ManifestDigest,
		RegistryDigest: build.Package.Registry.Digest(), IndexDigest: build.Index.Digest, PolicyDigest: policy.Digest,
		Destination: *destination, ArchivePath: *archivePath, Archive: archive, Verification: verification,
		ProviderCalls: 0, NetworkRequired: false,
	})
}

func runReplayBundleVerify(args []string) int {
	flags := flag.NewFlagSet("replay bundle verify", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	source := flags.String("source", "", "response-bundle capsule directory")
	reviewedFindingsPath := flags.String("reviewed-findings", "", "manifest of reviewed false-positive findings to suppress (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *source == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "replay bundle verify: --source is required and positional arguments are forbidden")
		return 2
	}
	reviewedFindings, err := loadReviewedResponseFindings(*reviewedFindingsPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay bundle verify reviewed-findings:", err)
		return 1
	}
	report, err := replay.VerifyResponseBundleDirectory(
		context.Background(), *source, safety.SecretsFromEnvironment(os.Environ()), reviewedFindings,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay bundle verify:", err)
		return 1
	}
	return writeCanonicalCommandOutput("replay bundle verify", report)
}

func loadReviewedResponseFindings(path string) ([]safety.ArtifactFinding, error) {
	if path == "" {
		return nil, nil
	}
	raw, err := readBoundedCommandFile(strings.TrimPrefix(path, "@"))
	if err != nil {
		return nil, err
	}
	var findings []safety.ArtifactFinding
	if err := json.Unmarshal(raw, &findings); err != nil {
		return nil, err
	}
	return findings, nil
}

func runReplayCaptureRun(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "replay capture-run: missing subcommand (attest, verify, stamp, or admit)")
		return 2
	}
	switch args[0] {
	case "attest":
		return runReplayCaptureRunAttest(args[1:])
	case "verify":
		return runReplayCaptureRunVerify(args[1:])
	case "stamp":
		return runReplayCaptureRunStamp(args[1:])
	case "admit":
		return runReplayCaptureRunAdmit(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "replay capture-run: unknown subcommand %q\n", args[0])
		return 2
	}
}

func runReplayCaptureRunAttest(args []string) int {
	flags := flag.NewFlagSet("replay capture-run attest", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	capture := flags.String("capture", "", "schema-3 JSONL capture")
	authorizedCalls := flags.Int("authorized-calls", 0, "authorized logical call budget")
	output := flags.String("output", "", "optional attestation JSON path")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *capture == "" || *authorizedCalls < 1 || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "replay capture-run attest: --capture and --authorized-calls are required; positional arguments are forbidden")
		return 2
	}
	attestation, err := replay.SealCaptureRunAttestation(*capture, *authorizedCalls)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay capture-run attest:", err)
		return 1
	}
	if *output != "" {
		raw, marshalErr := protocol.CanonicalMarshal(attestation)
		if marshalErr != nil {
			fmt.Fprintln(os.Stderr, "replay capture-run attest:", marshalErr)
			return 1
		}
		if writeErr := os.WriteFile(*output, append(raw, '\n'), 0o644); writeErr != nil {
			fmt.Fprintln(os.Stderr, "replay capture-run attest:", writeErr)
			return 1
		}
	}
	return writeCanonicalCommandOutput("replay capture-run attest", attestation)
}

func runReplayCaptureRunVerify(args []string) int {
	flags := flag.NewFlagSet("replay capture-run verify", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	capture := flags.String("capture", "", "schema-3 JSONL capture")
	attestationPath := flags.String("attestation", "", "capture-run attestation JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *capture == "" || *attestationPath == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "replay capture-run verify: --capture and --attestation are required; positional arguments are forbidden")
		return 2
	}
	raw, err := os.ReadFile(*attestationPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay capture-run verify:", err)
		return 1
	}
	var attestation replay.CaptureRunAttestation
	if err := protocol.DecodeStrict(bytes.TrimSpace(raw), &attestation); err != nil {
		fmt.Fprintln(os.Stderr, "replay capture-run verify:", err)
		return 1
	}
	if err := replay.VerifyCaptureRunAttestation(*capture, attestation); err != nil {
		fmt.Fprintln(os.Stderr, "replay capture-run verify:", err)
		return 1
	}
	return writeCanonicalCommandOutput("replay capture-run verify", attestation)
}

func runReplayCaptureRunStamp(args []string) int {
	flags := flag.NewFlagSet("replay capture-run stamp", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	capture := flags.String("capture", "", "schema-3 JSONL capture")
	destination := flags.String("destination", "", "new stamped capture path")
	stampPath := flags.String("stamp", "", "research lineage stamp JSON")
	output := flags.String("output", "", "optional stamp report JSON path")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *capture == "" || *destination == "" || *stampPath == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "replay capture-run stamp: --capture, --destination, and --stamp are required; positional arguments are forbidden")
		return 2
	}
	raw, err := readBoundedCommandFile(*stampPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay capture-run stamp:", err)
		return 1
	}
	var stamp replay.CaptureResearchLineageStamp
	if err := protocol.DecodeStrict(bytes.TrimSpace(raw), &stamp); err != nil {
		fmt.Fprintln(os.Stderr, "replay capture-run stamp:", err)
		return 1
	}
	report, err := replay.StampCaptureResearchLineage(*capture, *destination, stamp)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay capture-run stamp:", err)
		return 1
	}
	if *output != "" {
		encoded, marshalErr := protocol.CanonicalMarshal(report)
		if marshalErr != nil {
			fmt.Fprintln(os.Stderr, "replay capture-run stamp:", marshalErr)
			return 1
		}
		if writeErr := os.WriteFile(*output, append(encoded, '\n'), 0o644); writeErr != nil {
			fmt.Fprintln(os.Stderr, "replay capture-run stamp:", writeErr)
			return 1
		}
	}
	return writeCanonicalCommandOutput("replay capture-run stamp", report)
}

func runReplayCaptureRunAdmit(args []string) int {
	flags := flag.NewFlagSet("replay capture-run admit", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	capture := flags.String("capture", "", "schema-3 JSONL capture")
	authorizedCalls := flags.Int("authorized-calls", 0, "authorized logical call budget")
	output := flags.String("output", "", "optional admission JSON path")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *capture == "" || *authorizedCalls < 1 || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "replay capture-run admit: --capture and --authorized-calls are required; positional arguments are forbidden")
		return 2
	}
	admission, err := replay.AdmitCaptureResearchLineage(*capture, *authorizedCalls)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay capture-run admit:", err)
		return 1
	}
	if *output != "" {
		encoded, marshalErr := protocol.CanonicalMarshal(admission)
		if marshalErr != nil {
			fmt.Fprintln(os.Stderr, "replay capture-run admit:", marshalErr)
			return 1
		}
		if writeErr := os.WriteFile(*output, append(encoded, '\n'), 0o644); writeErr != nil {
			fmt.Fprintln(os.Stderr, "replay capture-run admit:", writeErr)
			return 1
		}
	}
	if code := writeCanonicalCommandOutput("replay capture-run admit", admission); code != 0 {
		return code
	}
	if admission.Admission != replay.CaptureResearchAdmissionAdmitted {
		return 1
	}
	return 0
}

func runReplayStudy(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "replay study: missing subcommand (analyze-identical-response|bind|portfolio)")
		return 2
	}
	switch args[0] {
	case "analyze-identical-response":
		return runReplayStudyAnalyzeIdenticalResponse(args[1:])
	case "bind":
		return runReplayStudyBind(args[1:])
	case "portfolio":
		return runReplayStudyPortfolio(args[1:])
	case "capsule":
		return runReplayStudyCapsule(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "replay study: unknown subcommand %q\n", args[0])
		return 2
	}
}

type identicalResponseCapsuleBuildReport struct {
	SchemaVersion       string                         `json:"schema_version"`
	CapsuleID           string                         `json:"capsule_id"`
	ManifestDigest      string                         `json:"manifest_digest"`
	RegistryDigest      string                         `json:"registry_digest"`
	BaseCapsuleID       string                         `json:"base_capsule_id"`
	Destination         string                         `json:"destination"`
	ArchivePath         string                         `json:"archive_path"`
	Archive             capsule.ArchiveReport          `json:"archive"`
	LedgerPath          string                         `json:"ledger_path"`
	LedgerDigest        string                         `json:"ledger_digest"`
	ChallengePackPath   string                         `json:"challenge_pack_path"`
	ChallengePackDigest string                         `json:"challenge_pack_digest"`
	Family              capsule.VerificationReport     `json:"family"`
	Claims              claimledger.VerificationReport `json:"claims"`
	ProviderCalls       int                            `json:"provider_calls"`
	NetworkRequired     bool                           `json:"network_required"`
}

type identicalResponseCapsuleVerifyReport struct {
	SchemaVersion string                         `json:"schema_version"`
	Family        capsule.VerificationReport     `json:"family"`
	Claims        claimledger.VerificationReport `json:"claims"`
	ChallengePack claimledger.ChallengePack      `json:"challenge_pack"`
	Offline       bool                           `json:"offline"`
	ProviderCalls int                            `json:"provider_calls"`
}

func runReplayStudyCapsule(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "replay study capsule: missing subcommand (build or verify)")
		return 2
	}
	switch args[0] {
	case "build":
		return runReplayStudyCapsuleBuild(args[1:])
	case "verify":
		return runReplayStudyCapsuleVerify(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "replay study capsule: unknown subcommand %q\n", args[0])
		return 2
	}
}

func runReplayStudyCapsuleBuild(args []string) int {
	flags := flag.NewFlagSet("replay study capsule build", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	basePath := flags.String("base-capsule", "", "verified response-bundle capsule directory")
	studyManifestPath := flags.String("study-manifest", "", "locked study manifest JSON")
	studyRecordPath := flags.String("study-record", "", "complete study record JSON")
	authorizationPath := flags.String("live-authorization", "", "exact live authorization plan JSON")
	routeAttestationPath := flags.String("route-attestation", "", "bounded route attestation JSON")
	captureRunPath := flags.String("capture-run-attestation", "", "complete capture-run attestation JSON")
	admissionPath := flags.String("admission", "", "admitted research-lineage JSON")
	analysisPath := flags.String("offline-analysis", "", "offline identical-response analysis JSON")
	destination := flags.String("destination", "", "new outer capsule directory")
	archivePath := flags.String("archive", "", "new deterministic outer capsule archive")
	ledgerPath := flags.String("claim-ledger", "", "new sealed claim ledger JSON")
	challengePath := flags.String("challenge-pack", "", "new executable challenge pack JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *basePath == "" || *studyManifestPath == "" || *studyRecordPath == "" || *authorizationPath == "" || *routeAttestationPath == "" || *captureRunPath == "" || *admissionPath == "" || *analysisPath == "" || *destination == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "replay study capsule build: --base-capsule, --study-manifest, --study-record, --live-authorization, --route-attestation, --capture-run-attestation, --admission, --offline-analysis, and --destination are required; positional arguments are forbidden")
		return 2
	}
	if *archivePath == "" {
		*archivePath = *destination + ".tar.gz"
	}
	if *ledgerPath == "" {
		*ledgerPath = *destination + ".claims.json"
	}
	if *challengePath == "" {
		*challengePath = *destination + ".challenge-pack.json"
	}
	base, err := replay.LoadResponseBundlePackage(context.Background(), *basePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay study capsule build base:", err)
		return 1
	}
	read := func(path string) ([]byte, bool) {
		raw, readErr := readBoundedCommandFile(path)
		if readErr != nil {
			fmt.Fprintln(os.Stderr, "replay study capsule build:", readErr)
			return nil, false
		}
		return raw, true
	}
	manifestRaw, ok := read(*studyManifestPath)
	if !ok {
		return 1
	}
	recordRaw, ok := read(*studyRecordPath)
	if !ok {
		return 1
	}
	authorizationRaw, ok := read(*authorizationPath)
	if !ok {
		return 1
	}
	routeRaw, ok := read(*routeAttestationPath)
	if !ok {
		return 1
	}
	captureRunRaw, ok := read(*captureRunPath)
	if !ok {
		return 1
	}
	admissionRaw, ok := read(*admissionPath)
	if !ok {
		return 1
	}
	analysisRaw, ok := read(*analysisPath)
	if !ok {
		return 1
	}
	build, err := replay.BuildIdenticalResponseCapsule(context.Background(), base, replay.IdenticalResponseCapsuleArtifacts{
		StudyManifest: manifestRaw, StudyRecord: recordRaw, LiveAuthorization: authorizationRaw, RouteAttestation: routeRaw,
		CaptureRunAttestation: captureRunRaw, ResearchAdmission: admissionRaw, OfflineAnalysis: analysisRaw,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay study capsule build:", err)
		return 1
	}
	ledger, err := claimledger.BuildIdenticalResponseLedger(context.Background(), build.Package.Registry, build.Package.Manifest, build.Package.Payloads)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay study capsule build claim ledger:", err)
		return 1
	}
	challengePack, err := claimledger.BuildChallengePack(context.Background(), build.Package.Registry, build.Package.Manifest, build.Package.Payloads, ledger)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay study capsule build challenge pack:", err)
		return 1
	}
	ledgerBytes, err := protocol.CanonicalMarshal(ledger)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay study capsule build claim ledger:", err)
		return 1
	}
	challengeBytes, err := protocol.CanonicalMarshal(challengePack)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay study capsule build challenge pack:", err)
		return 1
	}
	archive, err := publishCapsuleArtifacts(context.Background(), *destination, *archivePath, build.Package.Registry, build.Package.Manifest, build.Package.Payloads, capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic}, []capsuleSidecar{
		{Destination: *ledgerPath, Payload: append(ledgerBytes, '\n')},
		{Destination: *challengePath, Payload: append(challengeBytes, '\n')},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay study capsule build publish:", err)
		return 1
	}
	published, err := replay.LoadIdenticalResponseCapsulePackage(context.Background(), *destination, base.Registry)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay study capsule build reload:", err)
		return 1
	}
	family, err := replay.VerifyIdenticalResponseCapsuleFamily(context.Background(), published, base)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay study capsule build family:", err)
		return 1
	}
	claims, err := claimledger.VerifyLedger(context.Background(), published.Registry, published.Manifest, published.Payloads, ledger)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay study capsule build claims:", err)
		return 1
	}
	return writeCanonicalCommandOutput("replay study capsule build", identicalResponseCapsuleBuildReport{
		SchemaVersion: "evalwitness.identical-response-050-capsule-build.v1", CapsuleID: published.Manifest.CapsuleID,
		ManifestDigest: published.Manifest.ManifestDigest, RegistryDigest: published.Registry.Digest(), BaseCapsuleID: base.Manifest.CapsuleID,
		Destination: *destination, ArchivePath: *archivePath, Archive: archive, LedgerPath: *ledgerPath, LedgerDigest: ledger.Digest,
		ChallengePackPath: *challengePath, ChallengePackDigest: challengePack.Digest, Family: family, Claims: claims,
		ProviderCalls: 0, NetworkRequired: false,
	})
}

func runReplayStudyCapsuleVerify(args []string) int {
	flags := flag.NewFlagSet("replay study capsule verify", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	basePath := flags.String("base-capsule", "", "verified response-bundle capsule directory")
	source := flags.String("source", "", "outer capsule directory")
	ledgerPath := flags.String("claim-ledger", "", "sealed claim ledger JSON")
	challengePath := flags.String("challenge-pack", "", "executable challenge pack JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *basePath == "" || *source == "" || *ledgerPath == "" || *challengePath == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "replay study capsule verify: --base-capsule, --source, --claim-ledger, and --challenge-pack are required; positional arguments are forbidden")
		return 2
	}
	base, err := replay.LoadResponseBundlePackage(context.Background(), *basePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay study capsule verify base:", err)
		return 1
	}
	child, err := replay.LoadIdenticalResponseCapsulePackage(context.Background(), *source, base.Registry)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay study capsule verify source:", err)
		return 1
	}
	family, err := replay.VerifyIdenticalResponseCapsuleFamily(context.Background(), child, base)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay study capsule verify family:", err)
		return 1
	}
	ledger, err := readClaimLedger(*ledgerPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay study capsule verify ledger:", err)
		return 1
	}
	claims, err := claimledger.VerifyLedger(context.Background(), child.Registry, child.Manifest, child.Payloads, ledger)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay study capsule verify claims:", err)
		return 1
	}
	challengeRaw, err := readBoundedCommandFile(*challengePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay study capsule verify challenge pack:", err)
		return 1
	}
	var challengePack claimledger.ChallengePack
	if err := protocol.DecodeStrict(bytes.TrimSpace(challengeRaw), &challengePack); err != nil {
		fmt.Fprintln(os.Stderr, "replay study capsule verify challenge pack:", err)
		return 1
	}
	if err := challengePack.Validate(); err != nil || challengePack.SourceCapsuleID != child.Manifest.CapsuleID || challengePack.LedgerDigest != ledger.Digest {
		fmt.Fprintln(os.Stderr, "replay study capsule verify challenge pack: identity mismatch")
		return 1
	}
	return writeCanonicalCommandOutput("replay study capsule verify", identicalResponseCapsuleVerifyReport{
		SchemaVersion: "evalwitness.identical-response-050-capsule-verification.v1", Family: family, Claims: claims,
		ChallengePack: challengePack, Offline: true, ProviderCalls: 0,
	})
}

func runReplayStudyAnalyzeIdenticalResponse(args []string) int {
	flags := flag.NewFlagSet("replay study analyze-identical-response", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	capture := flags.String("capture", "", "schema-3 joint_absolute JSONL capture")
	studyRecord := flags.String("study-record", "", "authorized study record JSON")
	root := flags.String("root", "eval/trajectories/terminal_trajs", "terminal trajectory root")
	trajs := flags.String("trajs", "forge_gpt54", "trajectory set under the terminal trajectory root")
	epsilon := flags.Float64("epsilon", 0.02, "minimum best-versus-runner-up selection margin")
	output := flags.String("output", "", "optional new canonical analysis JSON path")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *capture == "" || *studyRecord == "" || *epsilon <= 0 || *epsilon >= 1 || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "replay study analyze-identical-response: --capture and --study-record are required, epsilon must be in (0,1), and positional arguments are forbidden")
		return 2
	}
	record, closeRecord, err := readStudyRecord(*studyRecord)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay study analyze-identical-response:", err)
		return 1
	}
	defer closeRecord()
	tasks, err := loadTerminalEvalTasks(filepath.Join(*root, *trajs), 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay study analyze-identical-response:", err)
		return 1
	}
	lockedTasks := make(map[string]struct{})
	for _, assignment := range record.Study.Manifest.Data.Split.Assignments {
		for _, taskID := range assignment.TaskIDs {
			lockedTasks[taskID] = struct{}{}
		}
	}
	outcomes := make(map[string][]int)
	for _, task := range tasks {
		if _, locked := lockedTasks[task.Name]; !locked {
			continue
		}
		rewards := make([]int, len(task.Trials))
		for index, trial := range task.Trials {
			rewards[index] = trial.Reward
		}
		outcomes[task.Name] = rewards
	}
	analysis, err := replay.AnalyzeIdenticalResponse(*capture, record, outcomes, *epsilon)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay study analyze-identical-response:", err)
		return 1
	}
	encoded, err := json.Marshal(analysis)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay study analyze-identical-response:", err)
		return 1
	}
	if *output != "" {
		file, openErr := os.OpenFile(*output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if openErr != nil {
			fmt.Fprintln(os.Stderr, "replay study analyze-identical-response:", openErr)
			return 1
		}
		_, writeErr := file.Write(append(encoded, '\n'))
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			fmt.Fprintln(os.Stderr, "replay study analyze-identical-response:", errors.Join(writeErr, closeErr))
			return 1
		}
	}
	fmt.Println(string(encoded))
	return 0
}

func runReplayStudyBind(args []string) int {
	flags := flag.NewFlagSet("replay study bind", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	capture := flags.String("capture", "", "schema-3 JSONL capture")
	authorizedCalls := flags.Int("authorized-calls", 0, "authorized logical call budget")
	attestation := flags.String("attestation", "", "capture-run attestation JSON")
	admission := flags.String("admission", "", "research-lineage admission JSON")
	ledger := flags.String("claim-ledger", "", "070 sidecar claim ledger JSON")
	policy := flags.String("bundle-policy", "", "optional sealed bundle policy JSON")
	study := flags.String("study-record", "", "optional study record JSON")
	analysis := flags.String("offline-analysis", "", "optional offline analysis JSON")
	output := flags.String("output", "", "optional bind certificate JSON path")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *capture == "" || *authorizedCalls < 1 || *attestation == "" || *admission == "" || *ledger == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "replay study bind: --capture, --authorized-calls, --attestation, --admission, and --claim-ledger are required; positional arguments are forbidden")
		return 2
	}
	certificate, err := replay.BindStudyEvidence(replay.StudyBindInput{
		CapturePath:         *capture,
		AuthorizedCalls:     *authorizedCalls,
		AttestationPath:     *attestation,
		AdmissionPath:       *admission,
		ClaimLedgerPath:     *ledger,
		BundlePolicyPath:    *policy,
		StudyRecordPath:     *study,
		OfflineAnalysisPath: *analysis,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay study bind:", err)
		return 1
	}
	if *output != "" {
		encoded, marshalErr := protocol.CanonicalMarshal(certificate)
		if marshalErr != nil {
			fmt.Fprintln(os.Stderr, "replay study bind:", marshalErr)
			return 1
		}
		if writeErr := os.WriteFile(*output, append(encoded, '\n'), 0o644); writeErr != nil {
			fmt.Fprintln(os.Stderr, "replay study bind:", writeErr)
			return 1
		}
	}
	if code := writeCanonicalCommandOutput("replay study bind", certificate); code != 0 {
		return code
	}
	if certificate.BindStatus != replay.StudyBindStatusComplete {
		return 1
	}
	return 0
}

func runReplayStudyPortfolio(args []string) int {
	flags := flag.NewFlagSet("replay study portfolio", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	bind := flags.String("bind", "", "050-bind certificate JSON")
	ledger := flags.String("claim-ledger", "", "070 sidecar claim ledger JSON")
	output := flags.String("output", "", "optional portfolio JSON path")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *bind == "" || *ledger == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "replay study portfolio: --bind and --claim-ledger are required; positional arguments are forbidden")
		return 2
	}
	portfolio, err := replay.BuildStudyPortfolio(*bind, *ledger)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay study portfolio:", err)
		return 1
	}
	if *output != "" {
		encoded, marshalErr := protocol.CanonicalMarshal(portfolio)
		if marshalErr != nil {
			fmt.Fprintln(os.Stderr, "replay study portfolio:", marshalErr)
			return 1
		}
		if writeErr := os.WriteFile(*output, append(encoded, '\n'), 0o644); writeErr != nil {
			fmt.Fprintln(os.Stderr, "replay study portfolio:", writeErr)
			return 1
		}
	}
	if code := writeCanonicalCommandOutput("replay study portfolio", portfolio); code != 0 {
		return code
	}
	if portfolio.BindStatus != replay.StudyBindStatusComplete || portfolio.ExplorerPresent {
		return 1
	}
	return 0
}

func parseResponseCaptureFlags(values []string) (map[string]string, error) {
	sources := make(map[string]string, len(values))
	for _, value := range values {
		name, path, found := strings.Cut(value, "=")
		if !found || name == "" || path == "" || name != strings.TrimSpace(name) || path != strings.TrimSpace(path) {
			return nil, fmt.Errorf("capture %q must be name=path", value)
		}
		if _, duplicate := sources[name]; duplicate {
			return nil, fmt.Errorf("capture name %q is repeated", name)
		}
		sources[name] = path
	}
	return sources, nil
}
