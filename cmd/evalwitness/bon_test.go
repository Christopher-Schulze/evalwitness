package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/config"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "base"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return dir
}

func TestBonAuthorizationPreviewUsesTheFullEvidenceBudget(t *testing.T) {
	cfg := config.Default()
	cfg.EvidenceTokens = 256
	criteria := []verifier.Criterion{verifier.BuiltinCriteria["code_review"]}
	service, err := verification.NewProductionService(cfg, version)
	if err != nil {
		t.Fatal(err)
	}
	policy := verification.PolicyFromConfig(cfg, cfg.DefaultReps, cfg.Epsilon, cfg.BiasMitigation, cfg.SPRT.Enabled, "pairwise")
	maximum, err := service.Plan(verification.Input{
		Entrypoint: "best-of-n", Mode: verification.ModePairwise, Task: "repair the defect",
		Trajectories: []string{strings.Repeat("x", cfg.EvidenceTokens*4+1024), strings.Repeat("x", cfg.EvidenceTokens*4+1024)},
		Criteria:     criteria, Policy: policy,
	})
	if err != nil {
		t.Fatal(err)
	}
	placeholder, err := service.Plan(verification.Input{
		Entrypoint: "best-of-n", Mode: verification.ModePairwise, Task: "repair the defect",
		Trajectories: []string{"placeholder A", "placeholder B"}, Criteria: criteria, Policy: policy,
	})
	if err != nil {
		t.Fatal(err)
	}
	if maximum.Requests.MaximumInputTokens <= placeholder.Requests.MaximumInputTokens {
		t.Fatalf("maximum input = %d, placeholder = %d", maximum.Requests.MaximumInputTokens, placeholder.Requests.MaximumInputTokens)
	}
}

func TestRunBonAttemptsCapturesTranscriptAndDiff(t *testing.T) {
	repo := initTestRepo(t)
	runDir := t.TempDir()
	attempts, err := runBonAttempts(context.Background(), bonRunSpec{
		RepoRoot: repo,
		RunDir:   runDir,
		N:        2,
		CmdArgs:  []string{"sh", "-c", `echo "attempt $EVALWITNESS_BON_ATTEMPT solving" && echo "solution $EVALWITNESS_BON_ATTEMPT" > solution.txt`},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 2 {
		t.Fatalf("attempts = %d", len(attempts))
	}
	for i, a := range attempts {
		if a.ExitCode != 0 {
			t.Errorf("attempt %d exit = %d", i, a.ExitCode)
		}
		if !strings.Contains(a.Transcript, "solving") {
			t.Errorf("attempt %d transcript = %q", i, a.Transcript)
		}
		if a.DiffEmpty || !strings.Contains(a.Diff, "solution.txt") {
			t.Errorf("attempt %d diff missing new file: empty=%t", i, a.DiffEmpty)
		}
		if _, err := os.Stat(a.DiffPath); err != nil {
			t.Errorf("attempt %d diff file: %v", i, err)
		}
		wt := bonWorktreePath(runDir, i)
		if _, err := os.Stat(wt); !os.IsNotExist(err) {
			t.Errorf("attempt %d worktree not cleaned up: %s", i, wt)
		}
	}
	// Attempts must be isolated: each diff carries its own attempt content.
	if !strings.Contains(attempts[0].Diff, "solution 0") || !strings.Contains(attempts[1].Diff, "solution 1") {
		t.Errorf("attempt isolation broken")
	}

	traj := attempts[0].trajectoryText([]string{"sh", "-c", "..."})
	if !strings.Contains(traj, "--- Agent Step 1 ---") || !strings.Contains(traj, "--- Agent Step 2 ---") {
		t.Errorf("trajectory format wrong:\n%s", traj)
	}
	if !strings.Contains(traj, "exit code: 0") {
		t.Errorf("trajectory missing exit code")
	}
}

func TestBonAttemptSnapshotsDoNotWriteObjectsIntoUserRepository(t *testing.T) {
	repo := initTestRepo(t)
	before, err := gitRun(repo, "count-objects", "-v")
	if err != nil {
		t.Fatal(err)
	}
	_, err = runBonAttempts(context.Background(), bonRunSpec{
		RepoRoot: repo, RunDir: t.TempDir(), N: 1,
		CmdArgs: []string{"sh", "-c", `printf 'attempt-only-content' > attempt.bin`},
	})
	if err != nil {
		t.Fatal(err)
	}
	after, err := gitRun(repo, "count-objects", "-v")
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("attempt snapshot leaked Git objects into the user repository:\nbefore:\n%safter:\n%s", before, after)
	}
}

func TestBoundedRejectWriterFailsBeforeGrowingPastLimit(t *testing.T) {
	writer := &boundedRejectWriter{limit: 4}
	written, err := writer.Write([]byte("12345"))
	if !errors.Is(err, errBonOutputLimit) || written != 4 || string(writer.data) != "1234" {
		t.Fatalf("bounded reject writer: written=%d data=%q err=%v", written, writer.data, err)
	}
}

func TestRunBonAttemptNonZeroExit(t *testing.T) {
	repo := initTestRepo(t)
	attempts, err := runBonAttempts(context.Background(), bonRunSpec{
		RepoRoot: repo,
		RunDir:   t.TempDir(),
		N:        2,
		CmdArgs:  []string{"sh", "-c", "echo broken >&2; exit 3"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if attempts[0].ExitCode != 3 {
		t.Errorf("exit = %d, want 3", attempts[0].ExitCode)
	}
	if !strings.Contains(attempts[0].Transcript, "broken") {
		t.Errorf("stderr not captured: %q", attempts[0].Transcript)
	}
	if !attempts[0].DiffEmpty {
		t.Errorf("expected empty diff")
	}
}

func TestApplyBonDiff(t *testing.T) {
	repo := initTestRepo(t)
	runDir := t.TempDir()
	_, destination, err := prepareBonSource(repo, runDir, false)
	if err != nil {
		t.Fatal(err)
	}
	attempts, err := runBonAttempts(context.Background(), bonRunSpec{
		RepoRoot: repo,
		RunDir:   runDir,
		N:        2,
		CmdArgs:  []string{"sh", "-c", `echo "winner $EVALWITNESS_BON_ATTEMPT" > solution.txt`},
	})
	if err != nil {
		t.Fatal(err)
	}
	summary, err := applyBonDiff(repo, runDir, destination, attempts[1])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(summary, "solution.txt") {
		t.Fatalf("patch summary = %q", summary)
	}
	data, err := os.ReadFile(filepath.Join(repo, "solution.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "winner 1" {
		t.Errorf("applied content = %q", data)
	}
	// Dirty tree must refuse a second apply.
	if _, err := applyBonDiff(repo, runDir, destination, attempts[0]); err == nil {
		t.Errorf("apply on dirty tree must fail")
	}
	staged, err := gitRun(repo, "diff", "--cached", "--name-only")
	if err != nil {
		t.Fatal(err)
	}
	if staged != "" {
		t.Fatalf("apply unexpectedly staged files: %q", staged)
	}
}

func TestPrepareBonSourceFailsClosedUnlessWorkingTreeSnapshotIsExplicit(t *testing.T) {
	repo := initTestRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "new.txt"), []byte("untracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := t.TempDir()
	if _, _, err := prepareBonSource(repo, runDir, false); err == nil {
		t.Fatal("dirty source was accepted without --include-working-tree")
	}
	beforeIndex, err := gitRun(repo, "write-tree")
	if err != nil {
		t.Fatal(err)
	}
	source, destination, err := prepareBonSource(repo, runDir, true)
	if err != nil {
		t.Fatal(err)
	}
	afterIndex, err := gitRun(repo, "write-tree")
	if err != nil {
		t.Fatal(err)
	}
	if beforeIndex != afterIndex {
		t.Fatalf("source snapshot changed the user index: %q != %q", beforeIndex, afterIndex)
	}
	if source.Mode != "working_tree_snapshot" || source.SnapshotCommit == source.HeadCommit || source.DestinationDigest != destination.Digest {
		t.Fatalf("source=%+v destination=%+v", source, destination)
	}
	show, err := gitRun(repo, "show", source.SnapshotCommit+":new.txt")
	if err != nil || show != "untracked\n" {
		t.Fatalf("snapshot omitted allowed untracked file: %v %q", err, show)
	}
}

func TestBonAttemptBoundsOutputAndRedactsExplicitEnvironment(t *testing.T) {
	repo := initTestRepo(t)
	t.Setenv("BON_TEST_SECRET", "super-secret-bon-value")
	attempts, err := runBonAttempts(context.Background(), bonRunSpec{
		RepoRoot: repo, RunDir: t.TempDir(), N: 1, PassEnv: []string{"BON_TEST_SECRET"},
		CmdArgs: []string{"sh", "-c", `dd if=/dev/zero bs=1048576 count=5 2>/dev/null; printf '%s\n' "$BON_TEST_SECRET"`},
	})
	if err != nil {
		t.Fatal(err)
	}
	attempt := attempts[0]
	if !attempt.TranscriptTruncated || attempt.TranscriptDropped == 0 || len(attempt.Transcript) > maxTranscriptBytes {
		t.Fatalf("attempt output was not bounded: %+v bytes=%d", attempt, len(attempt.Transcript))
	}
	if strings.Contains(attempt.Transcript, "super-secret-bon-value") {
		t.Fatal("explicitly passed secret survived transcript redaction")
	}
	if !strings.Contains(attempt.Transcript, "[REDACTED]") {
		t.Fatal("test did not observe explicit secret redaction")
	}
	artifact, err := os.ReadFile(attempt.TranscriptPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(artifact), "super-secret-bon-value") {
		t.Fatal("explicitly passed secret survived artifact redaction")
	}
}

func TestBonParentCancellationKillsProcessGroupAndCleansWorktree(t *testing.T) {
	repo := initTestRepo(t)
	runDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(150*time.Millisecond, cancel)
	started := time.Now()
	_, err := runBonAttempts(ctx, bonRunSpec{
		RepoRoot: repo, RunDir: runDir, N: 1,
		CmdArgs: []string{"sh", "-c", `sleep 30 & wait`},
	})
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("cancellation error = %v", err)
	}
	if time.Since(started) > 5*time.Second {
		t.Fatalf("process group cancellation took %s", time.Since(started))
	}
	if _, statErr := os.Stat(bonWorktreePath(runDir, 0)); !os.IsNotExist(statErr) {
		t.Fatalf("cancelled attempt retained worktree: %v", statErr)
	}
}

func TestBonAttemptTimeoutKillsProcessGroupAndCapturesResult(t *testing.T) {
	repo := initTestRepo(t)
	runDir := t.TempDir()
	started := time.Now()
	attempts, err := runBonAttempts(context.Background(), bonRunSpec{
		RepoRoot: repo, RunDir: runDir, N: 1, TimeoutSec: 1,
		CmdArgs: []string{"sh", "-c", `sleep 30 & wait`},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 1 || !attempts[0].TimedOut || attempts[0].ExitCode == 0 {
		t.Fatalf("timeout attempt = %+v", attempts)
	}
	if time.Since(started) > 5*time.Second {
		t.Fatalf("attempt timeout took %s", time.Since(started))
	}
	if _, statErr := os.Stat(bonWorktreePath(runDir, 0)); !os.IsNotExist(statErr) {
		t.Fatalf("timed-out attempt retained worktree: %v", statErr)
	}
}

func TestBonArtifactWriteFailureReportsENOSPCAndStillCleansWorktree(t *testing.T) {
	repo := initTestRepo(t)
	runDir := t.TempDir()
	_, err := runBonAttempts(context.Background(), bonRunSpec{
		RepoRoot: repo, RunDir: runDir, N: 1,
		CmdArgs:       []string{"sh", "-c", `echo result > result.txt`},
		WriteArtifact: func(string, []byte) error { return syscall.ENOSPC },
	})
	if !errors.Is(err, syscall.ENOSPC) {
		t.Fatalf("artifact write failure = %v", err)
	}
	if _, statErr := os.Stat(bonWorktreePath(runDir, 0)); !os.IsNotExist(statErr) {
		t.Fatalf("artifact failure retained worktree: %v", statErr)
	}
}

func TestBonCleanupFailureReportsExactRetainedPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell worktree metadata fixture is Unix-only")
	}
	repo := initTestRepo(t)
	runDir := t.TempDir()
	worktree := bonWorktreePath(runDir, 0)
	_, err := runBonAttempts(context.Background(), bonRunSpec{
		RepoRoot: repo, RunDir: runDir, N: 1,
		CmdArgs: []string{"sh", "-c", `gitdir=$(sed 's/^gitdir: //' .git); mv "$gitdir" "$gitdir.moved"; echo result > result.txt`},
	})
	if err == nil || !strings.Contains(err.Error(), "cleanup retained worktree "+worktree) {
		t.Fatalf("cleanup failure = %v", err)
	}
	if _, statErr := os.Stat(worktree); statErr != nil {
		t.Fatalf("cleanup failure did not retain exact worktree: %v", statErr)
	}
}

func TestBonManifestRejectsArtifactTampering(t *testing.T) {
	repo := initTestRepo(t)
	runDir := t.TempDir()
	source, destination, err := prepareBonSource(repo, runDir, false)
	if err != nil {
		t.Fatal(err)
	}
	attempts, err := runBonAttempts(context.Background(), bonRunSpec{
		RepoRoot: repo, RunDir: runDir, Source: source.SnapshotCommit, N: 2,
		CmdArgs: []string{"sh", "-c", `echo result > result.txt`},
	})
	if err != nil {
		t.Fatal(err)
	}
	manifestPath, err := writeBonManifest(bonRunManifest{
		RepositoryRoot: repo, RunDirectory: runDir, Source: source, Destination: destination,
		Task: "test", Command: []string{"sh", "-c", "echo result > result.txt"},
		Criteria: []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Policy:   validBonPolicy(), Attempts: attempts, PlanFingerprint: strings.Repeat("a", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadBonManifest(manifestPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(attempts[0].TranscriptPath, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadBonManifest(manifestPath); err == nil {
		t.Fatal("tampered attempt artifact was accepted")
	}
}

func TestBonManifestRejectsNonPrivatePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission fixture requires Unix permissions")
	}
	repo := initTestRepo(t)
	runDir := t.TempDir()
	source, destination, err := prepareBonSource(repo, runDir, false)
	if err != nil {
		t.Fatal(err)
	}
	attempts, err := runBonAttempts(context.Background(), bonRunSpec{
		RepoRoot: repo, RunDir: runDir, Source: source.SnapshotCommit, N: 2,
		CmdArgs: []string{"sh", "-c", `echo result > result.txt`},
	})
	if err != nil {
		t.Fatal(err)
	}
	manifestPath, err := writeBonManifest(bonRunManifest{
		RepositoryRoot: repo, RunDirectory: runDir, Source: source, Destination: destination,
		Task: "test", Command: []string{"sh", "-c", "echo result > result.txt"},
		Criteria: []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Policy:   validBonPolicy(), Attempts: attempts, PlanFingerprint: strings.Repeat("a", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(manifestPath, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadBonManifest(manifestPath); err == nil || !strings.Contains(err.Error(), "permissions are not private") {
		t.Fatalf("non-private manifest error = %v", err)
	}
}

func TestBonManifestRejectsSymlinkedArtifacts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires Unix permissions")
	}
	repo := initTestRepo(t)
	runDir := t.TempDir()
	source, destination, err := prepareBonSource(repo, runDir, false)
	if err != nil {
		t.Fatal(err)
	}
	attempts, err := runBonAttempts(context.Background(), bonRunSpec{
		RepoRoot: repo, RunDir: runDir, Source: source.SnapshotCommit, N: 2,
		CmdArgs: []string{"sh", "-c", `echo result > result.txt`},
	})
	if err != nil {
		t.Fatal(err)
	}
	manifestPath, err := writeBonManifest(bonRunManifest{
		RepositoryRoot: repo, RunDirectory: runDir, Source: source, Destination: destination,
		Task: "test", Command: []string{"sh", "-c", "echo result > result.txt"},
		Criteria: []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Policy:   validBonPolicy(), Attempts: attempts, PlanFingerprint: strings.Repeat("a", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.diff")
	if err := os.WriteFile(outside, []byte(attempts[0].Diff), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(attempts[0].DiffPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, attempts[0].DiffPath); err != nil {
		t.Fatal(err)
	}
	if _, err := loadBonManifest(manifestPath); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("symlinked artifact error = %v", err)
	}
}

func TestBonCommandRejectsEmbeddedSecrets(t *testing.T) {
	t.Setenv("BON_COMMAND_SECRET", "secret-value-for-command")
	if err := validateBonCommandArguments([]string{"agent", "--token=secret-value-for-command"}, []string{"BON_COMMAND_SECRET"}); err == nil {
		t.Fatal("command embedded an explicitly passed environment secret")
	}
	if err := validateBonCommandArguments([]string{"sh", "-c", `printf '%s' "$BON_COMMAND_SECRET"`}, []string{"BON_COMMAND_SECRET"}); err != nil {
		t.Fatalf("environment reference was rejected: %v", err)
	}
}

func validBonPolicy() verification.Policy {
	cfg := config.Default()
	return verification.PolicyFromConfig(cfg, cfg.DefaultReps, cfg.Epsilon, cfg.BiasMitigation, cfg.SPRT.Enabled, "pairwise")
}
