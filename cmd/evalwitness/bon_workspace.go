package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

type bonSourceSnapshot struct {
	Mode              string `json:"mode"`
	HeadCommit        string `json:"head_commit"`
	SnapshotCommit    string `json:"snapshot_commit"`
	ContentTree       string `json:"content_tree"`
	SnapshotDigest    string `json:"snapshot_digest"`
	DestinationDigest string `json:"destination_digest"`
}

type bonDestinationState struct {
	HeadCommit   string `json:"head_commit"`
	ContentTree  string `json:"content_tree"`
	IndexTree    string `json:"index_tree"`
	StatusDigest string `json:"status_digest"`
	Digest       string `json:"digest"`
}

func prepareBonSource(repoRoot, runDir string, includeWorkingTree bool) (bonSourceSnapshot, bonDestinationState, error) {
	destination, err := captureBonDestination(repoRoot, runDir)
	if err != nil {
		return bonSourceSnapshot{}, bonDestinationState{}, err
	}
	status, err := gitRun(repoRoot, "status", "--porcelain=v2", "-z", "--untracked-files=all")
	if err != nil {
		return bonSourceSnapshot{}, bonDestinationState{}, fmt.Errorf("read source status: %w", err)
	}
	if !includeWorkingTree && status != "" {
		return bonSourceSnapshot{}, bonDestinationState{}, errors.New("source worktree is dirty; commit changes or use --include-working-tree to snapshot them without changing the index")
	}
	mode := "clean_head"
	snapshotCommit := destination.HeadCommit
	if includeWorkingTree {
		mode = "working_tree_snapshot"
		persistentTree, treeErr := snapshotBonContentTreePersistent(repoRoot, runDir)
		if treeErr != nil {
			return bonSourceSnapshot{}, bonDestinationState{}, treeErr
		}
		if persistentTree != destination.ContentTree {
			return bonSourceSnapshot{}, bonDestinationState{}, errors.New("source worktree changed while creating its explicit snapshot")
		}
		snapshotCommit, err = commitBonTree(repoRoot, persistentTree, destination.HeadCommit)
		if err != nil {
			return bonSourceSnapshot{}, bonDestinationState{}, err
		}
	}
	snapshotDigest, err := digestJSON(struct {
		Head string `json:"head"`
		Tree string `json:"tree"`
		Mode string `json:"mode"`
	}{destination.HeadCommit, destination.ContentTree, mode})
	if err != nil {
		return bonSourceSnapshot{}, bonDestinationState{}, err
	}
	return bonSourceSnapshot{
		Mode: mode, HeadCommit: destination.HeadCommit, SnapshotCommit: snapshotCommit,
		ContentTree: destination.ContentTree, SnapshotDigest: snapshotDigest, DestinationDigest: destination.Digest,
	}, destination, nil
}

func captureBonDestination(repoRoot, runDir string) (bonDestinationState, error) {
	head, err := gitRun(repoRoot, "rev-parse", "HEAD")
	if err != nil {
		return bonDestinationState{}, fmt.Errorf("resolve destination HEAD: %w", err)
	}
	indexTree, err := gitRun(repoRoot, "write-tree")
	if err != nil {
		return bonDestinationState{}, fmt.Errorf("resolve destination index: %w", err)
	}
	status, err := gitRun(repoRoot, "status", "--porcelain=v2", "-z", "--untracked-files=all")
	if err != nil {
		return bonDestinationState{}, fmt.Errorf("resolve destination status: %w", err)
	}
	contentTree, err := snapshotBonContentTree(repoRoot, runDir)
	if err != nil {
		return bonDestinationState{}, err
	}
	state := bonDestinationState{
		HeadCommit: strings.TrimSpace(head), ContentTree: strings.TrimSpace(contentTree),
		IndexTree: strings.TrimSpace(indexTree), StatusDigest: digestBytes([]byte(status)),
	}
	state.Digest, err = digestJSON(struct {
		Head   string `json:"head"`
		Tree   string `json:"tree"`
		Index  string `json:"index"`
		Status string `json:"status"`
	}{state.HeadCommit, state.ContentTree, state.IndexTree, state.StatusDigest})
	if err != nil {
		return bonDestinationState{}, err
	}
	return state, nil
}

type bonContentSnapshot struct {
	Tree        string
	Environment []string
	objectDir   string
}

func snapshotBonContentTree(worktree, runDir string) (tree string, err error) {
	snapshot, err := createBonContentSnapshot(worktree, runDir, false)
	if err != nil {
		return "", err
	}
	return snapshot.Tree, snapshot.Close()
}

func snapshotBonContentTreePersistent(worktree, runDir string) (string, error) {
	snapshot, err := createBonContentSnapshot(worktree, runDir, true)
	if err != nil {
		return "", err
	}
	return snapshot.Tree, snapshot.Close()
}

func createBonContentSnapshot(worktree, runDir string, persistentObjects bool) (snapshot bonContentSnapshot, err error) {
	indexFile, err := os.CreateTemp(runDir, "snapshot-index-*")
	if err != nil {
		return bonContentSnapshot{}, fmt.Errorf("create temporary snapshot index: %w", err)
	}
	indexPath := indexFile.Name()
	if err := indexFile.Close(); err != nil {
		return bonContentSnapshot{}, err
	}
	if err := os.Remove(indexPath); err != nil {
		return bonContentSnapshot{}, err
	}
	defer func() {
		removeErr := os.Remove(indexPath)
		if removeErr != nil && !os.IsNotExist(removeErr) {
			err = errors.Join(err, fmt.Errorf("remove temporary snapshot index: %w", removeErr))
		}
		if err != nil {
			err = errors.Join(err, snapshot.Close())
		}
	}()
	environment := withBonEnvironment(os.Environ(), "GIT_INDEX_FILE", indexPath)
	if !persistentObjects {
		objectDirectory, objectErr := os.MkdirTemp(runDir, "snapshot-objects-*")
		if objectErr != nil {
			return bonContentSnapshot{}, fmt.Errorf("create temporary snapshot object directory: %w", objectErr)
		}
		if chmodErr := os.Chmod(objectDirectory, safety.SensitiveDirectoryMode); chmodErr != nil {
			return bonContentSnapshot{}, errors.Join(chmodErr, os.RemoveAll(objectDirectory))
		}
		commonObjects, commonErr := bonCommonObjectDirectory(worktree)
		if commonErr != nil {
			return bonContentSnapshot{}, errors.Join(commonErr, os.RemoveAll(objectDirectory))
		}
		environment = withBonEnvironment(environment, "GIT_OBJECT_DIRECTORY", objectDirectory)
		environment = withBonEnvironment(environment, "GIT_ALTERNATE_OBJECT_DIRECTORIES", commonObjects)
		snapshot.objectDir = objectDirectory
	}
	if output, runErr := gitRunEnv(worktree, environment, "read-tree", "HEAD"); runErr != nil {
		return bonContentSnapshot{}, errors.Join(fmt.Errorf("initialize snapshot index: %w: %s", runErr, output), snapshot.Close())
	}
	if output, runErr := gitRunEnv(worktree, environment, "add", "-A", "--", "."); runErr != nil {
		return bonContentSnapshot{}, errors.Join(fmt.Errorf("populate snapshot index: %w: %s", runErr, output), snapshot.Close())
	}
	output, runErr := gitRunEnv(worktree, environment, "write-tree")
	if runErr != nil {
		return bonContentSnapshot{}, errors.Join(fmt.Errorf("write snapshot tree: %w: %s", runErr, output), snapshot.Close())
	}
	snapshot.Tree = strings.TrimSpace(output)
	snapshot.Environment = environment
	return snapshot, nil
}

func (snapshot bonContentSnapshot) Close() error {
	if snapshot.objectDir == "" {
		return nil
	}
	if err := os.RemoveAll(snapshot.objectDir); err != nil {
		return fmt.Errorf("remove temporary snapshot objects %s: %w", snapshot.objectDir, err)
	}
	return nil
}

func bonCommonObjectDirectory(worktree string) (string, error) {
	output, err := gitRun(worktree, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("resolve common Git directory: %w: %s", err, output)
	}
	commonDirectory := strings.TrimSpace(output)
	if !filepath.IsAbs(commonDirectory) {
		commonDirectory = filepath.Join(worktree, commonDirectory)
	}
	return filepath.Clean(filepath.Join(commonDirectory, "objects")), nil

}

func withBonEnvironment(environment []string, name, value string) []string {
	prefix := name + "="
	result := make([]string, 0, len(environment)+1)
	for _, item := range environment {
		if !strings.HasPrefix(item, prefix) {
			result = append(result, item)
		}
	}
	return append(result, prefix+value)
}

func commitBonTree(repoRoot, tree, parent string) (string, error) {
	environment := append(os.Environ(),
		"GIT_AUTHOR_NAME=EvalWitness", "GIT_AUTHOR_EMAIL=local@evalwitness.invalid", "GIT_AUTHOR_DATE=@0 +0000",
		"GIT_COMMITTER_NAME=EvalWitness", "GIT_COMMITTER_EMAIL=local@evalwitness.invalid", "GIT_COMMITTER_DATE=@0 +0000",
	)
	output, err := gitRunEnvInput(repoRoot, environment, "EvalWitness working-tree snapshot\n", "commit-tree", tree, "-p", parent)
	if err != nil {
		return "", fmt.Errorf("create detached source snapshot: %w: %s", err, output)
	}
	return strings.TrimSpace(output), nil
}

func diffBonWorkingTree(worktree, baseCommit, runDir string) (string, error) {
	snapshot, err := createBonContentSnapshot(worktree, runDir, false)
	if err != nil {
		return "", err
	}
	output, runErr := gitRunBoundedEnv(worktree, snapshot.Environment, maxAttemptDiffBytes, "diff", "--binary", baseCommit, snapshot.Tree)
	closeErr := snapshot.Close()
	err = errors.Join(runErr, closeErr)
	if err != nil {
		return "", fmt.Errorf("capture attempt diff: %w: %s", err, output)
	}
	return output, nil
}

func gitRunBoundedEnv(dir string, environment []string, limit int, args ...string) (string, error) {
	command := exec.Command("git", args...)
	command.Dir = dir
	if environment != nil {
		command.Env = environment
	}
	stdout := &boundedRejectWriter{limit: limit}
	stderr := newBoundedTailWriter(1 << 20)
	command.Stdout = stdout
	command.Stderr = stderr
	err := command.Run()
	errorOutput, _ := stderr.Snapshot()
	if errors.Is(err, errBonOutputLimit) {
		return string(stdout.data), fmt.Errorf("git output exceeds %d bytes: %w", limit, errBonOutputLimit)
	}
	if err != nil {
		return string(errorOutput), err
	}
	return string(stdout.data), nil
}

func applyBonDiff(repoRoot, runDir string, expected bonDestinationState, attempt bonAttempt) (string, error) {
	if attempt.DiffEmpty {
		return "", fmt.Errorf("winning attempt %d produced no changes", attempt.Index)
	}
	current, err := captureBonDestination(repoRoot, runDir)
	if err != nil {
		return "", err
	}
	if current.Digest != expected.Digest {
		return "", fmt.Errorf("destination changed since selection: expected %s, observed %s", expected.Digest, current.Digest)
	}
	summary, err := gitRun(repoRoot, "apply", "--stat", attempt.DiffPath)
	if err != nil {
		return "", fmt.Errorf("summarize winning patch: %w: %s", err, summary)
	}
	if output, checkErr := gitRun(repoRoot, "apply", "--check", attempt.DiffPath); checkErr != nil {
		return summary, fmt.Errorf("winning patch conflicts with destination: %w: %s", checkErr, output)
	}
	if output, applyErr := gitRun(repoRoot, "apply", attempt.DiffPath); applyErr != nil {
		return summary, fmt.Errorf("apply winning patch without staging: %w: %s", applyErr, output)
	}
	afterIndex, err := gitRun(repoRoot, "write-tree")
	if err != nil {
		return summary, fmt.Errorf("verify destination index: %w", err)
	}
	if strings.TrimSpace(afterIndex) != expected.IndexTree {
		return summary, errors.New("winning patch unexpectedly changed the destination index")
	}
	return summary, nil
}

func gitRunEnv(dir string, environment []string, args ...string) (string, error) {
	return gitRunEnvInput(dir, environment, "", args...)
}

func gitRunEnvInput(dir string, environment []string, input string, args ...string) (string, error) {
	command := exec.Command("git", args...)
	command.Dir = dir
	command.Env = environment
	if input != "" {
		command.Stdin = strings.NewReader(input)
	}
	output, err := command.CombinedOutput()
	return string(output), err
}

func digestJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return digestBytes(encoded), nil
}

func digestBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func secureArtifactPath(runDir, name string) string {
	return filepath.Join(runDir, name)
}
