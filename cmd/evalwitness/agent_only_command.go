package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/Christopher-Schulze/evalwitness/internal/agentstudy"
	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

const (
	defaultAgentStudyPlan    = "eval/governance/controlled-corruption-v3-plan.json"
	defaultAgentStudyAudit   = "eval/governance/controlled-corruption-v3-natural-audit.json"
	defaultAgentStudyRelease = "eval/governance/controlled-corruption-v3-release.json"
	defaultAgentStudySeed    = "evalwitness-agent-only-controlled-relation-v1"
)

func runAgentStudy(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "agent-study: usage: evalwitness agent-study <build|validate|schema>")
		return 2
	}
	switch args[0] {
	case "build":
		return runAgentStudyBuild(args[1:])
	case "validate":
		return runAgentStudyValidate(args[1:])
	case "schema":
		schema, err := agentstudy.Schema()
		if err != nil {
			return agentStudyOperationError("schema", err)
		}
		return printAgentStudyJSON(schema)
	default:
		fmt.Fprintf(os.Stderr, "agent-study: unknown command %q\n", args[0])
		return 2
	}
}

func runAgentStudyBuild(args []string) int {
	flags := flag.NewFlagSet("agent-study build", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	planPath := flags.String("plan", defaultAgentStudyPlan, "frozen v3 corpus development plan (@file)")
	auditPath := flags.String("audit", defaultAgentStudyAudit, "frozen v3 corpus development audit (@file)")
	releasePath := flags.String("release", defaultAgentStudyRelease, "frozen v3 controlled-corruption release (@file)")
	calibration := flags.Int("calibration", 20, "number of calibration cases")
	test := flags.Int("test", 20, "number of test cases")
	seed := flags.String("seed", defaultAgentStudySeed, "deterministic selection seed")
	destination := flags.String("destination", "", "new study artifact path; stdout when omitted")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "agent-study build: positional arguments are forbidden")
		return 2
	}
	plan, audit, release, err := loadAgentCorpus(*planPath, *auditPath, *releasePath)
	if err != nil {
		return agentStudyInputError("build", err)
	}
	value, err := agentstudy.Build(agentstudy.BuildInputs{PlanDigest: plan.Digest, AuditDigest: audit.Digest, Release: release, CalibrationCases: *calibration, TestCases: *test, Seed: *seed})
	if err != nil {
		return agentStudyOperationError("build", err)
	}
	encoded, err := agentstudy.EncodeIndented(value)
	if err != nil {
		return agentStudyOperationError("build", err)
	}
	return writeAgentStudyDestination("build", *destination, encoded)
}

func runAgentStudyValidate(args []string) int {
	flags := flag.NewFlagSet("agent-study validate", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	studyPath := flags.String("study", "", "agent-only study artifact (@file)")
	planPath := flags.String("plan", defaultAgentStudyPlan, "frozen v3 corpus development plan (@file)")
	auditPath := flags.String("audit", defaultAgentStudyAudit, "frozen v3 corpus development audit (@file)")
	releasePath := flags.String("release", defaultAgentStudyRelease, "frozen v3 controlled-corruption release (@file)")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *studyPath == "" {
		fmt.Fprintln(os.Stderr, "agent-study validate: --study is required; positional arguments are forbidden")
		return 2
	}
	plan, audit, release, err := loadAgentCorpus(*planPath, *auditPath, *releasePath)
	if err != nil {
		return agentStudyInputError("validate", err)
	}
	reader, closeStudy, err := openStudyDocument(*studyPath)
	if err != nil {
		return agentStudyInputError("validate", err)
	}
	defer closeStudy()
	value, err := agentstudy.Decode(reader)
	if err != nil {
		return agentStudyOperationError("validate", err)
	}
	if value.PlanDigest != plan.Digest || value.AuditDigest != audit.Digest {
		return agentStudyOperationError("validate", errors.New("study plan or audit digest does not match the supplied corpus governance"))
	}
	if err := value.ValidateAgainstRelease(release); err != nil {
		return agentStudyOperationError("validate", err)
	}
	return printAgentStudyJSON(value)
}

func loadAgentCorpus(planPath, auditPath, releasePath string) (mutation.CorpusDevelopmentPlan, mutation.CorpusDevelopmentAuditV3, mutation.CorpusReleaseV3, error) {
	planReader, closePlan, err := openStudyDocument(planPath)
	if err != nil {
		return mutation.CorpusDevelopmentPlan{}, mutation.CorpusDevelopmentAuditV3{}, mutation.CorpusReleaseV3{}, err
	}
	plan, err := mutation.DecodeCorpusDevelopmentPlan(planReader)
	closePlan()
	if err != nil {
		return mutation.CorpusDevelopmentPlan{}, mutation.CorpusDevelopmentAuditV3{}, mutation.CorpusReleaseV3{}, err
	}
	auditReader, closeAudit, err := openStudyDocument(auditPath)
	if err != nil {
		return mutation.CorpusDevelopmentPlan{}, mutation.CorpusDevelopmentAuditV3{}, mutation.CorpusReleaseV3{}, err
	}
	audit, err := mutation.DecodeCorpusDevelopmentAuditV3(auditReader, plan)
	closeAudit()
	if err != nil {
		return mutation.CorpusDevelopmentPlan{}, mutation.CorpusDevelopmentAuditV3{}, mutation.CorpusReleaseV3{}, err
	}
	releaseReader, closeRelease, err := openStudyDocument(releasePath)
	if err != nil {
		return mutation.CorpusDevelopmentPlan{}, mutation.CorpusDevelopmentAuditV3{}, mutation.CorpusReleaseV3{}, err
	}
	release, err := mutation.DecodeCorpusReleaseV3(releaseReader, plan, audit)
	closeRelease()
	if err != nil {
		return mutation.CorpusDevelopmentPlan{}, mutation.CorpusDevelopmentAuditV3{}, mutation.CorpusReleaseV3{}, err
	}
	return plan, audit, release, nil
}

func writeAgentStudyDestination(scope, destination string, encoded []byte) int {
	if destination == "" {
		return writeCommandOutput("agent-study "+scope, encoded)
	}
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return agentStudyOperationError(scope, fmt.Errorf("create destination: %w", err))
	}
	if _, err := file.Write(encoded); err != nil {
		_ = file.Close()
		return agentStudyOperationError(scope, fmt.Errorf("write destination: %w", err))
	}
	if err := file.Close(); err != nil {
		return agentStudyOperationError(scope, fmt.Errorf("close destination: %w", err))
	}
	return 0
}

func printAgentStudyJSON(value any) int {
	encoded, err := agentstudy.EncodeIndented(value)
	if err != nil {
		return agentStudyOperationError("output", err)
	}
	return writeCommandOutput("agent-study", encoded)
}

func agentStudyInputError(scope string, err error) int {
	fmt.Fprintln(os.Stderr, "agent-study "+scope+":", err)
	return 2
}

func agentStudyOperationError(scope string, err error) int {
	fmt.Fprintln(os.Stderr, "agent-study "+scope+":", err)
	return 1
}
