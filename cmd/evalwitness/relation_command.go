package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/relation"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

func runRelation(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "relation: usage: evalwitness relation <analyze-ambiguity|assign-primary|assign-tie|bundle|compare|handbook|judgment|judgment-batch|kit|materialize|materialize-v3|packet|packet-v3|pilot-change-receipt|pilot-inspection|pilot-inspection-session-finalize|pilot-inspection-session-guide|pilot-inspection-session-record|pilot-inspection-session-start|pilot-inspection-session-status|pilot-launch-dossier|plan|plan-v2|plan-v3|pilot-readiness|pilot-sample|pilot-sample-v3|primary-sample|primary-sample-v3|scarcity-sentinel-v3|probe-batch|qualification|qualify|render-kit|render-owner-inspection-public-attestation|render-pilot-change-atlas|render-pilot-inspection|render-pilot-launch-brief|render-scarcity-inspection|render-scarcity-public-brief|replay|replay-v3|reveal|reviewer|study-amendment|study-amendment-v3|terminal-ledger|translate|validate|verify-kit|verify-pilot-inspection|schema>")
		return 2
	}
	switch args[0] {
	case "analyze-ambiguity":
		return runRelationAnalyzeAmbiguity(args[1:])
	case "assign-primary":
		return runRelationAssignPrimary(args[1:])
	case "assign-tie":
		return runRelationAssignTie(args[1:])
	case "bundle":
		return runRelationBundle(args[1:])
	case "compare":
		return runRelationCompare(args[1:])
	case "handbook":
		return runRelationHandbook(args[1:])
	case "judgment":
		return runRelationJudgment(args[1:])
	case "judgment-batch":
		return runRelationJudgmentBatch(args[1:])
	case "kit":
		return runRelationKit(args[1:])
	case "materialize":
		return runRelationMaterialize(args[1:])
	case "materialize-v3":
		return runRelationMaterializeV3(args[1:])
	case "packet":
		return runRelationPacket(args[1:])
	case "packet-v3":
		return runRelationPacketV3(args[1:])
	case "pilot-change-receipt":
		return runRelationPilotChangeReceipt(args[1:])
	case "pilot-inspection":
		return runRelationPilotInspection(args[1:])
	case "pilot-inspection-session-finalize":
		return runRelationPilotInspectionSessionFinalize(args[1:])
	case "pilot-inspection-session-guide":
		return runRelationPilotInspectionSessionGuide(args[1:])
	case "pilot-inspection-session-record":
		return runRelationPilotInspectionSessionRecord(args[1:])
	case "pilot-inspection-session-start":
		return runRelationPilotInspectionSessionStart(args[1:])
	case "pilot-inspection-session-status":
		return runRelationPilotInspectionSessionStatus(args[1:])
	case "pilot-launch-dossier":
		return runRelationPilotLaunchDossier(args[1:])
	case "plan":
		return runRelationPlan(args[1:])
	case "plan-v2":
		return runRelationPlanV2(args[1:])
	case "plan-v3":
		return runRelationPlanV3(args[1:])
	case "pilot-sample":
		return runRelationPilotSample(args[1:])
	case "pilot-sample-v3":
		return runRelationPilotSampleV3(args[1:])
	case "pilot-readiness":
		return runRelationPilotReadiness(args[1:])
	case "primary-sample":
		return runRelationPrimarySample(args[1:])
	case "primary-sample-v3":
		return runRelationPrimarySampleV3(args[1:])
	case "scarcity-sentinel-v3":
		return runRelationScarcitySentinelV3(args[1:])
	case "probe-batch":
		return runRelationProbeBatch(args[1:])
	case "qualification":
		return runRelationQualification(args[1:])
	case "qualify":
		return runRelationQualify(args[1:])
	case "render-kit":
		return runRelationRenderKit(args[1:])
	case "render-owner-inspection-public-attestation":
		return runRelationRenderOwnerInspectionPublicAttestation(args[1:])
	case "render-pilot-change-atlas":
		return runRelationRenderPilotChangeAtlas(args[1:])
	case "render-pilot-inspection":
		return runRelationRenderPilotInspection(args[1:])
	case "render-pilot-launch-brief":
		return runRelationRenderPilotLaunchBrief(args[1:])
	case "render-scarcity-inspection":
		return runRelationRenderScarcityInspection(args[1:])
	case "render-scarcity-public-brief":
		return runRelationRenderScarcityPublicBrief(args[1:])
	case "replay":
		return runRelationReplay(args[1:])
	case "replay-v3":
		return runRelationReplayV3(args[1:])
	case "reveal":
		return runRelationReveal(args[1:])
	case "reviewer":
		return runRelationReviewer(args[1:])
	case "study-amendment":
		return runRelationStudyAmendment(args[1:])
	case "study-amendment-v3":
		return runRelationStudyAmendmentV3(args[1:])
	case "terminal-ledger":
		return runRelationTerminalLedger(args[1:])
	case "translate":
		return runRelationTranslate(args[1:])
	case "validate":
		return runRelationValidate(args[1:])
	case "verify-kit":
		return runRelationVerifyKit(args[1:])
	case "verify-pilot-inspection":
		return runRelationVerifyPilotInspection(args[1:])
	case "schema":
		return runRelationSchema(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "relation: unknown command %q\n", args[0])
		return 2
	}
}

func runRelationPacket(args []string) int {
	flags := flag.NewFlagSet("relation packet", flag.ContinueOnError)
	planPath := flags.String("plan", "", "governed relation audit plan (@file)")
	releasePath := flags.String("release", "", "controlled-corruption release (@file)")
	materialPath := flags.String("material", "", "validated relation case material (@file)")
	keyPath := flags.String("key-file", "", "owner-only 32-byte hexadecimal blinding key file")
	keyID := flags.String("key-id", "", "owner-managed blinding key identity")
	privateRootPath := flags.String("private-root", "", "owner-only mapping vault outside the repository root")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	planReader, closePlan, err := openStudyDocument(*planPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation packet:", err)
		return 2
	}
	defer closePlan()
	plan, err := relation.DecodePlan(planReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation packet:", err)
		return 2
	}
	releaseReader, closeRelease, err := openStudyDocument(*releasePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation packet:", err)
		return 2
	}
	defer closeRelease()
	release, err := mutation.DecodeCorpusRelease(releaseReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation packet:", err)
		return 2
	}
	materialReader, closeMaterial, err := openStudyDocument(*materialPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation packet:", err)
		return 2
	}
	defer closeMaterial()
	material, err := relation.DecodeCaseMaterial(materialReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation packet:", err)
		return 2
	}
	key, err := readRelationBlindingKeyFile(*keyPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation packet:", err)
		return 2
	}
	defer func() {
		for index := range key {
			key[index] = 0
		}
	}()
	packet, mapping, err := relation.BuildBlindedPacket(plan, release, material, strings.TrimSpace(*keyID), key)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation packet:", err)
		return 1
	}
	encodedMapping, err := relation.EncodeIndented(mapping)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation packet: encode private mapping:", err)
		return 1
	}
	policy, err := safety.CurrentPathPolicy()
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation packet:", err)
		return 1
	}
	privateRoot, err := safety.CreateCacheRoot(policy, strings.TrimSpace(*privateRootPath))
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation packet:", err)
		return 1
	}
	privatePath := filepath.Join("mappings", mapping.Digest+".json")
	if err := privateRoot.PublishSensitiveExclusive(privatePath, encodedMapping); err != nil {
		fmt.Fprintln(os.Stderr, "relation packet: publish private mapping:", err)
		return 1
	}
	return printRelationJSON(packet)
}

func runRelationPacketV3(args []string) int {
	flags := flag.NewFlagSet("relation packet-v3", flag.ContinueOnError)
	planPath := flags.String("plan", "", "governed v3 relation audit plan (@file)")
	corpusPlanPath := flags.String("corpus-plan", "", "frozen v3 corpus development plan (@file)")
	auditPath := flags.String("corpus-audit", "", "frozen v3 corpus development audit (@file)")
	releasePath := flags.String("release", "", "frozen v3 controlled-corruption release (@file)")
	materialPath := flags.String("material", "", "validated v3 relation case material (@file)")
	keyPath := flags.String("key-file", "", "owner-only 32-byte hexadecimal blinding key file")
	keyID := flags.String("key-id", "", "owner-managed blinding key identity")
	privateRootPath := flags.String("private-root", "", "owner-only mapping vault outside the repository root")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	governedPlan, err := loadRelationPlanV3(*planPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation packet-v3:", err)
		return 2
	}
	plan, err := relation.ReviewPlanV3(governedPlan)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation packet-v3:", err)
		return 2
	}
	corpusPlan, audit, release, err := loadRelationCorpusV3(*corpusPlanPath, *auditPath, *releasePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation packet-v3:", err)
		return 2
	}
	materialReader, closeMaterial, err := openStudyDocument(*materialPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation packet-v3:", err)
		return 2
	}
	defer closeMaterial()
	material, err := relation.DecodeCaseMaterial(materialReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation packet-v3:", err)
		return 2
	}
	key, err := readRelationBlindingKeyFile(*keyPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation packet-v3:", err)
		return 2
	}
	defer zeroBytes(key)
	packet, mapping, err := relation.BuildBlindedPacketV3(plan, corpusPlan, audit, release, material, strings.TrimSpace(*keyID), key)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation packet-v3:", err)
		return 1
	}
	encodedMapping, err := relation.EncodeIndented(mapping)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation packet-v3: encode private mapping:", err)
		return 1
	}
	policy, err := safety.CurrentPathPolicy()
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation packet-v3:", err)
		return 1
	}
	privateRoot, err := safety.CreateCacheRoot(policy, strings.TrimSpace(*privateRootPath))
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation packet-v3:", err)
		return 1
	}
	privatePath := filepath.Join("mappings", mapping.Digest+".json")
	if err := privateRoot.PublishSensitiveExclusive(privatePath, encodedMapping); err != nil {
		fmt.Fprintln(os.Stderr, "relation packet-v3: publish private mapping:", err)
		return 1
	}
	return printRelationJSON(packet)
}

func runRelationPlan(args []string) int {
	flags := flag.NewFlagSet("relation plan", flag.ContinueOnError)
	path := flags.String("plan", "", "optional governed relation audit plan (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *path == "" {
		plan, err := relation.DefaultPlan()
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation plan:", err)
			return 1
		}
		return printRelationJSON(plan)
	}
	reader, closePlan, err := openStudyDocument(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation plan:", err)
		return 2
	}
	defer closePlan()
	plan, err := relation.DecodePlan(reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation plan:", err)
		return 2
	}
	return printRelationJSON(plan)
}

func runRelationPlanV2(args []string) int {
	flags := flag.NewFlagSet("relation plan-v2", flag.ContinueOnError)
	releasePath := flags.String("release", "", "frozen v2 controlled-corruption release (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	releaseReader, closeRelease, err := openStudyDocument(*releasePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation plan-v2:", err)
		return 2
	}
	defer closeRelease()
	release, err := mutation.DecodeCorpusRelease(releaseReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation plan-v2:", err)
		return 2
	}
	plan, err := relation.BuildPlanV2(release)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation plan-v2:", err)
		return 1
	}
	return printRelationJSON(plan)
}

func runRelationPlanV3(args []string) int {
	flags := flag.NewFlagSet("relation plan-v3", flag.ContinueOnError)
	corpusPlanPath := flags.String("corpus-plan", "", "frozen v3 corpus development plan (@file)")
	auditPath := flags.String("corpus-audit", "", "frozen v3 corpus development audit (@file)")
	releasePath := flags.String("release", "", "frozen v3 controlled-corruption release (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	corpusPlan, audit, release, err := loadRelationCorpusV3(*corpusPlanPath, *auditPath, *releasePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation plan-v3:", err)
		return 2
	}
	plan, err := relation.BuildPlanV3(corpusPlan, audit, release)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation plan-v3:", err)
		return 1
	}
	return printRelationJSON(plan)
}

func runRelationPrimarySampleV3(args []string) int {
	flags := flag.NewFlagSet("relation primary-sample-v3", flag.ContinueOnError)
	planPath := flags.String("plan", "", "governed v3 relation audit plan (@file)")
	corpusPlanPath := flags.String("corpus-plan", "", "frozen v3 corpus development plan (@file)")
	auditPath := flags.String("corpus-audit", "", "frozen v3 corpus development audit (@file)")
	releasePath := flags.String("release", "", "frozen v3 controlled-corruption release (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, err := loadRelationPlanV3(*planPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation primary-sample-v3:", err)
		return 2
	}
	corpusPlan, audit, release, err := loadRelationCorpusV3(*corpusPlanPath, *auditPath, *releasePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation primary-sample-v3:", err)
		return 2
	}
	expectedPlan, err := relation.BuildPlanV3(corpusPlan, audit, release)
	if err != nil || expectedPlan.Digest != plan.Digest {
		fmt.Fprintln(os.Stderr, "relation primary-sample-v3: relation plan does not reproduce from the frozen corpus")
		return 1
	}
	sample, err := relation.BuildPrimarySampleV3(plan, release)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation primary-sample-v3:", err)
		return 1
	}
	return printRelationJSON(sample)
}

func runRelationScarcitySentinelV3(args []string) int {
	flags := flag.NewFlagSet("relation scarcity-sentinel-v3", flag.ContinueOnError)
	planPath := flags.String("plan", "", "governed v3 relation audit plan (@file)")
	primaryPath := flags.String("primary-sample", "", "governed v3 primary sample (@file)")
	corpusPlanPath := flags.String("corpus-plan", "", "frozen v3 corpus development plan (@file)")
	auditPath := flags.String("corpus-audit", "", "frozen v3 corpus development audit (@file)")
	releasePath := flags.String("release", "", "frozen v3 controlled-corruption release (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, err := loadRelationPlanV3(*planPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation scarcity-sentinel-v3:", err)
		return 2
	}
	primary, err := loadRelationPrimaryV3(*primaryPath, plan)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation scarcity-sentinel-v3:", err)
		return 2
	}
	corpusPlan, audit, release, err := loadRelationCorpusV3(*corpusPlanPath, *auditPath, *releasePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation scarcity-sentinel-v3:", err)
		return 2
	}
	if expectedPlan, buildErr := relation.BuildPlanV3(corpusPlan, audit, release); buildErr != nil || expectedPlan.Digest != plan.Digest {
		fmt.Fprintln(os.Stderr, "relation scarcity-sentinel-v3: relation plan does not reproduce from the frozen corpus")
		return 1
	}
	sentinel, err := relation.BuildScarcitySentinelV3(plan, primary, release)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation scarcity-sentinel-v3:", err)
		return 1
	}
	return printRelationJSON(sentinel)
}

func runRelationPilotSampleV3(args []string) int {
	flags := flag.NewFlagSet("relation pilot-sample-v3", flag.ContinueOnError)
	planPath := flags.String("plan", "", "governed v3 relation audit plan (@file)")
	primaryPath := flags.String("primary-sample", "", "governed v3 primary sample (@file)")
	sentinelPath := flags.String("scarcity-sentinel", "", "governed v3 scarcity sentinel (@file)")
	corpusPlanPath := flags.String("corpus-plan", "", "frozen v3 corpus development plan (@file)")
	auditPath := flags.String("corpus-audit", "", "frozen v3 corpus development audit (@file)")
	releasePath := flags.String("release", "", "frozen v3 controlled-corruption release (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, err := loadRelationPlanV3(*planPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation pilot-sample-v3:", err)
		return 2
	}
	primary, err := loadRelationPrimaryV3(*primaryPath, plan)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation pilot-sample-v3:", err)
		return 2
	}
	sentinel, err := loadRelationSentinelV3(*sentinelPath, plan, primary)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation pilot-sample-v3:", err)
		return 2
	}
	corpusPlan, audit, release, err := loadRelationCorpusV3(*corpusPlanPath, *auditPath, *releasePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation pilot-sample-v3:", err)
		return 2
	}
	if expectedPlan, buildErr := relation.BuildPlanV3(corpusPlan, audit, release); buildErr != nil || expectedPlan.Digest != plan.Digest {
		fmt.Fprintln(os.Stderr, "relation pilot-sample-v3: relation plan does not reproduce from the frozen corpus")
		return 1
	}
	pilot, err := relation.BuildPilotSampleV3(plan, primary, sentinel, release)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation pilot-sample-v3:", err)
		return 1
	}
	return printRelationJSON(pilot)
}

func runRelationStudyAmendmentV3(args []string) int {
	flags := flag.NewFlagSet("relation study-amendment-v3", flag.ContinueOnError)
	planPath := flags.String("plan", "", "governed v3 relation audit plan (@file)")
	primaryPath := flags.String("primary-sample", "", "governed v3 primary sample (@file)")
	sentinelPath := flags.String("scarcity-sentinel", "", "governed v3 scarcity sentinel (@file)")
	pilotPath := flags.String("pilot-sample", "", "governed v3 pilot sample (@file)")
	issuedAt := flags.String("issued-at", "", "preregistration amendment time (RFC3339 UTC)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, err := loadRelationPlanV3(*planPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation study-amendment-v3:", err)
		return 2
	}
	primary, err := loadRelationPrimaryV3(*primaryPath, plan)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation study-amendment-v3:", err)
		return 2
	}
	sentinel, err := loadRelationSentinelV3(*sentinelPath, plan, primary)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation study-amendment-v3:", err)
		return 2
	}
	pilotReader, closePilot, err := openStudyDocument(*pilotPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation study-amendment-v3:", err)
		return 2
	}
	defer closePilot()
	pilot, err := relation.DecodePilotSampleV3(pilotReader, plan, primary, sentinel)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation study-amendment-v3:", err)
		return 2
	}
	amendment, err := relation.BuildStudyAmendmentV3(plan, pilot, primary, sentinel, *issuedAt)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation study-amendment-v3:", err)
		return 1
	}
	return printRelationJSON(amendment)
}

func loadRelationCorpusV3(planPath, auditPath, releasePath string) (mutation.CorpusDevelopmentPlan, mutation.CorpusDevelopmentAuditV3, mutation.CorpusReleaseV3, error) {
	plan, err := loadCorpusDevelopmentPlanV3(planPath)
	if err != nil {
		return mutation.CorpusDevelopmentPlan{}, mutation.CorpusDevelopmentAuditV3{}, mutation.CorpusReleaseV3{}, err
	}
	audit, err := loadCorpusDevelopmentAuditV3(auditPath, plan)
	if err != nil {
		return mutation.CorpusDevelopmentPlan{}, mutation.CorpusDevelopmentAuditV3{}, mutation.CorpusReleaseV3{}, err
	}
	releaseReader, closeRelease, err := openStudyDocument(releasePath)
	if err != nil {
		return mutation.CorpusDevelopmentPlan{}, mutation.CorpusDevelopmentAuditV3{}, mutation.CorpusReleaseV3{}, err
	}
	defer closeRelease()
	release, err := mutation.DecodeCorpusReleaseV3(releaseReader, plan, audit)
	if err != nil {
		return mutation.CorpusDevelopmentPlan{}, mutation.CorpusDevelopmentAuditV3{}, mutation.CorpusReleaseV3{}, err
	}
	return plan, audit, release, nil
}

func loadRelationPlanV3(path string) (relation.RelationPlanV3, error) {
	reader, closeDocument, err := openStudyDocument(path)
	if err != nil {
		return relation.RelationPlanV3{}, err
	}
	defer closeDocument()
	return relation.DecodePlanV3(reader)
}

func loadRelationPrimaryV3(path string, plan relation.RelationPlanV3) (relation.PrimarySampleV3, error) {
	reader, closeDocument, err := openStudyDocument(path)
	if err != nil {
		return relation.PrimarySampleV3{}, err
	}
	defer closeDocument()
	return relation.DecodePrimarySampleV3(reader, plan)
}

func loadRelationSentinelV3(path string, plan relation.RelationPlanV3, primary relation.PrimarySampleV3) (relation.ScarcitySentinelV3, error) {
	reader, closeDocument, err := openStudyDocument(path)
	if err != nil {
		return relation.ScarcitySentinelV3{}, err
	}
	defer closeDocument()
	return relation.DecodeScarcitySentinelV3(reader, plan, primary)
}

func runRelationMaterialize(args []string) int {
	flags := flag.NewFlagSet("relation materialize", flag.ContinueOnError)
	rootPath := flags.String("root", ".", "repository root containing fetched trajectory sources")
	planPath := flags.String("plan", "", "governed relation audit plan (@file)")
	releasePath := flags.String("release", "", "controlled-corruption release (@file)")
	caseID := flags.String("case-id", "", "exact frozen mutation case identity")
	budget := flags.Int("evidence-budget", relation.RelationEvidenceBudgetTokens, "fixed evidence budget per source trajectory")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	planReader, closePlan, err := openStudyDocument(*planPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation materialize:", err)
		return 2
	}
	defer closePlan()
	plan, err := relation.DecodePlan(planReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation materialize:", err)
		return 2
	}
	releaseReader, closeRelease, err := openStudyDocument(*releasePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation materialize:", err)
		return 2
	}
	defer closeRelease()
	release, err := mutation.DecodeCorpusRelease(releaseReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation materialize:", err)
		return 2
	}
	material, err := relation.MaterializeCase(*rootPath, plan, release, *caseID, *budget)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation materialize:", err)
		return 1
	}
	return printRelationJSON(material)
}

func runRelationMaterializeV3(args []string) int {
	flags := flag.NewFlagSet("relation materialize-v3", flag.ContinueOnError)
	rootPath := flags.String("root", ".", "repository root containing fetched trajectory sources")
	planPath := flags.String("plan", "", "governed v3 relation audit plan (@file)")
	corpusPlanPath := flags.String("corpus-plan", "", "frozen v3 corpus development plan (@file)")
	auditPath := flags.String("corpus-audit", "", "frozen v3 corpus development audit (@file)")
	releasePath := flags.String("release", "", "frozen v3 controlled-corruption release (@file)")
	caseID := flags.String("case-id", "", "exact frozen v3 mutation case identity")
	budget := flags.Int("evidence-budget", relation.RelationEvidenceBudgetTokens, "fixed evidence budget per source trajectory")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	governedPlan, err := loadRelationPlanV3(*planPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation materialize-v3:", err)
		return 2
	}
	plan, err := relation.ReviewPlanV3(governedPlan)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation materialize-v3:", err)
		return 2
	}
	corpusPlan, audit, release, err := loadRelationCorpusV3(*corpusPlanPath, *auditPath, *releasePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation materialize-v3:", err)
		return 2
	}
	material, err := relation.MaterializeCaseV3(*rootPath, plan, corpusPlan, audit, release, *caseID, *budget)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation materialize-v3:", err)
		return 1
	}
	return printRelationJSON(material)
}

func runRelationPrimarySample(args []string) int {
	flags := flag.NewFlagSet("relation primary-sample", flag.ContinueOnError)
	planPath := flags.String("plan", "", "governed relation audit plan (@file)")
	releasePath := flags.String("release", "", "controlled-corruption release (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	planReader, closePlan, err := openStudyDocument(*planPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation primary-sample:", err)
		return 2
	}
	defer closePlan()
	plan, err := relation.DecodePlan(planReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation primary-sample:", err)
		return 2
	}
	releaseReader, closeRelease, err := openStudyDocument(*releasePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation primary-sample:", err)
		return 2
	}
	defer closeRelease()
	release, err := mutation.DecodeCorpusRelease(releaseReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation primary-sample:", err)
		return 2
	}
	sample, err := relation.BuildPrimarySample(plan, release)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation primary-sample:", err)
		return 1
	}
	return printRelationJSON(sample)
}

func runRelationPilotSample(args []string) int {
	flags := flag.NewFlagSet("relation pilot-sample", flag.ContinueOnError)
	planPath := flags.String("plan", "", "governed relation audit plan (@file)")
	primaryPath := flags.String("primary-sample", "", "governed relation primary sample (@file)")
	releasePath := flags.String("release", "", "controlled-corruption release (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	planReader, closePlan, err := openStudyDocument(*planPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation pilot-sample:", err)
		return 2
	}
	defer closePlan()
	plan, err := relation.DecodePlan(planReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation pilot-sample:", err)
		return 2
	}
	primaryReader, closePrimary, err := openStudyDocument(*primaryPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation pilot-sample:", err)
		return 2
	}
	defer closePrimary()
	primary, err := relation.DecodePrimarySample(primaryReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation pilot-sample:", err)
		return 2
	}
	releaseReader, closeRelease, err := openStudyDocument(*releasePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation pilot-sample:", err)
		return 2
	}
	defer closeRelease()
	release, err := mutation.DecodeCorpusRelease(releaseReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation pilot-sample:", err)
		return 2
	}
	sample, err := relation.BuildPilotSample(plan, primary, release)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation pilot-sample:", err)
		return 1
	}
	return printRelationJSON(sample)
}

func runRelationReplay(args []string) int {
	flags := flag.NewFlagSet("relation replay", flag.ContinueOnError)
	rootPath := flags.String("root", ".", "repository root containing fetched trajectory sources")
	releasePath := flags.String("release", "", "controlled-corruption release (@file)")
	caseID := flags.String("case-id", "", "exact frozen mutation case identity")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	releaseReader, closeRelease, err := openStudyDocument(*releasePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation replay:", err)
		return 2
	}
	defer closeRelease()
	release, err := mutation.DecodeCorpusRelease(releaseReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation replay:", err)
		return 2
	}
	material, err := relation.ReplayCase(*rootPath, release, *caseID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation replay:", err)
		return 1
	}
	return printRelationJSON(material.Receipt)
}

func runRelationReplayV3(args []string) int {
	flags := flag.NewFlagSet("relation replay-v3", flag.ContinueOnError)
	rootPath := flags.String("root", ".", "repository root containing fetched trajectory sources")
	corpusPlanPath := flags.String("corpus-plan", "", "frozen v3 corpus development plan (@file)")
	auditPath := flags.String("corpus-audit", "", "frozen v3 corpus development audit (@file)")
	releasePath := flags.String("release", "", "frozen v3 controlled-corruption release (@file)")
	caseID := flags.String("case-id", "", "exact frozen v3 mutation case identity")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	corpusPlan, audit, release, err := loadRelationCorpusV3(*corpusPlanPath, *auditPath, *releasePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation replay-v3:", err)
		return 2
	}
	material, err := relation.ReplayCaseV3(*rootPath, corpusPlan, audit, release, *caseID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation replay-v3:", err)
		return 1
	}
	return printRelationJSON(material.Receipt)
}

func runRelationTranslate(args []string) int {
	flags := flag.NewFlagSet("relation translate", flag.ContinueOnError)
	planPath := flags.String("plan", "", "governed relation audit plan (@file)")
	family := flags.String("family", "", "revealed controlled-relation family")
	observationsPath := flags.String("observations", "", "post-reveal normalized axis observations (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	planReader, closePlan, err := openStudyDocument(*planPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation translate:", err)
		return 2
	}
	defer closePlan()
	plan, err := relation.DecodeReviewPlan(planReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation translate:", err)
		return 2
	}
	observationsReader, closeObservations, err := openStudyDocument(*observationsPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation translate:", err)
		return 2
	}
	defer closeObservations()
	observations, err := relation.DecodeObservations(observationsReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation translate:", err)
		return 2
	}
	result, err := relation.Translate(plan, mutation.Family(*family), observations)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation translate:", err)
		return 1
	}
	return printRelationJSON(result)
}

func runRelationStudyAmendment(args []string) int {
	flags := flag.NewFlagSet("relation study-amendment", flag.ContinueOnError)
	planPath := flags.String("plan", "", "governed relation audit plan (@file)")
	pilotPath := flags.String("pilot-sample", "", "governed relation pilot sample (@file)")
	samplePath := flags.String("primary-sample", "", "governed relation primary sample (@file)")
	issuedAt := flags.String("issued-at", "", "preregistration amendment time (RFC3339 UTC)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	planReader, closePlan, err := openStudyDocument(*planPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation study-amendment:", err)
		return 2
	}
	defer closePlan()
	plan, err := relation.DecodePlan(planReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation study-amendment:", err)
		return 2
	}
	pilotReader, closePilot, err := openStudyDocument(*pilotPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation study-amendment:", err)
		return 2
	}
	defer closePilot()
	pilot, err := relation.DecodePilotSample(pilotReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation study-amendment:", err)
		return 2
	}
	sampleReader, closeSample, err := openStudyDocument(*samplePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation study-amendment:", err)
		return 2
	}
	defer closeSample()
	sample, err := relation.DecodePrimarySample(sampleReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation study-amendment:", err)
		return 2
	}
	amendment, err := relation.BuildStudyAmendment(plan, pilot, sample, *issuedAt)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation study-amendment:", err)
		return 1
	}
	return printRelationJSON(amendment)
}

func runRelationValidate(args []string) int {
	flags := flag.NewFlagSet("relation validate", flag.ContinueOnError)
	documentType := flags.String("type", "", "named relation artifact type")
	path := flags.String("document", "", "relation document (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	reader, closeDocument, err := openStudyDocument(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation validate:", err)
		return 2
	}
	defer closeDocument()
	result := struct {
		Valid  bool   `json:"valid"`
		Type   string `json:"type"`
		Digest string `json:"digest"`
	}{Valid: true, Type: *documentType}
	switch *documentType {
	case "blind-packet":
		value, err := relation.DecodeBlindPacket(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "pilot-change-receipt":
		value, err := relation.DecodePilotChangeReceipt(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "case-material":
		value, err := relation.DecodeCaseMaterial(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "condition-probe":
		value, err := relation.DecodeConditionProbe(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "condition-probe-batch":
		value, err := relation.DecodeConditionProbeBatch(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "formal-human-comparison":
		value, err := relation.DecodeFormalHumanComparison(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "pair-judgment":
		value, err := relation.DecodePairJudgment(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "plan":
		value, err := relation.DecodePlan(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "pilot-sample":
		value, err := relation.DecodePilotSample(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "pilot-readiness":
		value, err := relation.DecodeRelationPilotReadiness(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "pilot-inspection":
		value, err := relation.DecodePilotInspectionRecord(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "pilot-launch-dossier":
		value, err := relation.DecodePilotLaunchDossier(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "primary-sample":
		value, err := relation.DecodePrimarySample(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "scarcity-public-evidence":
		value, err := relation.DecodeScarcityPublicEvidence(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "owner-inspection-public-attestation":
		value, err := relation.DecodeOwnerInspectionPublicAttestation(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "private-mapping":
		value, err := relation.DecodePrivateMapping(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "relation-resolution":
		value, err := relation.DecodeRelationResolution(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "qualification-answer-key":
		value, err := relation.DecodeQualificationAnswerKey(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "qualification-report":
		value, err := relation.DecodeQualificationReport(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "qualification-set":
		value, err := relation.DecodeQualificationSet(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "replay-receipt":
		value, err := relation.DecodeReplayReceipt(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "review-assignment":
		value, err := relation.DecodeReviewAssignment(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "review-bundle":
		value, err := relation.DecodeReviewBundle(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "reviewer-handbook":
		value, err := relation.DecodeReviewerHandbook(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "reviewer-kit":
		value, err := relation.DecodeReviewerKit(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "reviewer-record":
		value, err := relation.DecodeReviewerRecord(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "judgment-batch":
		value, err := relation.DecodeJudgmentBatch(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "mapping-reveal":
		value, err := relation.DecodeMappingReveal(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "prereveal-ambiguity":
		value, err := relation.DecodeRelationAmbiguityAnalysis(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "study-amendment":
		value, err := relation.DecodeStudyAmendment(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "terminal-ledger":
		value, err := relation.DecodeTerminalRelationLedger(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	case "translation-result":
		value, err := relation.DecodeTranslationResult(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "relation validate:", err)
			return 1
		}
		result.Digest = value.Digest
	default:
		fmt.Fprintln(os.Stderr, "relation validate: unknown --type")
		return 2
	}
	return printRelationJSON(result)
}

func runRelationSchema(args []string) int {
	flags := flag.NewFlagSet("relation schema", flag.ContinueOnError)
	documentType := flags.String("type", "", "named relation artifact type")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	schema, err := relation.Schema(*documentType)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation schema:", err)
		return 2
	}
	return printRelationJSON(schema)
}

func readRelationBlindingKeyFile(path string) (key []byte, returnErr error) {
	path = strings.TrimPrefix(strings.TrimSpace(path), "@")
	if path == "" || path == "-" {
		return nil, fmt.Errorf("relation blinding key file path is required and stdin is forbidden")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("relation blinding key must be a regular file")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("relation blinding key file permissions must not grant group or other access")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("close relation blinding key: %w", closeErr)
		}
	}()
	return relation.DecodeBlindingKey(file)
}

func printRelationJSON(value any) int {
	encoded, err := relation.EncodeIndented(value)
	if err != nil {
		fmt.Fprintln(os.Stderr, "relation: encode output:", err)
		return 1
	}
	return writeCommandOutput("relation", encoded)
}
