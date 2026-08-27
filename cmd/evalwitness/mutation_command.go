package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

func runMutation(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "mutation: usage: evalwitness mutation <validate|schema|construct-repair|construct-challenge|verification-evidence|control|corpus>")
		return 2
	}
	switch args[0] {
	case "validate":
		return runMutationValidate(args[1:])
	case "schema":
		return runMutationSchema(args[1:])
	case "corpus":
		return runMutationCorpus(args[1:])
	case "construct-repair":
		return runMutationConstructRepair(args[1:])
	case "construct-challenge":
		return runMutationConstructChallenge(args[1:])
	case "verification-evidence":
		return runMutationVerificationEvidence(args[1:])
	case "control":
		return runMutationControl(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "mutation: unknown command %q\n", args[0])
		return 2
	}
}

func runMutationControl(args []string) int {
	if len(args) == 0 || args[0] != "validate" {
		fmt.Fprintln(os.Stderr, "mutation control: usage: evalwitness mutation control validate --original @file --mutated @file")
		return 2
	}
	flags := flag.NewFlagSet("mutation control validate", flag.ContinueOnError)
	originalPath := flags.String("original", "", "original formal control (@file)")
	mutatedPath := flags.String("mutated", "", "mutated formal control (@file)")
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	originalReader, closeOriginal, err := openStudyDocument(*originalPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation control validate:", err)
		return 2
	}
	defer closeOriginal()
	mutatedReader, closeMutated, err := openStudyDocument(*mutatedPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation control validate:", err)
		return 2
	}
	defer closeMutated()
	original, err := mutation.DecodeFormalControl(originalReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation control validate:", err)
		return 2
	}
	mutated, err := mutation.DecodeFormalControl(mutatedReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation control validate:", err)
		return 2
	}
	proof, err := mutation.ValidateFormalControlPair(original, mutated)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation control validate:", err)
		return 1
	}
	return printMutationJSON(struct {
		Valid bool                  `json:"valid"`
		Proof mutation.OutcomeProof `json:"outcome_proof"`
	}{Valid: true, Proof: proof})
}

func runMutationValidate(args []string) int {
	flags := flag.NewFlagSet("mutation validate", flag.ContinueOnError)
	path := flags.String("manifest", "", "mutation manifest JSON (@file or - for stdin)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	reader, closeDocument, err := openStudyDocument(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation validate:", err)
		return 2
	}
	defer closeDocument()
	manifest, err := mutation.DecodeManifest(reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation validate:", err)
		return 2
	}
	return printMutationJSON(struct {
		Valid      bool   `json:"valid"`
		MutationID string `json:"mutation_id"`
		Digest     string `json:"digest"`
	}{Valid: true, MutationID: manifest.MutationID, Digest: manifest.Digest})
}

func runMutationSchema(args []string) int {
	flags := flag.NewFlagSet("mutation schema", flag.ContinueOnError)
	document := flags.String("type", "manifest", "manifest/witness/blind-review-packet/construct-firewall/construct-firewall-v2/construct-repair-evidence/construct-firewall-challenge/verification-evidence-assessment/verification-evidence-challenge/corpus-spec/corpus-development-plan/corpus-development-audit/corpus-development-audit-v3/corpus-release/corpus-release-v3/reduction-witness/formal-control")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	schema, err := mutation.Schema(*document)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation schema:", err)
		return 2
	}
	return printMutationJSON(schema)
}

func runMutationCorpus(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "mutation corpus: usage: evalwitness mutation corpus <spec|plan-v2|audit-v2|freeze-v2|verify-v2-audit|plan-v3|audit-v3|validate-v3-audit|build-v3|validate-v3-release|build|validate>")
		return 2
	}
	switch args[0] {
	case "spec":
		return runMutationCorpusSpec(args[1:])
	case "plan-v2":
		return runMutationCorpusPlanV2(args[1:])
	case "plan-v3":
		return runMutationCorpusPlanV3(args[1:])
	case "audit-v2":
		return runMutationCorpusAuditV2(args[1:])
	case "audit-v3":
		return runMutationCorpusAuditV3(args[1:])
	case "freeze-v2":
		return runMutationCorpusFreezeV2(args[1:])
	case "build":
		return runMutationCorpusBuild(args[1:])
	case "validate":
		return runMutationCorpusValidate(args[1:])
	case "verify-v2-audit":
		return runMutationCorpusVerifyV2Audit(args[1:])
	case "validate-v3-audit":
		return runMutationCorpusValidateV3Audit(args[1:])
	case "build-v3":
		return runMutationCorpusBuildV3(args[1:])
	case "validate-v3-release":
		return runMutationCorpusValidateV3Release(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "mutation corpus: unknown command %q\n", args[0])
		return 2
	}
}

func runMutationCorpusPlanV3(args []string) int {
	flags := flag.NewFlagSet("mutation corpus plan-v3", flag.ContinueOnError)
	path := flags.String("plan", "", "optional v3 development plan to validate (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, err := loadCorpusDevelopmentPlanV3(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus plan-v3:", err)
		return 2
	}
	return printMutationJSON(plan)
}

func runMutationCorpusAuditV3(args []string) int {
	flags := flag.NewFlagSet("mutation corpus audit-v3", flag.ContinueOnError)
	root := flags.String("root", ".", "repository root containing fetched eval trajectories")
	planPath := flags.String("plan", "", "optional v3 development plan (@file)")
	auditedAt := flags.String("audited-at", "", "audit date (YYYY-MM-DD)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, err := loadCorpusDevelopmentPlanV3(*planPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus audit-v3:", err)
		return 2
	}
	sources, err := mutation.DiscoverDefaultCorpusSources(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus audit-v3:", err)
		return 1
	}
	audit, err := mutation.AuditCorpusV3(plan, sources, *auditedAt)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus audit-v3:", err)
		return 1
	}
	return printMutationJSON(audit)
}

func runMutationCorpusValidateV3Audit(args []string) int {
	flags := flag.NewFlagSet("mutation corpus validate-v3-audit", flag.ContinueOnError)
	planPath := flags.String("plan", "", "v3 development plan (@file)")
	auditPath := flags.String("audit", "", "v3 development audit (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, err := loadCorpusDevelopmentPlanV3(*planPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus validate-v3-audit:", err)
		return 2
	}
	reader, closeDocument, err := openStudyDocument(*auditPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus validate-v3-audit:", err)
		return 2
	}
	defer closeDocument()
	audit, err := mutation.DecodeCorpusDevelopmentAuditV3(reader, plan)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus validate-v3-audit:", err)
		return 1
	}
	return printMutationJSON(struct {
		Valid            bool   `json:"valid"`
		PlanDigest       string `json:"plan_digest"`
		AuditDigest      string `json:"audit_digest"`
		TotalAttempts    int    `json:"total_attempts"`
		AppliedAttempts  int    `json:"applied_attempts"`
		RejectedAttempts int    `json:"rejected_attempts"`
		SelectedCases    int    `json:"selected_cases"`
		QuotasSatisfied  bool   `json:"quotas_satisfied"`
	}{
		Valid: true, PlanDigest: plan.Digest, AuditDigest: audit.Digest, TotalAttempts: audit.TotalAttempts,
		AppliedAttempts: audit.AppliedAttempts, RejectedAttempts: audit.RejectedAttempts,
		SelectedCases: audit.SelectedCases, QuotasSatisfied: audit.QuotasSatisfied,
	})
}

func runMutationCorpusBuildV3(args []string) int {
	flags := flag.NewFlagSet("mutation corpus build-v3", flag.ContinueOnError)
	root := flags.String("root", ".", "repository root containing fetched eval trajectories")
	planPath := flags.String("plan", "", "v3 development plan (@file)")
	auditPath := flags.String("audit", "", "v3 development audit (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, err := loadCorpusDevelopmentPlanV3(*planPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus build-v3:", err)
		return 2
	}
	audit, err := loadCorpusDevelopmentAuditV3(*auditPath, plan)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus build-v3:", err)
		return 2
	}
	sources, err := mutation.DiscoverDefaultCorpusSources(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus build-v3:", err)
		return 1
	}
	release, err := mutation.BuildCorpusReleaseV3(plan, audit, sources)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus build-v3:", err)
		return 1
	}
	return printMutationJSON(release)
}

func runMutationCorpusValidateV3Release(args []string) int {
	flags := flag.NewFlagSet("mutation corpus validate-v3-release", flag.ContinueOnError)
	planPath := flags.String("plan", "", "v3 development plan (@file)")
	auditPath := flags.String("audit", "", "v3 development audit (@file)")
	releasePath := flags.String("release", "", "v3 corpus release (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, err := loadCorpusDevelopmentPlanV3(*planPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus validate-v3-release:", err)
		return 2
	}
	audit, err := loadCorpusDevelopmentAuditV3(*auditPath, plan)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus validate-v3-release:", err)
		return 2
	}
	reader, closeDocument, err := openStudyDocument(*releasePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus validate-v3-release:", err)
		return 2
	}
	defer closeDocument()
	release, err := mutation.DecodeCorpusReleaseV3(reader, plan, audit)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus validate-v3-release:", err)
		return 1
	}
	return printMutationJSON(struct {
		Valid         bool   `json:"valid"`
		ReleaseDigest string `json:"release_digest"`
		SelectedCases int    `json:"selected_cases"`
		CoreCases     int    `json:"core_cases"`
		SentinelCases int    `json:"sentinel_cases"`
	}{Valid: true, ReleaseDigest: release.Digest, SelectedCases: release.SelectedCases, CoreCases: release.Policy.CoreCases, SentinelCases: release.Policy.ScarcitySentinelCases})
}

func runMutationCorpusPlanV2(args []string) int {
	flags := flag.NewFlagSet("mutation corpus plan-v2", flag.ContinueOnError)
	path := flags.String("plan", "", "optional v2 development plan to validate (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, err := loadCorpusDevelopmentPlan(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus plan-v2:", err)
		return 2
	}
	return printMutationJSON(plan)
}

func runMutationCorpusAuditV2(args []string) int {
	flags := flag.NewFlagSet("mutation corpus audit-v2", flag.ContinueOnError)
	root := flags.String("root", ".", "repository root containing fetched eval trajectories")
	planPath := flags.String("plan", "", "optional v2 development plan (@file)")
	auditedAt := flags.String("audited-at", "", "audit date (YYYY-MM-DD)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, err := loadCorpusDevelopmentPlan(*planPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus audit-v2:", err)
		return 2
	}
	sources, err := mutation.DiscoverDefaultCorpusSources(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus audit-v2:", err)
		return 1
	}
	audit, err := mutation.AuditCorpusV2(plan, sources, *auditedAt)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus audit-v2:", err)
		return 1
	}
	return printMutationJSON(audit)
}

func runMutationCorpusFreezeV2(args []string) int {
	flags := flag.NewFlagSet("mutation corpus freeze-v2", flag.ContinueOnError)
	planPath := flags.String("plan", "", "v2 development plan (@file)")
	auditPath := flags.String("audit", "", "v2 development audit (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, err := loadCorpusDevelopmentPlan(*planPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus freeze-v2:", err)
		return 2
	}
	audit, err := loadCorpusDevelopmentAudit(*auditPath, plan)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus freeze-v2:", err)
		return 2
	}
	spec, err := mutation.FreezeCorpusSpecV2(plan, audit)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus freeze-v2:", err)
		return 1
	}
	return printMutationJSON(spec)
}

func runMutationCorpusSpec(args []string) int {
	flags := flag.NewFlagSet("mutation corpus spec", flag.ContinueOnError)
	path := flags.String("spec", "", "optional governed corpus spec to validate (@file or - for stdin)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *path == "" {
		spec, err := mutation.DefaultCorpusSpec()
		if err != nil {
			fmt.Fprintln(os.Stderr, "mutation corpus spec:", err)
			return 1
		}
		return printMutationJSON(spec)
	}
	reader, closeDocument, err := openStudyDocument(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus spec:", err)
		return 2
	}
	defer closeDocument()
	spec, err := mutation.DecodeCorpusSpec(reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus spec:", err)
		return 2
	}
	return printMutationJSON(spec)
}

func runMutationCorpusBuild(args []string) int {
	flags := flag.NewFlagSet("mutation corpus build", flag.ContinueOnError)
	root := flags.String("root", ".", "repository root containing fetched eval trajectories")
	specPath := flags.String("spec", "", "optional governed corpus spec (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	spec, err := mutation.DefaultCorpusSpec()
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus build:", err)
		return 1
	}
	if *specPath != "" {
		reader, closeDocument, err := openStudyDocument(*specPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "mutation corpus build:", err)
			return 2
		}
		defer closeDocument()
		spec, err = mutation.DecodeCorpusSpec(reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "mutation corpus build:", err)
			return 2
		}
	}
	sources, err := mutation.DiscoverDefaultCorpusSources(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus build:", err)
		return 1
	}
	release, err := mutation.BuildCorpus(spec, sources)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus build:", err)
		return 1
	}
	return printMutationJSON(release)
}

func runMutationCorpusValidate(args []string) int {
	flags := flag.NewFlagSet("mutation corpus validate", flag.ContinueOnError)
	path := flags.String("release", "", "corruption corpus release JSON (@file or - for stdin)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	reader, closeDocument, err := openStudyDocument(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus validate:", err)
		return 2
	}
	defer closeDocument()
	release, err := mutation.DecodeCorpusRelease(reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus validate:", err)
		return 2
	}
	if err := mutation.VerifyNoCorpusLeakage(release); err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus validate:", err)
		return 1
	}
	return printMutationJSON(struct {
		Valid       bool   `json:"valid"`
		Digest      string `json:"digest"`
		Sources     int    `json:"sources"`
		SourceTasks int    `json:"source_tasks"`
		Cases       int    `json:"cases"`
	}{Valid: true, Digest: release.Digest, Sources: len(release.Sources), SourceTasks: release.TaskCount, Cases: len(release.Cases)})
}

func runMutationCorpusVerifyV2Audit(args []string) int {
	flags := flag.NewFlagSet("mutation corpus verify-v2-audit", flag.ContinueOnError)
	planPath := flags.String("plan", "", "v2 development plan (@file)")
	auditPath := flags.String("audit", "", "v2 development audit (@file)")
	releasePath := flags.String("release", "", "v2 corpus release (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, err := loadCorpusDevelopmentPlan(*planPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus verify-v2-audit:", err)
		return 2
	}
	audit, err := loadCorpusDevelopmentAudit(*auditPath, plan)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus verify-v2-audit:", err)
		return 2
	}
	releaseReader, closeRelease, err := openStudyDocument(*releasePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus verify-v2-audit:", err)
		return 2
	}
	defer closeRelease()
	release, err := mutation.DecodeCorpusRelease(releaseReader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus verify-v2-audit:", err)
		return 2
	}
	if err := mutation.VerifyCorpusV2AgainstAudit(release, plan, audit); err != nil {
		fmt.Fprintln(os.Stderr, "mutation corpus verify-v2-audit:", err)
		return 1
	}
	return printMutationJSON(struct {
		Valid         bool   `json:"valid"`
		ReleaseDigest string `json:"release_digest"`
		AuditDigest   string `json:"audit_digest"`
	}{Valid: true, ReleaseDigest: release.Digest, AuditDigest: audit.Digest})
}

func loadCorpusDevelopmentPlan(path string) (mutation.CorpusDevelopmentPlan, error) {
	if path == "" {
		return mutation.DefaultCorpusDevelopmentPlanV2()
	}
	reader, closeDocument, err := openStudyDocument(path)
	if err != nil {
		return mutation.CorpusDevelopmentPlan{}, err
	}
	defer closeDocument()
	return mutation.DecodeCorpusDevelopmentPlan(reader)
}

func loadCorpusDevelopmentPlanV3(path string) (mutation.CorpusDevelopmentPlan, error) {
	if path == "" {
		return mutation.DefaultCorpusDevelopmentPlanV3()
	}
	reader, closeDocument, err := openStudyDocument(path)
	if err != nil {
		return mutation.CorpusDevelopmentPlan{}, err
	}
	defer closeDocument()
	plan, err := mutation.DecodeCorpusDevelopmentPlan(reader)
	if err != nil {
		return mutation.CorpusDevelopmentPlan{}, err
	}
	if plan.SchemaVersion != mutation.CorpusDevelopmentPlanSchemaVersionV3 {
		return mutation.CorpusDevelopmentPlan{}, fmt.Errorf("expected v3 development plan, got %q", plan.SchemaVersion)
	}
	return plan, nil
}

func loadCorpusDevelopmentAudit(path string, plan mutation.CorpusDevelopmentPlan) (mutation.CorpusDevelopmentAudit, error) {
	reader, closeDocument, err := openStudyDocument(path)
	if err != nil {
		return mutation.CorpusDevelopmentAudit{}, err
	}
	defer closeDocument()
	return mutation.DecodeCorpusDevelopmentAudit(reader, plan)
}

func loadCorpusDevelopmentAuditV3(path string, plan mutation.CorpusDevelopmentPlan) (mutation.CorpusDevelopmentAuditV3, error) {
	reader, closeDocument, err := openStudyDocument(path)
	if err != nil {
		return mutation.CorpusDevelopmentAuditV3{}, err
	}
	defer closeDocument()
	return mutation.DecodeCorpusDevelopmentAuditV3(reader, plan)
}

func printMutationJSON(value any) int {
	encoded, err := mutation.EncodeIndented(value)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation: encode output:", err)
		return 1
	}
	return writeCommandOutput("mutation", encoded)
}
