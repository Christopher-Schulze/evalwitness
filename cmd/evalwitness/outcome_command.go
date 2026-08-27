package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/outcome"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

func runOutcome(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "outcome: usage: evalwitness outcome <plan|sample|pilot-sample|pilot-sample-v1|pilot-materials|pilot-inspect|natural-request|natural-inventory|packet|evidence|record|label|qualification|qualify|review|validate|schema|agreement|resolve|preservation>")
		return 2
	}
	switch args[0] {
	case "plan":
		return runOutcomePlan(args[1:])
	case "sample":
		return runOutcomeSample(args[1:])
	case "pilot-sample":
		return runOutcomePilotSample(args[1:])
	case "pilot-sample-v1":
		return runOutcomePilotSampleV1(args[1:])
	case "pilot-materials":
		return runOutcomePilotMaterials(args[1:])
	case "pilot-inspect":
		return runOutcomePilotInspect(args[1:])
	case "natural-inventory":
		return runOutcomeNaturalInventory(args[1:])
	case "natural-request":
		return runOutcomeNaturalRequest(args[1:])
	case "packet":
		return runOutcomePacket(args[1:])
	case "evidence":
		return runOutcomeEvidence(args[1:])
	case "record":
		return runOutcomeRecord(args[1:])
	case "label":
		return runOutcomeLabel(args[1:])
	case "qualification":
		return runOutcomeQualification(args[1:])
	case "qualify":
		return runOutcomeQualify(args[1:])
	case "review":
		return runOutcomeReview(args[1:])
	case "validate":
		return runOutcomeValidate(args[1:])
	case "schema":
		return runOutcomeSchema(args[1:])
	case "agreement":
		return runOutcomeAgreement(args[1:])
	case "resolve":
		return runOutcomeResolve(args[1:])
	case "preservation":
		return runOutcomePreservation(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "outcome: unknown command %q\n", args[0])
		return 2
	}
}

func runOutcomeEvidence(args []string) int {
	flags := flag.NewFlagSet("outcome evidence", flag.ContinueOnError)
	path := flags.String("draft", "", "strict outcome-evidence draft (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	reader, closeDraft, err := openStudyDocument(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome evidence:", err)
		return 2
	}
	defer closeDraft()
	draft, err := outcome.DecodeEvidenceDraft(reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome evidence:", err)
		return 1
	}
	evidence, err := outcome.SealEvidenceDraft(draft)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome evidence:", err)
		return 1
	}
	return printOutcomeJSON(evidence)
}

func runOutcomeRecord(args []string) int {
	flags := flag.NewFlagSet("outcome record", flag.ContinueOnError)
	path := flags.String("draft", "", "strict outcome-record draft (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	reader, closeDraft, err := openStudyDocument(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome record:", err)
		return 2
	}
	defer closeDraft()
	draft, err := outcome.DecodeRecordDraft(reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome record:", err)
		return 1
	}
	record, err := outcome.SealRecordDraft(draft)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome record:", err)
		return 1
	}
	return printOutcomeJSON(record)
}

func runOutcomeLabel(args []string) int {
	flags := flag.NewFlagSet("outcome label", flag.ContinueOnError)
	path := flags.String("draft", "", "strict blinded-label draft (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	reader, closeDraft, err := openStudyDocument(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome label:", err)
		return 2
	}
	defer closeDraft()
	draft, err := outcome.DecodeLabelDraft(reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome label:", err)
		return 1
	}
	label, err := outcome.SealLabelDraft(draft)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome label:", err)
		return 1
	}
	return printOutcomeJSON(label)
}

func runOutcomeQualification(args []string) int {
	flags := flag.NewFlagSet("outcome qualification", flag.ContinueOnError)
	path := flags.String("set", "", "optional governed qualification set (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *path == "" {
		set, err := outcome.DefaultQualificationSet()
		if err != nil {
			fmt.Fprintln(os.Stderr, "outcome qualification:", err)
			return 1
		}
		return printOutcomeJSON(set)
	}
	reader, closeSet, err := openStudyDocument(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome qualification:", err)
		return 2
	}
	defer closeSet()
	set, err := outcome.DecodeQualificationSet(reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome qualification:", err)
		return 1
	}
	return printOutcomeJSON(set)
}

func runOutcomeQualify(args []string) int {
	flags := flag.NewFlagSet("outcome qualify", flag.ContinueOnError)
	setPath := flags.String("set", "", "governed qualification set (@file)")
	labelsPath := flags.String("labels", "", "sealed qualification label array (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	setReader, closeSet, err := openStudyDocument(*setPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome qualify:", err)
		return 2
	}
	defer closeSet()
	set, err := outcome.DecodeQualificationSet(setReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome qualify:", err)
		return 1
	}
	labelsReader, closeLabels, err := openStudyDocument(*labelsPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome qualify:", err)
		return 2
	}
	defer closeLabels()
	labels, err := outcome.DecodeQualificationLabels(labelsReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome qualify:", err)
		return 1
	}
	report, err := outcome.ScoreQualification(set, labels)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome qualify:", err)
		return 1
	}
	return printOutcomeJSON(report)
}

func runOutcomeNaturalRequest(args []string) int {
	flags := flag.NewFlagSet("outcome natural-request", flag.ContinueOnError)
	path := flags.String("request", "", "optional governed natural inventory request (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *path == "" {
		request, err := outcome.DefaultNaturalInventoryRequest()
		if err != nil {
			fmt.Fprintln(os.Stderr, "outcome natural-request:", err)
			return 1
		}
		return printOutcomeJSON(request)
	}
	reader, closeRequest, err := openStudyDocument(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome natural-request:", err)
		return 2
	}
	defer closeRequest()
	request, err := outcome.DecodeNaturalInventoryRequest(reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome natural-request:", err)
		return 1
	}
	return printOutcomeJSON(request)
}

func runOutcomeNaturalInventory(args []string) int {
	flags := flag.NewFlagSet("outcome natural-inventory", flag.ContinueOnError)
	planPath := flags.String("plan", "", "governed outcome plan (@file)")
	requestPath := flags.String("request", "", "sealed natural inventory request (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	planReader, closePlan, err := openStudyDocument(*planPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome natural-inventory:", err)
		return 2
	}
	defer closePlan()
	plan, err := outcome.DecodePlan(planReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome natural-inventory:", err)
		return 1
	}
	requestReader, closeRequest, err := openStudyDocument(*requestPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome natural-inventory:", err)
		return 2
	}
	defer closeRequest()
	request, err := outcome.DecodeNaturalInventoryRequest(requestReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome natural-inventory:", err)
		return 1
	}
	inventory, err := outcome.BuildNaturalInventory(plan, request)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome natural-inventory:", err)
		return 1
	}
	return printOutcomeJSON(inventory)
}

func runOutcomePacket(args []string) int {
	flags := flag.NewFlagSet("outcome packet", flag.ContinueOnError)
	requestPath := flags.String("request", "", "private blind-build request (@file)")
	keyPath := flags.String("key-file", "", "owner-only file containing a 32-byte lowercase hex key")
	privateRootPath := flags.String("private-root", "", "owner-only mapping vault outside the repository root")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	requestReader, closeRequest, err := openStudyDocument(*requestPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome packet:", err)
		return 2
	}
	defer closeRequest()
	request, err := outcome.DecodeBlindBuildRequest(requestReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome packet:", err)
		return 1
	}
	key, err := readBlindingKeyFile(*keyPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome packet:", err)
		return 2
	}
	packet, mapping, err := outcome.BuildBlindedPacketFromRequest(request, key)
	for index := range key {
		key[index] = 0
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome packet:", err)
		return 1
	}
	encodedMapping, err := outcome.EncodeIndented(mapping)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome packet: encode private mapping:", err)
		return 1
	}
	policy, err := safety.CurrentPathPolicy()
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome packet:", err)
		return 1
	}
	privateRoot, err := safety.CreateCacheRoot(policy, strings.TrimSpace(*privateRootPath))
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome packet:", err)
		return 1
	}
	privatePath := filepath.Join("mappings", packet.PacketID+".json")
	if err := privateRoot.PublishSensitiveExclusive(privatePath, encodedMapping); err != nil {
		fmt.Fprintln(os.Stderr, "outcome packet: publish private mapping:", err)
		return 1
	}
	return printOutcomeJSON(packet)
}

func runOutcomeSample(args []string) int {
	flags := flag.NewFlagSet("outcome sample", flag.ContinueOnError)
	planPath := flags.String("plan", "", "governed outcome plan (@file)")
	releasePath := flags.String("release", "", "controlled-corruption release (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	planReader, closePlan, err := openStudyDocument(*planPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome sample:", err)
		return 2
	}
	defer closePlan()
	plan, err := outcome.DecodePlan(planReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome sample:", err)
		return 2
	}
	releaseReader, closeRelease, err := openStudyDocument(*releasePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome sample:", err)
		return 2
	}
	defer closeRelease()
	release, err := mutation.DecodeCorpusRelease(releaseReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome sample:", err)
		return 2
	}
	commitment, err := outcome.BuildMutationSampleCommitment(plan, release)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome sample:", err)
		return 1
	}
	return printOutcomeJSON(commitment)
}

func runOutcomePilotSample(args []string) int {
	flags := flag.NewFlagSet("outcome pilot-sample", flag.ContinueOnError)
	planPath := flags.String("plan", "", "governed outcome plan (@file)")
	inventoryPath := flags.String("inventory", "", "governed natural inventory (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, err := readOutcomeDocument(*planPath, outcome.DecodePlan)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome pilot-sample:", err)
		return 2
	}
	inventory, err := readOutcomeDocument(*inventoryPath, outcome.DecodeNaturalInventory)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome pilot-sample:", err)
		return 2
	}
	commitment, err := outcome.BuildOutcomePilotSampleCommitment(plan, inventory)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome pilot-sample:", err)
		return 1
	}
	return printOutcomeJSON(commitment)
}

func runOutcomePilotSampleV1(args []string) int {
	flags := flag.NewFlagSet("outcome pilot-sample-v1", flag.ContinueOnError)
	planPath := flags.String("plan", "", "governed outcome plan (@file)")
	samplePath := flags.String("sample", "", "governed mutation sample commitment (@file)")
	releasePath := flags.String("release", "", "controlled-corruption release (@file)")
	inventoryPath := flags.String("inventory", "", "governed natural inventory (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, err := readOutcomeDocument(*planPath, outcome.DecodePlan)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome pilot-sample-v1:", err)
		return 2
	}
	sample, err := readOutcomeDocument(*samplePath, outcome.DecodeSampleCommitment)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome pilot-sample-v1:", err)
		return 2
	}
	releaseReader, closeRelease, err := openStudyDocument(*releasePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome pilot-sample-v1:", err)
		return 2
	}
	defer closeRelease()
	release, err := mutation.DecodeCorpusRelease(releaseReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome pilot-sample-v1:", err)
		return 2
	}
	inventory, err := readOutcomeDocument(*inventoryPath, outcome.DecodeNaturalInventory)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome pilot-sample-v1:", err)
		return 2
	}
	commitment, err := outcome.BuildPilotSampleCommitment(plan, sample, release, inventory)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome pilot-sample-v1:", err)
		return 1
	}
	return printOutcomeJSON(commitment)
}

func runOutcomePilotMaterials(args []string) int {
	flags := flag.NewFlagSet("outcome pilot-materials", flag.ContinueOnError)
	rootPath := flags.String("root", ".", "repository root containing governed results and raw trajectory sources")
	planPath := flags.String("plan", "", "governed outcome plan (@file)")
	requestPath := flags.String("natural-request", "", "governed natural inventory request (@file)")
	inventoryPath := flags.String("inventory", "", "governed natural inventory (@file)")
	pilotPath := flags.String("pilot-sample", "", "governed outcome pilot v2 sample (@file)")
	keyPath := flags.String("key-file", "", "owner-only file containing a 32-byte lowercase hex key")
	keyID := flags.String("key-id", "", "owner-selected identifier for the blinding key")
	privateRootPath := flags.String("private-root", "", "owner-only pilot vault outside the repository root")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, err := readOutcomeDocument(*planPath, outcome.DecodePlan)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome pilot-materials:", err)
		return 2
	}
	request, err := readOutcomeDocument(*requestPath, outcome.DecodeNaturalInventoryRequest)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome pilot-materials:", err)
		return 2
	}
	inventory, err := readOutcomeDocument(*inventoryPath, outcome.DecodeNaturalInventory)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome pilot-materials:", err)
		return 2
	}
	pilot, err := readOutcomeDocument(*pilotPath, outcome.DecodeOutcomePilotSampleCommitment)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome pilot-materials:", err)
		return 2
	}
	key, err := readBlindingKeyFile(*keyPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome pilot-materials:", err)
		return 2
	}
	materials, err := outcome.BuildOutcomePilotMaterials(*rootPath, plan, request, inventory, pilot, *keyID, key, outcome.OutcomePilotEvidenceBudgetTokens)
	for index := range key {
		key[index] = 0
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome pilot-materials:", err)
		return 1
	}
	privatePayload, err := outcome.SealOutcomePilotPrivateMaterials(pilot.Digest, materials.Mappings, materials.Bindings)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome pilot-materials: seal private payload:", err)
		return 1
	}
	encodedPrivate, err := outcome.EncodeIndented(privatePayload)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome pilot-materials: encode private payload:", err)
		return 1
	}
	policy, err := safety.CurrentPathPolicy()
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome pilot-materials:", err)
		return 1
	}
	privateRoot, err := safety.CreateCacheRoot(policy, strings.TrimSpace(*privateRootPath))
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome pilot-materials:", err)
		return 1
	}
	privatePath := filepath.Join("outcome-pilots", pilot.Digest+".json")
	if err := privateRoot.PublishSensitiveExclusive(privatePath, encodedPrivate); err != nil {
		fmt.Fprintln(os.Stderr, "outcome pilot-materials: publish private payload:", err)
		return 1
	}
	return printOutcomeJSON(materials.Items)
}

func runOutcomePilotInspect(args []string) int {
	flags := flag.NewFlagSet("outcome pilot-inspect", flag.ContinueOnError)
	pilotPath := flags.String("pilot-sample", "", "governed outcome pilot v2 sample (@file)")
	bundlePath := flags.String("bundle", "", "sealed restricted development bundle (@file)")
	privateMaterialsPath := flags.String("private-materials", "", "sealed owner-only pilot private-materials artifact (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	pilot, err := readOutcomeDocument(*pilotPath, outcome.DecodeOutcomePilotSampleCommitment)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome pilot-inspect:", err)
		return 2
	}
	bundle, err := readOutcomeDocument(*bundlePath, outcome.DecodeReviewBundle)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome pilot-inspect:", err)
		return 2
	}
	privateMaterials, err := readOutcomeDocument(*privateMaterialsPath, outcome.DecodeOutcomePilotPrivateMaterials)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome pilot-inspect:", err)
		return 2
	}
	inspection, err := outcome.BuildOutcomePilotInspection(pilot, bundle, privateMaterials)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome pilot-inspect:", err)
		return 1
	}
	return printOutcomeJSON(inspection)
}

func runOutcomePlan(args []string) int {
	flags := flag.NewFlagSet("outcome plan", flag.ContinueOnError)
	path := flags.String("plan", "", "optional governed outcome plan to validate (@file or - for stdin)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *path == "" {
		plan, err := outcome.DefaultPlan()
		if err != nil {
			fmt.Fprintln(os.Stderr, "outcome plan:", err)
			return 1
		}
		return printOutcomeJSON(plan)
	}
	reader, closeDocument, err := openStudyDocument(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome plan:", err)
		return 2
	}
	defer closeDocument()
	plan, err := outcome.DecodePlan(reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome plan:", err)
		return 1
	}
	return printOutcomeJSON(plan)
}

func runOutcomeValidate(args []string) int {
	flags := flag.NewFlagSet("outcome validate", flag.ContinueOnError)
	documentType := flags.String("type", "plan", "outcome document type")
	path := flags.String("document", "", "outcome JSON document (@file or - for stdin)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	reader, closeDocument, err := openStudyDocument(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome validate:", err)
		return 2
	}
	defer closeDocument()
	var digest string
	switch *documentType {
	case "plan":
		value, decodeErr := outcome.DecodePlan(reader)
		err, digest = decodeErr, value.Digest
	case "record":
		value, decodeErr := outcome.DecodeRecord(reader)
		err, digest = decodeErr, value.Digest
	case "blind-packet":
		value, decodeErr := outcome.DecodeBlindPacket(reader)
		err, digest = decodeErr, value.Digest
	case "private-mapping":
		value, decodeErr := outcome.DecodePrivateMapping(reader)
		err, digest = decodeErr, value.Digest
	case "label":
		value, decodeErr := outcome.DecodeLabel(reader)
		err, digest = decodeErr, value.Digest
	case "resolution":
		value, decodeErr := outcome.DecodeResolution(reader)
		err, digest = decodeErr, value.Digest
	case "agreement":
		value, decodeErr := outcome.DecodeAgreementReport(reader)
		err, digest = decodeErr, value.Digest
	case "preservation":
		value, decodeErr := outcome.DecodePreservation(reader)
		err, digest = decodeErr, value.Digest
	case "sample-commitment":
		value, decodeErr := outcome.DecodeSampleCommitment(reader)
		err, digest = decodeErr, value.Digest
	case "pilot-sample-v1":
		value, decodeErr := outcome.DecodePilotSampleCommitment(reader)
		err, digest = decodeErr, value.Digest
	case "pilot-readiness-v1":
		value, decodeErr := outcome.DecodePilotReadiness(reader)
		err, digest = decodeErr, value.Digest
	case "pilot-sample":
		value, decodeErr := outcome.DecodeOutcomePilotSampleCommitment(reader)
		err, digest = decodeErr, value.Digest
	case "pilot-readiness":
		value, decodeErr := outcome.DecodeOutcomePilotReadiness(reader)
		err, digest = decodeErr, value.Digest
	case "pilot-source-binding":
		value, decodeErr := outcome.DecodeOutcomePilotSourceBinding(reader)
		err, digest = decodeErr, value.Digest
	case "pilot-private-materials":
		value, decodeErr := outcome.DecodeOutcomePilotPrivateMaterials(reader)
		err, digest = decodeErr, value.Digest
	case "pilot-inspection":
		value, decodeErr := outcome.DecodeOutcomePilotInspection(reader)
		err, digest = decodeErr, value.Digest
	case "natural-inventory-request":
		value, decodeErr := outcome.DecodeNaturalInventoryRequest(reader)
		err, digest = decodeErr, value.Digest
	case "natural-inventory":
		value, decodeErr := outcome.DecodeNaturalInventory(reader)
		err, digest = decodeErr, value.Digest
	case "executable-log":
		value, decodeErr := outcome.DecodeExecutionLog(reader)
		err, digest = decodeErr, value.Digest
	case "qualification-set":
		value, decodeErr := outcome.DecodeQualificationSet(reader)
		err, digest = decodeErr, value.Digest
	case "qualification-report":
		value, decodeErr := outcome.DecodeQualificationReport(reader)
		err, digest = decodeErr, value.Digest
	case "review-bundle":
		value, decodeErr := outcome.DecodeReviewBundle(reader)
		err, digest = decodeErr, value.Digest
	case "reviewer-record":
		value, decodeErr := outcome.DecodeReviewerRecord(reader)
		err, digest = decodeErr, value.Digest
	case "review-assignment":
		value, decodeErr := outcome.DecodeReviewAssignment(reader)
		err, digest = decodeErr, value.Digest
	case "label-batch":
		value, decodeErr := outcome.DecodeLabelBatch(reader)
		err, digest = decodeErr, value.Digest
	case "mapping-reveal":
		value, decodeErr := outcome.DecodeMappingReveal(reader)
		err, digest = decodeErr, value.Digest
	case "adjudication-ledger":
		value, decodeErr := outcome.DecodeAdjudicationLedger(reader)
		err, digest = decodeErr, value.Digest
	case "reviewer-handbook":
		value, decodeErr := outcome.DecodeReviewerHandbook(reader)
		err, digest = decodeErr, value.Digest
	case "reviewer-kit":
		value, decodeErr := outcome.DecodeReviewerKit(reader)
		err, digest = decodeErr, value.Digest
	case "blinding-protocol":
		value, decodeErr := outcome.DecodeBlindingProtocol(reader)
		err, digest = decodeErr, value.Digest
	case "blinding-probe":
		value, decodeErr := outcome.DecodeBlindingProbe(reader)
		err, digest = decodeErr, value.Digest
	case "blinding-probe-batch":
		value, decodeErr := outcome.DecodeBlindingProbeBatch(reader)
		err, digest = decodeErr, value.Digest
	case "blinding-analysis":
		value, decodeErr := outcome.DecodeBlindingAnalysis(reader)
		err, digest = decodeErr, value.Digest
	case "rubric-ambiguity-analysis":
		value, decodeErr := outcome.DecodeRubricAmbiguityAnalysis(reader)
		err, digest = decodeErr, value.Digest
	case "source-audit":
		value, decodeErr := outcome.DecodeOutcomeSourceAudit(reader)
		err, digest = decodeErr, value.Digest
	default:
		fmt.Fprintln(os.Stderr, "outcome validate: unsupported document type")
		return 2
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome validate:", err)
		return 1
	}
	return printOutcomeJSON(struct {
		Valid  bool   `json:"valid"`
		Digest string `json:"digest"`
	}{Valid: true, Digest: digest})
}

func runOutcomeSchema(args []string) int {
	flags := flag.NewFlagSet("outcome schema", flag.ContinueOnError)
	documentType := flags.String("type", "plan", "outcome schema type")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	schema, err := outcome.Schema(*documentType)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome schema:", err)
		return 2
	}
	return printOutcomeJSON(schema)
}

func runOutcomeAgreement(args []string) int {
	flags := flag.NewFlagSet("outcome agreement", flag.ContinueOnError)
	path := flags.String("pairs", "", "agreement pair array (@file or - for stdin)")
	iterations := flags.Int("bootstrap-iterations", 10_000, "task-cluster bootstrap iterations")
	seed := flags.String("seed", "", "frozen bootstrap seed")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	reader, closeDocument, err := openStudyDocument(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome agreement:", err)
		return 2
	}
	defer closeDocument()
	pairs, err := outcome.DecodeAgreementPairs(reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome agreement:", err)
		return 2
	}
	report, err := outcome.ComputeAgreement(pairs, *iterations, *seed)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome agreement:", err)
		return 1
	}
	return printOutcomeJSON(report)
}

func runOutcomeResolve(args []string) int {
	flags := flag.NewFlagSet("outcome resolve", flag.ContinueOnError)
	leftPath := flags.String("left", "", "first primary label (@file)")
	rightPath := flags.String("right", "", "second primary label (@file)")
	leftQualificationPath := flags.String("left-qualification", "", "first reviewer's passing qualification report (@file)")
	rightQualificationPath := flags.String("right-qualification", "", "second reviewer's passing qualification report (@file)")
	tiePath := flags.String("tie-break", "", "optional independent third label (@file)")
	tieQualificationPath := flags.String("tie-qualification", "", "optional third reviewer's passing qualification report (@file)")
	resolvedAt := flags.String("resolved-at", "", "RFC3339 resolution time")
	rule := flags.String("rule", "", "frozen conflict-resolution rule")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	left, err := readOutcomeLabel(*leftPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome resolve:", err)
		return 2
	}
	right, err := readOutcomeLabel(*rightPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome resolve:", err)
		return 2
	}
	leftQualification, err := readOutcomeQualificationReport(*leftQualificationPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome resolve:", err)
		return 2
	}
	rightQualification, err := readOutcomeQualificationReport(*rightQualificationPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome resolve:", err)
		return 2
	}
	var tieBreak *outcome.Label
	var tieQualification *outcome.QualificationReport
	if *tiePath != "" {
		value, readErr := readOutcomeLabel(*tiePath)
		if readErr != nil {
			fmt.Fprintln(os.Stderr, "outcome resolve:", readErr)
			return 2
		}
		tieBreak = &value
	}
	if *tieQualificationPath != "" {
		value, readErr := readOutcomeQualificationReport(*tieQualificationPath)
		if readErr != nil {
			fmt.Fprintln(os.Stderr, "outcome resolve:", readErr)
			return 2
		}
		tieQualification = &value
	}
	resolution, err := outcome.ResolveQualifiedLabels(
		[]outcome.Label{left, right}, []outcome.QualificationReport{leftQualification, rightQualification},
		tieBreak, tieQualification, *resolvedAt, *rule,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome resolve:", err)
		return 1
	}
	return printOutcomeJSON(resolution)
}

func runOutcomePreservation(args []string) int {
	flags := flag.NewFlagSet("outcome preservation", flag.ContinueOnError)
	sourcePath := flags.String("source", "", "source outcome record (@file)")
	intervenedPath := flags.String("intervened", "", "intervened outcome record (@file)")
	mechanism := flags.String("mechanism", "", "independent preservation mechanism")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	source, err := readOutcomeRecord(*sourcePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome preservation:", err)
		return 2
	}
	intervened, err := readOutcomeRecord(*intervenedPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome preservation:", err)
		return 2
	}
	preservation, err := outcome.EvaluatePreservation(source, intervened, *mechanism)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome preservation:", err)
		return 1
	}
	return printOutcomeJSON(preservation)
}

func readOutcomeLabel(path string) (outcome.Label, error) {
	reader, closeDocument, err := openStudyDocument(path)
	if err != nil {
		return outcome.Label{}, err
	}
	defer closeDocument()
	return outcome.DecodeLabel(reader)
}

func readOutcomeRecord(path string) (outcome.Record, error) {
	reader, closeDocument, err := openStudyDocument(path)
	if err != nil {
		return outcome.Record{}, err
	}
	defer closeDocument()
	return outcome.DecodeRecord(reader)
}

func readOutcomeQualificationReport(path string) (outcome.QualificationReport, error) {
	reader, closeDocument, err := openStudyDocument(path)
	if err != nil {
		return outcome.QualificationReport{}, err
	}
	defer closeDocument()
	return outcome.DecodeQualificationReport(reader)
}

func readBlindingKeyFile(path string) (key []byte, returnErr error) {
	path = strings.TrimPrefix(strings.TrimSpace(path), "@")
	if path == "" || path == "-" {
		return nil, fmt.Errorf("blinding key file path is required and stdin is forbidden")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("blinding key must be a regular file")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("blinding key file permissions must not grant group or other access")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("close blinding key: %w", closeErr)
		}
	}()
	return outcome.DecodeBlindingKey(file)
}

func printOutcomeJSON(value any) int {
	encoded, err := outcome.EncodeIndented(value)
	if err != nil {
		fmt.Fprintln(os.Stderr, "outcome: encode output:", err)
		return 1
	}
	return writeCommandOutput("outcome", encoded)
}
