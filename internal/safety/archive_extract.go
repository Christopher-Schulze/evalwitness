package safety

import (
	"archive/tar"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func extractTarGzipPlan(ctx context.Context, request ArchiveExtractRequest, plan archivePlan) (ArchiveExtractResult, error) {
	policy, err := CurrentPathPolicy()
	if err != nil {
		return ArchiveExtractResult{}, err
	}
	destination, err := policy.ValidateMutationRoot(request.Destination)
	if err != nil {
		return ArchiveExtractResult{}, err
	}
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		return ArchiveExtractResult{}, &Error{Kind: ErrorArtifactPolicyViolation, Operation: OperationExtract, Path: destination, Cause: err}
	}
	parent := filepath.Dir(destination)
	if err := os.MkdirAll(parent, SensitiveDirectoryMode); err != nil {
		return ArchiveExtractResult{}, &Error{Kind: ErrorInvalidInput, Operation: OperationCreate, Path: parent, Cause: err}
	}
	staging, err := os.MkdirTemp(parent, "."+filepath.Base(destination)+".extract-*")
	if err != nil {
		return ArchiveExtractResult{}, &Error{Kind: ErrorInsufficientSpace, Operation: OperationCreate, Path: parent, Cause: err}
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(staging)
		}
	}()
	if err := os.Chmod(staging, SensitiveDirectoryMode); err != nil {
		return ArchiveExtractResult{}, &Error{Kind: ErrorUnsafePermissions, Operation: OperationCreate, Path: staging, Cause: err}
	}
	if err := extractIntoStaging(ctx, request, plan, staging); err != nil {
		return ArchiveExtractResult{}, err
	}
	if err := os.Rename(staging, destination); err != nil {
		return ArchiveExtractResult{}, &Error{Kind: ErrorConcurrentMutation, Operation: OperationPublish, Path: destination, Cause: err}
	}
	committed = true
	if err := syncDirectory(parent); err != nil {
		return ArchiveExtractResult{}, &Error{Kind: ErrorConcurrentMutation, Operation: OperationPublish, Path: destination, Cause: err}
	}
	return archiveResult(destination, plan), nil
}

func extractIntoStaging(ctx context.Context, request ArchiveExtractRequest, plan archivePlan, staging string) error {
	reservation, err := newDiskReservation(staging, plan.ExpandedBytes+request.Limits.ReservationHeadroomBytes)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(staging)
	if err != nil {
		_ = reservation.close()
		return &Error{Kind: ErrorContainmentViolation, Operation: OperationExtract, Path: staging, Cause: err}
	}
	for sourceIndex, source := range plan.Sources {
		if err := extractTarGzipSource(ctx, sourceIndex, source, request, root, reservation); err != nil {
			_ = root.Close()
			_ = reservation.close()
			return err
		}
	}
	if err := reservation.close(); err != nil {
		_ = root.Close()
		return err
	}
	if err := syncArchiveDirectories(root); err != nil {
		_ = root.Close()
		return err
	}
	if err := root.Close(); err != nil {
		return &Error{Kind: ErrorConcurrentMutation, Operation: OperationExtract, Path: staging, Cause: err}
	}
	return nil
}

func extractTarGzipSource(ctx context.Context, sourceIndex int, source archiveSourcePlan, request ArchiveExtractRequest, root *os.Root, reservation *diskReservation) error {
	stream, compressedBytes, err := openTarGzipStream(source.Path)
	if err != nil || compressedBytes != source.CompressedBytes {
		return archiveSourceError(sourceIndex, err)
	}
	expectedRoots, err := validateExpectedRoots(request.ExpectedRoots, request.Limits)
	if err != nil {
		_ = stream.close()
		return err
	}
	for entryIndex, expected := range source.Entries {
		if err := ctx.Err(); err != nil {
			_ = stream.close()
			return archiveSourceError(sourceIndex, err)
		}
		header, err := stream.tarReader.Next()
		if err != nil {
			_ = stream.close()
			return archiveSourceError(sourceIndex, err)
		}
		location := archiveLocation(sourceIndex, entryIndex)
		actual, err := validateArchiveHeader(header, request.Limits, expectedRoots, nil, location)
		if err != nil {
			_ = stream.close()
			return err
		}
		if actual != expected {
			_ = stream.close()
			return &Error{Kind: ErrorConcurrentMutation, Operation: OperationExtract, Path: location}
		}
		if err := extractArchiveEntry(root, stream.tarReader, actual, reservation, location); err != nil {
			_ = stream.close()
			return err
		}
	}
	if _, err := stream.tarReader.Next(); !errors.Is(err, io.EOF) {
		_ = stream.close()
		return archiveSourceError(sourceIndex, err)
	}
	digest, err := stream.finish()
	if err != nil || digest != source.SHA256 {
		return &Error{Kind: ErrorConcurrentMutation, Operation: OperationExtract, Path: archiveLocation(sourceIndex, -1), Cause: err}
	}
	return nil
}

func extractArchiveEntry(root *os.Root, reader *tar.Reader, entry archiveEntry, reservation *diskReservation, location string) error {
	relative := filepath.FromSlash(entry.Name)
	if entry.Kind == archiveDirectory {
		if err := root.MkdirAll(relative, SensitiveDirectoryMode); err != nil {
			return &Error{Kind: ErrorContainmentViolation, Operation: OperationExtract, Path: location, Cause: err}
		}
		return nil
	}
	if err := root.MkdirAll(filepath.Dir(relative), SensitiveDirectoryMode); err != nil {
		return &Error{Kind: ErrorContainmentViolation, Operation: OperationExtract, Path: location, Cause: err}
	}
	if err := reservation.release(entry.Size); err != nil {
		return err
	}
	file, err := root.OpenFile(relative, os.O_CREATE|os.O_EXCL|os.O_WRONLY, SensitiveFileMode)
	if err != nil {
		return &Error{Kind: ErrorNameCollision, Operation: OperationExtract, Path: location, Cause: err}
	}
	if _, err := io.CopyN(file, reader, entry.Size); err != nil {
		_ = file.Close()
		return &Error{Kind: ErrorInvalidInput, Operation: OperationExtract, Path: location, Cause: err}
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return &Error{Kind: ErrorConcurrentMutation, Operation: OperationExtract, Path: location, Cause: err}
	}
	if err := file.Close(); err != nil {
		return &Error{Kind: ErrorConcurrentMutation, Operation: OperationExtract, Path: location, Cause: err}
	}
	return nil
}

func syncArchiveDirectories(root *os.Root) error {
	var directories []string
	err := fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			directories = append(directories, path)
		}
		return nil
	})
	if err != nil {
		return &Error{Kind: ErrorConcurrentMutation, Operation: OperationExtract, Cause: err}
	}
	for index := len(directories) - 1; index >= 0; index-- {
		if err := syncRootDirectory(root, directories[index]); err != nil {
			return err
		}
	}
	return nil
}

func archiveResult(destination string, plan archivePlan) ArchiveExtractResult {
	result := ArchiveExtractResult{
		Destination:   destination,
		Files:         plan.Files,
		Directories:   plan.Directories,
		ExpandedBytes: plan.ExpandedBytes,
		Sources:       make([]ArchiveSourceEvidence, 0, len(plan.Sources)),
	}
	for _, source := range plan.Sources {
		result.Sources = append(result.Sources, ArchiveSourceEvidence{
			Name:            filepath.Base(source.Path),
			SHA256:          source.SHA256,
			CompressedBytes: source.CompressedBytes,
		})
	}
	return result
}
