package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	capsuleartifact "github.com/Christopher-Schulze/evalwitness/internal/capsule"
	releaseartifact "github.com/Christopher-Schulze/evalwitness/internal/release"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

type releaseArtifactReport struct {
	SchemaVersion string `json:"schema_version"`
	Artifact      string `json:"artifact"`
	Destination   string `json:"destination"`
	SHA256        string `json:"sha256"`
	Bytes         int    `json:"bytes"`
}

type releaseSignatureReport struct {
	SchemaVersion   string `json:"schema_version"`
	KeyID           string `json:"key_id"`
	Destination     string `json:"destination"`
	EnvelopeSHA256  string `json:"envelope_sha256"`
	TrustRootSHA256 string `json:"trust_root_sha256"`
	PolicySHA256    string `json:"policy_sha256"`
}

func runRelease(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "release: missing subcommand (source-archive|source-index|manifest|sbom|statement|sign|verify)")
		return 2
	}
	switch args[0] {
	case "source-archive":
		return runReleaseSourceArchive(args[1:])
	case "source-index":
		return runReleaseSourceIndex(args[1:])
	case "manifest":
		return runReleaseManifest(args[1:])
	case "sbom":
		return runReleaseSBOM(args[1:])
	case "statement":
		return runReleaseStatement(args[1:])
	case "sign":
		return runReleaseSign(args[1:])
	case "verify":
		return runReleaseVerify(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "release: unknown subcommand %q\n", args[0])
		return 2
	}
}

func runReleaseSourceIndex(args []string) int {
	flags := flag.NewFlagSet("release source-index", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	repositoryRoot := flags.String("repository-root", "", "clean Git repository root")
	destination := flags.String("destination", "", "new canonical portable source-tree provenance path")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "release source-index:", err)
		return 2
	}
	if *repositoryRoot == "" || *destination == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "release source-index: --repository-root and --destination are required; positional arguments are forbidden")
		return 2
	}
	if err := requireNewCommandTargets([]string{*destination}); err != nil {
		fmt.Fprintln(os.Stderr, "release source-index:", err)
		return 1
	}
	source, err := capsuleartifact.CollectGitSourceTreeProvenance(context.Background(), *repositoryRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release source-index:", err)
		return 1
	}
	if err := capsuleartifact.ValidatePortableSourceTreeProvenance(source); err != nil {
		fmt.Fprintln(os.Stderr, "release source-index:", err)
		return 1
	}
	raw, err := capsuleartifact.EncodeSourceTreeProvenance(source)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release source-index:", err)
		return 1
	}
	return writeReleaseArtifact("source-index", *destination, raw)
}

func runReleaseSourceArchive(args []string) int {
	flags := flag.NewFlagSet("release source-archive", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	repositoryRoot := flags.String("repository-root", "", "clean Git repository root")
	commit := flags.String("commit", "", "exact current Git commit")
	destination := flags.String("destination", "", "new canonical source archive path outside the repository")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "release source-archive:", err)
		return 2
	}
	if *repositoryRoot == "" || *commit == "" || *destination == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "release source-archive: --repository-root, --commit, and --destination are required; positional arguments are forbidden")
		return 2
	}
	report, err := releaseartifact.CreateSourceArchive(context.Background(), *repositoryRoot, *commit, *destination)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release source-archive:", err)
		return 1
	}
	raw, err := releaseartifact.EncodeSourceArchiveReport(report)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release source-archive:", err)
		return 1
	}
	return writeCommandOutput("release source-archive", raw)
}

func runReleaseManifest(args []string) int {
	flags := flag.NewFlagSet("release manifest", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	assets := flags.String("assets", "", "release asset root")
	commit := flags.String("commit", "", "exact Git commit")
	sourceArchiveSHA256 := flags.String("source-archive-sha256", "", "deterministic source archive SHA-256")
	created := flags.String("created", "", "canonical UTC RFC3339 commit time")
	externalPublication := flags.String("external-publication", "not_authorized", "not_authorized or authorized_by_tag")
	destination := flags.String("destination", "", "new manifest path")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "release manifest:", err)
		return 2
	}
	if *assets == "" || *commit == "" || *sourceArchiveSHA256 == "" || *created == "" || *destination == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "release manifest: --assets, --commit, --source-archive-sha256, --created, and --destination are required; positional arguments are forbidden")
		return 2
	}
	if err := requireNewCommandTargets([]string{*destination}); err != nil {
		fmt.Fprintln(os.Stderr, "release manifest:", err)
		return 1
	}
	manifest, err := releaseartifact.BuildManifest(*assets, version, *commit, *sourceArchiveSHA256, *created)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release manifest:", err)
		return 1
	}
	manifest.Truth.ExternalPublication = *externalPublication
	raw, err := releaseartifact.EncodeManifest(manifest)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release manifest:", err)
		return 1
	}
	return writeReleaseArtifact("manifest", *destination, raw)
}

func runReleaseSBOM(args []string) int {
	flags := flag.NewFlagSet("release sbom", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	assets := flags.String("assets", "", "release asset root")
	manifestPath := flags.String("manifest", "", "canonical release manifest")
	destination := flags.String("destination", "", "new SPDX SBOM path")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "release sbom:", err)
		return 2
	}
	if *assets == "" || *manifestPath == "" || *destination == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "release sbom: --assets, --manifest, and --destination are required; positional arguments are forbidden")
		return 2
	}
	if err := requireNewCommandTargets([]string{*destination}); err != nil {
		fmt.Fprintln(os.Stderr, "release sbom:", err)
		return 1
	}
	manifestRaw, err := readBoundedCommandFile(*manifestPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release sbom:", err)
		return 1
	}
	manifest, err := releaseartifact.DecodeManifest(manifestRaw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release sbom:", err)
		return 1
	}
	document, err := releaseartifact.BuildSBOM(*assets, manifest)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release sbom:", err)
		return 1
	}
	raw, err := releaseartifact.EncodeSBOM(document, manifest)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release sbom:", err)
		return 1
	}
	return writeReleaseArtifact("sbom", *destination, raw)
}

func runReleaseStatement(args []string) int {
	flags := flag.NewFlagSet("release statement", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	manifestPath := flags.String("manifest", "", "canonical release manifest")
	sbomPath := flags.String("sbom", "", "canonical SPDX SBOM")
	destination := flags.String("destination", "", "new in-toto statement path")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "release statement:", err)
		return 2
	}
	if *manifestPath == "" || *sbomPath == "" || *destination == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "release statement: --manifest, --sbom, and --destination are required; positional arguments are forbidden")
		return 2
	}
	if err := requireNewCommandTargets([]string{*destination}); err != nil {
		fmt.Fprintln(os.Stderr, "release statement:", err)
		return 1
	}
	manifestRaw, sbomRaw, err := readReleasePair(*manifestPath, *sbomPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release statement:", err)
		return 1
	}
	statement, err := releaseartifact.BuildStatement(manifestRaw, sbomRaw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release statement:", err)
		return 1
	}
	raw, err := releaseartifact.EncodeStatement(statement, manifestRaw, sbomRaw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release statement:", err)
		return 1
	}
	return writeReleaseArtifact("statement", *destination, raw)
}

func runReleaseSign(args []string) int {
	flags := flag.NewFlagSet("release sign", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	manifestPath := flags.String("manifest", "", "canonical release manifest")
	sbomPath := flags.String("sbom", "", "canonical SPDX SBOM")
	statementPath := flags.String("statement", "", "canonical in-toto release statement")
	keyPath := flags.String("key-file", "", "existing mode-0600 Ed25519 private key file")
	destination := flags.String("destination", "", "new signature-material directory")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "release sign:", err)
		return 2
	}
	if *manifestPath == "" || *sbomPath == "" || *statementPath == "" || *keyPath == "" || *destination == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "release sign: --manifest, --sbom, --statement, --key-file, and --destination are required; positional arguments are forbidden")
		return 2
	}
	if err := requireNewCommandTargets([]string{*destination}); err != nil {
		fmt.Fprintln(os.Stderr, "release sign:", err)
		return 1
	}
	manifestRaw, sbomRaw, err := readReleasePair(*manifestPath, *sbomPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release sign:", err)
		return 1
	}
	statementRaw, err := readBoundedCommandFile(*statementPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release sign:", err)
		return 1
	}
	privateKeyRaw, err := readReleasePrivateKey(*keyPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release sign:", err)
		return 1
	}
	signed, err := releaseartifact.SealStatement(manifestRaw, sbomRaw, statementRaw, privateKeyRaw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release sign:", err)
		return 1
	}
	envelopeRaw, err := releaseartifact.EncodeEnvelope(signed.Envelope)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release sign:", err)
		return 1
	}
	rootRaw, err := releaseartifact.EncodeTrustRoot(signed.TrustRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release sign:", err)
		return 1
	}
	policyRaw, err := releaseartifact.EncodeSignaturePolicy(signed.Policy, signed.TrustRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release sign:", err)
		return 1
	}
	if err := writeReleaseSignatureDirectory(*destination, envelopeRaw, rootRaw, policyRaw); err != nil {
		fmt.Fprintln(os.Stderr, "release sign:", err)
		return 1
	}
	report := releaseSignatureReport{
		SchemaVersion: "evalwitness.release-signature-report.v1", KeyID: signed.Policy.AllowedKeyIDs[0], Destination: *destination,
		EnvelopeSHA256: protocol.DigestBytes(envelopeRaw), TrustRootSHA256: protocol.DigestBytes(rootRaw), PolicySHA256: protocol.DigestBytes(policyRaw),
	}
	return writeCanonicalCommandOutput("release sign", report)
}

func runReleaseVerify(args []string) int {
	flags := flag.NewFlagSet("release verify", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	assets := flags.String("assets", "", "release asset root")
	manifestPath := flags.String("manifest", "", "canonical release manifest")
	sbomPath := flags.String("sbom", "", "canonical SPDX SBOM")
	statementPath := flags.String("statement", "", "canonical in-toto release statement")
	signaturePath := flags.String("signature", "", "signature-material directory")
	allowUnsigned := flags.Bool("allow-unsigned-development", false, "explicitly allow an unsigned development candidate")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "release verify:", err)
		return 2
	}
	if *assets == "" || *manifestPath == "" || *sbomPath == "" || *statementPath == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "release verify: --assets, --manifest, --sbom, and --statement are required; positional arguments are forbidden")
		return 2
	}
	manifestRaw, sbomRaw, err := readReleasePair(*manifestPath, *sbomPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release verify:", err)
		return 1
	}
	statementRaw, err := readBoundedCommandFile(*statementPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release verify:", err)
		return 1
	}
	envelopeRaw, trustRootRaw, policyRaw, err := readReleaseSignatureDirectory(*signaturePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release verify:", err)
		return 1
	}
	report, err := releaseartifact.Verify(*assets, manifestRaw, sbomRaw, statementRaw, envelopeRaw, trustRootRaw, policyRaw, *allowUnsigned)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release verify:", err)
		return 1
	}
	return writeCanonicalCommandOutput("release verify", report)
}

func readReleasePair(manifestPath, sbomPath string) ([]byte, []byte, error) {
	manifestRaw, err := readBoundedCommandFile(manifestPath)
	if err != nil {
		return nil, nil, err
	}
	sbomRaw, err := readBoundedCommandFile(sbomPath)
	if err != nil {
		return nil, nil, err
	}
	return manifestRaw, sbomRaw, nil
}

func readReleasePrivateKey(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o600 || info.Size() < 1 || info.Size() > 129 {
		return nil, fmt.Errorf("private key path %q must be a mode-0600 bounded regular file", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil || int64(len(raw)) != info.Size() {
		return nil, errors.Join(err, errors.New("private key read was incomplete"))
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(info, after) || after.Size() != info.Size() || after.ModTime() != info.ModTime() {
		return nil, errors.New("private key changed during read")
	}
	return raw, nil
}

func readReleaseSignatureDirectory(path string) ([]byte, []byte, []byte, error) {
	if path == "" {
		return nil, nil, nil, nil
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, nil, nil, fmt.Errorf("signature path %q is not a real directory", path)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, nil, nil, err
	}
	expected := []string{"envelope.dsse.json", "policy.json", "trust-root.json"}
	if len(entries) != len(expected) {
		return nil, nil, nil, errors.New("signature directory does not contain the exact canonical file set")
	}
	for index, entry := range entries {
		if entry.Name() != expected[index] || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil, nil, nil, errors.New("signature directory does not contain the exact canonical file set")
		}
	}
	envelopeRaw, err := readBoundedCommandFile(filepath.Join(path, expected[0]))
	if err != nil {
		return nil, nil, nil, err
	}
	policyRaw, err := readBoundedCommandFile(filepath.Join(path, expected[1]))
	if err != nil {
		return nil, nil, nil, err
	}
	rootRaw, err := readBoundedCommandFile(filepath.Join(path, expected[2]))
	if err != nil {
		return nil, nil, nil, err
	}
	return envelopeRaw, rootRaw, policyRaw, nil
}

func writeReleaseSignatureDirectory(destination string, envelopeRaw, rootRaw, policyRaw []byte) error {
	pathPolicy, err := safety.CurrentPathPolicy()
	if err != nil {
		return err
	}
	validated, err := pathPolicy.ValidateMutationRoot(destination)
	if err != nil {
		return err
	}
	destination = validated
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	if err := os.Mkdir(destination, 0o755); err != nil {
		return err
	}
	created := make([]os.FileInfo, 0, 3)
	paths := []string{
		filepath.Join(destination, "envelope.dsse.json"),
		filepath.Join(destination, "policy.json"),
		filepath.Join(destination, "trust-root.json"),
	}
	raws := [][]byte{envelopeRaw, policyRaw, rootRaw}
	committed := false
	defer func() {
		if committed {
			return
		}
		for index, info := range created {
			current, err := os.Lstat(paths[index])
			if err == nil && os.SameFile(info, current) {
				_ = os.Remove(paths[index])
			}
		}
		_ = os.Remove(destination)
	}()
	for index, path := range paths {
		if err := writeNewPublicCommandFile(path, raws[index]); err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		created = append(created, info)
	}
	committed = true
	return nil
}

func writeReleaseArtifact(kind, destination string, raw []byte) int {
	if err := writeNewPublicCommandFile(destination, raw); err != nil {
		fmt.Fprintln(os.Stderr, "release "+kind+":", err)
		return 1
	}
	report := releaseArtifactReport{
		SchemaVersion: "evalwitness.release-artifact-report.v1", Artifact: kind, Destination: destination,
		SHA256: protocol.DigestBytes(raw), Bytes: len(raw),
	}
	return writeCanonicalCommandOutput("release "+kind, report)
}
