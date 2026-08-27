package study

import (
	"fmt"
	"strings"
)

func ProtocolReport(record Record) (string, error) {
	if err := record.Validate(); err != nil {
		return "", err
	}
	m := record.Study.Manifest
	adjustedAlpha := m.Inference.NominalAlpha / float64(len(m.Inference.PrimaryFamily))
	var report strings.Builder
	fmt.Fprintf(&report, "# Registered Study Protocol: %s\n\n", m.Identity.Title)
	fmt.Fprintf(&report, "- Study ID: `%s`\n", record.Study.StudyID)
	fmt.Fprintf(&report, "- Manifest digest: `%s`\n", record.Study.ManifestDigest)
	fmt.Fprintf(&report, "- Lifecycle state: `%s`\n", record.State)
	fmt.Fprintf(&report, "- Locked at: `%s`\n", m.Identity.LockedAt.UTC().Format("2006-01-02T15:04:05Z"))
	fmt.Fprintf(&report, "- Registered-report timestamp: `%s`\n", m.Publication.RegisteredReportTimestamp.UTC().Format("2006-01-02T15:04:05Z"))
	fmt.Fprintf(&report, "- Canonical policy: `%s`\n\n", m.CanonicalPolicy)

	fmt.Fprintf(&report, "## Question and hypotheses\n\n%s\n\n", m.Identity.ResearchQuestion)
	fmt.Fprintf(&report, "Primary null: %s\n\nPrimary alternative: %s\n\n", m.Hypotheses.PrimaryNull, m.Hypotheses.PrimaryAlternative)

	report.WriteString("## Data and split boundary\n\n")
	fmt.Fprintf(&report, "Primary unit: `%s`; split algorithm: `%s`; split digest: `%s`.\n\n", m.Data.PrimaryUnit, m.Data.Split.Algorithm, m.Data.Split.Digest)
	report.WriteString("| dataset | version | permitted roles | tasks | previously accessed | dataset digest |\n|---|---|---|---:|---:|---|\n")
	for _, dataset := range m.Data.Datasets {
		roles := make([]string, len(dataset.PermittedRoles))
		for index, role := range dataset.PermittedRoles {
			roles[index] = string(role)
		}
		fmt.Fprintf(&report, "| %s | %s | %s | %d | %t | `%s` |\n", dataset.ID, dataset.Version, strings.Join(roles, ", "), dataset.TaskCount, dataset.PreviouslyAccessed, dataset.DatasetDigest)
	}

	report.WriteString("\n## Arms\n\n| arm | entrypoint | route | model | selection | candidates | repetitions |\n|---|---|---|---|---|---:|---:|\n")
	for _, arm := range m.Arms {
		fmt.Fprintf(&report, "| %s | %s | `%s` | %s | %s | %d | %d |\n", arm.ID, arm.Entrypoint, arm.RouteID, arm.RequestedModel, arm.SelectionMode, arm.Candidates, arm.Repetitions)
	}

	report.WriteString("\n## Locked outcomes and inference\n\n")
	fmt.Fprintf(&report, "Primary outcome `%s` uses `%s` in direction `%s` as a `%s` question with margin %.6f.\n\n", m.Outcomes.Primary.ID, m.Outcomes.Primary.Metric, m.Outcomes.Primary.Direction, m.Outcomes.Primary.Question, m.Outcomes.Primary.Margin)
	fmt.Fprintf(&report, "Test `%s`; interval `%s`; cluster unit `%s`; nominal alpha %.6f; %s-adjusted alpha %.6f; target power %.6f; minimum effect %.6f; decidable tasks %d; disagreement %.6f; discordant win probability %.6f.\n\n",
		m.Inference.Test, m.Inference.IntervalMethod, m.Inference.ClusterUnit, m.Inference.NominalAlpha,
		m.Inference.MultiplicityMethod, adjustedAlpha, m.Inference.TargetPower, m.Inference.MinimumEffect,
		m.Inference.DecidableTasks, m.Inference.DisagreementRate, m.Inference.DiscordantWinProbability)
	fmt.Fprintf(&report, "Design method `%s`; locked power at minimum effect %.12f; design evidence `%s`.\n\n", m.Inference.DesignMethod, m.Inference.PowerAtMinimumEffect, m.Inference.DesignEvidenceDigest)
	fmt.Fprintf(&report, "Primary family: `%s`. Sequential rule: enabled=%t method=`%s` maximum looks=%d.\n\n", strings.Join(m.Inference.PrimaryFamily, "`, `"), m.Inference.Sequential.Enabled, m.Inference.Sequential.Method, m.Inference.Sequential.MaximumLooks)

	report.WriteString("## Failure and denominator policy\n\n")
	fmt.Fprintf(&report, "Missing score: %s\n\nProvider failure: %s\n\nRoute failure: %s\n\nTimeout: %s\n\nAbstention: %s\n\nBudget exhaustion: %s\n\nRetry exhaustion: %s\n\nIncomplete cell: %s\n\nDenominator: %s\n\n",
		m.Failures.MissingScore, m.Failures.ProviderFailure, m.Failures.RouteFailure, m.Failures.Timeout,
		m.Failures.Abstention, m.Failures.BudgetExhaustion, m.Failures.RetryExhaustion,
		m.Failures.IncompleteCell, m.Failures.DenominatorPolicy)

	report.WriteString("## Execution and budget\n\n")
	fmt.Fprintf(&report, "Commit `%s`; binary `%s`; analysis `%s`; platform `%s`; dirty builds forbidden. Expected/hard calls %d/%d; attempts %d; input/output tokens %d/%d; duration %ds; concurrency %d; cost $%.4f.\n\n",
		m.Execution.Commit, m.Execution.BinaryDigest, m.Execution.AnalysisDigest, m.Execution.Platform,
		m.Budget.ExpectedCalls, m.Budget.HardCalls, m.Budget.HardAttempts, m.Budget.HardInputTokens,
		m.Budget.HardOutputTokens, m.Budget.HardDurationSeconds, m.Budget.HardConcurrent, m.Budget.HardCostUSD)
	for _, providerPlan := range m.Providers {
		expectedModels := providerPlan.ExpectedServedModel
		if len(providerPlan.ExpectedServedModels) > 0 {
			expectedModels = strings.Join(providerPlan.ExpectedServedModels, ", ")
		}
		fmt.Fprintf(&report, "Arm `%s` attestation observed `%s`, expires `%s`; served model(s) `%s` under `%s`; checkpoint `%s` from `%s` under `%s`; retry `%s`, maximum retries %d, request timeout %ds.\n\n",
			providerPlan.ArmID, providerPlan.AttestationObservedAt.UTC().Format("2006-01-02T15:04:05Z"), providerPlan.AttestationExpiresAt.UTC().Format("2006-01-02T15:04:05Z"),
			expectedModels, providerPlan.ServedIdentityPolicy, providerPlan.ExpectedCheckpointAssertion,
			providerPlan.ExpectedCheckpointAssertionSource, providerPlan.CheckpointAssertionPolicy, providerPlan.RetryPolicyVersion,
			providerPlan.MaxRetries, providerPlan.RequestTimeoutSeconds)
	}

	report.WriteString("## Controls and publication\n\n")
	fmt.Fprintf(&report, "Random control `%s`; task-independent selector `%s`; positive control `%s` from `%s`.\n\n", m.Controls.RandomSelectionID, m.Controls.TaskIndependentSelector, m.Controls.PositiveControl, m.Controls.PositiveControlSource)
	fmt.Fprintf(&report, "Allowed claim IDs: `%s`. Independent reproduction required: %t.\n\n", strings.Join(m.Publication.AllowedClaimIDs, "`, `"), m.Publication.IndependentReproductionGate)

	report.WriteString("## Locked reliability contracts\n\n")
	fmt.Fprintf(&report, "Protocol `%s` / corpus `%s` / request corpus `%s` / schemas `%s`; trace mapping `%s`; outcome `%s`; adjudication `%s`; profile projection `%s`.\n",
		m.Reliability.ProtocolVersion, m.Reliability.ProtocolCorpusDigest, m.Reliability.ProtocolRequestCorpusDigest,
		m.Reliability.ProtocolSchemaDigest, m.Reliability.TraceMappingPolicy,
		m.Reliability.OutcomeContractDigest, m.Reliability.AdjudicationContractDigest, m.Reliability.ProfileProjectionDigest)
	return report.String(), nil
}
