package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSWEbenchTasksComputesRunsAndRewards(t *testing.T) {
	root := t.TempDir()
	writeSWEbenchRun(t, root, "run_a", []swebenchCacheItem{
		swebenchFixtureItem("repo__project-1", 1, "patch a1"),
		swebenchFixtureItem("repo__project-2", 0, "patch a2"),
	})
	writeSWEbenchRun(t, root, "run_b", []swebenchCacheItem{
		swebenchFixtureItem("repo__project-1", 0, "patch b1"),
		swebenchFixtureItem("repo__project-2", 0, "patch b2"),
	})
	writeSWEbenchRun(t, root, "run_c", []swebenchCacheItem{
		swebenchFixtureItem("repo__project-1", 1, "patch c1"),
	})

	tasks, runs, err := loadSWEbenchTasks(root, nil, 0)
	if err != nil {
		t.Fatalf("loadSWEbenchTasks: %v", err)
	}
	if got, want := len(runs), 3; got != want {
		t.Fatalf("runs len = %d, want %d", got, want)
	}
	if got, want := len(tasks), 2; got != want {
		t.Fatalf("tasks len = %d, want %d", got, want)
	}
	if got, want := tasks[0].InstanceID, "repo__project-1"; got != want {
		t.Fatalf("first task id = %q, want %q", got, want)
	}
	if got, want := len(tasks[0].Trials), 3; got != want {
		t.Fatalf("first task trials = %d, want %d", got, want)
	}
	if got, want := tasks[0].Trials[1].Reward, 0; got != want {
		t.Fatalf("middle reward = %d, want %d", got, want)
	}
	if tasks[0].Trials[0].Problem == "" {
		t.Fatal("problem was not extracted")
	}
	if tasks[0].Trials[0].Trace == "" {
		t.Fatal("trace was not formatted")
	}
}

func TestExtractAndFormatSWEbenchMessages(t *testing.T) {
	item := swebenchFixtureItem("repo__project-1", 1, "diff --git a/x b/x")

	problem := extractSWEbenchProblem(item.Messages)
	if problem == "" {
		t.Fatal("expected problem")
	}
	if !containsAll(problem, "<pr_description>", "Broken behavior", "<instructions>", "Fix it") {
		t.Fatalf("problem missing expected blocks: %q", problem)
	}

	trace := formatSWEbenchTrace(item.Messages)
	if !containsAll(trace, "--- Agent Step 1 ---", "I will inspect", "[Output]", "tests passed") {
		t.Fatalf("trace missing expected content: %q", trace)
	}
	if containsAll(trace, "Broken behavior") {
		t.Fatalf("trace duplicated problem block: %q", trace)
	}
}

func writeSWEbenchRun(t *testing.T, root, run string, items []swebenchCacheItem) {
	t.Helper()
	dir := filepath.Join(root, run)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data_cache.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func swebenchFixtureItem(id string, reward float64, patch string) swebenchCacheItem {
	messages, _ := json.Marshal([]map[string]any{
		{
			"role":    "system",
			"content": "system prompt",
		},
		{
			"role": "user",
			"content": "<pr_description>\nBroken behavior\n</pr_description>\n\n" +
				"<instructions>\nFix it\n</instructions>",
		},
		{
			"role":    "assistant",
			"content": "I will inspect the failing path.",
		},
		{
			"role":    "tool",
			"content": "tests passed",
		},
	})
	return swebenchCacheItem{
		InstanceID:   id,
		Messages:     string(messages),
		ModelName:    "fixture-model",
		NumSteps:     2,
		OutputPatch:  patch,
		Reward:       reward,
		TrajectoryID: id,
	}
}

func containsAll(s string, needles ...string) bool {
	for _, needle := range needles {
		if !strings.Contains(s, needle) {
			return false
		}
	}
	return true
}
