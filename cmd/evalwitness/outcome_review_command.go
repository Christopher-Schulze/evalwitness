package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Christopher-Schulze/evalwitness/internal/outcome"
)

func runOutcomeReview(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "outcome review: usage: evalwitness outcome review <handbook|bundle|pilot-readiness|reviewer|assign-primary|kit|verify-kit|render-kit|label-batch|analyze-rubric|blinding-protocol|blinding-probe-batch|assign-tie|reveal|analyze-blinding|adjudicate|analyze-sources>")
		return 2
	}
	switch args[0] {
	case "handbook":
		return runOutcomeReviewHandbook(args[1:])
	case "bundle":
		return runOutcomeReviewBundle(args[1:])
	case "pilot-readiness":
		return runOutcomePilotReadiness(args[1:])
	case "reviewer":
		return runOutcomeReviewer(args[1:])
	case "assign-primary":
		return runOutcomeAssignPrimary(args[1:])
	case "kit":
		return runOutcomeReviewKit(args[1:])
	case "verify-kit":
		return runOutcomeVerifyReviewKit(args[1:])
	case "render-kit":
		return runOutcomeRenderReviewKit(args[1:])
	case "label-batch":
		return runOutcomeLabelBatch(args[1:])
	case "analyze-rubric":
		return runOutcomeAnalyzeRubric(args[1:])
	case "blinding-protocol":
		return runOutcomeBlindingProtocol(args[1:])
	case "blinding-probe-batch":
		return runOutcomeBlindingProbeBatch(args[1:])
	case "assign-tie":
		return runOutcomeAssignTie(args[1:])
	case "reveal":
		return runOutcomeMappingReveal(args[1:])
	case "analyze-blinding":
		return runOutcomeAnalyzeBlinding(args[1:])
	case "adjudicate":
		return runOutcomeAdjudicate(args[1:])
	case "analyze-sources":
		return runOutcomeAnalyzeSources(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "outcome review: unknown command %q\n", args[0])
		return 2
	}
}

func runOutcomeReviewHandbook(args []string) int {
	flags := flag.NewFlagSet("outcome review handbook", flag.ContinueOnError)
	handbookPath := flags.String("handbook", "", "optional sealed reviewer handbook (@file)")
	qualificationPath := flags.String("qualification-set", "", "optional governed qualification set used to verify the handbook (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	qualification, err := reviewerHandbookQualification(*qualificationPath)
	if err != nil {
		return outcomeReviewInputError("handbook", err)
	}
	if *handbookPath == "" {
		handbook, buildErr := outcome.DefaultReviewerHandbook(qualification)
		if buildErr != nil {
			return outcomeReviewOperationError("handbook", buildErr)
		}
		return printOutcomeJSON(handbook)
	}
	handbook, err := readOutcomeDocument(*handbookPath, outcome.DecodeReviewerHandbook)
	if err != nil {
		return outcomeReviewInputError("handbook", err)
	}
	if err := outcome.VerifyReviewerHandbook(handbook, qualification); err != nil {
		return outcomeReviewOperationError("handbook", err)
	}
	return printOutcomeJSON(handbook)
}

func runOutcomeReviewBundle(args []string) int {
	flags := flag.NewFlagSet("outcome review bundle", flag.ContinueOnError)
	planPath := flags.String("plan", "", "governed outcome plan (@file)")
	qualificationPath := flags.String("qualification-set", "", "governed qualification set (@file)")
	itemsPath := flags.String("items", "", "blind review item array (@file)")
	handbookPath := flags.String("handbook", "", "sealed reviewer handbook (@file)")
	dataRole := flags.String("data-role", "", "development, calibration, or test")
	visibility := flags.String("visibility", "", "public or restricted")
	createdAt := flags.String("created-at", "", "RFC3339 bundle creation time")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, err := readOutcomeDocument(*planPath, outcome.DecodePlan)
	if err != nil {
		return outcomeReviewInputError("bundle", err)
	}
	qualification, err := readOutcomeDocument(*qualificationPath, outcome.DecodeQualificationSet)
	if err != nil {
		return outcomeReviewInputError("bundle", err)
	}
	handbook, err := readOutcomeDocument(*handbookPath, outcome.DecodeReviewerHandbook)
	if err != nil {
		return outcomeReviewInputError("bundle", err)
	}
	items, err := readOutcomeDocument(*itemsPath, outcome.DecodeReviewItems)
	if err != nil {
		return outcomeReviewInputError("bundle", err)
	}
	bundle, err := outcome.BuildReviewBundle(
		plan, qualification, handbook, items, outcome.ReviewDataRole(*dataRole), outcome.ReviewVisibility(*visibility), *createdAt,
	)
	if err != nil {
		return outcomeReviewOperationError("bundle", err)
	}
	return printOutcomeJSON(bundle)
}

func runOutcomePilotReadiness(args []string) int {
	flags := flag.NewFlagSet("outcome review pilot-readiness", flag.ContinueOnError)
	pilotPath := flags.String("pilot-sample", "", "sealed pilot sample commitment (@file)")
	bundlePath := flags.String("bundle", "", "sealed development pilot review bundle (@file)")
	qualificationPath := flags.String("qualification-set", "", "governed qualification set (@file)")
	handbookPath := flags.String("handbook", "", "sealed reviewer handbook (@file)")
	protocolPath := flags.String("protocol", "", "sealed semantic-blinding protocol (@file)")
	inspectionPath := flags.String("inspection", "", "sealed structural reviewability inspection (@file)")
	mappingsPath := flags.String("mappings", "", "complete owner-only private-mapping array (@file)")
	privateMaterialsPath := flags.String("private-materials", "", "sealed owner-only pilot private-materials artifact (@file)")
	preparedAt := flags.String("prepared-at", "", "RFC3339 readiness time after bundle and protocol freeze")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	pilot, err := readOutcomeDocument(*pilotPath, outcome.DecodeOutcomePilotSampleCommitment)
	if err != nil {
		return outcomeReviewInputError("pilot-readiness", err)
	}
	bundle, err := readOutcomeDocument(*bundlePath, outcome.DecodeReviewBundle)
	if err != nil {
		return outcomeReviewInputError("pilot-readiness", err)
	}
	qualification, err := readOutcomeDocument(*qualificationPath, outcome.DecodeQualificationSet)
	if err != nil {
		return outcomeReviewInputError("pilot-readiness", err)
	}
	handbook, err := readOutcomeDocument(*handbookPath, outcome.DecodeReviewerHandbook)
	if err != nil {
		return outcomeReviewInputError("pilot-readiness", err)
	}
	protocol, err := readOutcomeDocument(*protocolPath, outcome.DecodeBlindingProtocol)
	if err != nil {
		return outcomeReviewInputError("pilot-readiness", err)
	}
	inspection, err := readOutcomeDocument(*inspectionPath, outcome.DecodeOutcomePilotInspection)
	if err != nil {
		return outcomeReviewInputError("pilot-readiness", err)
	}
	mappings, err := readOutcomePrivateMappings(*mappingsPath, *privateMaterialsPath)
	if err != nil {
		return outcomeReviewInputError("pilot-readiness", err)
	}
	readiness, err := outcome.BuildOutcomePilotReadiness(pilot, bundle, qualification, handbook, protocol, inspection, mappings, *preparedAt)
	if err != nil {
		return outcomeReviewOperationError("pilot-readiness", err)
	}
	return printOutcomeJSON(readiness)
}

func runOutcomeReviewer(args []string) int {
	flags := flag.NewFlagSet("outcome review reviewer", flag.ContinueOnError)
	alias := flags.String("alias", "", "pseudonymous reviewer alias")
	role := flags.String("role", "", "primary or tie_break")
	consentedAt := flags.String("consented-at", "", "RFC3339 consent time")
	independent := flags.Bool("independence-attested", false, "reviewer attests independence")
	authorship := flags.Bool("authorship-policy-accepted", false, "reviewer accepts the authorship policy")
	privateContact := flags.Bool("contact-held-privately", false, "contact details are held outside public artifacts")
	var conflicts repeatedStringFlag
	flags.Var(&conflicts, "conflict", "declared conflict of interest; repeat as needed")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	reviewer, err := outcome.NewReviewerRecord(
		*alias, outcome.ReviewerRole(*role), *consentedAt, *independent, *authorship, *privateContact, conflicts,
	)
	if err != nil {
		return outcomeReviewOperationError("reviewer", err)
	}
	return printOutcomeJSON(reviewer)
}

func runOutcomeAssignPrimary(args []string) int {
	flags := flag.NewFlagSet("outcome review assign-primary", flag.ContinueOnError)
	bundlePath := flags.String("bundle", "", "sealed review bundle (@file)")
	reviewerPath := flags.String("reviewer", "", "sealed reviewer record (@file)")
	qualificationPath := flags.String("qualification", "", "reviewer-specific passing qualification report (@file)")
	slot := flags.Int("slot", 0, "primary reviewer slot one or two")
	seedPath := flags.String("seed-file", "", "owner-only file containing a 32-byte lowercase hex ordering seed")
	assignedAt := flags.String("assigned-at", "", "RFC3339 assignment time")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	bundle, reviewer, qualification, seed, err := readReviewAssignmentInputs(*bundlePath, *reviewerPath, *qualificationPath, *seedPath)
	if err != nil {
		return outcomeReviewInputError("assign-primary", err)
	}
	defer zeroBytes(seed)
	assignment, err := outcome.BuildPrimaryAssignment(bundle, reviewer, qualification, *slot, seed, *assignedAt)
	if err != nil {
		return outcomeReviewOperationError("assign-primary", err)
	}
	return printOutcomeJSON(assignment)
}

func runOutcomeReviewKit(args []string) int {
	flags := flag.NewFlagSet("outcome review kit", flag.ContinueOnError)
	bundlePath := flags.String("bundle", "", "sealed review bundle (@file)")
	assignmentPath := flags.String("assignment", "", "sealed reviewer assignment (@file)")
	handbookPath := flags.String("handbook", "", "sealed reviewer handbook (@file)")
	generatedAt := flags.String("generated-at", "", "RFC3339 kit generation time")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	bundle, err := readOutcomeDocument(*bundlePath, outcome.DecodeReviewBundle)
	if err != nil {
		return outcomeReviewInputError("kit", err)
	}
	assignment, err := readOutcomeDocument(*assignmentPath, outcome.DecodeReviewAssignment)
	if err != nil {
		return outcomeReviewInputError("kit", err)
	}
	handbook, err := readOutcomeDocument(*handbookPath, outcome.DecodeReviewerHandbook)
	if err != nil {
		return outcomeReviewInputError("kit", err)
	}
	kit, err := outcome.BuildReviewerKit(bundle, assignment, handbook, *generatedAt)
	if err != nil {
		return outcomeReviewOperationError("kit", err)
	}
	return printOutcomeJSON(kit)
}

func runOutcomeVerifyReviewKit(args []string) int {
	flags := flag.NewFlagSet("outcome review verify-kit", flag.ContinueOnError)
	kitPath := flags.String("kit", "", "sealed reviewer kit (@file)")
	bundlePath := flags.String("bundle", "", "sealed review bundle (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	kit, err := readOutcomeDocument(*kitPath, outcome.DecodeReviewerKit)
	if err != nil {
		return outcomeReviewInputError("verify-kit", err)
	}
	bundle, err := readOutcomeDocument(*bundlePath, outcome.DecodeReviewBundle)
	if err != nil {
		return outcomeReviewInputError("verify-kit", err)
	}
	if err := outcome.VerifyReviewerKit(kit, bundle); err != nil {
		return outcomeReviewOperationError("verify-kit", err)
	}
	return printOutcomeJSON(struct {
		Valid  bool   `json:"valid"`
		Digest string `json:"digest"`
	}{Valid: true, Digest: kit.Digest})
}

func runOutcomeRenderReviewKit(args []string) int {
	flags := flag.NewFlagSet("outcome review render-kit", flag.ContinueOnError)
	kitPath := flags.String("kit", "", "sealed reviewer kit (@file)")
	bundlePath := flags.String("bundle", "", "sealed review bundle used to verify the kit (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	kit, err := readOutcomeDocument(*kitPath, outcome.DecodeReviewerKit)
	if err != nil {
		return outcomeReviewInputError("render-kit", err)
	}
	bundle, err := readOutcomeDocument(*bundlePath, outcome.DecodeReviewBundle)
	if err != nil {
		return outcomeReviewInputError("render-kit", err)
	}
	if err := outcome.VerifyReviewerKit(kit, bundle); err != nil {
		return outcomeReviewOperationError("render-kit", err)
	}
	rendered, err := outcome.RenderReviewerKitMarkdown(kit)
	if err != nil {
		return outcomeReviewOperationError("render-kit", err)
	}
	return writeCommandOutput("outcome review render-kit", []byte(rendered))
}

func runOutcomeLabelBatch(args []string) int {
	flags := flag.NewFlagSet("outcome review label-batch", flag.ContinueOnError)
	assignmentPath := flags.String("assignment", "", "sealed review assignment (@file)")
	labelsPath := flags.String("labels", "", "complete sealed outcome-label array (@file)")
	committedAt := flags.String("committed-at", "", "RFC3339 commitment time")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	assignment, err := readOutcomeDocument(*assignmentPath, outcome.DecodeReviewAssignment)
	if err != nil {
		return outcomeReviewInputError("label-batch", err)
	}
	labels, err := readOutcomeDocument(*labelsPath, outcome.DecodeLabels)
	if err != nil {
		return outcomeReviewInputError("label-batch", err)
	}
	batch, err := outcome.BuildLabelBatch(assignment, labels, *committedAt)
	if err != nil {
		return outcomeReviewOperationError("label-batch", err)
	}
	return printOutcomeJSON(batch)
}

func runOutcomeAnalyzeRubric(args []string) int {
	flags := flag.NewFlagSet("outcome review analyze-rubric", flag.ContinueOnError)
	bundlePath := flags.String("bundle", "", "sealed review bundle (@file)")
	leftAssignmentPath := flags.String("left-assignment", "", "first primary assignment (@file)")
	leftBatchPath := flags.String("left-batch", "", "first primary label commitment (@file)")
	rightAssignmentPath := flags.String("right-assignment", "", "second primary assignment (@file)")
	rightBatchPath := flags.String("right-batch", "", "second primary label commitment (@file)")
	analyzedAt := flags.String("analyzed-at", "", "RFC3339 prereveal analysis time after both primary commitments")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	bundle, err := readOutcomeDocument(*bundlePath, outcome.DecodeReviewBundle)
	if err != nil {
		return outcomeReviewInputError("analyze-rubric", err)
	}
	leftAssignment, leftBatch, rightAssignment, rightBatch, err := readPrimaryReviewInputs(
		*leftAssignmentPath, *leftBatchPath, *rightAssignmentPath, *rightBatchPath,
	)
	if err != nil {
		return outcomeReviewInputError("analyze-rubric", err)
	}
	analysis, err := outcome.BuildRubricAmbiguityAnalysis(bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, *analyzedAt)
	if err != nil {
		return outcomeReviewOperationError("analyze-rubric", err)
	}
	return printOutcomeJSON(analysis)
}

func runOutcomeBlindingProtocol(args []string) int {
	flags := flag.NewFlagSet("outcome review blinding-protocol", flag.ContinueOnError)
	bundlePath := flags.String("bundle", "", "sealed review bundle (@file)")
	mappingsPath := flags.String("mappings", "", "complete owner-only private-mapping array (@file)")
	privateMaterialsPath := flags.String("private-materials", "", "sealed owner-only pilot private-materials artifact (@file)")
	createdAt := flags.String("created-at", "", "RFC3339 protocol freeze time before assignment")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	bundle, err := readOutcomeDocument(*bundlePath, outcome.DecodeReviewBundle)
	if err != nil {
		return outcomeReviewInputError("blinding-protocol", err)
	}
	mappings, err := readOutcomePrivateMappings(*mappingsPath, *privateMaterialsPath)
	if err != nil {
		return outcomeReviewInputError("blinding-protocol", err)
	}
	protocol, err := outcome.BuildBlindingProtocol(bundle, mappings, *createdAt)
	if err != nil {
		return outcomeReviewOperationError("blinding-protocol", err)
	}
	return printOutcomeJSON(protocol)
}

func runOutcomeBlindingProbeBatch(args []string) int {
	flags := flag.NewFlagSet("outcome review blinding-probe-batch", flag.ContinueOnError)
	protocolPath := flags.String("protocol", "", "sealed post-label blinding protocol (@file)")
	assignmentPath := flags.String("assignment", "", "sealed primary review assignment (@file)")
	labelBatchPath := flags.String("label-batch", "", "committed primary label batch (@file)")
	draftsPath := flags.String("drafts", "", "complete blinding-probe draft array (@file)")
	committedAt := flags.String("committed-at", "", "RFC3339 probe commitment time before reveal")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	protocol, err := readOutcomeDocument(*protocolPath, outcome.DecodeBlindingProtocol)
	if err != nil {
		return outcomeReviewInputError("blinding-probe-batch", err)
	}
	assignment, err := readOutcomeDocument(*assignmentPath, outcome.DecodeReviewAssignment)
	if err != nil {
		return outcomeReviewInputError("blinding-probe-batch", err)
	}
	labels, err := readOutcomeDocument(*labelBatchPath, outcome.DecodeLabelBatch)
	if err != nil {
		return outcomeReviewInputError("blinding-probe-batch", err)
	}
	drafts, err := readOutcomeDocument(*draftsPath, outcome.DecodeBlindingProbeDrafts)
	if err != nil {
		return outcomeReviewInputError("blinding-probe-batch", err)
	}
	batch, err := outcome.BuildBlindingProbeBatch(protocol, assignment, labels, drafts, *committedAt)
	if err != nil {
		return outcomeReviewOperationError("blinding-probe-batch", err)
	}
	return printOutcomeJSON(batch)
}

func runOutcomeAssignTie(args []string) int {
	flags := flag.NewFlagSet("outcome review assign-tie", flag.ContinueOnError)
	bundlePath := flags.String("bundle", "", "sealed review bundle (@file)")
	reviewerPath := flags.String("reviewer", "", "sealed tie-break reviewer record (@file)")
	qualificationPath := flags.String("qualification", "", "tie-break reviewer passing qualification report (@file)")
	leftAssignmentPath := flags.String("left-assignment", "", "first primary assignment (@file)")
	leftBatchPath := flags.String("left-batch", "", "first primary label commitment (@file)")
	rightAssignmentPath := flags.String("right-assignment", "", "second primary assignment (@file)")
	rightBatchPath := flags.String("right-batch", "", "second primary label commitment (@file)")
	seedPath := flags.String("seed-file", "", "owner-only file containing a 32-byte lowercase hex ordering seed")
	assignedAt := flags.String("assigned-at", "", "RFC3339 tie-break assignment time")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	bundle, reviewer, qualification, seed, err := readReviewAssignmentInputs(*bundlePath, *reviewerPath, *qualificationPath, *seedPath)
	if err != nil {
		return outcomeReviewInputError("assign-tie", err)
	}
	defer zeroBytes(seed)
	leftAssignment, leftBatch, rightAssignment, rightBatch, err := readPrimaryReviewInputs(
		*leftAssignmentPath, *leftBatchPath, *rightAssignmentPath, *rightBatchPath,
	)
	if err != nil {
		return outcomeReviewInputError("assign-tie", err)
	}
	assignment, err := outcome.BuildTieBreakAssignment(
		bundle, reviewer, qualification, leftAssignment, leftBatch, rightAssignment, rightBatch, seed, *assignedAt,
	)
	if err != nil {
		return outcomeReviewOperationError("assign-tie", err)
	}
	return printOutcomeJSON(assignment)
}

func runOutcomeMappingReveal(args []string) int {
	flags := flag.NewFlagSet("outcome review reveal", flag.ContinueOnError)
	bundlePath := flags.String("bundle", "", "sealed review bundle (@file)")
	leftAssignmentPath := flags.String("left-assignment", "", "first primary assignment (@file)")
	leftBatchPath := flags.String("left-batch", "", "first primary label commitment (@file)")
	rightAssignmentPath := flags.String("right-assignment", "", "second primary assignment (@file)")
	rightBatchPath := flags.String("right-batch", "", "second primary label commitment (@file)")
	tieAssignmentPath := flags.String("tie-assignment", "", "optional tie-break assignment (@file)")
	tieBatchPath := flags.String("tie-batch", "", "optional tie-break label commitment (@file)")
	leftSeedPath := flags.String("left-seed-file", "", "owner-only first-primary ordering seed file")
	rightSeedPath := flags.String("right-seed-file", "", "owner-only second-primary ordering seed file")
	tieSeedPath := flags.String("tie-seed-file", "", "optional owner-only tie-break ordering seed file")
	mappingsPath := flags.String("mappings", "", "complete private-mapping array (@file)")
	privateMaterialsPath := flags.String("private-materials", "", "sealed owner-only pilot private-materials artifact (@file)")
	revealedAt := flags.String("revealed-at", "", "RFC3339 reveal time")
	revealedBy := flags.String("revealed-by", "", "pseudonymous reveal actor")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	bundle, err := readOutcomeDocument(*bundlePath, outcome.DecodeReviewBundle)
	if err != nil {
		return outcomeReviewInputError("reveal", err)
	}
	leftAssignment, leftBatch, rightAssignment, rightBatch, err := readPrimaryReviewInputs(
		*leftAssignmentPath, *leftBatchPath, *rightAssignmentPath, *rightBatchPath,
	)
	if err != nil {
		return outcomeReviewInputError("reveal", err)
	}
	tieAssignment, tieBatch, err := readOptionalTieReviewInputs(*tieAssignmentPath, *tieBatchPath)
	if err != nil {
		return outcomeReviewInputError("reveal", err)
	}
	seeds, err := readReviewRevealSeeds(leftAssignment, rightAssignment, tieAssignment, *leftSeedPath, *rightSeedPath, *tieSeedPath)
	if err != nil {
		return outcomeReviewInputError("reveal", err)
	}
	defer zeroAssignmentSeeds(seeds)
	mappings, err := readOutcomePrivateMappings(*mappingsPath, *privateMaterialsPath)
	if err != nil {
		return outcomeReviewInputError("reveal", err)
	}
	reveal, err := outcome.BuildMappingReveal(
		bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, tieAssignment, tieBatch, mappings, seeds, *revealedAt, *revealedBy,
	)
	if err != nil {
		return outcomeReviewOperationError("reveal", err)
	}
	return printOutcomeJSON(reveal)
}

func runOutcomeAnalyzeBlinding(args []string) int {
	flags := flag.NewFlagSet("outcome review analyze-blinding", flag.ContinueOnError)
	bundlePath := flags.String("bundle", "", "sealed review bundle (@file)")
	revealPath := flags.String("reveal", "", "sealed post-commit mapping reveal (@file)")
	mappingsPath := flags.String("mappings", "", "complete owner-only private-mapping array (@file)")
	privateMaterialsPath := flags.String("private-materials", "", "sealed owner-only pilot private-materials artifact (@file)")
	leftPath := flags.String("left-probes", "", "slot-one committed blinding-probe batch (@file)")
	rightPath := flags.String("right-probes", "", "slot-two committed blinding-probe batch (@file)")
	analyzedAt := flags.String("analyzed-at", "", "RFC3339 analysis time")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	bundle, err := readOutcomeDocument(*bundlePath, outcome.DecodeReviewBundle)
	if err != nil {
		return outcomeReviewInputError("analyze-blinding", err)
	}
	reveal, err := readOutcomeDocument(*revealPath, outcome.DecodeMappingReveal)
	if err != nil {
		return outcomeReviewInputError("analyze-blinding", err)
	}
	mappings, err := readOutcomePrivateMappings(*mappingsPath, *privateMaterialsPath)
	if err != nil {
		return outcomeReviewInputError("analyze-blinding", err)
	}
	left, err := readOutcomeDocument(*leftPath, outcome.DecodeBlindingProbeBatch)
	if err != nil {
		return outcomeReviewInputError("analyze-blinding", err)
	}
	right, err := readOutcomeDocument(*rightPath, outcome.DecodeBlindingProbeBatch)
	if err != nil {
		return outcomeReviewInputError("analyze-blinding", err)
	}
	analysis, err := outcome.BuildBlindingAnalysis(bundle, reveal, mappings, left, right, *analyzedAt)
	if err != nil {
		return outcomeReviewOperationError("analyze-blinding", err)
	}
	return printOutcomeJSON(analysis)
}

func runOutcomeAdjudicate(args []string) int {
	flags := flag.NewFlagSet("outcome review adjudicate", flag.ContinueOnError)
	bundlePath := flags.String("bundle", "", "sealed review bundle (@file)")
	leftAssignmentPath := flags.String("left-assignment", "", "first primary assignment (@file)")
	leftBatchPath := flags.String("left-batch", "", "first primary label commitment (@file)")
	rightAssignmentPath := flags.String("right-assignment", "", "second primary assignment (@file)")
	rightBatchPath := flags.String("right-batch", "", "second primary label commitment (@file)")
	tieAssignmentPath := flags.String("tie-assignment", "", "optional tie-break assignment (@file)")
	tieBatchPath := flags.String("tie-batch", "", "optional tie-break label commitment (@file)")
	revealPath := flags.String("reveal", "", "sealed mapping reveal (@file)")
	rubricPath := flags.String("rubric-analysis", "", "sealed prereveal rubric ambiguity analysis (@file)")
	blindingPath := flags.String("blinding-analysis", "", "sealed post-reveal semantic blinding analysis (@file)")
	completedAt := flags.String("completed-at", "", "RFC3339 adjudication completion time")
	rule := flags.String("rule", "", "frozen conflict-resolution rule")
	iterations := flags.Int("bootstrap-iterations", 10_000, "task-cluster bootstrap iterations")
	seed := flags.String("bootstrap-seed", "", "frozen bootstrap seed")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	bundle, err := readOutcomeDocument(*bundlePath, outcome.DecodeReviewBundle)
	if err != nil {
		return outcomeReviewInputError("adjudicate", err)
	}
	leftAssignment, leftBatch, rightAssignment, rightBatch, err := readPrimaryReviewInputs(
		*leftAssignmentPath, *leftBatchPath, *rightAssignmentPath, *rightBatchPath,
	)
	if err != nil {
		return outcomeReviewInputError("adjudicate", err)
	}
	tieAssignment, tieBatch, err := readOptionalTieReviewInputs(*tieAssignmentPath, *tieBatchPath)
	if err != nil {
		return outcomeReviewInputError("adjudicate", err)
	}
	reveal, err := readOutcomeDocument(*revealPath, outcome.DecodeMappingReveal)
	if err != nil {
		return outcomeReviewInputError("adjudicate", err)
	}
	rubricAmbiguity, err := readOutcomeDocument(*rubricPath, outcome.DecodeRubricAmbiguityAnalysis)
	if err != nil {
		return outcomeReviewInputError("adjudicate", err)
	}
	blinding, err := readOutcomeDocument(*blindingPath, outcome.DecodeBlindingAnalysis)
	if err != nil {
		return outcomeReviewInputError("adjudicate", err)
	}
	result, err := outcome.BuildAdjudicationLedger(
		bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, tieAssignment, tieBatch, reveal,
		rubricAmbiguity, blinding, *completedAt, *rule, *iterations, *seed,
	)
	if err != nil {
		return outcomeReviewOperationError("adjudicate", err)
	}
	return printOutcomeJSON(result)
}

func runOutcomeAnalyzeSources(args []string) int {
	flags := flag.NewFlagSet("outcome review analyze-sources", flag.ContinueOnError)
	bundlePath := flags.String("bundle", "", "sealed review bundle (@file)")
	revealPath := flags.String("reveal", "", "sealed post-commit mapping reveal (@file)")
	ledgerPath := flags.String("ledger", "", "terminal adjudication ledger (@file)")
	mappingsPath := flags.String("mappings", "", "complete owner-only private-mapping array (@file)")
	privateMaterialsPath := flags.String("private-materials", "", "sealed owner-only pilot private-materials artifact (@file)")
	recordsPath := flags.String("records", "", "sealed pre-human outcome-record array (@file)")
	resolutionsPath := flags.String("resolutions", "", "terminal human-resolution array (@file)")
	analyzedAt := flags.String("analyzed-at", "", "RFC3339 analysis time strictly after ledger completion")
	iterations := flags.Int("bootstrap-iterations", 10_000, "task-cluster bootstrap iterations")
	seed := flags.String("bootstrap-seed", "", "frozen bootstrap seed")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	bundle, err := readOutcomeDocument(*bundlePath, outcome.DecodeReviewBundle)
	if err != nil {
		return outcomeReviewInputError("analyze-sources", err)
	}
	reveal, err := readOutcomeDocument(*revealPath, outcome.DecodeMappingReveal)
	if err != nil {
		return outcomeReviewInputError("analyze-sources", err)
	}
	ledger, err := readOutcomeDocument(*ledgerPath, outcome.DecodeAdjudicationLedger)
	if err != nil {
		return outcomeReviewInputError("analyze-sources", err)
	}
	mappings, err := readOutcomePrivateMappings(*mappingsPath, *privateMaterialsPath)
	if err != nil {
		return outcomeReviewInputError("analyze-sources", err)
	}
	records, err := readOutcomeDocument(*recordsPath, outcome.DecodeRecords)
	if err != nil {
		return outcomeReviewInputError("analyze-sources", err)
	}
	resolutions, err := readOutcomeDocument(*resolutionsPath, outcome.DecodeResolutions)
	if err != nil {
		return outcomeReviewInputError("analyze-sources", err)
	}
	audit, err := outcome.BuildOutcomeSourceAudit(
		bundle, reveal, ledger, mappings, records, resolutions, *analyzedAt, *iterations, *seed,
	)
	if err != nil {
		return outcomeReviewOperationError("analyze-sources", err)
	}
	return printOutcomeJSON(audit)
}

func readReviewAssignmentInputs(bundlePath, reviewerPath, qualificationPath, seedPath string) (outcome.ReviewBundle, outcome.ReviewerRecord, outcome.QualificationReport, []byte, error) {
	bundle, err := readOutcomeDocument(bundlePath, outcome.DecodeReviewBundle)
	if err != nil {
		return outcome.ReviewBundle{}, outcome.ReviewerRecord{}, outcome.QualificationReport{}, nil, err
	}
	reviewer, err := readOutcomeDocument(reviewerPath, outcome.DecodeReviewerRecord)
	if err != nil {
		return outcome.ReviewBundle{}, outcome.ReviewerRecord{}, outcome.QualificationReport{}, nil, err
	}
	qualification, err := readOutcomeDocument(qualificationPath, outcome.DecodeQualificationReport)
	if err != nil {
		return outcome.ReviewBundle{}, outcome.ReviewerRecord{}, outcome.QualificationReport{}, nil, err
	}
	seed, err := readBlindingKeyFile(seedPath)
	if err != nil {
		return outcome.ReviewBundle{}, outcome.ReviewerRecord{}, outcome.QualificationReport{}, nil, err
	}
	return bundle, reviewer, qualification, seed, nil
}

func reviewerHandbookQualification(path string) (outcome.QualificationSet, error) {
	if path == "" {
		return outcome.DefaultQualificationSet()
	}
	return readOutcomeDocument(path, outcome.DecodeQualificationSet)
}

func readPrimaryReviewInputs(leftAssignmentPath, leftBatchPath, rightAssignmentPath, rightBatchPath string) (outcome.ReviewAssignment, outcome.LabelBatch, outcome.ReviewAssignment, outcome.LabelBatch, error) {
	leftAssignment, err := readOutcomeDocument(leftAssignmentPath, outcome.DecodeReviewAssignment)
	if err != nil {
		return outcome.ReviewAssignment{}, outcome.LabelBatch{}, outcome.ReviewAssignment{}, outcome.LabelBatch{}, err
	}
	leftBatch, err := readOutcomeDocument(leftBatchPath, outcome.DecodeLabelBatch)
	if err != nil {
		return outcome.ReviewAssignment{}, outcome.LabelBatch{}, outcome.ReviewAssignment{}, outcome.LabelBatch{}, err
	}
	rightAssignment, err := readOutcomeDocument(rightAssignmentPath, outcome.DecodeReviewAssignment)
	if err != nil {
		return outcome.ReviewAssignment{}, outcome.LabelBatch{}, outcome.ReviewAssignment{}, outcome.LabelBatch{}, err
	}
	rightBatch, err := readOutcomeDocument(rightBatchPath, outcome.DecodeLabelBatch)
	if err != nil {
		return outcome.ReviewAssignment{}, outcome.LabelBatch{}, outcome.ReviewAssignment{}, outcome.LabelBatch{}, err
	}
	return leftAssignment, leftBatch, rightAssignment, rightBatch, nil
}

func readOptionalTieReviewInputs(assignmentPath, batchPath string) (*outcome.ReviewAssignment, *outcome.LabelBatch, error) {
	if assignmentPath == "" && batchPath == "" {
		return nil, nil, nil
	}
	if assignmentPath == "" || batchPath == "" {
		return nil, nil, fmt.Errorf("tie assignment and batch paths must be supplied together")
	}
	assignment, err := readOutcomeDocument(assignmentPath, outcome.DecodeReviewAssignment)
	if err != nil {
		return nil, nil, err
	}
	batch, err := readOutcomeDocument(batchPath, outcome.DecodeLabelBatch)
	if err != nil {
		return nil, nil, err
	}
	return &assignment, &batch, nil
}

func readReviewRevealSeeds(left, right outcome.ReviewAssignment, tie *outcome.ReviewAssignment, leftPath, rightPath, tiePath string) ([]outcome.AssignmentSeed, error) {
	leftSeed, err := readBlindingKeyFile(leftPath)
	if err != nil {
		return nil, err
	}
	rightSeed, err := readBlindingKeyFile(rightPath)
	if err != nil {
		zeroBytes(leftSeed)
		return nil, err
	}
	seeds := []outcome.AssignmentSeed{{AssignmentDigest: left.Digest, Seed: leftSeed}, {AssignmentDigest: right.Digest, Seed: rightSeed}}
	if tie == nil {
		if tiePath != "" {
			zeroAssignmentSeeds(seeds)
			return nil, fmt.Errorf("tie ordering seed requires a tie-break assignment")
		}
		return seeds, nil
	}
	tieSeed, err := readBlindingKeyFile(tiePath)
	if err != nil {
		zeroAssignmentSeeds(seeds)
		return nil, err
	}
	return append(seeds, outcome.AssignmentSeed{AssignmentDigest: tie.Digest, Seed: tieSeed}), nil
}

func readOutcomeDocument[T any](path string, decode func(io.Reader) (T, error)) (T, error) {
	reader, closeDocument, err := openStudyDocument(path)
	if err != nil {
		var zero T
		return zero, err
	}
	defer closeDocument()
	return decode(reader)
}

func readOutcomePrivateMappings(mappingsPath, privateMaterialsPath string) ([]outcome.PrivateMapping, error) {
	if (mappingsPath == "") == (privateMaterialsPath == "") {
		return nil, errors.New("exactly one of --mappings or --private-materials is required")
	}
	if mappingsPath != "" {
		return readOutcomeDocument(mappingsPath, outcome.DecodePrivateMappings)
	}
	materials, err := readOutcomeDocument(privateMaterialsPath, outcome.DecodeOutcomePilotPrivateMaterials)
	if err != nil {
		return nil, err
	}
	return materials.Mappings, nil
}

func zeroBytes(values []byte) {
	for index := range values {
		values[index] = 0
	}
}

func zeroAssignmentSeeds(seeds []outcome.AssignmentSeed) {
	for _, seed := range seeds {
		zeroBytes(seed.Seed)
	}
}

func outcomeReviewInputError(command string, err error) int {
	fmt.Fprintf(os.Stderr, "outcome review %s: %v\n", command, err)
	return 2
}

func outcomeReviewOperationError(command string, err error) int {
	fmt.Fprintf(os.Stderr, "outcome review %s: %v\n", command, err)
	return 1
}
