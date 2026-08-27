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
	"github.com/Christopher-Schulze/evalwitness/internal/explorer"
	"github.com/Christopher-Schulze/evalwitness/internal/profile"
	"github.com/Christopher-Schulze/evalwitness/internal/reliance"
	"github.com/Christopher-Schulze/evalwitness/internal/replay"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

type identicalResponseEvidencePaths struct {
	baseCapsule        string
	capsule            string
	ledger             string
	challengePack      string
	reproductionReport string
}

func runClaimReport(args []string) int {
	flags := flag.NewFlagSet("claim report", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	capsulePath := flags.String("capsule", "", "verified public capsule directory")
	ledgerPath := flags.String("ledger", "", "canonical claim ledger")
	repositoryRoot := flags.String("repository-root", "", "repository root containing the bound TASK 069 release files")
	relianceCapsulePath := flags.String("reliance-capsule", "", "optional verified TASK 065 reliance capsule directory")
	relianceLedgerPath := flags.String("reliance-ledger", "", "optional canonical TASK 065 reliance claim ledger")
	profilePath := flags.String("profile", "", "optional verified TASK 058 reliability profile JSON")
	identicalBasePath := flags.String("identical-response-base-capsule", "", "optional verified TASK 070 response-bundle capsule directory")
	identicalCapsulePath := flags.String("identical-response-capsule", "", "optional verified TASK 070 outer capsule directory")
	identicalLedgerPath := flags.String("identical-response-ledger", "", "optional canonical TASK 070 claim ledger")
	identicalChallengePath := flags.String("identical-response-challenge-pack", "", "optional canonical TASK 070 challenge pack")
	identicalReproductionPath := flags.String("identical-response-reproduction-report", "", "optional verified TASK 070 reproduction report")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "claim report:", err)
		return 2
	}
	identicalPaths := identicalResponseEvidencePaths{
		baseCapsule: *identicalBasePath, capsule: *identicalCapsulePath, ledger: *identicalLedgerPath,
		challengePack: *identicalChallengePath, reproductionReport: *identicalReproductionPath,
	}
	if *capsulePath == "" || *ledgerPath == "" || *repositoryRoot == "" || flags.NArg() != 0 ||
		!validReliancePaths(*relianceCapsulePath, *relianceLedgerPath) || !identicalPaths.valid() {
		fmt.Fprintln(os.Stderr, "claim report: --capsule, --ledger, and --repository-root are required; reliance paths must be supplied together; all five identical-response paths must be supplied together; positional arguments are forbidden")
		return 2
	}
	report, err := buildClaimReportModelWithEvidence(context.Background(), *capsulePath, *ledgerPath, *repositoryRoot,
		*relianceCapsulePath, *relianceLedgerPath, *profilePath, identicalPaths)
	if err != nil {
		fmt.Fprintln(os.Stderr, "claim report:", err)
		return 1
	}
	raw, err := explorer.EncodeReport(report)
	if err != nil {
		fmt.Fprintln(os.Stderr, "claim report:", err)
		return 1
	}
	return writeCommandOutput("claim report", append(raw, '\n'))
}

// loadVerifiedProfile decodes a profile strictly (DisallowUnknownFields),
// validates it and enforces digest equality; shared semantics with the claim
// verify claimcheck gate.
func loadVerifiedProfile(path string) (profile.Profile, error) {
	f, err := os.Open(path)
	if err != nil {
		return profile.Profile{}, err
	}
	defer func() { _ = f.Close() }()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	var p profile.Profile
	if err := dec.Decode(&p); err != nil {
		return profile.Profile{}, err
	}
	if err := p.Validate(); err != nil {
		return profile.Profile{}, err
	}
	dig, err := p.DigestValue()
	if err != nil {
		return profile.Profile{}, err
	}
	if dig != p.Digest {
		return profile.Profile{}, fmt.Errorf("profile digest mismatch: %s vs %s", p.Digest, dig)
	}
	return p, nil
}

func buildClaimReportModel(ctx context.Context, capsulePath string, ledgerPath string, repositoryRoot string) (explorer.Report, error) {
	return buildClaimReportModelWithReliance(ctx, capsulePath, ledgerPath, repositoryRoot, "", "")
}

func buildClaimReportModelWithReliance(
	ctx context.Context,
	capsulePath string,
	ledgerPath string,
	repositoryRoot string,
	relianceCapsulePath string,
	relianceLedgerPath string,
) (explorer.Report, error) {
	if ctx == nil || capsulePath == "" || ledgerPath == "" || repositoryRoot == "" ||
		!validReliancePaths(relianceCapsulePath, relianceLedgerPath) {
		return explorer.Report{}, errors.New("claim report requires context and all evidence paths")
	}
	base, ledger, autopsy, err := loadClaimReportEvidence(ctx, capsulePath, ledgerPath)
	if err != nil {
		return explorer.Report{}, err
	}
	report, err := explorer.BuildReport(ctx, repositoryRoot,
		capsule.Package(base), ledger, autopsy)
	if err != nil {
		return explorer.Report{}, fmt.Errorf("build evidence explorer report: %w", err)
	}
	return addClaimReportReliance(ctx, report, base, relianceCapsulePath, relianceLedgerPath)
}

func loadClaimReportEvidence(
	ctx context.Context,
	capsulePath string,
	ledgerPath string,
) (capsule.ReferencePackage, claimledger.Ledger, claimledger.Autopsy, error) {
	registry, err := capsule.ReferenceRegistry()
	if err != nil {
		return capsule.ReferencePackage{}, claimledger.Ledger{}, claimledger.Autopsy{}, fmt.Errorf("build reference registry: %w", err)
	}
	manifest, payloads, err := capsule.LoadDirectory(
		ctx,
		capsulePath,
		registry,
		capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic},
	)
	if err != nil {
		return capsule.ReferencePackage{}, claimledger.Ledger{}, claimledger.Autopsy{}, fmt.Errorf("load verified capsule: %w", err)
	}
	ledger, err := readClaimLedger(ledgerPath)
	if err != nil {
		return capsule.ReferencePackage{}, claimledger.Ledger{}, claimledger.Autopsy{}, fmt.Errorf("read claim ledger: %w", err)
	}
	autopsy, err := claimledger.BuildAutopsy(ctx, registry, manifest, payloads, ledger)
	if err != nil {
		return capsule.ReferencePackage{}, claimledger.Ledger{}, claimledger.Autopsy{}, fmt.Errorf("build claim autopsy: %w", err)
	}
	return capsule.ReferencePackage{Registry: registry, Manifest: manifest, Payloads: payloads}, ledger, autopsy, nil
}

func addClaimReportReliance(
	ctx context.Context,
	report explorer.Report,
	base capsule.ReferencePackage,
	capsulePath string,
	ledgerPath string,
) (explorer.Report, error) {
	if capsulePath == "" {
		return report, nil
	}
	child, err := reliance.LoadEvidenceRelianceCapsule(ctx, capsulePath, base)
	if err != nil {
		return explorer.Report{}, fmt.Errorf("load verified evidence reliance capsule: %w", err)
	}
	ledger, err := readClaimLedger(ledgerPath)
	if err != nil {
		return explorer.Report{}, fmt.Errorf("read evidence reliance claim ledger: %w", err)
	}
	report, err = explorer.AddEvidenceReliance(ctx, report, base, child, ledger)
	if err != nil {
		return explorer.Report{}, fmt.Errorf("bind evidence reliance explorer view: %w", err)
	}
	return report, nil
}

func validReliancePaths(capsulePath string, ledgerPath string) bool {
	return (capsulePath == "") == (ledgerPath == "")
}

func (paths identicalResponseEvidencePaths) valid() bool {
	values := []string{paths.baseCapsule, paths.capsule, paths.ledger, paths.challengePack, paths.reproductionReport}
	present := 0
	for _, value := range values {
		if value != "" {
			present++
		}
	}
	return present == 0 || present == len(values)
}

func buildClaimReportModelWithEvidence(
	ctx context.Context,
	capsulePath string,
	ledgerPath string,
	repositoryRoot string,
	relianceCapsulePath string,
	relianceLedgerPath string,
	profilePath string,
	identicalPaths identicalResponseEvidencePaths,
) (explorer.Report, error) {
	report, err := buildClaimReportModelWithReliance(ctx, capsulePath, ledgerPath, repositoryRoot, relianceCapsulePath, relianceLedgerPath)
	if err != nil {
		return explorer.Report{}, err
	}
	if profilePath != "" {
		p, loadErr := loadVerifiedProfile(profilePath)
		if loadErr != nil {
			return explorer.Report{}, loadErr
		}
		report, err = explorer.AddProfile(report, p)
		if err != nil {
			return explorer.Report{}, err
		}
	}
	return addClaimReportIdenticalResponse(ctx, report, identicalPaths)
}

func addClaimReportIdenticalResponse(ctx context.Context, report explorer.Report, paths identicalResponseEvidencePaths) (explorer.Report, error) {
	if paths.baseCapsule == "" {
		return report, nil
	}
	base, err := replay.LoadResponseBundlePackage(ctx, paths.baseCapsule)
	if err != nil {
		return explorer.Report{}, fmt.Errorf("load identical-response base capsule: %w", err)
	}
	child, err := replay.LoadIdenticalResponseCapsulePackage(ctx, paths.capsule, base.Registry)
	if err != nil {
		return explorer.Report{}, fmt.Errorf("load identical-response outer capsule: %w", err)
	}
	ledger, err := readClaimLedger(paths.ledger)
	if err != nil {
		return explorer.Report{}, fmt.Errorf("read identical-response claim ledger: %w", err)
	}
	pack, err := readIdenticalResponseChallengePack(paths.challengePack)
	if err != nil {
		return explorer.Report{}, err
	}
	reproduction, err := readBoundedCommandFile(paths.reproductionReport)
	if err != nil {
		return explorer.Report{}, fmt.Errorf("read identical-response reproduction report: %w", err)
	}
	return explorer.AddIdenticalResponse(ctx, report, base, child, ledger, pack, reproduction)
}

func readIdenticalResponseChallengePack(path string) (claimledger.ChallengePack, error) {
	raw, err := readBoundedCommandFile(path)
	if err != nil {
		return claimledger.ChallengePack{}, fmt.Errorf("read identical-response challenge pack: %w", err)
	}
	var pack claimledger.ChallengePack
	if err := protocol.DecodeStrict(bytes.TrimSpace(raw), &pack); err != nil {
		return claimledger.ChallengePack{}, fmt.Errorf("decode identical-response challenge pack: %w", err)
	}
	return pack, nil
}
