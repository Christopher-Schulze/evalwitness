package capsule

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/safety"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const ArchiveSchemaVersion = "evalwitness.capsule-archive.v1"

func CreateArchive(ctx context.Context, source, destination string, registry *Registry, options VerificationOptions) (ArchiveReport, error) {
	if ctx == nil || source == "" || destination == "" || registry == nil {
		return ArchiveReport{}, errors.New("capsule archive requires context, source, destination, and registry")
	}
	if _, err := VerifyDirectory(ctx, source, registry, options); err != nil {
		return ArchiveReport{}, err
	}
	limits := options.Limits
	if !limits.Valid() {
		limits = DefaultVerificationLimits()
	}
	manifest, err := loadCapsuleDocuments(source, registry, limits)
	if err != nil {
		return ArchiveReport{}, err
	}
	payloads, err := loadPayloads(ctx, source, manifest, limits)
	if err != nil {
		return ArchiveReport{}, err
	}
	files, err := encodedCapsuleFiles(registry, manifest, payloads)
	if err != nil {
		return ArchiveReport{}, err
	}
	path, err := validateNewDestination(destination)
	if err != nil {
		return ArchiveReport{}, err
	}
	fileMode, directoryMode := capsuleModes(manifest)
	return publishArchive(ctx, path, manifest.CapsuleID, files, fileMode, directoryMode)
}

func publishArchive(ctx context.Context, destination, capsuleID string, files []capsuleFile, fileMode, directoryMode fs.FileMode) (ArchiveReport, error) {
	parent := filepath.Dir(destination)
	if err := os.MkdirAll(parent, safety.SensitiveDirectoryMode); err != nil {
		return ArchiveReport{}, fmt.Errorf("create capsule archive parent: %w", err)
	}
	temporary, err := os.CreateTemp(parent, "."+filepath.Base(destination)+".candidate-*")
	if err != nil {
		return ArchiveReport{}, fmt.Errorf("create capsule archive candidate: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(fileMode); err != nil {
		_ = temporary.Close()
		return ArchiveReport{}, err
	}
	root := archiveRoot(capsuleID)
	if err := writeDeterministicArchive(ctx, temporary, root, files, fileMode, directoryMode); err != nil {
		_ = temporary.Close()
		return ArchiveReport{}, err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return ArchiveReport{}, err
	}
	if err := temporary.Close(); err != nil {
		return ArchiveReport{}, err
	}
	if err := inspectCreatedArchive(ctx, temporaryPath, root); err != nil {
		return ArchiveReport{}, err
	}
	if err := os.Link(temporaryPath, destination); err != nil {
		return ArchiveReport{}, fmt.Errorf("publish capsule archive without overwrite: %w", err)
	}
	if err := syncDirectory(parent); err != nil {
		return ArchiveReport{}, err
	}
	info, err := os.Stat(destination)
	if err != nil {
		return ArchiveReport{}, err
	}
	raw, err := readRegularFile(destination, info.Size())
	if err != nil {
		return ArchiveReport{}, err
	}
	return ArchiveReport{
		SchemaVersion: ArchiveSchemaVersion, CapsuleID: capsuleID, ArchiveRoot: root,
		SHA256: protocol.DigestBytes(raw), Bytes: int64(len(raw)), Files: len(files), Deterministic: true,
	}, nil
}

func writeDeterministicArchive(ctx context.Context, destination io.Writer, root string, files []capsuleFile, fileMode, directoryMode fs.FileMode) error {
	gzipWriter, err := gzip.NewWriterLevel(destination, gzip.BestCompression)
	if err != nil {
		return err
	}
	gzipWriter.Header = gzip.Header{ModTime: time.Unix(0, 0).UTC(), OS: 255}
	tarWriter := tar.NewWriter(gzipWriter)
	for _, directory := range []string{root, root + "/components", root + "/components/" + DigestAlgorithm} {
		if err := writeTarDirectory(tarWriter, directory, directoryMode); err != nil {
			return closeArchiveWriters(tarWriter, gzipWriter, err)
		}
	}
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return closeArchiveWriters(tarWriter, gzipWriter, err)
		}
		if err := writeTarFile(tarWriter, root+"/"+file.Path, file.Data, fileMode); err != nil {
			return closeArchiveWriters(tarWriter, gzipWriter, err)
		}
	}
	return closeArchiveWriters(tarWriter, gzipWriter, nil)
}

func writeTarDirectory(writer *tar.Writer, name string, mode fs.FileMode) error {
	header := &tar.Header{
		Name: name + "/", Typeflag: tar.TypeDir, Mode: int64(mode),
		ModTime: time.Unix(0, 0).UTC(), Format: tar.FormatUSTAR,
	}
	return writer.WriteHeader(header)
}

func writeTarFile(writer *tar.Writer, name string, data []byte, mode fs.FileMode) error {
	header := &tar.Header{
		Name: name, Typeflag: tar.TypeReg, Mode: int64(mode), Size: int64(len(data)),
		ModTime: time.Unix(0, 0).UTC(), Format: tar.FormatUSTAR,
	}
	if err := writer.WriteHeader(header); err != nil {
		return err
	}
	_, err := writer.Write(data)
	return err
}

func closeArchiveWriters(tarWriter *tar.Writer, gzipWriter *gzip.Writer, cause error) error {
	tarErr := tarWriter.Close()
	gzipErr := gzipWriter.Close()
	return errors.Join(cause, tarErr, gzipErr)
}

func inspectCreatedArchive(ctx context.Context, path, root string) error {
	limits := safety.DefaultArchiveLimits()
	result, err := safety.InspectTarGzip(ctx, safety.ArchiveInspectRequest{
		Sources: []string{path}, ExpectedRoots: []string{root}, Limits: limits,
	})
	if err != nil {
		return err
	}
	if result.Files < 1 || result.Directories != 3 || len(result.Sources) != 1 {
		return errors.New("created capsule archive has an invalid deterministic inventory")
	}
	return nil
}

func archiveRoot(capsuleID string) string {
	return "evalwitness-capsule-" + strings.ToLower(capsuleID)
}
