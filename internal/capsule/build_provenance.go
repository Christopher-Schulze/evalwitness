package capsule

import (
	"bytes"
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Christopher-Schulze/evalwitness/internal/product"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

var recordedBuildSettingKeys = map[string]struct{}{
	"-buildmode": {}, "-compiler": {}, "CGO_ENABLED": {}, "DefaultGODEBUG": {},
	"GO386": {}, "GOAMD64": {}, "GOARCH": {}, "GOARM": {}, "GOARM64": {}, "GOFIPS140": {}, "GOOS": {},
	"vcs": {}, "vcs.modified": {}, "vcs.revision": {}, "vcs.time": {},
}

func CollectBuildProvenance(ctx context.Context, repositoryRoot, binaryPath, artifactKind string) (SourceTreeProvenance, BuildProvenance, error) {
	if ctx == nil || repositoryRoot == "" || binaryPath == "" || !validIdentifier(artifactKind) {
		return SourceTreeProvenance{}, BuildProvenance{}, errors.New("build provenance collection requires context, repository, binary, and artifact kind")
	}
	source, err := collectSourceTreeProvenance(ctx, repositoryRoot)
	if err != nil {
		return SourceTreeProvenance{}, BuildProvenance{}, err
	}
	binaryDigest, binaryBytes, err := digestRegularBuildArtifact(binaryPath)
	if err != nil {
		return SourceTreeProvenance{}, BuildProvenance{}, err
	}
	info, err := buildinfo.ReadFile(binaryPath)
	if err != nil {
		return SourceTreeProvenance{}, BuildProvenance{}, fmt.Errorf("read Go build information: %w", err)
	}
	settings, targetGOOS, targetGOARCH, embeddedRevision, embeddedModified, err := collectBuildSettings(info.Settings)
	if err != nil {
		return SourceTreeProvenance{}, BuildProvenance{}, err
	}
	matchStatus := "embedded-vcs-unavailable"
	if embeddedRevision != "" {
		if embeddedRevision != source.Commit {
			return SourceTreeProvenance{}, BuildProvenance{}, errors.New("binary embedded VCS revision differs from the source-tree commit")
		}
		if embeddedModified != "unavailable" && (embeddedModified == "true") != source.Dirty {
			return SourceTreeProvenance{}, BuildProvenance{}, errors.New("binary embedded VCS modified state differs from the source tree")
		}
		matchStatus = "matched"
	}
	dependencies := collectBuildDependencies(info.Deps)
	dependencyDigest, err := protocol.Digest(dependencies)
	if err != nil {
		return SourceTreeProvenance{}, BuildProvenance{}, err
	}
	build := BuildProvenance{
		SchemaVersion: BuildProvenanceSchemaVersion, ProductVersion: product.Version, ArtifactKind: artifactKind,
		BinaryDigest: binaryDigest, BinaryBytes: binaryBytes, BinaryIncluded: false,
		Commit: source.Commit, Dirty: source.Dirty, SourceTreeDigest: source.Digest,
		GoVersion: info.GoVersion, MainPackage: info.Path, TargetGOOS: targetGOOS, TargetGOARCH: targetGOARCH,
		BuildFlags: settings, Dependencies: dependencies, DependencyManifestDigest: dependencyDigest,
		EmbeddedVCSRevision: embeddedRevision, EmbeddedVCSModified: embeddedModified,
		SourceMatchStatus: matchStatus, Reproducibility: "digest-bound-external-binary",
	}
	build.Digest, err = buildProvenanceDigest(build)
	if err != nil {
		return SourceTreeProvenance{}, BuildProvenance{}, err
	}
	if err := build.Validate(); err != nil {
		return SourceTreeProvenance{}, BuildProvenance{}, err
	}
	return source, build, nil
}

func collectSourceTreeProvenance(ctx context.Context, repositoryRoot string) (SourceTreeProvenance, error) {
	portablePath := os.Getenv(PortableSourceTreeEnvironment)
	portableRoot := os.Getenv(PortableSourceRootEnvironment)
	if portablePath != "" || portableRoot != "" {
		if portablePath == "" || portableRoot == "" {
			return SourceTreeProvenance{}, errors.New("portable source-tree provenance requires both evidence and repository-root environment variables")
		}
		match, err := sameDirectory(repositoryRoot, portableRoot)
		if err != nil {
			return SourceTreeProvenance{}, fmt.Errorf("compare portable source-tree repository root: %w", err)
		}
		if match {
			return LoadPortableSourceTreeProvenance(repositoryRoot, portablePath)
		}
	}
	return collectGitSourceTreeProvenance(ctx, repositoryRoot)
}

func CollectSourceTreeProvenance(ctx context.Context, repositoryRoot string) (SourceTreeProvenance, error) {
	if ctx == nil || repositoryRoot == "" {
		return SourceTreeProvenance{}, errors.New("source-tree provenance collection requires context and repository")
	}
	return collectSourceTreeProvenance(ctx, repositoryRoot)
}

func CollectGitSourceTreeProvenance(ctx context.Context, repositoryRoot string) (SourceTreeProvenance, error) {
	if ctx == nil || repositoryRoot == "" {
		return SourceTreeProvenance{}, errors.New("git source-tree provenance collection requires context and repository")
	}
	return collectGitSourceTreeProvenance(ctx, repositoryRoot)
}

func collectGitSourceTreeProvenance(ctx context.Context, repositoryRoot string) (SourceTreeProvenance, error) {
	root, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return SourceTreeProvenance{}, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return SourceTreeProvenance{}, fmt.Errorf("resolve repository root: %w", err)
	}
	topLevelRaw, err := gitOutput(ctx, root, "rev-parse", "--show-toplevel")
	if err != nil {
		return SourceTreeProvenance{}, err
	}
	topLevel, err := filepath.EvalSymlinks(strings.TrimSpace(string(topLevelRaw)))
	if err != nil {
		return SourceTreeProvenance{}, errors.New("build provenance cannot resolve the Git top level")
	}
	sameRoot, err := sameDirectory(root, topLevel)
	if err != nil || !sameRoot {
		return SourceTreeProvenance{}, errors.New("build provenance root is not the Git top level")
	}
	commitRaw, err := gitOutput(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return SourceTreeProvenance{}, err
	}
	commit := strings.TrimSpace(string(commitRaw))
	status, err := gitOutput(ctx, root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return SourceTreeProvenance{}, err
	}
	trackedRaw, err := gitOutput(ctx, root, "ls-files", "--stage", "-z")
	if err != nil {
		return SourceTreeProvenance{}, err
	}
	tracked, err := parseTrackedSourceEntries(trackedRaw)
	if err != nil {
		return SourceTreeProvenance{}, err
	}
	untrackedRaw, err := gitOutput(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return SourceTreeProvenance{}, err
	}
	untracked, err := parseNULTerminatedPaths(untrackedRaw)
	if err != nil {
		return SourceTreeProvenance{}, err
	}
	entries := make([]SourceTreeEntry, 0, len(tracked)+len(untracked))
	for _, entry := range tracked {
		materialized, err := materializeSourceTreeEntry(root, entry.Path, entry.GitMode, "present")
		if errors.Is(err, os.ErrNotExist) {
			entry.Kind = "file"
			entry.State = "deleted"
			entries = append(entries, entry)
			continue
		}
		if err != nil {
			return SourceTreeProvenance{}, err
		}
		entries = append(entries, materialized)
	}
	for _, sourcePath := range untracked {
		if slices.ContainsFunc(entries, func(entry SourceTreeEntry) bool { return entry.Path == sourcePath }) {
			return SourceTreeProvenance{}, fmt.Errorf("source-tree path %q is both tracked and untracked", sourcePath)
		}
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(sourcePath)))
		if err != nil {
			return SourceTreeProvenance{}, err
		}
		mode := "100644"
		if info.Mode()&os.ModeSymlink != 0 {
			mode = "120000"
		} else if info.Mode().Perm()&0o111 != 0 {
			mode = "100755"
		}
		entry, err := materializeSourceTreeEntry(root, sourcePath, mode, "untracked")
		if err != nil {
			return SourceTreeProvenance{}, err
		}
		entries = append(entries, entry)
	}
	slices.SortFunc(entries, func(left, right SourceTreeEntry) int { return strings.Compare(left.Path, right.Path) })
	var total int64
	for _, entry := range entries {
		total += entry.Bytes
	}
	source := SourceTreeProvenance{
		SchemaVersion: SourceTreeProvenanceSchemaVersion, Algorithm: SourceTreeAlgorithm, VCS: "git",
		Commit: commit, Dirty: len(status) > 0, StatusDigest: protocol.DigestBytes(status),
		Files: len(entries), Bytes: total, Entries: entries,
	}
	source.Digest, err = sourceTreeProvenanceDigest(source)
	if err != nil {
		return SourceTreeProvenance{}, err
	}
	if err := source.Validate(); err != nil {
		return SourceTreeProvenance{}, err
	}
	return source, nil
}

func sameDirectory(left string, right string) (bool, error) {
	leftInfo, err := os.Stat(left)
	if err != nil {
		return false, err
	}
	rightInfo, err := os.Stat(right)
	if err != nil {
		return false, err
	}
	return leftInfo.IsDir() && rightInfo.IsDir() && os.SameFile(leftInfo, rightInfo), nil
}

func parseTrackedSourceEntries(raw []byte) ([]SourceTreeEntry, error) {
	records := bytes.Split(raw, []byte{0})
	entries := make([]SourceTreeEntry, 0, len(records))
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		if len(record) == 0 {
			continue
		}
		tab := bytes.IndexByte(record, '\t')
		if tab < 0 || !utf8.Valid(record[tab+1:]) {
			return nil, errors.New("git index record is invalid")
		}
		metadata := strings.Fields(string(record[:tab]))
		if len(metadata) != 3 || metadata[2] != "0" {
			return nil, errors.New("git index contains an unsupported or unmerged entry")
		}
		sourcePath := string(record[tab+1:])
		if !validSourcePath(sourcePath) {
			return nil, fmt.Errorf("git index path %q is invalid", sourcePath)
		}
		if _, duplicate := seen[sourcePath]; duplicate {
			return nil, fmt.Errorf("git index path %q is duplicated", sourcePath)
		}
		seen[sourcePath] = struct{}{}
		entries = append(entries, SourceTreeEntry{Path: sourcePath, GitMode: metadata[0]})
	}
	return entries, nil
}

func parseNULTerminatedPaths(raw []byte) ([]string, error) {
	records := bytes.Split(raw, []byte{0})
	paths := make([]string, 0, len(records))
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		if len(record) == 0 {
			continue
		}
		if !utf8.Valid(record) || !validSourcePath(string(record)) {
			return nil, errors.New("git path output contains an invalid path")
		}
		value := string(record)
		if _, duplicate := seen[value]; duplicate {
			return nil, fmt.Errorf("git path %q is duplicated", value)
		}
		seen[value] = struct{}{}
		paths = append(paths, value)
	}
	slices.Sort(paths)
	return paths, nil
}

func materializeSourceTreeEntry(root, sourcePath, gitMode, state string) (SourceTreeEntry, error) {
	if !validSourcePath(sourcePath) {
		return SourceTreeEntry{}, errors.New("source-tree path is invalid")
	}
	filePath := filepath.Join(root, filepath.FromSlash(sourcePath))
	info, err := os.Lstat(filePath)
	if err != nil {
		return SourceTreeEntry{}, err
	}
	entry := SourceTreeEntry{Path: sourcePath, GitMode: gitMode, State: state}
	if info.Mode()&os.ModeSymlink != 0 {
		if gitMode != "120000" {
			return SourceTreeEntry{}, fmt.Errorf("source-tree path %q changed to an undeclared symlink", sourcePath)
		}
		target, err := os.Readlink(filePath)
		if err != nil {
			return SourceTreeEntry{}, err
		}
		entry.Kind = "symlink"
		entry.LinkTarget = target
		entry.Bytes = int64(len([]byte(target)))
		entry.SHA256 = protocol.DigestBytes([]byte(target))
		return entry, nil
	}
	if !info.Mode().IsRegular() || gitMode == "120000" {
		return SourceTreeEntry{}, fmt.Errorf("source-tree path %q is not the declared regular file", sourcePath)
	}
	digest, size, err := digestRegularBuildArtifact(filePath)
	if err != nil {
		return SourceTreeEntry{}, err
	}
	entry.Kind = "file"
	entry.Bytes = size
	entry.SHA256 = digest
	return entry, nil
}

func digestRegularBuildArtifact(filePath string) (string, int64, error) {
	info, err := os.Lstat(filePath)
	if err != nil {
		return "", 0, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "", 0, fmt.Errorf("build artifact %q is not a regular file", filePath)
	}
	file, err := os.Open(filePath)
	if err != nil {
		return "", 0, err
	}
	digest := sha256.New()
	written, copyErr := io.Copy(digest, file)
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil || written != info.Size() {
		return "", 0, errors.Join(copyErr, closeErr, errors.New("build artifact digest read was incomplete"))
	}
	return hex.EncodeToString(digest.Sum(nil)), written, nil
}

func collectBuildSettings(values []debug.BuildSetting) ([]BuildSetting, string, string, string, string, error) {
	settings := make([]BuildSetting, 0, len(values))
	targetGOOS := ""
	targetGOARCH := ""
	embeddedRevision := ""
	embeddedModified := "unavailable"
	for _, value := range values {
		if _, recorded := recordedBuildSettingKeys[value.Key]; !recorded {
			continue
		}
		settings = append(settings, BuildSetting{Key: value.Key, Value: value.Value})
		switch value.Key {
		case "GOOS":
			targetGOOS = value.Value
		case "GOARCH":
			targetGOARCH = value.Value
		case "vcs.revision":
			embeddedRevision = value.Value
		case "vcs.modified":
			embeddedModified = value.Value
		}
	}
	slices.SortFunc(settings, func(left, right BuildSetting) int { return strings.Compare(left.Key, right.Key) })
	if targetGOOS == "" || targetGOARCH == "" {
		return nil, "", "", "", "", errors.New("go binary build information omits GOOS or GOARCH")
	}
	return settings, targetGOOS, targetGOARCH, embeddedRevision, embeddedModified, nil
}

func collectBuildDependencies(values []*debug.Module) []BuildDependency {
	dependencies := make([]BuildDependency, 0, len(values))
	for _, value := range values {
		if value == nil {
			continue
		}
		dependency := BuildDependency{Path: value.Path, Version: value.Version, Sum: value.Sum}
		if value.Replace != nil {
			dependency.ReplacedByPath = value.Replace.Path
			dependency.ReplacedBy = value.Replace.Version
			dependency.ReplacedBySum = value.Replace.Sum
		}
		dependencies = append(dependencies, dependency)
	}
	slices.SortFunc(dependencies, func(left, right BuildDependency) int {
		return strings.Compare(left.Path+"\x00"+left.Version, right.Path+"\x00"+right.Version)
	})
	return dependencies
}

func gitOutput(ctx context.Context, root string, arguments ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", root}, arguments...)...)
	output, err := command.Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return nil, fmt.Errorf("git %s failed: %s", strings.Join(arguments, " "), strings.TrimSpace(string(exitError.Stderr)))
		}
		return nil, fmt.Errorf("git %s failed: %w", strings.Join(arguments, " "), err)
	}
	return output, nil
}
