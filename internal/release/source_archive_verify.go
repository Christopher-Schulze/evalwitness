package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/internal/product"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	sourceArchiveReportPath  = "evidence/source-archive-report.json"
	sourceTreeProvenancePath = "source/source-tree-provenance.json"
)

func (r SourceArchiveReport) Validate() error {
	if r.SchemaVersion != SourceArchiveReportSchemaVersion || r.Product != ProductName || !product.ValidVersion(r.Version) ||
		r.ArchiveRoot != ProductName+"-"+r.Version || r.Format != sourceArchiveFormat || !r.Deterministic {
		return errors.New("source archive report identity is invalid")
	}
	if !validGitCommit(r.GitCommit) || !validSHA256(r.SourceTreeDigest) || !validSHA256(r.SHA256) || r.Bytes < 1 || r.Bytes > maximumAssetBytes ||
		r.ExpandedBytes < 1 || r.ExpandedBytes > maximumSourceExpandedBytes ||
		r.Files < 1 || r.Files > maximumSourceEntries || r.Directories < 1 || r.Files+r.Directories > maximumSourceEntries {
		return errors.New("source archive report bounds or source identity are invalid")
	}
	return nil
}

func EncodeSourceArchiveReport(report SourceArchiveReport) ([]byte, error) {
	if err := report.Validate(); err != nil {
		return nil, err
	}
	return protocol.CanonicalMarshal(report)
}

func DecodeSourceArchiveReport(raw []byte) (SourceArchiveReport, error) {
	var report SourceArchiveReport
	if err := protocol.DecodeStrict(raw, &report); err != nil {
		return SourceArchiveReport{}, fmt.Errorf("decode source archive report: %w", err)
	}
	canonical, err := EncodeSourceArchiveReport(report)
	if err != nil || !bytes.Equal(canonical, raw) {
		return SourceArchiveReport{}, errors.New("source archive report is not canonical JSON")
	}
	return report, nil
}

func VerifySourceArchiveReport(assetRoot string, manifest Manifest) error {
	reportRaw, err := readManifestAsset(assetRoot, manifest, sourceArchiveReportPath)
	if err != nil {
		return err
	}
	report, err := DecodeSourceArchiveReport(reportRaw)
	if err != nil {
		return err
	}
	archivePath := "source/" + ProductName + "-" + manifest.ProductVersion + "-source.tar.gz"
	archiveIndex := slices.IndexFunc(manifest.Assets, func(asset Asset) bool { return asset.Path == archivePath })
	if archiveIndex < 0 {
		return errors.New("release manifest has no canonical source archive")
	}
	archiveAsset := manifest.Assets[archiveIndex]
	if report.Product != manifest.Product || report.Version != manifest.ProductVersion || report.GitCommit != manifest.GitCommit ||
		report.SHA256 != manifest.SourceArchiveSHA256 || report.SHA256 != archiveAsset.SHA256 || report.Bytes != archiveAsset.Bytes {
		return errors.New("source archive report differs from the release manifest")
	}
	sourceRaw, err := readManifestAsset(assetRoot, manifest, sourceTreeProvenancePath)
	if err != nil {
		return err
	}
	source, err := capsule.DecodeSourceTreeProvenance(sourceRaw)
	if err != nil {
		return err
	}
	if err := capsule.ValidatePortableSourceTreeProvenance(source); err != nil {
		return err
	}
	if source.Commit != manifest.GitCommit || source.Digest != report.SourceTreeDigest || source.Files != report.Files || source.Bytes != report.ExpandedBytes {
		return errors.New("portable source-tree provenance differs from the source archive report")
	}

	limits := safety.DefaultArchiveLimits()
	limits.MaxEntries = maximumSourceEntries
	limits.MaxExpandedBytes = maximumSourceExpandedBytes
	limits.MaxEntryBytes = maximumSourceFileBytes
	inspection, err := safety.InspectTarGzip(context.Background(), safety.ArchiveInspectRequest{
		Sources:       []string{filepath.Join(assetRoot, filepath.FromSlash(archivePath))},
		ExpectedRoots: []string{report.ArchiveRoot}, Limits: limits,
	})
	if err != nil {
		return fmt.Errorf("verify source archive safety: %w", err)
	}
	if inspection.Files != report.Files || inspection.Directories != report.Directories || inspection.ExpandedBytes != report.ExpandedBytes || len(inspection.Sources) != 1 ||
		inspection.Sources[0].SHA256 != report.SHA256 || inspection.Sources[0].CompressedBytes != report.Bytes {
		return errors.New("source archive contents differ from their canonical report")
	}
	archiveFile := filepath.Join(assetRoot, filepath.FromSlash(archivePath))
	if err := verifyCanonicalUSTAR(archiveFile, report, source); err != nil {
		return err
	}
	verifiedBytes, verifiedDigest, err := digestStableRegularFile(archiveFile)
	if err != nil {
		return err
	}
	if verifiedBytes != report.Bytes || verifiedDigest != report.SHA256 {
		return errors.New("source archive changed during canonical-format verification")
	}
	return nil
}

func verifyCanonicalUSTAR(archivePath string, report SourceArchiveReport, source capsule.SourceTreeProvenance) (returnErr error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("close canonical source archive: %w", closeErr)
		}
	}()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("open canonical source gzip: %w", err)
	}
	defer func() {
		if closeErr := compressed.Close(); closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("close canonical source gzip: %w", closeErr)
		}
	}()
	if !compressed.ModTime.IsZero() || compressed.Name != "" || compressed.Comment != "" || len(compressed.Extra) != 0 || compressed.OS != 255 {
		return errors.New("source gzip header is not canonical")
	}

	reader := tar.NewReader(compressed)
	directories := make([]string, 0, report.Directories-1)
	files := make([]string, 0, report.Files)
	filePhase := false
	entryIndex := 0
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read canonical source tar: %w", err)
		}
		if err := validateCanonicalUSTARHeader(header); err != nil {
			return fmt.Errorf("source archive entry %d: %w", entryIndex, err)
		}
		if entryIndex == 0 {
			if header.Typeflag != tar.TypeDir || header.Name != report.ArchiveRoot+"/" {
				return errors.New("source archive has no canonical first root directory")
			}
			entryIndex++
			continue
		}
		prefix := report.ArchiveRoot + "/"
		if !strings.HasPrefix(header.Name, prefix) {
			return fmt.Errorf("source archive entry %q escapes its canonical root", header.Name)
		}
		relative := strings.TrimPrefix(header.Name, prefix)
		if header.Typeflag == tar.TypeDir {
			if filePhase || !strings.HasSuffix(relative, "/") {
				return errors.New("source archive directories are not a canonical leading block")
			}
			directories = append(directories, strings.TrimSuffix(relative, "/"))
		} else {
			filePhase = true
			if strings.HasSuffix(relative, "/") {
				return errors.New("source archive regular file has a directory name")
			}
			if len(files) >= len(source.Entries) {
				return errors.New("source archive has more files than its portable provenance")
			}
			expected := source.Entries[len(files)]
			expectedMode := int64(0o644)
			if expected.GitMode == "100755" {
				expectedMode = 0o755
			}
			if relative != expected.Path || header.Mode != expectedMode || header.Size != expected.Bytes {
				return fmt.Errorf("source archive file %q differs from its portable provenance metadata", relative)
			}
			hasher := sha256.New()
			written, err := io.CopyN(hasher, reader, header.Size)
			if err != nil || written != header.Size || hex.EncodeToString(hasher.Sum(nil)) != expected.SHA256 {
				return fmt.Errorf("source archive file %q differs from its portable provenance bytes", relative)
			}
			files = append(files, relative)
		}
		entryIndex++
	}
	if entryIndex != report.Files+report.Directories || !slices.IsSorted(directories) || !slices.IsSorted(files) ||
		hasAdjacentDuplicate(directories) || hasAdjacentDuplicate(files) {
		return errors.New("source archive entry ordering or counts are not canonical")
	}
	requiredDirectories := make(map[string]struct{})
	for _, filePath := range files {
		for directory := path.Dir(filePath); directory != "."; directory = path.Dir(directory) {
			requiredDirectories[directory] = struct{}{}
		}
	}
	expectedDirectories := make([]string, 0, len(requiredDirectories))
	for directory := range requiredDirectories {
		expectedDirectories = append(expectedDirectories, directory)
	}
	slices.Sort(expectedDirectories)
	if !slices.Equal(directories, expectedDirectories) {
		return errors.New("source archive directory inventory is not exact")
	}
	return nil
}

func validateCanonicalUSTARHeader(header *tar.Header) error {
	if header.Format != tar.FormatUSTAR || len(header.PAXRecords) != 0 || header.Linkname != "" ||
		header.Uid != 0 || header.Gid != 0 || header.Uname != "" || header.Gname != "" ||
		header.Devmajor != 0 || header.Devminor != 0 || !header.ModTime.Equal(time.Unix(0, 0)) ||
		!header.AccessTime.IsZero() || !header.ChangeTime.IsZero() {
		return errors.New("header is not strict deterministic USTAR")
	}
	switch header.Typeflag {
	case tar.TypeDir:
		if header.Mode != 0o755 || header.Size != 0 {
			return errors.New("directory mode or size is not canonical")
		}
	case tar.TypeReg:
		if header.Mode != 0o644 && header.Mode != 0o755 || header.Size < 0 || header.Size > maximumSourceFileBytes {
			return errors.New("regular-file mode or size is not canonical")
		}
	default:
		return errors.New("header type is not a regular file or directory")
	}
	return nil
}

func hasAdjacentDuplicate(values []string) bool {
	for index := 1; index < len(values); index++ {
		if values[index] == values[index-1] {
			return true
		}
	}
	return false
}
