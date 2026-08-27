package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func configureNoNetworkCLI(t *testing.T, serverURL string) {
	t.Helper()
	t.Setenv("EVALWITNESS_PROVIDER", "boundary-test")
	t.Setenv("EVALWITNESS_WIRE_FORMAT", "openai")
	t.Setenv("EVALWITNESS_BASE_URL", serverURL)
	t.Setenv("EVALWITNESS_MODEL", "boundary-model")
	t.Setenv("EVALWITNESS_API_KEY", "synthetic-boundary-key")
	t.Setenv("EVALWITNESS_ALLOW_INSECURE", "true")
	t.Setenv("EVALWITNESS_CACHE_DIR", filepath.Join(t.TempDir(), "cache"))
	t.Setenv("EVALWITNESS_NO_CACHE", "true")
}

func TestDefaultLiveEntrypointsDoNotDispatchNetworkRequests(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		response.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	configureNoNetworkCLI(t, server.URL)

	if code := runProbe(nil); code != 0 {
		t.Fatalf("probe authorization preview exit = %d", code)
	}
	_ = runDoctor([]string{"--live", "--output", "json"})
	if code := runAttest(nil); code != 0 {
		t.Fatalf("attest authorization preview exit = %d", code)
	}
	if code := runAttest([]string{
		"--mode", "joint_absolute", "--task", "production-shaped task",
		"--trajectory", "candidate A", "--trajectory", "candidate B",
	}); code != 0 {
		t.Fatalf("production-shaped attest authorization preview exit = %d", code)
	}
	_ = runVerify([]string{
		"--mode", "pairwise", "--task", "test task", "--criteria", "code_review", "--no-cache",
		"--trajectory", "candidate A", "--trajectory", "candidate B",
	})
	marker := filepath.Join(t.TempDir(), "agent-command-ran")
	if code := runBon([]string{
		"-n", "2", "--task", "test task", "--include-working-tree", "--", "sh", "-c", "touch " + marker,
	}); code != 0 {
		t.Fatalf("Best-of-N authorization preview exit = %d", code)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("Best-of-N ran its agent command before live authorization: %v", err)
	}
	terminalRoot, swebenchRoot := writeLiveBoundaryEvalFixtures(t)
	if code := runEvalTerminal([]string{
		"--dry-run", "--limit", "1", "--root", terminalRoot, "--trajs", "fixture",
	}); code != 0 {
		t.Fatalf("Terminal-Bench dry run exit = %d", code)
	}
	if code := runEvalSWEbench([]string{
		"--dry-run", "--limit", "1", "--root", swebenchRoot,
	}); code != 0 {
		t.Fatalf("SWE-bench dry run exit = %d", code)
	}
	t.Setenv("EVALWITNESS_REPLAY_FROM", "../../scripts/tests/golden-delta-replay.jsonl")
	t.Setenv("EVALWITNESS_THINKING_MODE", "disabled")
	if code := runVerify([]string{
		"--provider", "replay", "--model", "golden-delta", "--base-url", "https://replay.invalid/v1",
		"--mode", "delta", "--task", "@../../scripts/tests/sample-task.txt",
		"--trajectory", "@../../scripts/tests/sample-traj-a.txt", "--trajectory", "@../../scripts/tests/sample-traj-b.txt",
		"--criteria", "generic", "--n-reps", "1", "--no-bias-mit", "--no-cache",
	}); code != 0 {
		t.Fatalf("replay verification exit = %d", code)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("default live, dry-run, or replay entrypoints dispatched %d HTTP requests", got)
	}
}

func writeLiveBoundaryEvalFixtures(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	terminalRoot := filepath.Join(root, "terminal")
	terminalTask := filepath.Join(terminalRoot, "fixture", "task")
	if err := os.MkdirAll(terminalTask, 0o700); err != nil {
		t.Fatal(err)
	}
	terminalTemplate := `{"trial_name":"TRIAL","task_name":"task","reward":REWARD,"n_output_tokens":1,"duration_seconds":1,"cost_cents":0,"trajectory":{"steps":[{"source":"user","message":"fix the fixture"},{"source":"assistant","message":"done"}]}}`
	for name, reward := range map[string]string{"pass": "1", "fail": "0"} {
		raw := strings.ReplaceAll(strings.ReplaceAll(terminalTemplate, "TRIAL", name), "REWARD", reward)
		if err := os.WriteFile(filepath.Join(terminalTask, name+"_trajectory.json"), []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	swebenchRoot := filepath.Join(root, "swebench")
	for name, reward := range map[string]string{"run-pass": "1", "run-fail": "0"} {
		runRoot := filepath.Join(swebenchRoot, name)
		if err := os.MkdirAll(runRoot, 0o700); err != nil {
			t.Fatal(err)
		}
		raw := `[{"instance_id":"fixture-1","messages":"[{\"role\":\"user\",\"content\":\"fix the fixture\"},{\"role\":\"assistant\",\"content\":\"done\"}]","model_name":"fixture","num_steps":1,"output_patch":"diff --git a/file b/file\n","reward":` + reward + `,"trajectory_id":"` + name + `"}]`
		if err := os.WriteFile(filepath.Join(runRoot, "data_cache.json"), []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return terminalRoot, swebenchRoot
}
