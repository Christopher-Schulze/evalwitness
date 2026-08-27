package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/relation"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

func runRelationQualification(args []string) int {
	flags := flag.NewFlagSet("relation qualification", flag.ContinueOnError)
	planPath := flags.String("plan", "", "governed relation audit plan (@file)")
	keyPath := flags.String("key-file", "", "owner-only 32-byte hexadecimal qualification blinding key")
	keyID := flags.String("key-id", "", "owner-managed qualification key identity")
	privateRootPath := flags.String("private-root", "", "owner-only answer-key vault outside the repository root")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, err := readRelationDocument(*planPath, relation.DecodeReviewPlan)
	if err != nil {
		return relationReviewInputError("qualification", err)
	}
	key, err := readRelationBlindingKeyFile(*keyPath)
	if err != nil {
		return relationReviewInputError("qualification", err)
	}
	defer zeroBytes(key)
	set, answerKey, err := relation.DefaultQualification(plan, strings.TrimSpace(*keyID), key)
	if err != nil {
		return relationReviewOperationError("qualification", err)
	}
	encodedKey, err := relation.EncodeIndented(answerKey)
	if err != nil {
		return relationReviewOperationError("qualification", err)
	}
	root, err := createRelationPrivateRoot(*privateRootPath)
	if err != nil {
		return relationReviewOperationError("qualification", err)
	}
	privatePath := filepath.Join("qualification-keys", answerKey.Digest+".json")
	if err := root.PublishSensitiveExclusive(privatePath, encodedKey); err != nil {
		return relationReviewOperationError("qualification", fmt.Errorf("publish owner-only answer key: %w", err))
	}
	return printRelationJSON(set)
}

func runRelationHandbook(args []string) int {
	flags := flag.NewFlagSet("relation handbook", flag.ContinueOnError)
	planPath := flags.String("plan", "", "governed relation audit plan (@file)")
	qualificationPath := flags.String("qualification-set", "", "relation qualification set (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, err := readRelationDocument(*planPath, relation.DecodeReviewPlan)
	if err != nil {
		return relationReviewInputError("handbook", err)
	}
	qualification, err := readRelationDocument(*qualificationPath, relation.DecodeQualificationSet)
	if err != nil {
		return relationReviewInputError("handbook", err)
	}
	handbook, err := relation.DefaultReviewerHandbook(plan, qualification)
	if err != nil {
		return relationReviewOperationError("handbook", err)
	}
	return printRelationJSON(handbook)
}

func runRelationQualify(args []string) int {
	flags := flag.NewFlagSet("relation qualify", flag.ContinueOnError)
	qualificationPath := flags.String("qualification-set", "", "relation qualification set (@file)")
	answerKeyPath := flags.String("answer-key", "", "owner-only qualification answer key (@file)")
	responsesPath := flags.String("responses", "", "complete reviewer response array (@file)")
	reviewerAlias := flags.String("reviewer-alias", "", "pseudonymous reviewer alias")
	qualifiedAt := flags.String("qualified-at", "", "RFC3339 supervised qualification completion time")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	qualification, err := readRelationDocument(*qualificationPath, relation.DecodeQualificationSet)
	if err != nil {
		return relationReviewInputError("qualify", err)
	}
	answerKey, err := readPrivateRelationDocument(*answerKeyPath, relation.DecodeQualificationAnswerKey)
	if err != nil {
		return relationReviewInputError("qualify", err)
	}
	responses, err := readRelationDocument(*responsesPath, relation.DecodeQualificationResponses)
	if err != nil {
		return relationReviewInputError("qualify", err)
	}
	report, err := relation.GradeQualification(qualification, answerKey, *reviewerAlias, responses, *qualifiedAt)
	if err != nil {
		return relationReviewOperationError("qualify", err)
	}
	return printRelationJSON(report)
}

func runRelationReviewer(args []string) int {
	flags := flag.NewFlagSet("relation reviewer", flag.ContinueOnError)
	protocolVersion := flags.String("protocol-version", "v1", "relation protocol version: v1, v2, or v3")
	alias := flags.String("alias", "", "pseudonymous reviewer alias")
	role := flags.String("role", "", "primary or tie_break")
	consentedAt := flags.String("consented-at", "", "RFC3339 consent time")
	independent := flags.Bool("independence-attested", false, "reviewer attests independence")
	authorship := flags.Bool("authorship-policy-accepted", false, "reviewer accepts the authorship policy")
	privateContact := flags.Bool("contact-held-privately", false, "contact details are held outside reviewer artifacts")
	var conflicts repeatedStringFlag
	flags.Var(&conflicts, "conflict", "declared conflict of interest; repeat as needed")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	protocol := relation.ProtocolVersionV1
	switch *protocolVersion {
	case "v1":
	case "v2":
		protocol = relation.ProtocolVersionV2
	case "v3":
		protocol = relation.ProtocolVersionV3
	default:
		return relationReviewInputError("reviewer", errors.New("protocol version must be v1, v2, or v3"))
	}
	reviewer, err := relation.NewReviewerRecordForProtocol(protocol, *alias, relation.ReviewerRole(*role), *consentedAt, *independent, *authorship, *privateContact, conflicts)
	if err != nil {
		return relationReviewOperationError("reviewer", err)
	}
	return printRelationJSON(reviewer)
}

func runRelationBundle(args []string) int {
	flags := flag.NewFlagSet("relation bundle", flag.ContinueOnError)
	planPath := flags.String("plan", "", "governed relation audit plan (@file)")
	sampleDigest := flags.String("sample-digest", "", "pilot or primary sample digest")
	dataRole := flags.String("data-role", "", "development_pilot or primary_audit")
	packetsPath := flags.String("packets", "", "complete blind-packet array (@file)")
	mappingsPath := flags.String("mappings", "", "complete owner-only private-mapping array (@file)")
	qualificationPath := flags.String("qualification-set", "", "relation qualification set (@file)")
	handbookPath := flags.String("handbook", "", "relation reviewer handbook (@file)")
	createdAt := flags.String("created-at", "", "RFC3339 bundle creation time")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, err := readRelationDocument(*planPath, relation.DecodeReviewPlan)
	if err != nil {
		return relationReviewInputError("bundle", err)
	}
	packets, err := readRelationDocument(*packetsPath, relation.DecodeBlindPackets)
	if err != nil {
		return relationReviewInputError("bundle", err)
	}
	mappings, err := readPrivateRelationDocument(*mappingsPath, relation.DecodePrivateMappings)
	if err != nil {
		return relationReviewInputError("bundle", err)
	}
	qualification, err := readRelationDocument(*qualificationPath, relation.DecodeQualificationSet)
	if err != nil {
		return relationReviewInputError("bundle", err)
	}
	handbook, err := readRelationDocument(*handbookPath, relation.DecodeReviewerHandbook)
	if err != nil {
		return relationReviewInputError("bundle", err)
	}
	bundle, err := relation.BuildReviewBundle(plan, *sampleDigest, relation.ReviewDataRole(*dataRole), packets, mappings, qualification, handbook, *createdAt)
	if err != nil {
		return relationReviewOperationError("bundle", err)
	}
	return printRelationJSON(bundle)
}

func runRelationPilotReadiness(args []string) int {
	flags := flag.NewFlagSet("relation pilot-readiness", flag.ContinueOnError)
	planPath := flags.String("plan", "", "governed relation audit plan (@file)")
	pilotPath := flags.String("pilot-sample", "", "governed relation pilot sample (@file)")
	primaryV3Path := flags.String("primary-sample-v3", "", "governed v3 primary sample required for a v3 pilot (@file)")
	sentinelV3Path := flags.String("scarcity-sentinel-v3", "", "governed v3 scarcity sentinel required for a v3 pilot (@file)")
	bundlePath := flags.String("bundle", "", "complete restricted relation pilot bundle (@file)")
	mappingsPath := flags.String("mappings", "", "complete owner-only private-mapping array (@file)")
	qualificationPath := flags.String("qualification-set", "", "relation qualification set (@file)")
	handbookPath := flags.String("handbook", "", "relation reviewer handbook (@file)")
	preparedAt := flags.String("prepared-at", "", "RFC3339 readiness preparation time after bundle creation")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, err := readRelationDocument(*planPath, relation.DecodeReviewPlan)
	if err != nil {
		return relationReviewInputError("pilot-readiness", err)
	}
	pilot, err := loadReviewPilotSample(*pilotPath, *planPath, *primaryV3Path, *sentinelV3Path, plan)
	if err != nil {
		return relationReviewInputError("pilot-readiness", err)
	}
	bundle, err := readRelationDocument(*bundlePath, relation.DecodeReviewBundle)
	if err != nil {
		return relationReviewInputError("pilot-readiness", err)
	}
	mappings, err := readPrivateRelationDocument(*mappingsPath, relation.DecodePrivateMappings)
	if err != nil {
		return relationReviewInputError("pilot-readiness", err)
	}
	qualification, err := readRelationDocument(*qualificationPath, relation.DecodeQualificationSet)
	if err != nil {
		return relationReviewInputError("pilot-readiness", err)
	}
	handbook, err := readRelationDocument(*handbookPath, relation.DecodeReviewerHandbook)
	if err != nil {
		return relationReviewInputError("pilot-readiness", err)
	}
	readiness, err := relation.BuildRelationPilotReadiness(plan, pilot, bundle, mappings, qualification, handbook, *preparedAt)
	if err != nil {
		return relationReviewOperationError("pilot-readiness", err)
	}
	return printRelationJSON(readiness)
}

func runRelationPilotInspection(args []string) int {
	flags := flag.NewFlagSet("relation pilot-inspection", flag.ContinueOnError)
	readinessPath := flags.String("readiness", "", "sealed relation pilot readiness (@file)")
	bundlePath := flags.String("bundle", "", "complete restricted relation pilot bundle (@file)")
	mappingsPath := flags.String("mappings", "", "complete owner-only private-mapping array (@file)")
	decisionsPath := flags.String("decisions", "", "owner-only complete inspection decision array (@file)")
	inspectorAlias := flags.String("inspector-alias", "", "pseudonymous owner inspector alias")
	inspectedAt := flags.String("inspected-at", "", "RFC3339 inspection completion time after readiness")
	privateRootPath := flags.String("private-root", "", "owner-only inspection vault outside the repository root")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	readiness, err := readRelationDocument(*readinessPath, relation.DecodeRelationPilotReadiness)
	if err != nil {
		return relationReviewInputError("pilot-inspection", err)
	}
	bundle, err := readRelationDocument(*bundlePath, relation.DecodeReviewBundle)
	if err != nil {
		return relationReviewInputError("pilot-inspection", err)
	}
	mappings, err := readPrivateRelationDocument(*mappingsPath, relation.DecodePrivateMappings)
	if err != nil {
		return relationReviewInputError("pilot-inspection", err)
	}
	decisions, err := readPrivateRelationDocument(*decisionsPath, relation.DecodePilotInspectionDecisionDrafts)
	if err != nil {
		return relationReviewInputError("pilot-inspection", err)
	}
	record, err := relation.BuildPilotInspectionRecord(readiness, bundle, mappings, decisions, *inspectorAlias, *inspectedAt)
	if err != nil {
		return relationReviewOperationError("pilot-inspection", err)
	}
	encoded, err := relation.EncodeIndented(record)
	if err != nil {
		return relationReviewOperationError("pilot-inspection", fmt.Errorf("encode owner-only inspection: %w", err))
	}
	root, err := createRelationPrivateRoot(*privateRootPath)
	if err != nil {
		return relationReviewOperationError("pilot-inspection", err)
	}
	privatePath := filepath.Join("pilot-inspections", record.Digest+".json")
	if err := root.PublishSensitiveExclusive(privatePath, encoded); err != nil {
		return relationReviewOperationError("pilot-inspection", fmt.Errorf("publish owner-only inspection: %w", err))
	}
	return printRelationJSON(struct {
		Published            bool                                  `json:"published"`
		Digest               string                                `json:"digest"`
		RelativePath         string                                `json:"relative_path"`
		OverallStatus        relation.PilotInspectionOverallStatus `json:"overall_status"`
		HumanStudyStatus     string                                `json:"human_study_status"`
		ExternalActionStatus relation.ExternalActionStatus         `json:"external_action_status"`
	}{
		Published: true, Digest: record.Digest, RelativePath: privatePath, OverallStatus: record.OverallStatus,
		HumanStudyStatus: record.HumanStudyStatus, ExternalActionStatus: record.ExternalActionStatus,
	})
}

func runRelationPilotLaunchDossier(args []string) int {
	flags := flag.NewFlagSet("relation pilot-launch-dossier", flag.ContinueOnError)
	planPath := flags.String("plan", "", "governed relation audit plan (@file)")
	pilotPath := flags.String("pilot-sample", "", "sealed relation development-pilot sample (@file)")
	primaryV3Path := flags.String("primary-sample-v3", "", "governed v3 primary sample required for a v3 pilot (@file)")
	sentinelV3Path := flags.String("scarcity-sentinel-v3", "", "governed v3 scarcity sentinel required for a v3 pilot (@file)")
	bundlePath := flags.String("bundle", "", "complete restricted relation pilot bundle (@file)")
	mappingsPath := flags.String("mappings", "", "complete owner-only private-mapping array (@file)")
	qualificationPath := flags.String("qualification-set", "", "sealed relation qualification set (@file)")
	handbookPath := flags.String("handbook", "", "sealed relation reviewer handbook (@file)")
	readinessPath := flags.String("readiness", "", "sealed relation pilot readiness (@file)")
	preparedAt := flags.String("prepared-at", "", "RFC3339 dossier preparation time after readiness")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, err := readRelationDocument(*planPath, relation.DecodeReviewPlan)
	if err != nil {
		return relationReviewInputError("pilot-launch-dossier", err)
	}
	pilot, err := loadReviewPilotSample(*pilotPath, *planPath, *primaryV3Path, *sentinelV3Path, plan)
	if err != nil {
		return relationReviewInputError("pilot-launch-dossier", err)
	}
	bundle, err := readRelationDocument(*bundlePath, relation.DecodeReviewBundle)
	if err != nil {
		return relationReviewInputError("pilot-launch-dossier", err)
	}
	mappings, err := readPrivateRelationDocument(*mappingsPath, relation.DecodePrivateMappings)
	if err != nil {
		return relationReviewInputError("pilot-launch-dossier", err)
	}
	qualification, err := readRelationDocument(*qualificationPath, relation.DecodeQualificationSet)
	if err != nil {
		return relationReviewInputError("pilot-launch-dossier", err)
	}
	handbook, err := readRelationDocument(*handbookPath, relation.DecodeReviewerHandbook)
	if err != nil {
		return relationReviewInputError("pilot-launch-dossier", err)
	}
	readiness, err := readRelationDocument(*readinessPath, relation.DecodeRelationPilotReadiness)
	if err != nil {
		return relationReviewInputError("pilot-launch-dossier", err)
	}
	dossier, err := relation.BuildPilotLaunchDossier(plan, pilot, bundle, mappings, qualification, handbook, readiness, *preparedAt)
	if err != nil {
		return relationReviewOperationError("pilot-launch-dossier", err)
	}
	return printRelationJSON(dossier)
}

func runRelationRenderPilotLaunchBrief(args []string) int {
	flags := flag.NewFlagSet("relation render-pilot-launch-brief", flag.ContinueOnError)
	dossierPath := flags.String("dossier", "", "sealed relation pilot launch dossier (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	dossier, err := readRelationDocument(*dossierPath, relation.DecodePilotLaunchDossier)
	if err != nil {
		return relationReviewInputError("render-pilot-launch-brief", err)
	}
	rendered, err := relation.RenderPilotLaunchBriefMarkdown(dossier)
	if err != nil {
		return relationReviewOperationError("render-pilot-launch-brief", err)
	}
	return writeCommandOutput("relation render-pilot-launch-brief", []byte(rendered))
}

func runRelationVerifyPilotInspection(args []string) int {
	flags := flag.NewFlagSet("relation verify-pilot-inspection", flag.ContinueOnError)
	recordPath := flags.String("inspection", "", "sealed owner-only relation pilot inspection (@file)")
	readinessPath := flags.String("readiness", "", "sealed relation pilot readiness (@file)")
	bundlePath := flags.String("bundle", "", "complete restricted relation pilot bundle (@file)")
	mappingsPath := flags.String("mappings", "", "complete owner-only private-mapping array (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	record, err := readPrivateRelationDocument(*recordPath, relation.DecodePilotInspectionRecord)
	if err != nil {
		return relationReviewInputError("verify-pilot-inspection", err)
	}
	readiness, err := readRelationDocument(*readinessPath, relation.DecodeRelationPilotReadiness)
	if err != nil {
		return relationReviewInputError("verify-pilot-inspection", err)
	}
	bundle, err := readRelationDocument(*bundlePath, relation.DecodeReviewBundle)
	if err != nil {
		return relationReviewInputError("verify-pilot-inspection", err)
	}
	mappings, err := readPrivateRelationDocument(*mappingsPath, relation.DecodePrivateMappings)
	if err != nil {
		return relationReviewInputError("verify-pilot-inspection", err)
	}
	if err := relation.VerifyPilotInspectionRecord(record, readiness, bundle, mappings); err != nil {
		return relationReviewOperationError("verify-pilot-inspection", err)
	}
	return printRelationJSON(struct {
		Valid            bool   `json:"valid"`
		InspectionDigest string `json:"inspection_digest"`
		ReadinessDigest  string `json:"readiness_digest"`
	}{Valid: true, InspectionDigest: record.Digest, ReadinessDigest: readiness.Digest})
}

func runRelationRenderPilotInspection(args []string) int {
	flags := flag.NewFlagSet("relation render-pilot-inspection", flag.ContinueOnError)
	readinessPath := flags.String("readiness", "", "sealed relation pilot readiness (@file)")
	bundlePath := flags.String("bundle", "", "complete restricted relation pilot bundle (@file)")
	mappingsPath := flags.String("mappings", "", "complete owner-only private-mapping array (@file)")
	handbookPath := flags.String("handbook", "", "sealed relation reviewer handbook (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	readiness, err := readRelationDocument(*readinessPath, relation.DecodeRelationPilotReadiness)
	if err != nil {
		return relationReviewInputError("render-pilot-inspection", err)
	}
	bundle, err := readRelationDocument(*bundlePath, relation.DecodeReviewBundle)
	if err != nil {
		return relationReviewInputError("render-pilot-inspection", err)
	}
	mappings, err := readPrivateRelationDocument(*mappingsPath, relation.DecodePrivateMappings)
	if err != nil {
		return relationReviewInputError("render-pilot-inspection", err)
	}
	handbook, err := readRelationDocument(*handbookPath, relation.DecodeReviewerHandbook)
	if err != nil {
		return relationReviewInputError("render-pilot-inspection", err)
	}
	rendered, err := relation.RenderPilotInspectionMarkdown(readiness, bundle, mappings, handbook)
	if err != nil {
		return relationReviewOperationError("render-pilot-inspection", err)
	}
	return writeCommandOutput("relation render-pilot-inspection", []byte(rendered))
}

func runRelationPilotChangeReceipt(args []string) int {
	flags := flag.NewFlagSet("relation pilot-change-receipt", flag.ContinueOnError)
	readinessPath := flags.String("readiness", "", "sealed relation pilot readiness (@file)")
	bundlePath := flags.String("bundle", "", "complete restricted relation pilot bundle (@file)")
	mappingsPath := flags.String("mappings", "", "complete owner-only private-mapping array (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	readiness, err := readRelationDocument(*readinessPath, relation.DecodeRelationPilotReadiness)
	if err != nil {
		return relationReviewInputError("pilot-change-receipt", err)
	}
	bundle, err := readRelationDocument(*bundlePath, relation.DecodeReviewBundle)
	if err != nil {
		return relationReviewInputError("pilot-change-receipt", err)
	}
	mappings, err := readPrivateRelationDocument(*mappingsPath, relation.DecodePrivateMappings)
	if err != nil {
		return relationReviewInputError("pilot-change-receipt", err)
	}
	receipt, err := relation.BuildPilotChangeReceipt(readiness, bundle, mappings)
	if err != nil {
		return relationReviewOperationError("pilot-change-receipt", err)
	}
	return printRelationJSON(receipt)
}

func runRelationRenderPilotChangeAtlas(args []string) int {
	flags := flag.NewFlagSet("relation render-pilot-change-atlas", flag.ContinueOnError)
	receiptPath := flags.String("receipt", "", "optional sealed pilot change receipt for receipt-bound rendering (@file)")
	readinessPath := flags.String("readiness", "", "sealed relation pilot readiness (@file)")
	bundlePath := flags.String("bundle", "", "complete restricted relation pilot bundle (@file)")
	mappingsPath := flags.String("mappings", "", "complete owner-only private-mapping array (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	readiness, err := readRelationDocument(*readinessPath, relation.DecodeRelationPilotReadiness)
	if err != nil {
		return relationReviewInputError("render-pilot-change-atlas", err)
	}
	bundle, err := readRelationDocument(*bundlePath, relation.DecodeReviewBundle)
	if err != nil {
		return relationReviewInputError("render-pilot-change-atlas", err)
	}
	mappings, err := readPrivateRelationDocument(*mappingsPath, relation.DecodePrivateMappings)
	if err != nil {
		return relationReviewInputError("render-pilot-change-atlas", err)
	}
	var rendered string
	if strings.TrimSpace(*receiptPath) == "" {
		rendered, err = relation.RenderPilotChangeAtlasMarkdown(readiness, bundle, mappings)
	} else {
		var receipt relation.PilotChangeReceipt
		receipt, err = readPrivateRelationDocument(*receiptPath, relation.DecodePilotChangeReceipt)
		if err != nil {
			return relationReviewInputError("render-pilot-change-atlas", err)
		}
		rendered, err = relation.RenderPilotChangeAtlasWithReceiptMarkdown(receipt, readiness, bundle, mappings)
	}
	if err != nil {
		return relationReviewOperationError("render-pilot-change-atlas", err)
	}
	return writeCommandOutput("relation render-pilot-change-atlas", []byte(rendered))
}

func runRelationAssignPrimary(args []string) int {
	flags := flag.NewFlagSet("relation assign-primary", flag.ContinueOnError)
	bundlePath := flags.String("bundle", "", "sealed relation review bundle (@file)")
	mappingsPath := flags.String("mappings", "", "complete owner-only private-mapping array (@file)")
	reviewerPath := flags.String("reviewer", "", "sealed relation reviewer record (@file)")
	qualificationPath := flags.String("qualification", "", "reviewer-specific passing qualification report (@file)")
	slot := flags.Int("slot", 0, "primary reviewer slot one or two")
	seedPath := flags.String("seed-file", "", "owner-only 32-byte lowercase hex assignment seed")
	assignedAt := flags.String("assigned-at", "", "RFC3339 assignment planning time")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	bundle, err := readRelationDocument(*bundlePath, relation.DecodeReviewBundle)
	if err != nil {
		return relationReviewInputError("assign-primary", err)
	}
	mappings, err := readPrivateRelationDocument(*mappingsPath, relation.DecodePrivateMappings)
	if err != nil {
		return relationReviewInputError("assign-primary", err)
	}
	reviewer, err := readRelationDocument(*reviewerPath, relation.DecodeReviewerRecord)
	if err != nil {
		return relationReviewInputError("assign-primary", err)
	}
	qualification, err := readRelationDocument(*qualificationPath, relation.DecodeQualificationReport)
	if err != nil {
		return relationReviewInputError("assign-primary", err)
	}
	seed, err := readRelationBlindingKeyFile(*seedPath)
	if err != nil {
		return relationReviewInputError("assign-primary", err)
	}
	defer zeroBytes(seed)
	assignment, err := relation.BuildPrimaryAssignment(bundle, mappings, reviewer, qualification, *slot, seed, *assignedAt)
	if err != nil {
		return relationReviewOperationError("assign-primary", err)
	}
	return printRelationJSON(assignment)
}

func runRelationKit(args []string) int {
	flags := flag.NewFlagSet("relation kit", flag.ContinueOnError)
	bundlePath := flags.String("bundle", "", "sealed relation review bundle (@file)")
	assignmentPath := flags.String("assignment", "", "sealed relation review assignment (@file)")
	handbookPath := flags.String("handbook", "", "sealed relation reviewer handbook (@file)")
	generatedAt := flags.String("generated-at", "", "RFC3339 kit generation time")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	bundle, assignment, handbook, err := readRelationKitInputs(*bundlePath, *assignmentPath, *handbookPath)
	if err != nil {
		return relationReviewInputError("kit", err)
	}
	kit, err := relation.BuildReviewerKit(bundle, assignment, handbook, *generatedAt)
	if err != nil {
		return relationReviewOperationError("kit", err)
	}
	return printRelationJSON(kit)
}

func runRelationVerifyKit(args []string) int {
	flags := flag.NewFlagSet("relation verify-kit", flag.ContinueOnError)
	kitPath := flags.String("kit", "", "sealed relation reviewer kit (@file)")
	bundlePath := flags.String("bundle", "", "sealed relation review bundle (@file)")
	assignmentPath := flags.String("assignment", "", "sealed relation review assignment (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	kit, err := readRelationDocument(*kitPath, relation.DecodeReviewerKit)
	if err != nil {
		return relationReviewInputError("verify-kit", err)
	}
	bundle, err := readRelationDocument(*bundlePath, relation.DecodeReviewBundle)
	if err != nil {
		return relationReviewInputError("verify-kit", err)
	}
	assignment, err := readRelationDocument(*assignmentPath, relation.DecodeReviewAssignment)
	if err != nil {
		return relationReviewInputError("verify-kit", err)
	}
	if err := relation.VerifyReviewerKit(kit, bundle, assignment); err != nil {
		return relationReviewOperationError("verify-kit", err)
	}
	return printRelationJSON(struct {
		Valid  bool   `json:"valid"`
		Digest string `json:"digest"`
	}{Valid: true, Digest: kit.Digest})
}

func runRelationRenderKit(args []string) int {
	flags := flag.NewFlagSet("relation render-kit", flag.ContinueOnError)
	kitPath := flags.String("kit", "", "sealed relation reviewer kit (@file)")
	bundlePath := flags.String("bundle", "", "sealed relation review bundle (@file)")
	assignmentPath := flags.String("assignment", "", "sealed relation review assignment (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	kit, err := readRelationDocument(*kitPath, relation.DecodeReviewerKit)
	if err != nil {
		return relationReviewInputError("render-kit", err)
	}
	bundle, err := readRelationDocument(*bundlePath, relation.DecodeReviewBundle)
	if err != nil {
		return relationReviewInputError("render-kit", err)
	}
	assignment, err := readRelationDocument(*assignmentPath, relation.DecodeReviewAssignment)
	if err != nil {
		return relationReviewInputError("render-kit", err)
	}
	if err := relation.VerifyReviewerKit(kit, bundle, assignment); err != nil {
		return relationReviewOperationError("render-kit", err)
	}
	rendered, err := relation.RenderReviewerKitMarkdown(kit)
	if err != nil {
		return relationReviewOperationError("render-kit", err)
	}
	return writeCommandOutput("relation render-kit", []byte(rendered))
}

func runRelationJudgment(args []string) int {
	flags := flag.NewFlagSet("relation judgment", flag.ContinueOnError)
	bundlePath := flags.String("bundle", "", "sealed relation review bundle (@file)")
	assignmentPath := flags.String("assignment", "", "sealed relation review assignment (@file)")
	draftPath := flags.String("draft", "", "complete seven-axis pair-judgment draft (@file)")
	parentPath := flags.String("parent", "", "optional immediately preceding sealed judgment revision (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	bundle, err := readRelationDocument(*bundlePath, relation.DecodeReviewBundle)
	if err != nil {
		return relationReviewInputError("judgment", err)
	}
	assignment, err := readRelationDocument(*assignmentPath, relation.DecodeReviewAssignment)
	if err != nil {
		return relationReviewInputError("judgment", err)
	}
	draft, err := readRelationDocument(*draftPath, relation.DecodePairJudgmentDraft)
	if err != nil {
		return relationReviewInputError("judgment", err)
	}
	var parent *relation.PairJudgment
	if strings.TrimSpace(*parentPath) != "" {
		value, decodeErr := readRelationDocument(*parentPath, relation.DecodePairJudgment)
		if decodeErr != nil {
			return relationReviewInputError("judgment", decodeErr)
		}
		parent = &value
	}
	judgment, err := relation.BuildPairJudgment(bundle, assignment, draft, parent)
	if err != nil {
		return relationReviewOperationError("judgment", err)
	}
	return printRelationJSON(judgment)
}

func runRelationJudgmentBatch(args []string) int {
	flags := flag.NewFlagSet("relation judgment-batch", flag.ContinueOnError)
	bundlePath := flags.String("bundle", "", "sealed relation review bundle (@file)")
	assignmentPath := flags.String("assignment", "", "sealed relation review assignment (@file)")
	judgmentsPath := flags.String("judgments", "", "complete latest-revision judgment array (@file)")
	committedAt := flags.String("committed-at", "", "RFC3339 batch commitment time after every judgment")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	bundle, err := readRelationDocument(*bundlePath, relation.DecodeReviewBundle)
	if err != nil {
		return relationReviewInputError("judgment-batch", err)
	}
	assignment, err := readRelationDocument(*assignmentPath, relation.DecodeReviewAssignment)
	if err != nil {
		return relationReviewInputError("judgment-batch", err)
	}
	judgments, err := readRelationDocument(*judgmentsPath, relation.DecodePairJudgments)
	if err != nil {
		return relationReviewInputError("judgment-batch", err)
	}
	batch, err := relation.BuildJudgmentBatch(bundle, assignment, judgments, *committedAt)
	if err != nil {
		return relationReviewOperationError("judgment-batch", err)
	}
	return printRelationJSON(batch)
}

func runRelationAnalyzeAmbiguity(args []string) int {
	flags := flag.NewFlagSet("relation analyze-ambiguity", flag.ContinueOnError)
	bundlePath := flags.String("bundle", "", "sealed relation review bundle (@file)")
	leftAssignmentPath := flags.String("left-assignment", "", "first sealed primary assignment (@file)")
	leftBatchPath := flags.String("left-batch", "", "first complete primary judgment commitment (@file)")
	rightAssignmentPath := flags.String("right-assignment", "", "second sealed primary assignment (@file)")
	rightBatchPath := flags.String("right-batch", "", "second complete primary judgment commitment (@file)")
	analyzedAt := flags.String("analyzed-at", "", "RFC3339 analysis time after both primary commitments")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, err := readRelationPrimaryReviewInputs(*bundlePath, *leftAssignmentPath, *leftBatchPath, *rightAssignmentPath, *rightBatchPath)
	if err != nil {
		return relationReviewInputError("analyze-ambiguity", err)
	}
	analysis, err := relation.BuildRelationAmbiguityAnalysis(bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, *analyzedAt)
	if err != nil {
		return relationReviewOperationError("analyze-ambiguity", err)
	}
	return printRelationJSON(analysis)
}

func runRelationAssignTie(args []string) int {
	flags := flag.NewFlagSet("relation assign-tie", flag.ContinueOnError)
	bundlePath := flags.String("bundle", "", "sealed relation review bundle (@file)")
	mappingsPath := flags.String("mappings", "", "complete owner-only private-mapping array (@file)")
	reviewerPath := flags.String("reviewer", "", "sealed tie-break reviewer record (@file)")
	qualificationPath := flags.String("qualification", "", "reviewer-specific passing qualification report (@file)")
	ambiguityPath := flags.String("ambiguity", "", "sealed prereveal ambiguity analysis (@file)")
	leftAssignmentPath := flags.String("left-assignment", "", "slot-one primary assignment (@file)")
	leftBatchPath := flags.String("left-batch", "", "slot-one primary judgment batch (@file)")
	rightAssignmentPath := flags.String("right-assignment", "", "slot-two primary assignment (@file)")
	rightBatchPath := flags.String("right-batch", "", "slot-two primary judgment batch (@file)")
	seedPath := flags.String("seed-file", "", "owner-only 32-byte lowercase hex tie-break ordering seed")
	assignedAt := flags.String("assigned-at", "", "RFC3339 assignment time after prereveal analysis")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, err := readRelationPrimaryReviewInputs(*bundlePath, *leftAssignmentPath, *leftBatchPath, *rightAssignmentPath, *rightBatchPath)
	if err != nil {
		return relationReviewInputError("assign-tie", err)
	}
	mappings, err := readPrivateRelationDocument(*mappingsPath, relation.DecodePrivateMappings)
	if err != nil {
		return relationReviewInputError("assign-tie", err)
	}
	reviewer, err := readRelationDocument(*reviewerPath, relation.DecodeReviewerRecord)
	if err != nil {
		return relationReviewInputError("assign-tie", err)
	}
	qualification, err := readRelationDocument(*qualificationPath, relation.DecodeQualificationReport)
	if err != nil {
		return relationReviewInputError("assign-tie", err)
	}
	ambiguity, err := readRelationDocument(*ambiguityPath, relation.DecodeRelationAmbiguityAnalysis)
	if err != nil {
		return relationReviewInputError("assign-tie", err)
	}
	seed, err := readRelationBlindingKeyFile(*seedPath)
	if err != nil {
		return relationReviewInputError("assign-tie", err)
	}
	defer zeroBytes(seed)
	assignment, err := relation.BuildTieBreakAssignment(bundle, mappings, reviewer, qualification, ambiguity, leftAssignment, leftBatch, rightAssignment, rightBatch, seed, *assignedAt)
	if err != nil {
		return relationReviewOperationError("assign-tie", err)
	}
	return printRelationJSON(assignment)
}

func runRelationProbeBatch(args []string) int {
	flags := flag.NewFlagSet("relation probe-batch", flag.ContinueOnError)
	bundlePath := flags.String("bundle", "", "sealed relation review bundle (@file)")
	assignmentPath := flags.String("assignment", "", "sealed primary assignment (@file)")
	batchPath := flags.String("judgment-batch", "", "complete primary judgment commitment (@file)")
	draftsPath := flags.String("drafts", "", "complete post-label condition-probe draft array (@file)")
	committedAt := flags.String("committed-at", "", "RFC3339 probe commitment time after all probe submissions")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	bundle, err := readRelationDocument(*bundlePath, relation.DecodeReviewBundle)
	if err != nil {
		return relationReviewInputError("probe-batch", err)
	}
	assignment, err := readRelationDocument(*assignmentPath, relation.DecodeReviewAssignment)
	if err != nil {
		return relationReviewInputError("probe-batch", err)
	}
	batch, err := readRelationDocument(*batchPath, relation.DecodeJudgmentBatch)
	if err != nil {
		return relationReviewInputError("probe-batch", err)
	}
	drafts, err := readRelationDocument(*draftsPath, relation.DecodeConditionProbeDrafts)
	if err != nil {
		return relationReviewInputError("probe-batch", err)
	}
	probes, err := relation.BuildConditionProbeBatch(bundle, assignment, batch, drafts, *committedAt)
	if err != nil {
		return relationReviewOperationError("probe-batch", err)
	}
	return printRelationJSON(probes)
}

func runRelationReveal(args []string) int {
	flags := flag.NewFlagSet("relation reveal", flag.ContinueOnError)
	bundlePath := flags.String("bundle", "", "sealed relation review bundle (@file)")
	mappingsPath := flags.String("mappings", "", "complete owner-only private-mapping array (@file)")
	leftAssignmentPath := flags.String("left-assignment", "", "first primary assignment (@file)")
	leftBatchPath := flags.String("left-batch", "", "first primary judgment batch (@file)")
	leftProbesPath := flags.String("left-probes", "", "first primary condition-probe batch (@file)")
	leftSeedPath := flags.String("left-seed-file", "", "owner-only first assignment ordering seed")
	rightAssignmentPath := flags.String("right-assignment", "", "second primary assignment (@file)")
	rightBatchPath := flags.String("right-batch", "", "second primary judgment batch (@file)")
	rightProbesPath := flags.String("right-probes", "", "second primary condition-probe batch (@file)")
	rightSeedPath := flags.String("right-seed-file", "", "owner-only second assignment ordering seed")
	ambiguityPath := flags.String("ambiguity", "", "sealed prereveal ambiguity analysis (@file)")
	tieAssignmentPath := flags.String("tie-assignment", "", "optional disagreement-only tie-break assignment (@file)")
	tieBatchPath := flags.String("tie-batch", "", "optional complete tie-break judgment batch (@file)")
	tieSeedPath := flags.String("tie-seed-file", "", "optional owner-only tie-break assignment ordering seed")
	revealedAt := flags.String("revealed-at", "", "RFC3339 reveal time after all commitments")
	revealedBy := flags.String("revealed-by", "", "pseudonymous reveal actor")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, err := readRelationPrimaryReviewInputs(*bundlePath, *leftAssignmentPath, *leftBatchPath, *rightAssignmentPath, *rightBatchPath)
	if err != nil {
		return relationReviewInputError("reveal", err)
	}
	mappings, err := readPrivateRelationDocument(*mappingsPath, relation.DecodePrivateMappings)
	if err != nil {
		return relationReviewInputError("reveal", err)
	}
	leftProbes, err := readRelationDocument(*leftProbesPath, relation.DecodeConditionProbeBatch)
	if err != nil {
		return relationReviewInputError("reveal", err)
	}
	rightProbes, err := readRelationDocument(*rightProbesPath, relation.DecodeConditionProbeBatch)
	if err != nil {
		return relationReviewInputError("reveal", err)
	}
	ambiguity, err := readRelationDocument(*ambiguityPath, relation.DecodeRelationAmbiguityAnalysis)
	if err != nil {
		return relationReviewInputError("reveal", err)
	}
	tieAssignment, tieBatch, err := readOptionalRelationTieInputs(*tieAssignmentPath, *tieBatchPath)
	if err != nil {
		return relationReviewInputError("reveal", err)
	}
	leftSeed, err := readRelationBlindingKeyFile(*leftSeedPath)
	if err != nil {
		return relationReviewInputError("reveal", err)
	}
	defer zeroBytes(leftSeed)
	rightSeed, err := readRelationBlindingKeyFile(*rightSeedPath)
	if err != nil {
		return relationReviewInputError("reveal", err)
	}
	defer zeroBytes(rightSeed)
	seeds := []relation.AssignmentSeed{{AssignmentDigest: leftAssignment.Digest, Seed: leftSeed}, {AssignmentDigest: rightAssignment.Digest, Seed: rightSeed}}
	if tieAssignment != nil {
		tieSeed, seedErr := readRelationBlindingKeyFile(*tieSeedPath)
		if seedErr != nil {
			return relationReviewInputError("reveal", seedErr)
		}
		defer zeroBytes(tieSeed)
		seeds = append(seeds, relation.AssignmentSeed{AssignmentDigest: tieAssignment.Digest, Seed: tieSeed})
	}
	reveal, err := relation.BuildMappingReveal(bundle, mappings, leftAssignment, leftBatch, leftProbes, rightAssignment, rightBatch, rightProbes, ambiguity, tieAssignment, tieBatch, seeds, *revealedAt, *revealedBy)
	if err != nil {
		return relationReviewOperationError("reveal", err)
	}
	return printRelationJSON(reveal)
}

func runRelationCompare(args []string) int {
	flags := flag.NewFlagSet("relation compare", flag.ContinueOnError)
	planPath := flags.String("plan", "", "governed relation audit plan (@file)")
	bundlePath := flags.String("bundle", "", "sealed relation review bundle (@file)")
	mappingsPath := flags.String("mappings", "", "complete owner-only private-mapping array (@file)")
	revealPath := flags.String("reveal", "", "sealed mapping reveal (@file)")
	ambiguityPath := flags.String("ambiguity", "", "sealed prereveal ambiguity analysis (@file)")
	leftAssignmentPath := flags.String("left-assignment", "", "first primary assignment (@file)")
	leftBatchPath := flags.String("left-batch", "", "first primary judgment batch (@file)")
	leftProbesPath := flags.String("left-probes", "", "first primary condition-probe batch (@file)")
	rightAssignmentPath := flags.String("right-assignment", "", "second primary assignment (@file)")
	rightBatchPath := flags.String("right-batch", "", "second primary judgment batch (@file)")
	rightProbesPath := flags.String("right-probes", "", "second primary condition-probe batch (@file)")
	tieAssignmentPath := flags.String("tie-assignment", "", "optional disagreement-only tie-break assignment (@file)")
	tieBatchPath := flags.String("tie-batch", "", "optional complete tie-break judgment batch (@file)")
	completedAt := flags.String("completed-at", "", "RFC3339 formal-human comparison time after reveal")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, err := readRelationDocument(*planPath, relation.DecodeReviewPlan)
	if err != nil {
		return relationReviewInputError("compare", err)
	}
	bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, err := readRelationPrimaryReviewInputs(*bundlePath, *leftAssignmentPath, *leftBatchPath, *rightAssignmentPath, *rightBatchPath)
	if err != nil {
		return relationReviewInputError("compare", err)
	}
	mappings, err := readPrivateRelationDocument(*mappingsPath, relation.DecodePrivateMappings)
	if err != nil {
		return relationReviewInputError("compare", err)
	}
	reveal, err := readRelationDocument(*revealPath, relation.DecodeMappingReveal)
	if err != nil {
		return relationReviewInputError("compare", err)
	}
	ambiguity, err := readRelationDocument(*ambiguityPath, relation.DecodeRelationAmbiguityAnalysis)
	if err != nil {
		return relationReviewInputError("compare", err)
	}
	leftProbes, err := readRelationDocument(*leftProbesPath, relation.DecodeConditionProbeBatch)
	if err != nil {
		return relationReviewInputError("compare", err)
	}
	rightProbes, err := readRelationDocument(*rightProbesPath, relation.DecodeConditionProbeBatch)
	if err != nil {
		return relationReviewInputError("compare", err)
	}
	tieAssignment, tieBatch, err := readOptionalRelationTieInputs(*tieAssignmentPath, *tieBatchPath)
	if err != nil {
		return relationReviewInputError("compare", err)
	}
	result, err := relation.BuildFormalHumanComparison(
		plan, bundle, mappings, reveal, ambiguity,
		leftAssignment, leftBatch, leftProbes, rightAssignment, rightBatch, rightProbes,
		tieAssignment, tieBatch, *completedAt,
	)
	if err != nil {
		return relationReviewOperationError("compare", err)
	}
	return printRelationJSON(result)
}

func runRelationTerminalLedger(args []string) int {
	flags := flag.NewFlagSet("relation terminal-ledger", flag.ContinueOnError)
	bundlePath := flags.String("bundle", "", "sealed relation review bundle (@file)")
	revealPath := flags.String("reveal", "", "sealed mapping reveal (@file)")
	ambiguityPath := flags.String("ambiguity", "", "sealed prereveal ambiguity analysis (@file)")
	comparisonPath := flags.String("comparison", "", "sealed formal-human comparison (@file)")
	resolutionsPath := flags.String("resolutions", "", "complete relation-resolution array (@file)")
	completedAt := flags.String("completed-at", "", "RFC3339 terminal-ledger time after comparison")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	bundle, err := readRelationDocument(*bundlePath, relation.DecodeReviewBundle)
	if err != nil {
		return relationReviewInputError("terminal-ledger", err)
	}
	reveal, err := readRelationDocument(*revealPath, relation.DecodeMappingReveal)
	if err != nil {
		return relationReviewInputError("terminal-ledger", err)
	}
	ambiguity, err := readRelationDocument(*ambiguityPath, relation.DecodeRelationAmbiguityAnalysis)
	if err != nil {
		return relationReviewInputError("terminal-ledger", err)
	}
	comparison, err := readRelationDocument(*comparisonPath, relation.DecodeFormalHumanComparison)
	if err != nil {
		return relationReviewInputError("terminal-ledger", err)
	}
	resolutions, err := readRelationDocument(*resolutionsPath, relation.DecodeRelationResolutions)
	if err != nil {
		return relationReviewInputError("terminal-ledger", err)
	}
	ledger, err := relation.BuildTerminalRelationLedger(bundle, reveal, ambiguity, comparison, resolutions, *completedAt)
	if err != nil {
		return relationReviewOperationError("terminal-ledger", err)
	}
	return printRelationJSON(ledger)
}

func readOptionalRelationTieInputs(assignmentPath, batchPath string) (*relation.ReviewAssignment, *relation.JudgmentBatch, error) {
	hasAssignment, hasBatch := strings.TrimSpace(assignmentPath) != "", strings.TrimSpace(batchPath) != ""
	if hasAssignment != hasBatch {
		return nil, nil, errors.New("tie-break assignment and batch must be supplied together")
	}
	if !hasAssignment {
		return nil, nil, nil
	}
	assignment, err := readRelationDocument(assignmentPath, relation.DecodeReviewAssignment)
	if err != nil {
		return nil, nil, err
	}
	batch, err := readRelationDocument(batchPath, relation.DecodeJudgmentBatch)
	if err != nil {
		return nil, nil, err
	}
	return &assignment, &batch, nil
}

func readRelationPrimaryReviewInputs(bundlePath, leftAssignmentPath, leftBatchPath, rightAssignmentPath, rightBatchPath string) (relation.ReviewBundle, relation.ReviewAssignment, relation.JudgmentBatch, relation.ReviewAssignment, relation.JudgmentBatch, error) {
	bundle, err := readRelationDocument(bundlePath, relation.DecodeReviewBundle)
	if err != nil {
		return relation.ReviewBundle{}, relation.ReviewAssignment{}, relation.JudgmentBatch{}, relation.ReviewAssignment{}, relation.JudgmentBatch{}, err
	}
	leftAssignment, err := readRelationDocument(leftAssignmentPath, relation.DecodeReviewAssignment)
	if err != nil {
		return relation.ReviewBundle{}, relation.ReviewAssignment{}, relation.JudgmentBatch{}, relation.ReviewAssignment{}, relation.JudgmentBatch{}, err
	}
	leftBatch, err := readRelationDocument(leftBatchPath, relation.DecodeJudgmentBatch)
	if err != nil {
		return relation.ReviewBundle{}, relation.ReviewAssignment{}, relation.JudgmentBatch{}, relation.ReviewAssignment{}, relation.JudgmentBatch{}, err
	}
	rightAssignment, err := readRelationDocument(rightAssignmentPath, relation.DecodeReviewAssignment)
	if err != nil {
		return relation.ReviewBundle{}, relation.ReviewAssignment{}, relation.JudgmentBatch{}, relation.ReviewAssignment{}, relation.JudgmentBatch{}, err
	}
	rightBatch, err := readRelationDocument(rightBatchPath, relation.DecodeJudgmentBatch)
	return bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, err
}

func readRelationDocument[T any](path string, decoder func(io.Reader) (T, error)) (T, error) {
	var zero T
	reader, closeDocument, err := openStudyDocument(path)
	if err != nil {
		return zero, err
	}
	defer closeDocument()
	return decoder(reader)
}

func loadReviewPilotSample(path, planPath, primaryV3Path, sentinelV3Path string, reviewPlan relation.Plan) (relation.PilotSample, error) {
	if reviewPlan.ProtocolVersion != relation.ProtocolVersionV3 {
		return readRelationDocument(path, relation.DecodePilotSample)
	}
	governedPlan, err := loadRelationPlanV3(planPath)
	if err != nil {
		return relation.PilotSample{}, err
	}
	primary, err := loadRelationPrimaryV3(primaryV3Path, governedPlan)
	if err != nil {
		return relation.PilotSample{}, err
	}
	sentinel, err := loadRelationSentinelV3(sentinelV3Path, governedPlan, primary)
	if err != nil {
		return relation.PilotSample{}, err
	}
	pilot, err := readRelationDocument(path, func(reader io.Reader) (relation.PilotSampleV3, error) {
		return relation.DecodePilotSampleV3(reader, governedPlan, primary, sentinel)
	})
	if err != nil {
		return relation.PilotSample{}, err
	}
	return relation.ReviewPilotSampleV3(governedPlan, primary, sentinel, pilot)
}

func readPrivateRelationDocument[T any](path string, decoder func(io.Reader) (T, error)) (value T, returnErr error) {
	var zero T
	path = strings.TrimPrefix(strings.TrimSpace(path), "@")
	if path == "" || path == "-" {
		return zero, errors.New("owner-only relation document path is required and stdin is forbidden")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return zero, err
	}
	if !info.Mode().IsRegular() {
		return zero, errors.New("owner-only relation document must be a regular file")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return zero, errors.New("owner-only relation document permissions must not grant group or other access")
	}
	file, err := os.Open(path)
	if err != nil {
		return zero, err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("close owner-only relation document: %w", closeErr)
		}
	}()
	return decoder(file)
}

func readRelationKitInputs(bundlePath, assignmentPath, handbookPath string) (relation.ReviewBundle, relation.ReviewAssignment, relation.ReviewerHandbook, error) {
	bundle, err := readRelationDocument(bundlePath, relation.DecodeReviewBundle)
	if err != nil {
		return relation.ReviewBundle{}, relation.ReviewAssignment{}, relation.ReviewerHandbook{}, err
	}
	assignment, err := readRelationDocument(assignmentPath, relation.DecodeReviewAssignment)
	if err != nil {
		return relation.ReviewBundle{}, relation.ReviewAssignment{}, relation.ReviewerHandbook{}, err
	}
	handbook, err := readRelationDocument(handbookPath, relation.DecodeReviewerHandbook)
	return bundle, assignment, handbook, err
}

func createRelationPrivateRoot(path string) (*safety.CacheRoot, error) {
	policy, err := safety.CurrentPathPolicy()
	if err != nil {
		return nil, err
	}
	return safety.CreateCacheRoot(policy, strings.TrimSpace(path))
}

func relationReviewInputError(command string, err error) int {
	fmt.Fprintf(os.Stderr, "relation %s: %v\n", command, err)
	return 2
}

func relationReviewOperationError(command string, err error) int {
	fmt.Fprintf(os.Stderr, "relation %s: %v\n", command, err)
	return 1
}
