package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

type capsuleSidecar struct {
	Destination string
	Payload     []byte
}

type stagedCapsuleFile struct {
	staged string
	final  string
}

func publishCapsuleArtifacts(
	ctx context.Context,
	destination string,
	archivePath string,
	registry *capsule.Registry,
	manifest capsule.Manifest,
	payloads map[string][]byte,
	options capsule.VerificationOptions,
	sidecars []capsuleSidecar,
) (capsule.ArchiveReport, error) {
	if ctx == nil || destination == "" || archivePath == "" || registry == nil {
		return capsule.ArchiveReport{}, errors.New("capsule publication requires context, destinations, and registry")
	}
	targets := []string{destination, archivePath}
	for _, sidecar := range sidecars {
		if sidecar.Destination == "" || len(sidecar.Payload) == 0 {
			return capsule.ArchiveReport{}, errors.New("capsule publication sidecar is incomplete")
		}
		targets = append(targets, sidecar.Destination)
	}
	if err := requireNewCommandTargets(targets); err != nil {
		return capsule.ArchiveReport{}, err
	}

	directoryMode := publicationDirectoryMode(options.MaximumVisibility)
	directoryStage, err := newCapsuleStageRoot(destination, directoryMode)
	if err != nil {
		return capsule.ArchiveReport{}, err
	}
	stagingRoots := []string{directoryStage}
	defer func() {
		for _, root := range stagingRoots {
			_ = os.RemoveAll(root)
		}
	}()
	stagedDirectory := filepath.Join(directoryStage, "capsule")
	if err := capsule.WriteDirectory(ctx, stagedDirectory, registry, manifest, payloads); err != nil {
		return capsule.ArchiveReport{}, fmt.Errorf("stage capsule directory: %w", err)
	}

	archiveStage, err := newCapsuleStageRoot(archivePath, directoryMode)
	if err != nil {
		return capsule.ArchiveReport{}, err
	}
	stagingRoots = append(stagingRoots, archiveStage)
	stagedArchive := filepath.Join(archiveStage, "capsule.tar.gz")
	archive, err := capsule.CreateArchive(ctx, stagedDirectory, stagedArchive, registry, options)
	if err != nil {
		return capsule.ArchiveReport{}, fmt.Errorf("stage capsule archive: %w", err)
	}
	stagedFiles := []stagedCapsuleFile{{staged: stagedArchive, final: archivePath}}

	for index, sidecar := range sidecars {
		stage, stageErr := newCapsuleStageRoot(sidecar.Destination, safety.PublicDirectoryMode)
		if stageErr != nil {
			return capsule.ArchiveReport{}, stageErr
		}
		stagingRoots = append(stagingRoots, stage)
		staged := filepath.Join(stage, fmt.Sprintf("sidecar-%d.json", index))
		if stageErr := writeNewPublicCommandFile(staged, sidecar.Payload); stageErr != nil {
			return capsule.ArchiveReport{}, fmt.Errorf("stage capsule sidecar: %w", stageErr)
		}
		actual, stageErr := readBoundedCommandFile(staged)
		if stageErr != nil || !bytes.Equal(actual, sidecar.Payload) {
			return capsule.ArchiveReport{}, errors.Join(stageErr, errors.New("staged capsule sidecar differs from its source bytes"))
		}
		stagedFiles = append(stagedFiles, stagedCapsuleFile{staged: staged, final: sidecar.Destination})
	}
	if err := requireNewCommandTargets(targets); err != nil {
		return capsule.ArchiveReport{}, fmt.Errorf("capsule publication target changed during staging: %w", err)
	}

	publishedFiles := make([]string, 0, len(stagedFiles))
	directoryPublished := false
	committed := false
	defer func() {
		if committed {
			return
		}
		if directoryPublished {
			_ = os.RemoveAll(destination)
		}
		for index := len(publishedFiles) - 1; index >= 0; index-- {
			_ = os.Remove(publishedFiles[index])
		}
	}()
	for _, staged := range stagedFiles {
		if err := os.Link(staged.staged, staged.final); err != nil {
			return capsule.ArchiveReport{}, fmt.Errorf("publish capsule artifact %q without overwrite: %w", staged.final, err)
		}
		publishedFiles = append(publishedFiles, staged.final)
		if err := syncCommandDirectory(filepath.Dir(staged.final)); err != nil {
			return capsule.ArchiveReport{}, err
		}
	}
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		return capsule.ArchiveReport{}, errors.New("capsule destination appeared during group publication")
	}
	if err := capsule.RenameDirectoryNoReplace(stagedDirectory, destination); err != nil {
		return capsule.ArchiveReport{}, fmt.Errorf("publish capsule directory: %w", err)
	}
	directoryPublished = true
	if err := syncCommandDirectory(filepath.Dir(destination)); err != nil {
		return capsule.ArchiveReport{}, err
	}
	if _, err := capsule.VerifyDirectory(ctx, destination, registry, options); err != nil {
		return capsule.ArchiveReport{}, fmt.Errorf("verify published capsule directory: %w", err)
	}
	if err := verifyPublishedCapsuleArchive(archivePath, archive); err != nil {
		return capsule.ArchiveReport{}, err
	}
	committed = true
	return archive, nil
}

func verifyPublishedCapsuleArchive(path string, expected capsule.ArchiveReport) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() != expected.Bytes {
		return errors.Join(err, errors.New("published capsule archive has an invalid file identity"))
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	hash := sha256.New()
	written, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil || written != expected.Bytes || hex.EncodeToString(hash.Sum(nil)) != expected.SHA256 {
		return errors.Join(copyErr, closeErr, errors.New("published capsule archive differs from the staged archive"))
	}
	return nil
}

func newCapsuleStageRoot(destination string, mode fs.FileMode) (string, error) {
	policy, err := safety.CurrentPathPolicy()
	if err != nil {
		return "", err
	}
	validated, err := policy.ValidateMutationRoot(destination)
	if err != nil {
		return "", err
	}
	parent := filepath.Dir(validated)
	if err := os.MkdirAll(parent, mode); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(parent, "."+filepath.Base(validated)+".transaction-*")
	if err != nil {
		return "", err
	}
	if err := os.Chmod(stage, mode); err != nil {
		_ = os.RemoveAll(stage)
		return "", err
	}
	return stage, nil
}

func publicationDirectoryMode(maximum capsule.Visibility) fs.FileMode {
	if maximum == capsule.VisibilityPublic {
		return safety.PublicDirectoryMode
	}
	return safety.SensitiveDirectoryMode
}

func syncCommandDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return err
	}
	return directory.Close()
}
