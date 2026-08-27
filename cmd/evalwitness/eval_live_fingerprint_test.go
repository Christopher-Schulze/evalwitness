package main

import (
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/study"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
)

func TestEvalPlanFingerprintBindsStatisticalDesign(t *testing.T) {
	manifestDigest := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	base := evalPlan{
		Calls: evalEstimateInt{Expected: 12, Worst: 16},
		StatisticalDesign: &evalStatisticalPlan{
			TotalTasks: 89, DecidableTasks: 17, NominalAlpha: 0.05,
			AdjustedAlpha: 0.05, TargetPower: 0.80, AlternativeQ: 0.75,
		},
	}
	baseFingerprint, err := evalPlanFingerprint(base, manifestDigest)
	if err != nil {
		t.Fatal(err)
	}

	changed := base
	changed.StatisticalDesign = &evalStatisticalPlan{
		TotalTasks: 89, DecidableTasks: 17, NominalAlpha: 0.05,
		AdjustedAlpha: 0.025, TargetPower: 0.80, AlternativeQ: 0.75,
	}
	changedFingerprint, err := evalPlanFingerprint(changed, manifestDigest)
	if err != nil {
		t.Fatal(err)
	}
	if baseFingerprint == changedFingerprint {
		t.Fatal("statistical design change did not invalidate the authorization fingerprint")
	}
}

func TestBindTerminalResearchLineageUsesLockedSplitAndArm(t *testing.T) {
	record := study.Record{Study: study.LockedStudy{
		ManifestDigest: "manifest-digest",
		Manifest: study.Manifest{
			Arms: []study.Arm{{ID: "distribution-aware-vs-chosen-token", Entrypoint: "eval-terminal"}},
			Providers: []study.ProviderPlan{{
				ArmID: "distribution-aware-vs-chosen-token", ServedIdentityPolicy: "exact_observed_set",
				ExpectedServedModels: []string{"deepseek-v4-flash", "deepseek-v4-flash-202605"},
			}},
			Data: study.DataPlan{Split: study.SplitManifest{Assignments: []study.SplitAssignment{{
				GroupID: "group-1", Split: study.RoleDevelopment, TaskIDs: []string{"task-1"},
			}}}},
			Outcomes: study.OutcomePlan{Primary: study.Endpoint{ID: "paired_task_group_disagreement"}},
		},
	}}
	input := verification.Input{Entrypoint: "eval-terminal"}
	if err := bindTerminalResearchLineage(&input, record, "task-1"); err != nil {
		t.Fatal(err)
	}
	if input.StudyManifestDigest != "manifest-digest" || input.StudyVariant != string(study.RoleDevelopment) ||
		input.Lineage.AuditCaseID != "group-1" || input.Lineage.TransformationID != "paired_task_group_disagreement" ||
		input.Lineage.StudyCellID != "distribution-aware-vs-chosen-token" || input.ServedIdentityPolicy != "exact_observed_set" ||
		len(input.ExpectedServedModels) != 2 {
		t.Fatalf("research lineage = %+v", input)
	}
	if err := bindTerminalResearchLineage(&input, record, "missing"); err == nil {
		t.Fatal("missing locked split task was accepted")
	}
}

func TestFilterTerminalResearchTasksExcludesUnregisteredTasks(t *testing.T) {
	record := study.Record{Study: study.LockedStudy{Manifest: study.Manifest{
		Data: study.DataPlan{Split: study.SplitManifest{Assignments: []study.SplitAssignment{{
			TaskIDs: []string{"locked-a", "locked-b"},
		}}}},
	}}}
	tasks := []terminalEvalTask{{Name: "outside"}, {Name: "locked-b"}, {Name: "locked-a"}}
	filtered, err := filterTerminalResearchTasks(tasks, record)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 2 || filtered[0].Name != "locked-b" || filtered[1].Name != "locked-a" {
		t.Fatalf("filtered tasks = %+v", filtered)
	}
	if _, err := filterTerminalResearchTasks([]terminalEvalTask{{Name: "outside"}}, record); err == nil {
		t.Fatal("research capture accepted a task set outside the locked split")
	}
}
