package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

func runMutationConstructRepair(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "mutation construct-repair: usage: evalwitness mutation construct-repair <build|validate>")
		return 2
	}
	switch args[0] {
	case "build":
		return runMutationConstructRepairBuild(args[1:])
	case "validate":
		return runMutationConstructRepairValidate(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "mutation construct-repair: unknown command %q\n", args[0])
		return 2
	}
}

func runMutationConstructRepairBuild(args []string) int {
	flags := flag.NewFlagSet("mutation construct-repair build", flag.ContinueOnError)
	planPath := flags.String("plan", "", "v2 development plan (@file)")
	auditPath := flags.String("audit", "", "v2 development audit (@file)")
	releasePath := flags.String("release", "", "v2 corpus release (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, audit, release, err := loadConstructRepairParents(*planPath, *auditPath, *releasePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation construct-repair build:", err)
		return 2
	}
	evidence, err := mutation.BuildConstructRepairEvidence(plan, audit, release)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation construct-repair build:", err)
		return 1
	}
	return printMutationJSON(evidence)
}

func runMutationConstructRepairValidate(args []string) int {
	flags := flag.NewFlagSet("mutation construct-repair validate", flag.ContinueOnError)
	evidencePath := flags.String("evidence", "", "construct-repair evidence (@file)")
	planPath := flags.String("plan", "", "v2 development plan (@file)")
	auditPath := flags.String("audit", "", "v2 development audit (@file)")
	releasePath := flags.String("release", "", "v2 corpus release (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, audit, release, err := loadConstructRepairParents(*planPath, *auditPath, *releasePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation construct-repair validate:", err)
		return 2
	}
	reader, closeEvidence, err := openStudyDocument(*evidencePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation construct-repair validate:", err)
		return 2
	}
	defer closeEvidence()
	evidence, err := mutation.DecodeConstructRepairEvidence(reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation construct-repair validate:", err)
		return 2
	}
	if err := mutation.VerifyConstructRepairEvidence(evidence, plan, audit, release); err != nil {
		fmt.Fprintln(os.Stderr, "mutation construct-repair validate:", err)
		return 1
	}
	return printMutationJSON(struct {
		Valid          bool   `json:"valid"`
		EvidenceDigest string `json:"evidence_digest"`
		PlanDigest     string `json:"plan_digest"`
		AuditDigest    string `json:"audit_digest"`
		ReleaseDigest  string `json:"release_digest"`
		Cases          int    `json:"cases"`
	}{true, evidence.Digest, plan.Digest, audit.Digest, release.Digest, len(evidence.Cases)})
}

func loadConstructRepairParents(planPath, auditPath, releasePath string) (mutation.CorpusDevelopmentPlan, mutation.CorpusDevelopmentAudit, mutation.CorpusRelease, error) {
	plan, err := loadCorpusDevelopmentPlan(planPath)
	if err != nil {
		return mutation.CorpusDevelopmentPlan{}, mutation.CorpusDevelopmentAudit{}, mutation.CorpusRelease{}, err
	}
	audit, err := loadCorpusDevelopmentAudit(auditPath, plan)
	if err != nil {
		return mutation.CorpusDevelopmentPlan{}, mutation.CorpusDevelopmentAudit{}, mutation.CorpusRelease{}, err
	}
	reader, closeRelease, err := openStudyDocument(releasePath)
	if err != nil {
		return mutation.CorpusDevelopmentPlan{}, mutation.CorpusDevelopmentAudit{}, mutation.CorpusRelease{}, err
	}
	defer closeRelease()
	release, err := mutation.DecodeCorpusRelease(reader)
	if err != nil {
		return mutation.CorpusDevelopmentPlan{}, mutation.CorpusDevelopmentAudit{}, mutation.CorpusRelease{}, err
	}
	return plan, audit, release, nil
}
