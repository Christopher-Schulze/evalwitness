package mutation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

const maximumCorpusSourceBytes = 64 * 1024 * 1024

type sourceCatalogEntry struct {
	Family         string
	RelativePath   string
	Revision       string
	SPDX           string
	SourceURL      string
	Redistribution string
	Attribution    string
}

var sweSourceCatalog = []sourceCatalogEntry{
	{
		Family: "swe-bench/claude-opus-4-6", RelativePath: "eval/trajectories/swebench_verified_trajs/claude_opus_46/data_cache.json",
		Revision: "evalwitness-release-data-v0.2.0", SPDX: "MIT", SourceURL: "https://github.com/SWE-bench/SWE-bench",
		Redistribution: "reference_only", Attribution: "SWE-bench and upstream trajectory authors",
	},
	{
		Family: "swe-bench/gemini-3-flash", RelativePath: "eval/trajectories/swebench_verified_trajs/gemini_3_flash_high_reasoning",
		Revision: "evalwitness-release-data-v0.2.0", SPDX: "MIT", SourceURL: "https://github.com/SWE-bench/SWE-bench",
		Redistribution: "reference_only", Attribution: "SWE-bench and upstream trajectory authors",
	},
}

func DiscoverDefaultCorpusSources(root string) ([]SourceCandidate, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	terminal, err := discoverTerminalSources(root)
	if err != nil {
		return nil, err
	}
	result := append([]SourceCandidate(nil), terminal...)
	for _, entry := range sweSourceCatalog {
		candidates, discoverErr := discoverSWESources(root, entry)
		if discoverErr != nil {
			return nil, discoverErr
		}
		result = append(result, candidates...)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Source.ID < result[right].Source.ID })
	return result, nil
}

func discoverTerminalSources(root string) ([]SourceCandidate, error) {
	base := filepath.Join(root, "eval", "trajectories", "terminal_trajs", "forge_gpt54")
	entry := sourceCatalogEntry{
		Family: "terminal-bench-2/forge-gpt54", Revision: "evalwitness-release-data-v0.2.0", SPDX: "Apache-2.0",
		SourceURL:      "https://huggingface.co/datasets/harborframework/terminal-bench-2-leaderboard/tree/11e0eb7f6b1cca7b4aee5f3ef39ede09c5d99f60",
		Redistribution: "reference_only", Attribution: "Harbor Terminal-Bench 2 leaderboard and trajectory authors",
	}
	var result []SourceCandidate
	err := filepath.WalkDir(base, func(path string, directoryEntry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if directoryEntry.IsDir() || !strings.HasSuffix(directoryEntry.Name(), "_trajectory.json") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if len(raw) > maximumCorpusSourceBytes {
			return fmt.Errorf("Terminal-Bench source %q exceeds 64 MiB", path)
		}
		var metadata struct {
			TaskName  string  `json:"task_name"`
			TrialName string  `json:"trial_name"`
			Reward    float64 `json:"reward"`
		}
		if err := json.Unmarshal(raw, &metadata); err != nil {
			return err
		}
		if missing(metadata.TaskName, metadata.TrialName) {
			return fmt.Errorf("Terminal-Bench source %q has no task or trial identity", path)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		candidate, err := newSourceCandidate(raw, metadata.TaskName, "terminal-bench-2/"+metadata.TaskName, metadata.TrialName, filepath.ToSlash(relative), metadata.Reward, entry)
		if err != nil {
			return err
		}
		result = append(result, candidate)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover Terminal-Bench corruption sources: %w", err)
	}
	return result, nil
}

func discoverSWESources(root string, entry sourceCatalogEntry) ([]SourceCandidate, error) {
	path := filepath.Join(root, filepath.FromSlash(entry.RelativePath))
	var files []string
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		err = filepath.WalkDir(path, func(candidatePath string, directoryEntry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !directoryEntry.IsDir() && strings.HasPrefix(directoryEntry.Name(), "data_cache") && strings.HasSuffix(directoryEntry.Name(), ".json") {
				files = append(files, candidatePath)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		files = []string{path}
	}
	sort.Strings(files)
	var result []SourceCandidate
	for _, filePath := range files {
		file, err := os.Open(filePath)
		if err != nil {
			return nil, err
		}
		decoder := json.NewDecoder(file)
		start, err := decoder.Token()
		if err != nil || start != json.Delim('[') {
			_ = file.Close()
			return nil, fmt.Errorf("SWE-bench cache %q is not a JSON array", filePath)
		}
		itemIndex := 0
		for decoder.More() {
			var raw json.RawMessage
			if err := decoder.Decode(&raw); err != nil {
				_ = file.Close()
				return nil, err
			}
			if len(raw) > maximumCorpusSourceBytes {
				_ = file.Close()
				return nil, fmt.Errorf("SWE-bench source %q item %d exceeds 64 MiB", filePath, itemIndex)
			}
			var metadata struct {
				InstanceID   string  `json:"instance_id"`
				TrajectoryID string  `json:"trajectory_id"`
				Reward       float64 `json:"reward"`
			}
			if err := json.Unmarshal(raw, &metadata); err != nil {
				_ = file.Close()
				return nil, err
			}
			repository := strings.SplitN(metadata.InstanceID, "__", 2)[0]
			relative, err := filepath.Rel(root, filePath)
			if err != nil {
				_ = file.Close()
				return nil, err
			}
			location := filepath.ToSlash(relative) + "#/" + strconv.Itoa(itemIndex)
			candidate, err := newSourceCandidate(raw, metadata.InstanceID, repository, metadata.TrajectoryID, location, metadata.Reward, entry)
			if err != nil {
				_ = file.Close()
				return nil, err
			}
			result = append(result, candidate)
			itemIndex++
		}
		end, err := decoder.Token()
		closeErr := file.Close()
		if err != nil || end != json.Delim(']') {
			return nil, fmt.Errorf("SWE-bench cache %q has an invalid array boundary", filePath)
		}
		if closeErr != nil {
			return nil, closeErr
		}
	}
	return result, nil
}

func newSourceCandidate(raw []byte, taskID, repositoryID, trajectoryID, sourceLocation string, reward float64, entry sourceCatalogEntry) (SourceCandidate, error) {
	if missing(taskID, repositoryID, trajectoryID, sourceLocation, entry.Family, entry.Revision) {
		return SourceCandidate{}, errors.New("corruption source identity is incomplete")
	}
	trajectory, err := preprocess.IngestReader(bytes.NewReader(raw), preprocess.FrozenCanonicalizationV1IngestOptions())
	if err != nil {
		return SourceCandidate{}, err
	}
	outcomeValue := strconv.FormatFloat(reward, 'g', -1, 64)
	groupID := "group-" + digestText(repositoryID+"\x00"+taskID)
	source := CorpusSource{
		ID: "source-" + trajectory.Digest, TaskID: taskID, RepositoryID: repositoryID,
		SourceFamily: entry.Family, SourceFormat: trajectory.SourceFormat, SourceLocation: sourceLocation, SourceRevision: entry.Revision,
		SourceDigest: trajectory.SourceDigest, TrajectoryDigest: trajectory.Digest, PatchDigest: trajectoryPatchDigest(trajectory), SplitGroupID: groupID,
		NearDuplicateID: trajectoryNearDuplicateID(trajectory, repositoryID, taskID),
		Outcome:         SourceOutcome{Kind: "benchmark_reward", Value: outcomeValue, WitnessDigest: digestText(trajectory.SourceDigest + "\x00" + outcomeValue)},
		License:         LicenseMetadata{SPDX: entry.SPDX, SourceURL: entry.SourceURL, SourceRevision: entry.Revision, Redistribution: entry.Redistribution, Attribution: entry.Attribution},
		Privacy:         PrivacyMetadata{Classification: "public", RedactionPolicyDigest: digestText("evalwitness.default-redaction-policy.v1"), PublicReleaseAllowed: true},
	}
	return SourceCandidate{Source: source, Trajectory: trajectory}, nil
}

func trajectoryNearDuplicateID(trajectory preprocess.Trajectory, repositoryID, taskID string) string {
	for _, event := range trajectory.Events {
		if event.Message == nil || !strings.EqualFold(event.Message.Role, "user") {
			continue
		}
		var text []string
		for _, part := range event.Message.Parts {
			if part.Kind == preprocess.ContentText && strings.TrimSpace(part.Text) != "" {
				text = append(text, part.Text)
			}
		}
		if len(text) > 0 {
			normalized := strings.ToLower(strings.Join(strings.Fields(strings.Join(text, " ")), " "))
			return "near-" + digestText(normalized)
		}
	}
	normalizedIdentity := strings.ToLower(strings.Join(strings.Fields(repositoryID+" "+taskID), " "))
	return "near-" + digestText(normalizedIdentity)
}

func trajectoryPatchDigest(trajectory preprocess.Trajectory) string {
	var patches []string
	for _, event := range trajectory.Events {
		if event.FileChange != nil && event.FileChange.DiffDigest != "" {
			patches = append(patches, event.FileChange.DiffDigest)
		}
	}
	if len(patches) == 0 {
		return ""
	}
	sort.Strings(patches)
	digest, _ := digestJSON(patches)
	return digest
}
