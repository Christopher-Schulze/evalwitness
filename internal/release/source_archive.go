package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/internal/product"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

const (
	SourceArchiveReportSchemaVersion = "evalwitness.source-archive-report.v1"
	sourceArchiveFormat              = "ustar+gzip"
	maximumSourceEntries             = 100_000
	maximumSourceFileBytes           = int64(128 << 20)
	maximumSourceExpandedBytes       = int64(1 << 30)
)

type SourceArchiveReport struct {
	SchemaVersion    string `json:"schema_version"`
	Product          string `json:"product"`
	Version          string `json:"version"`
	GitCommit        string `json:"git_commit"`
	SourceTreeDigest string `json:"source_tree_digest"`
	ArchiveRoot      string `json:"archive_root"`
	Format           string `json:"format"`
	SHA256           string `json:"sha256"`
	Bytes            int64  `json:"bytes"`
	ExpandedBytes    int64  `json:"expanded_bytes"`
	Files            int    `json:"files"`
	Directories      int    `json:"directories"`
	Deterministic    bool   `json:"deterministic"`
}

type sourceTreeFile struct {
	Path string
	OID  string
	Size int64
	Mode int64
}

type sourceTree struct {
	Files         []sourceTreeFile
	Directories   []string
	ExpandedBytes int64
}

func CreateSourceArchive(ctx context.Context, repositoryRoot, commit, destination string) (SourceArchiveReport, error) {
	if ctx == nil || repositoryRoot == "" || destination == "" || !validGitCommit(commit) {
		return SourceArchiveReport{}, errors.New("source archive request is invalid")
	}
	repositoryRoot, destination, err := validateSourceArchivePaths(repositoryRoot, destination)
	if err != nil {
		return SourceArchiveReport{}, err
	}
	if err := verifySourceRepository(ctx, repositoryRoot, commit); err != nil {
		return SourceArchiveReport{}, err
	}
	sourceProvenance, err := capsule.CollectGitSourceTreeProvenance(ctx, repositoryRoot)
	if err != nil {
		return SourceArchiveReport{}, err
	}
	tree, err := readSourceTree(ctx, repositoryRoot, commit)
	if err != nil {
		return SourceArchiveReport{}, err
	}
	if sourceProvenance.Commit != commit || sourceProvenance.Dirty || sourceProvenance.Files != len(tree.Files) || sourceProvenance.Bytes != tree.ExpandedBytes {
		return SourceArchiveReport{}, errors.New("source-tree provenance differs from the committed archive tree")
	}

	parent := filepath.Dir(destination)
	temporary, err := os.CreateTemp(parent, ".evalwitness-source-archive-*")
	if err != nil {
		return SourceArchiveReport{}, fmt.Errorf("create source archive temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	linked := false
	complete := false
	defer func() {
		if linked && !complete {
			temporaryInfo, temporaryErr := os.Lstat(temporaryPath)
			destinationInfo, destinationErr := os.Lstat(destination)
			if temporaryErr == nil && destinationErr == nil && os.SameFile(temporaryInfo, destinationInfo) {
				_ = os.Remove(destination)
			}
		}
		_ = os.Remove(temporaryPath)
	}()

	archiveRoot := ProductName + "-" + product.Version
	if err := writeSourceArchive(ctx, repositoryRoot, archiveRoot, tree, temporary); err != nil {
		_ = temporary.Close()
		return SourceArchiveReport{}, err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return SourceArchiveReport{}, fmt.Errorf("sync source archive: %w", err)
	}
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return SourceArchiveReport{}, fmt.Errorf("set source archive mode: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return SourceArchiveReport{}, fmt.Errorf("close source archive: %w", err)
	}

	limits := safety.DefaultArchiveLimits()
	limits.MaxEntries = maximumSourceEntries
	limits.MaxExpandedBytes = maximumSourceExpandedBytes
	limits.MaxEntryBytes = maximumSourceFileBytes
	inspection, err := safety.InspectTarGzip(ctx, safety.ArchiveInspectRequest{
		Sources: []string{temporaryPath}, ExpectedRoots: []string{archiveRoot}, Limits: limits,
	})
	if err != nil {
		return SourceArchiveReport{}, fmt.Errorf("inspect source archive: %w", err)
	}
	if inspection.Files != len(tree.Files) || inspection.Directories != len(tree.Directories)+1 || inspection.ExpandedBytes != tree.ExpandedBytes {
		return SourceArchiveReport{}, errors.New("source archive inspection differs from the committed tree")
	}
	archiveBytes, archiveDigest, err := digestStableRegularFile(temporaryPath)
	if err != nil {
		return SourceArchiveReport{}, err
	}
	if len(inspection.Sources) != 1 || inspection.Sources[0].SHA256 != archiveDigest || inspection.Sources[0].CompressedBytes != archiveBytes {
		return SourceArchiveReport{}, errors.New("source archive inspection identity is inconsistent")
	}
	report := SourceArchiveReport{
		SchemaVersion:    SourceArchiveReportSchemaVersion,
		Product:          ProductName,
		Version:          product.Version,
		GitCommit:        commit,
		SourceTreeDigest: sourceProvenance.Digest,
		ArchiveRoot:      archiveRoot,
		Format:           sourceArchiveFormat,
		SHA256:           archiveDigest,
		Bytes:            archiveBytes,
		ExpandedBytes:    tree.ExpandedBytes,
		Files:            len(tree.Files),
		Directories:      len(tree.Directories) + 1,
		Deterministic:    true,
	}
	if err := report.Validate(); err != nil {
		return SourceArchiveReport{}, err
	}
	if err := verifyCanonicalUSTAR(temporaryPath, report, sourceProvenance); err != nil {
		return SourceArchiveReport{}, err
	}
	if err := verifySourceRepository(ctx, repositoryRoot, commit); err != nil {
		return SourceArchiveReport{}, fmt.Errorf("source repository changed during archive construction: %w", err)
	}
	if err := os.Link(temporaryPath, destination); err != nil {
		return SourceArchiveReport{}, fmt.Errorf("publish source archive without overwrite: %w", err)
	}
	linked = true
	if err := syncSourceArchiveDirectory(parent); err != nil {
		return SourceArchiveReport{}, err
	}
	publishedBytes, publishedDigest, err := digestStableRegularFile(destination)
	if err != nil {
		return SourceArchiveReport{}, err
	}
	if publishedBytes != archiveBytes || publishedDigest != archiveDigest {
		return SourceArchiveReport{}, errors.New("published source archive differs from its verified temporary file")
	}

	complete = true
	return report, nil
}

func validateSourceArchivePaths(repositoryRoot, destination string) (string, string, error) {
	repositoryAbsolute, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return "", "", fmt.Errorf("resolve repository root: %w", err)
	}
	repositoryCanonical, err := filepath.EvalSymlinks(repositoryAbsolute)
	if err != nil {
		return "", "", fmt.Errorf("resolve repository root links: %w", err)
	}
	repositoryInfo, err := os.Lstat(repositoryCanonical)
	if err != nil || !repositoryInfo.IsDir() || repositoryInfo.Mode()&os.ModeSymlink != 0 {
		return "", "", fmt.Errorf("repository root %q is not a real directory", repositoryRoot)
	}
	if filepath.Base(destination) != ProductName+"-"+product.Version+"-source.tar.gz" {
		return "", "", errors.New("source archive destination has a non-canonical filename")
	}
	if _, err := os.Lstat(destination); err == nil {
		return "", "", errors.New("source archive destination already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", "", fmt.Errorf("inspect source archive destination: %w", err)
	}
	parentAbsolute, err := filepath.Abs(filepath.Dir(destination))
	if err != nil {
		return "", "", fmt.Errorf("resolve source archive parent: %w", err)
	}
	parentInfo, err := os.Lstat(parentAbsolute)
	if err != nil || !parentInfo.IsDir() || parentInfo.Mode()&os.ModeSymlink != 0 {
		return "", "", errors.New("source archive parent must be an existing real directory")
	}
	parentCanonical, err := filepath.EvalSymlinks(parentAbsolute)
	if err != nil {
		return "", "", fmt.Errorf("resolve source archive parent links: %w", err)
	}
	destinationCanonical := filepath.Join(parentCanonical, filepath.Base(destination))
	relative, err := filepath.Rel(repositoryCanonical, destinationCanonical)
	if err != nil {
		return "", "", fmt.Errorf("compare source archive destination with repository: %w", err)
	}
	if relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", "", errors.New("source archive destination must be outside the repository")
	}
	return filepath.Clean(repositoryCanonical), destinationCanonical, nil
}

func verifySourceRepository(ctx context.Context, repositoryRoot, commit string) error {
	topLevelRaw, err := gitOutput(ctx, repositoryRoot, "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	topLevel, err := filepath.EvalSymlinks(strings.TrimSpace(string(topLevelRaw)))
	if err != nil {
		return errors.New("repository root is not the canonical Git top level")
	}
	topLevelInfo, topLevelErr := os.Stat(topLevel)
	repositoryInfo, repositoryErr := os.Stat(repositoryRoot)
	if topLevelErr != nil || repositoryErr != nil || !topLevelInfo.IsDir() || !repositoryInfo.IsDir() || !os.SameFile(topLevelInfo, repositoryInfo) {
		return errors.New("repository root is not the canonical Git top level")
	}
	headRaw, err := gitOutput(ctx, repositoryRoot, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(headRaw)) != commit {
		return errors.New("source archive commit is not the current HEAD")
	}
	status, err := gitOutput(ctx, repositoryRoot, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return err
	}
	if len(status) != 0 {
		return errors.New("source archive requires a clean Git worktree including untracked files")
	}
	return nil
}

func readSourceTree(ctx context.Context, repositoryRoot, commit string) (sourceTree, error) {
	raw, err := gitOutput(ctx, repositoryRoot, "ls-tree", "-r", "-z", "--full-tree", "--long", commit)
	if err != nil {
		return sourceTree{}, err
	}
	if len(raw) == 0 || raw[len(raw)-1] != 0 {
		return sourceTree{}, errors.New("git tree listing is empty or unterminated")
	}
	tree := sourceTree{Files: make([]sourceTreeFile, 0, bytes.Count(raw, []byte{0}))}
	directorySet := make(map[string]struct{})
	for len(raw) > 0 {
		record, remainder, found := bytes.Cut(raw, []byte{0})
		if !found {
			return sourceTree{}, errors.New("git tree listing has an unterminated record")
		}
		raw = remainder
		if len(record) == 0 {
			if len(raw) == 0 {
				break
			}
			return sourceTree{}, errors.New("git tree listing contains an empty record")
		}
		metadata, filePathRaw, found := bytes.Cut(record, []byte{'\t'})
		fields := bytes.Fields(metadata)
		if !found || len(fields) != 4 || string(fields[1]) != "blob" || string(fields[0]) != "100644" && string(fields[0]) != "100755" {
			return sourceTree{}, errors.New("git tree contains a non-regular or unsupported entry")
		}
		filePath := string(filePathRaw)
		if err := validateSourcePath(filePath); err != nil {
			return sourceTree{}, err
		}
		size, err := strconv.ParseInt(string(fields[3]), 10, 64)
		if err != nil || size < 0 || size > maximumSourceFileBytes || tree.ExpandedBytes > maximumSourceExpandedBytes-size {
			return sourceTree{}, fmt.Errorf("source file %q exceeds source archive limits", filePath)
		}
		oid := string(fields[2])
		if !validObjectID(oid) {
			return sourceTree{}, fmt.Errorf("source file %q has an invalid Git object ID", filePath)
		}
		mode := int64(0o644)
		if string(fields[0]) == "100755" {
			mode = 0o755
		}
		tree.Files = append(tree.Files, sourceTreeFile{Path: filePath, OID: oid, Size: size, Mode: mode})
		tree.ExpandedBytes += size
		for directory := path.Dir(filePath); directory != "."; directory = path.Dir(directory) {
			directorySet[directory] = struct{}{}
		}
		if len(tree.Files)+len(directorySet)+1 > maximumSourceEntries {
			return sourceTree{}, errors.New("source archive entry count exceeds the hard limit")
		}
	}
	if len(tree.Files) == 0 {
		return sourceTree{}, errors.New("git tree contains no regular files")
	}
	slices.SortFunc(tree.Files, func(left, right sourceTreeFile) int { return strings.Compare(left.Path, right.Path) })
	tree.Directories = make([]string, 0, len(directorySet))
	for directory := range directorySet {
		tree.Directories = append(tree.Directories, directory)
	}
	slices.Sort(tree.Directories)
	return tree, nil
}

func validateSourcePath(value string) error {
	if value == "" || !utf8.ValidString(value) || strings.Contains(value, "\\") || path.IsAbs(value) || path.Clean(value) != value {
		return fmt.Errorf("source path %q is not canonical", value)
	}
	for _, character := range value {
		if character > unicode.MaxASCII || unicode.IsControl(character) {
			return fmt.Errorf("source path %q is not portable USTAR ASCII", value)
		}
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("source path %q has an unsafe component", value)
		}
	}
	return nil
}

func validObjectID(value string) bool {
	return (len(value) == 40 || len(value) == 64) && validLowerHex(value)
}

func writeSourceArchive(ctx context.Context, repositoryRoot, archiveRoot string, tree sourceTree, destination *os.File) error {
	gzipWriter, err := gzip.NewWriterLevel(destination, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("create source gzip writer: %w", err)
	}
	gzipWriter.ModTime = time.Unix(0, 0).UTC()
	gzipWriter.OS = 255
	tarWriter := tar.NewWriter(gzipWriter)
	epoch := time.Unix(0, 0).UTC()
	writeDirectory := func(name string) error {
		return tarWriter.WriteHeader(&tar.Header{
			Typeflag: tar.TypeDir, Name: name + "/", Mode: 0o755, ModTime: epoch, Format: tar.FormatUSTAR,
		})
	}
	if err := writeDirectory(archiveRoot); err != nil {
		_ = gzipWriter.Close()
		return fmt.Errorf("write source archive root: %w", err)
	}
	for _, directory := range tree.Directories {
		if err := writeDirectory(archiveRoot + "/" + directory); err != nil {
			_ = gzipWriter.Close()
			return fmt.Errorf("write source directory %q: %w", directory, err)
		}
	}
	for _, file := range tree.Files {
		if err := ctx.Err(); err != nil {
			_ = gzipWriter.Close()
			return err
		}
		blob, err := gitOutput(ctx, repositoryRoot, "cat-file", "blob", file.OID)
		if err != nil {
			_ = gzipWriter.Close()
			return err
		}
		if int64(len(blob)) != file.Size {
			_ = gzipWriter.Close()
			return fmt.Errorf("git blob %q differs from its tree size", file.Path)
		}
		header := &tar.Header{
			Typeflag: tar.TypeReg, Name: archiveRoot + "/" + file.Path, Size: file.Size,
			Mode: file.Mode, ModTime: epoch, Format: tar.FormatUSTAR,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			_ = gzipWriter.Close()
			return fmt.Errorf("write source file header %q: %w", file.Path, err)
		}
		written, err := tarWriter.Write(blob)
		if err != nil || written != len(blob) {
			_ = gzipWriter.Close()
			return fmt.Errorf("write source file %q: %w", file.Path, errors.Join(err, errors.New("incomplete source blob write")))
		}
	}
	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		return fmt.Errorf("close source tar writer: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return fmt.Errorf("close source gzip writer: %w", err)
	}
	return nil
}

func gitOutput(ctx context.Context, repositoryRoot string, arguments ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", repositoryRoot}, arguments...)...)
	command.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	output, err := command.Output()
	if err == nil {
		return output, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		stderr := strings.TrimSpace(string(exitError.Stderr))
		if len(stderr) > 512 {
			stderr = stderr[:512]
		}
		return nil, fmt.Errorf("git %s failed: %w: %s", strings.Join(arguments, " "), err, stderr)
	}
	return nil, fmt.Errorf("run git %s: %w", strings.Join(arguments, " "), err)
}

func syncSourceArchiveDirectory(directory string) (returnErr error) {
	handle, err := os.Open(directory)
	if err != nil {
		return fmt.Errorf("open source archive directory for sync: %w", err)
	}
	defer func() {
		if closeErr := handle.Close(); closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("close source archive directory: %w", closeErr)
		}
	}()
	if err := handle.Sync(); err != nil {
		return fmt.Errorf("sync source archive directory: %w", err)
	}
	return nil
}
