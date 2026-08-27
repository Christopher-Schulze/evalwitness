package safety

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash"
	"io"
	"os"
	"path/filepath"
)

type tarGzipStream struct {
	file       *os.File
	buffered   *bufio.Reader
	gzipReader *gzip.Reader
	tarReader  *tar.Reader
	digest     hash.Hash
}

func inspectTarGzipSources(ctx context.Context, request ArchiveExtractRequest) (archivePlan, error) {
	expectedRoots, err := validateExpectedRoots(request.ExpectedRoots, request.Limits)
	if err != nil {
		return archivePlan{}, err
	}
	plan := archivePlan{Sources: make([]archiveSourcePlan, 0, len(request.Sources))}
	names := newArchiveNameIndex()
	for sourceIndex, sourcePath := range request.Sources {
		source, err := scanTarGzipSource(ctx, sourceIndex, sourcePath, request.Limits, expectedRoots, names, &plan)
		if err != nil {
			return archivePlan{}, err
		}
		plan.Sources = append(plan.Sources, source)
	}
	if err := validateArchivePlan(plan, request.Limits); err != nil {
		return archivePlan{}, err
	}
	return plan, nil
}

func scanTarGzipSource(ctx context.Context, sourceIndex int, sourcePath string, limits ArchiveLimits, expectedRoots map[string]struct{}, names *archiveNameIndex, plan *archivePlan) (archiveSourcePlan, error) {
	stream, compressedBytes, err := openTarGzipStream(sourcePath)
	if err != nil {
		return archiveSourcePlan{}, archiveSourceError(sourceIndex, err)
	}
	source := archiveSourcePlan{Path: sourcePath, CompressedBytes: compressedBytes}
	for entryIndex := 0; ; entryIndex++ {
		if err := ctx.Err(); err != nil {
			_ = stream.close()
			return archiveSourcePlan{}, archiveSourceError(sourceIndex, err)
		}
		header, nextErr := stream.tarReader.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			_ = stream.close()
			return archiveSourcePlan{}, archiveSourceError(sourceIndex, nextErr)
		}
		location := archiveLocation(sourceIndex, entryIndex)
		entry, err := validateArchiveHeader(header, limits, expectedRoots, names, location)
		if err != nil {
			_ = stream.close()
			return archiveSourcePlan{}, err
		}
		if err := addArchiveEntry(plan, entry, limits, location); err != nil {
			_ = stream.close()
			return archiveSourcePlan{}, err
		}
		if _, err := io.CopyN(io.Discard, stream.tarReader, entry.Size); err != nil {
			_ = stream.close()
			return archiveSourcePlan{}, archiveSourceError(sourceIndex, err)
		}
		source.Entries = append(source.Entries, entry)
	}
	digest, err := stream.finish()
	if err != nil {
		return archiveSourcePlan{}, archiveSourceError(sourceIndex, err)
	}
	source.SHA256 = digest
	return source, nil
}

func openTarGzipStream(path string) (*tarGzipStream, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, 0, errors.New("archive source is not a regular file")
	}
	digest := sha256.New()
	buffered := bufio.NewReader(io.TeeReader(file, digest))
	gzipReader, err := gzip.NewReader(buffered)
	if err != nil {
		_ = file.Close()
		return nil, 0, err
	}
	gzipReader.Multistream(false)
	return &tarGzipStream{
		file:       file,
		buffered:   buffered,
		gzipReader: gzipReader,
		tarReader:  tar.NewReader(gzipReader),
		digest:     digest,
	}, info.Size(), nil
}

func (s *tarGzipStream) finish() (string, error) {
	if _, err := io.Copy(io.Discard, s.gzipReader); err != nil {
		_ = s.close()
		return "", err
	}
	if err := s.gzipReader.Close(); err != nil {
		_ = s.file.Close()
		return "", err
	}
	if _, err := s.buffered.ReadByte(); !errors.Is(err, io.EOF) {
		_ = s.file.Close()
		if err == nil {
			return "", errors.New("archive has trailing or concatenated data")
		}
		return "", err
	}
	if err := s.file.Close(); err != nil {
		return "", err
	}
	return hex.EncodeToString(s.digest.Sum(nil)), nil
}

func (s *tarGzipStream) close() error {
	gzipErr := s.gzipReader.Close()
	fileErr := s.file.Close()
	if gzipErr != nil {
		return gzipErr
	}
	return fileErr
}

func addArchiveEntry(plan *archivePlan, entry archiveEntry, limits ArchiveLimits, location string) error {
	if plan.Files+plan.Directories >= limits.MaxEntries {
		return &Error{Kind: ErrorResourceLimit, Operation: OperationValidate, Path: location}
	}
	if entry.Size > limits.MaxExpandedBytes-plan.ExpandedBytes {
		return &Error{Kind: ErrorResourceLimit, Operation: OperationValidate, Path: location}
	}
	plan.ExpandedBytes += entry.Size
	if entry.Kind == archiveDirectory {
		plan.Directories++
	} else {
		plan.Files++
	}
	return nil
}

func validateArchivePlan(plan archivePlan, limits ArchiveLimits) error {
	var compressedBytes int64
	for _, source := range plan.Sources {
		if source.CompressedBytes > limits.MaxExpandedBytes-compressedBytes {
			return &Error{Kind: ErrorResourceLimit, Operation: OperationValidate}
		}
		compressedBytes += source.CompressedBytes
	}
	if compressedBytes == 0 || float64(plan.ExpandedBytes)/float64(compressedBytes) > limits.MaxCompressionRatio {
		return &Error{Kind: ErrorResourceLimit, Operation: OperationValidate}
	}
	return nil
}

func validateExpectedRoots(roots []string, limits ArchiveLimits) (map[string]struct{}, error) {
	validated := make(map[string]struct{}, len(roots))
	for index, root := range roots {
		name, err := normalizeArchiveName(root, archiveDirectory, limits, archiveLocation(-1, index))
		if err != nil || name != root || len(archiveAncestors(name)) != 0 {
			return nil, &Error{Kind: ErrorInvalidInput, Operation: OperationValidate, Cause: err}
		}
		validated[root] = struct{}{}
	}
	return validated, nil
}

func archiveSourceError(sourceIndex int, cause error) error {
	return &Error{Kind: ErrorInvalidInput, Operation: OperationValidate, Path: filepath.Base(archiveLocation(sourceIndex, -1)), Cause: cause}
}
