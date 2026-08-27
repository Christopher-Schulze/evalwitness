package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/relation"
	"github.com/Christopher-Schulze/evalwitness/internal/stress"
)

const (
	defaultStressPlanPath           = "eval/governance/controlled-corruption-v3-plan.json"
	defaultStressAuditPath          = "eval/governance/controlled-corruption-v3-natural-audit.json"
	defaultStressReleasePath        = "eval/governance/controlled-corruption-v3-release.json"
	defaultStressOwnerPath          = "eval/results/relation-owner-inspection-attestation.json"
	defaultStressOwnerPackageDigest = "533deaaecd328d972cdf770073afb0f56e560d4aadea59be1e111d0782eafd80"
)

type stressCorpusFlags struct {
	planPath    *string
	auditPath   *string
	releasePath *string
}

func runStress(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "stress: usage: evalwitness stress <catalog|arm-plan|analysis-design|held-out-lock|held-out-campaign|held-out-readiness|development-case-study|development-challenge|verify-development-challenge|verify-development-challenge-receipt|validate|schema>")
		return 2
	}
	switch args[0] {
	case "catalog":
		return runStressCatalog(args[1:])
	case "arm-plan":
		return runStressArmPlan(args[1:])
	case "analysis-design":
		return runStressAnalysisDesign(args[1:])
	case "held-out-lock":
		return runStressHeldOutLock(args[1:])
	case "held-out-campaign":
		return runStressHeldOutCampaign(args[1:])
	case "held-out-readiness":
		return runStressHeldOutReadiness(args[1:])
	case "development-case-study":
		return runStressDevelopmentCaseStudy(args[1:])
	case "development-challenge":
		return runStressDevelopmentChallenge(args[1:])
	case "verify-development-challenge":
		return runStressVerifyDevelopmentChallenge(args[1:])
	case "verify-development-challenge-receipt":
		return runStressVerifyDevelopmentChallengeReceipt(args[1:])
	case "validate":
		return runStressValidate(args[1:])
	case "schema":
		return runStressSchema(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "stress: unknown command %q\n", args[0])
		return 2
	}
}

func runStressVerifyDevelopmentChallengeReceipt(args []string) int {
	flags := flag.NewFlagSet("stress verify-development-challenge-receipt", flag.ContinueOnError)
	challengePath := flags.String("challenge", "", "self-contained stress development challenge (@file)")
	receiptPath := flags.String("receipt", "", "stress development challenge receipt (@file)")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "stress verify-development-challenge-receipt: positional arguments are forbidden")
		return 2
	}
	challengeReader, closeChallenge, err := openStudyDocument(*challengePath)
	if err != nil {
		return stressInputError("verify-development-challenge-receipt", err)
	}
	defer closeChallenge()
	challenge, err := stress.DecodeDevelopmentChallenge(challengeReader)
	if err != nil {
		return stressInputError("verify-development-challenge-receipt", err)
	}
	receiptReader, closeReceipt, err := openStudyDocument(*receiptPath)
	if err != nil {
		return stressInputError("verify-development-challenge-receipt", err)
	}
	defer closeReceipt()
	receipt, err := stress.DecodeDevelopmentChallengeReceipt(receiptReader)
	if err != nil {
		return stressInputError("verify-development-challenge-receipt", err)
	}
	verification, err := stress.VerifyDevelopmentChallengeReceipt(challenge, receipt)
	if err != nil {
		return stressOperationError("verify-development-challenge-receipt", err)
	}
	return printStressJSON("verify-development-challenge-receipt", verification)
}

func runStressDevelopmentChallenge(args []string) int {
	flags := flag.NewFlagSet("stress development-challenge", flag.ContinueOnError)
	root := flags.String("repository-root", ".", "repository root containing the public development fixtures")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "stress development-challenge: positional arguments are forbidden")
		return 2
	}
	value, err := stress.BuildDevelopmentChallenge(*root)
	if err != nil {
		return stressOperationError("development-challenge", err)
	}
	return printStressJSON("development-challenge", value)
}

func runStressVerifyDevelopmentChallenge(args []string) int {
	flags := flag.NewFlagSet("stress verify-development-challenge", flag.ContinueOnError)
	challengePath := flags.String("challenge", "", "self-contained stress development challenge (@file or - for stdin)")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "stress verify-development-challenge: positional arguments are forbidden")
		return 2
	}
	reader, closeChallenge, err := openStudyDocument(*challengePath)
	if err != nil {
		return stressInputError("verify-development-challenge", err)
	}
	defer closeChallenge()
	value, err := stress.DecodeDevelopmentChallenge(reader)
	if err != nil {
		return stressInputError("verify-development-challenge", err)
	}
	receipt, err := stress.VerifyDevelopmentChallenge(value)
	if err != nil {
		return stressOperationError("verify-development-challenge", err)
	}
	return printStressJSON("verify-development-challenge", receipt)
}

func runStressHeldOutCampaign(args []string) int {
	flags := flag.NewFlagSet("stress held-out-campaign", flag.ContinueOnError)
	root := flags.String("repository-root", ".", "repository root containing fetched trajectory sources")
	format := flags.String("format", "json", "output format: json or markdown")
	corpus := addStressCorpusFlags(flags)
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "stress held-out-campaign: --format must be json or markdown and positional arguments are forbidden")
		return 2
	}
	registry, replayed, err := loadStressReplay(*root, corpus)
	if err != nil {
		return stressOperationError("held-out-campaign", err)
	}
	armPlan, err := stress.BuildArmComparisonPlan(registry, replayed)
	if err != nil {
		return stressOperationError("held-out-campaign", err)
	}
	design, err := stress.BuildStressAnalysisDesign(armPlan, registry, replayed)
	if err != nil {
		return stressOperationError("held-out-campaign", err)
	}
	lock, err := stress.BuildHeldOutPartitionLock(design, armPlan, registry, replayed)
	if err != nil {
		return stressOperationError("held-out-campaign", err)
	}
	value, err := stress.BuildHeldOutCampaignPlan(lock, design, armPlan, registry, replayed)
	if err != nil {
		return stressOperationError("held-out-campaign", err)
	}
	switch *format {
	case "json":
		return printStressJSON("held-out-campaign", value)
	case "markdown":
		rendered, renderErr := stress.RenderHeldOutCampaignPlanMarkdown(value)
		if renderErr != nil {
			return stressOperationError("held-out-campaign", renderErr)
		}
		return writeCommandOutput("stress held-out-campaign", []byte(rendered))
	default:
		fmt.Fprintln(os.Stderr, "stress held-out-campaign: --format must be json or markdown")
		return 2
	}
}

func runStressCatalog(args []string) int {
	flags := flag.NewFlagSet("stress catalog", flag.ContinueOnError)
	corpus := addStressCorpusFlags(flags)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, audit, release, err := loadStressCorpus(corpus)
	if err != nil {
		return stressInputError("catalog", err)
	}
	registry, err := stress.BuildV3RelationRegistry(plan, audit, release)
	if err != nil {
		return stressOperationError("catalog", err)
	}
	return printStressJSON("catalog", registry)
}

func runStressArmPlan(args []string) int {
	flags := flag.NewFlagSet("stress arm-plan", flag.ContinueOnError)
	root := flags.String("repository-root", ".", "repository root containing fetched trajectory sources")
	corpus := addStressCorpusFlags(flags)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	registry, replayed, err := loadStressReplay(*root, corpus)
	if err != nil {
		return stressOperationError("arm-plan", err)
	}
	plan, err := stress.BuildArmComparisonPlan(registry, replayed)
	if err != nil {
		return stressOperationError("arm-plan", err)
	}
	return printStressJSON("arm-plan", plan)
}

func runStressAnalysisDesign(args []string) int {
	flags := flag.NewFlagSet("stress analysis-design", flag.ContinueOnError)
	root := flags.String("repository-root", ".", "repository root containing fetched trajectory sources")
	corpus := addStressCorpusFlags(flags)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	registry, replayed, err := loadStressReplay(*root, corpus)
	if err != nil {
		return stressOperationError("analysis-design", err)
	}
	armPlan, err := stress.BuildArmComparisonPlan(registry, replayed)
	if err != nil {
		return stressOperationError("analysis-design", err)
	}
	design, err := stress.BuildStressAnalysisDesign(armPlan, registry, replayed)
	if err != nil {
		return stressOperationError("analysis-design", err)
	}
	return printStressJSON("analysis-design", design)
}

func runStressHeldOutLock(args []string) int {
	flags := flag.NewFlagSet("stress held-out-lock", flag.ContinueOnError)
	root := flags.String("repository-root", ".", "repository root containing fetched trajectory sources")
	corpus := addStressCorpusFlags(flags)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	registry, replayed, err := loadStressReplay(*root, corpus)
	if err != nil {
		return stressOperationError("held-out-lock", err)
	}
	armPlan, err := stress.BuildArmComparisonPlan(registry, replayed)
	if err != nil {
		return stressOperationError("held-out-lock", err)
	}
	design, err := stress.BuildStressAnalysisDesign(armPlan, registry, replayed)
	if err != nil {
		return stressOperationError("held-out-lock", err)
	}
	lock, err := stress.BuildHeldOutPartitionLock(design, armPlan, registry, replayed)
	if err != nil {
		return stressOperationError("held-out-lock", err)
	}
	return printStressJSON("held-out-lock", lock)
}

func runStressHeldOutReadiness(args []string) int {
	flags := flag.NewFlagSet("stress held-out-readiness", flag.ContinueOnError)
	root := flags.String("repository-root", ".", "repository root containing fetched trajectory sources")
	ownerPath := flags.String("owner-attestation", defaultStressOwnerPath, "current owner-inspection public attestation (@file)")
	ownerPackageDigest := flags.String("owner-package-inventory-digest", defaultStressOwnerPackageDigest, "expected private owner package inventory SHA-256")
	format := flags.String("format", "json", "output format: json or markdown")
	corpus := addStressCorpusFlags(flags)
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "stress held-out-readiness: --format must be json or markdown and positional arguments are forbidden")
		return 2
	}
	registry, replayed, err := loadStressReplay(*root, corpus)
	if err != nil {
		return stressOperationError("held-out-readiness", err)
	}
	armPlan, err := stress.BuildArmComparisonPlan(registry, replayed)
	if err != nil {
		return stressOperationError("held-out-readiness", err)
	}
	design, err := stress.BuildStressAnalysisDesign(armPlan, registry, replayed)
	if err != nil {
		return stressOperationError("held-out-readiness", err)
	}
	lock, err := stress.BuildHeldOutPartitionLock(design, armPlan, registry, replayed)
	if err != nil {
		return stressOperationError("held-out-readiness", err)
	}
	ownerReader, closeOwner, err := openStudyDocument(*ownerPath)
	if err != nil {
		return stressInputError("held-out-readiness", err)
	}
	defer closeOwner()
	owner, err := relation.DecodeOwnerInspectionPublicAttestation(ownerReader)
	if err != nil {
		return stressInputError("held-out-readiness", err)
	}
	value, err := stress.BuildHeldOutRunReadinessRefusal(lock, design, armPlan, registry, replayed, owner, *ownerPackageDigest)
	if err != nil {
		return stressOperationError("held-out-readiness", err)
	}
	switch *format {
	case "json":
		return printStressJSON("held-out-readiness", value)
	case "markdown":
		rendered, renderErr := stress.RenderHeldOutRunReadinessRefusalMarkdown(value)
		if renderErr != nil {
			return stressOperationError("held-out-readiness", renderErr)
		}
		return writeCommandOutput("stress held-out-readiness", []byte(rendered))
	default:
		fmt.Fprintln(os.Stderr, "stress held-out-readiness: --format must be json or markdown")
		return 2
	}
}

func runStressDevelopmentCaseStudy(args []string) int {
	flags := flag.NewFlagSet("stress development-case-study", flag.ContinueOnError)
	root := flags.String("repository-root", ".", "repository root containing the public development fixtures")
	format := flags.String("format", "markdown", "output format: json, markdown, or svg")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "stress development-case-study: --format must be json, markdown, or svg and positional arguments are forbidden")
		return 2
	}
	value, err := stress.BuildDevelopmentCaseStudy(*root)
	if err != nil {
		return stressOperationError("development-case-study", err)
	}
	switch *format {
	case "json":
		return printStressJSON("development-case-study", value)
	case "markdown":
		rendered, renderErr := stress.RenderDevelopmentCaseStudyMarkdown(value)
		if renderErr != nil {
			return stressOperationError("development-case-study", renderErr)
		}
		return writeCommandOutput("stress development-case-study", []byte(rendered))
	case "svg":
		rendered, renderErr := stress.RenderDevelopmentCaseStudySVG(value)
		if renderErr != nil {
			return stressOperationError("development-case-study", renderErr)
		}
		return writeCommandOutput("stress development-case-study", rendered)
	default:
		fmt.Fprintln(os.Stderr, "stress development-case-study: --format must be json, markdown, or svg")
		return 2
	}
}

func runStressValidate(args []string) int {
	flags := flag.NewFlagSet("stress validate", flag.ContinueOnError)
	documentType := flags.String("type", "development-case-study", "document type: development-case-study, held-out-campaign-plan, or held-out-run-readiness-refusal")
	documentPath := flags.String("document", "", "stress document (@file or - for stdin)")
	root := flags.String("repository-root", ".", "repository root for exact fixture verification")
	ownerPath := flags.String("owner-attestation", defaultStressOwnerPath, "current owner-inspection public attestation (@file)")
	ownerPackageDigest := flags.String("owner-package-inventory-digest", defaultStressOwnerPackageDigest, "expected private owner package inventory SHA-256")
	corpus := addStressCorpusFlags(flags)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	reader, closeDocument, err := openStudyDocument(*documentPath)
	if err != nil {
		return stressInputError("validate", err)
	}
	defer closeDocument()
	switch *documentType {
	case "development-case-study":
		value, decodeErr := stress.DecodeDevelopmentCaseStudy(reader)
		if decodeErr != nil {
			return stressInputError("validate", decodeErr)
		}
		if validateErr := value.ValidateAgainstRepository(*root); validateErr != nil {
			return stressOperationError("validate", validateErr)
		}
		return printStressValidation(*documentType, value.Digest, value.EmpiricalUnits, value.ProviderCalls, value.NetworkRequired)
	case "held-out-campaign-plan":
		value, decodeErr := stress.DecodeHeldOutCampaignPlan(reader)
		if decodeErr != nil {
			return stressInputError("validate", decodeErr)
		}
		registry, replayed, loadErr := loadStressReplay(*root, corpus)
		if loadErr != nil {
			return stressOperationError("validate", loadErr)
		}
		armPlan, buildErr := stress.BuildArmComparisonPlan(registry, replayed)
		if buildErr != nil {
			return stressOperationError("validate", buildErr)
		}
		design, buildErr := stress.BuildStressAnalysisDesign(armPlan, registry, replayed)
		if buildErr != nil {
			return stressOperationError("validate", buildErr)
		}
		lock, buildErr := stress.BuildHeldOutPartitionLock(design, armPlan, registry, replayed)
		if buildErr != nil {
			return stressOperationError("validate", buildErr)
		}
		if validateErr := value.ValidateAgainst(lock, design, armPlan, registry, replayed); validateErr != nil {
			return stressOperationError("validate", validateErr)
		}
		return printStressValidation(*documentType, value.Digest, value.EmpiricalUnits, value.ProviderCalls, value.NetworkRequired)
	case "held-out-run-readiness-refusal":
		value, decodeErr := stress.DecodeHeldOutRunReadinessRefusal(reader)
		if decodeErr != nil {
			return stressInputError("validate", decodeErr)
		}
		registry, replayed, loadErr := loadStressReplay(*root, corpus)
		if loadErr != nil {
			return stressOperationError("validate", loadErr)
		}
		armPlan, buildErr := stress.BuildArmComparisonPlan(registry, replayed)
		if buildErr != nil {
			return stressOperationError("validate", buildErr)
		}
		design, buildErr := stress.BuildStressAnalysisDesign(armPlan, registry, replayed)
		if buildErr != nil {
			return stressOperationError("validate", buildErr)
		}
		lock, buildErr := stress.BuildHeldOutPartitionLock(design, armPlan, registry, replayed)
		if buildErr != nil {
			return stressOperationError("validate", buildErr)
		}
		ownerReader, closeOwner, openErr := openStudyDocument(*ownerPath)
		if openErr != nil {
			return stressInputError("validate", openErr)
		}
		defer closeOwner()
		owner, decodeErr := relation.DecodeOwnerInspectionPublicAttestation(ownerReader)
		if decodeErr != nil {
			return stressInputError("validate", decodeErr)
		}
		if validateErr := value.ValidateAgainst(lock, design, armPlan, registry, replayed, owner, *ownerPackageDigest); validateErr != nil {
			return stressOperationError("validate", validateErr)
		}
		return printStressValidation(*documentType, value.Digest, value.EmpiricalUnits, value.ProviderCalls, value.NetworkRequired)
	default:
		fmt.Fprintln(os.Stderr, "stress validate: --type must be development-case-study, held-out-campaign-plan, or held-out-run-readiness-refusal")
		return 2
	}
}

func printStressValidation(documentType, digest string, empiricalUnits, providerCalls int, networkRequired bool) int {
	return printStressJSON("validate", struct {
		Valid           bool   `json:"valid"`
		Type            string `json:"type"`
		Digest          string `json:"digest"`
		EmpiricalUnits  int    `json:"empirical_units"`
		ProviderCalls   int    `json:"provider_calls"`
		NetworkRequired bool   `json:"network_required"`
	}{true, documentType, digest, empiricalUnits, providerCalls, networkRequired})
}

func runStressSchema(args []string) int {
	flags := flag.NewFlagSet("stress schema", flag.ContinueOnError)
	documentType := flags.String("type", "relation", "stress schema type")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	schema, err := stress.Schema(*documentType)
	if err != nil {
		return stressInputError("schema", err)
	}
	return printStressJSON("schema", schema)
}

func addStressCorpusFlags(flags *flag.FlagSet) stressCorpusFlags {
	return stressCorpusFlags{
		planPath:    flags.String("plan", defaultStressPlanPath, "frozen v3 corpus development plan (@file)"),
		auditPath:   flags.String("audit", defaultStressAuditPath, "frozen v3 natural corpus audit (@file)"),
		releasePath: flags.String("release", defaultStressReleasePath, "frozen v3 controlled-corruption release (@file)"),
	}
}

func loadStressCorpus(paths stressCorpusFlags) (mutation.CorpusDevelopmentPlan, mutation.CorpusDevelopmentAuditV3, mutation.CorpusReleaseV3, error) {
	return loadRelationCorpusV3(*paths.planPath, *paths.auditPath, *paths.releasePath)
}

func loadStressReplay(repositoryRoot string, paths stressCorpusFlags) (stress.RelationRegistry, []stress.ReplayedRelationCaseV3, error) {
	plan, audit, release, err := loadStressCorpus(paths)
	if err != nil {
		return stress.RelationRegistry{}, nil, err
	}
	registry, err := stress.BuildV3RelationRegistry(plan, audit, release)
	if err != nil {
		return stress.RelationRegistry{}, nil, err
	}
	replayed, err := stress.ReplayV3RelationCorpus(repositoryRoot, plan, audit, release, registry)
	if err != nil {
		return stress.RelationRegistry{}, nil, err
	}
	return registry, replayed, nil
}

func printStressJSON(scope string, value any) int {
	encoded, err := stress.EncodeIndented(value)
	if err != nil {
		return stressOperationError(scope, err)
	}
	return writeCommandOutput("stress "+scope, encoded)
}

func stressInputError(scope string, err error) int {
	fmt.Fprintf(os.Stderr, "stress %s: %v\n", scope, err)
	return 2
}

func stressOperationError(scope string, err error) int {
	fmt.Fprintf(os.Stderr, "stress %s: %v\n", scope, err)
	return 1
}
