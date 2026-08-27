package capsule

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	PortableSourceTreeEnvironment = "EVALWITNESS_SOURCE_TREE_PROVENANCE"
	PortableSourceRootEnvironment = "EVALWITNESS_REPOSITORY_ROOT"
	maximumPortableSourceBytes    = int64(64 << 20)
)

func EncodeSourceTreeProvenance(source SourceTreeProvenance) ([]byte, error) {
	if err := source.Validate(); err != nil {
		return nil, err
	}
	return protocol.CanonicalMarshal(source)
}

func DecodeSourceTreeProvenance(raw []byte) (SourceTreeProvenance, error) {
	var source SourceTreeProvenance
	if err := protocol.DecodeStrict(raw, &source); err != nil {
		return SourceTreeProvenance{}, fmt.Errorf("decode source-tree provenance: %w", err)
	}
	canonical, err := EncodeSourceTreeProvenance(source)
	if err != nil || !bytes.Equal(canonical, raw) {
		return SourceTreeProvenance{}, errors.New("source-tree provenance is not canonical JSON")
	}
	return source, nil
}

func LoadPortableSourceTreeProvenance(repositoryRoot, sourcePath string) (SourceTreeProvenance, error) {
	if repositoryRoot == "" || !filepath.IsAbs(sourcePath) {
		return SourceTreeProvenance{}, errors.New("portable source-tree provenance requires a repository root and absolute evidence path")
	}
	before, err := os.Lstat(sourcePath)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() < 1 || before.Size() > maximumPortableSourceBytes {
		return SourceTreeProvenance{}, fmt.Errorf("portable source-tree evidence %q is not a bounded regular file", sourcePath)
	}
	raw, err := os.ReadFile(sourcePath)
	if err != nil || int64(len(raw)) != before.Size() {
		return SourceTreeProvenance{}, errors.Join(err, errors.New("portable source-tree evidence read was incomplete"))
	}
	after, err := os.Lstat(sourcePath)
	if err != nil || !os.SameFile(before, after) || after.Size() != before.Size() || after.ModTime() != before.ModTime() {
		return SourceTreeProvenance{}, errors.New("portable source-tree evidence changed during read")
	}
	source, err := DecodeSourceTreeProvenance(raw)
	if err != nil {
		return SourceTreeProvenance{}, err
	}
	if err := VerifyPortableSourceTree(repositoryRoot, source); err != nil {
		return SourceTreeProvenance{}, err
	}
	return source, nil
}

func VerifyPortableSourceTree(repositoryRoot string, source SourceTreeProvenance) error {
	if err := ValidatePortableSourceTreeProvenance(source); err != nil {
		return err
	}
	root, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve portable source root: %w", err)
	}
	rootInfo, err := os.Lstat(root)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("portable source root is not a real directory")
	}
	expected := make(map[string]SourceTreeEntry, len(source.Entries))
	for _, entry := range source.Entries {
		expected[entry.Path] = entry
	}
	actual := make([]SourceTreeEntry, 0, len(expected))
	err = filepath.WalkDir(root, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if filePath == root {
			return nil
		}
		relative, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		canonical := filepath.ToSlash(relative)
		if entry.IsDir() {
			if canonical == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return fmt.Errorf("portable source path %q is not a regular file", canonical)
		}
		declared, found := expected[canonical]
		if !found {
			return fmt.Errorf("portable source has undeclared file %q", canonical)
		}
		materialized, err := materializeSourceTreeEntry(root, canonical, declared.GitMode, "present")
		if err != nil {
			return err
		}
		if materialized != declared {
			return fmt.Errorf("portable source file %q differs from its manifest-bound provenance", canonical)
		}
		actual = append(actual, materialized)
		return nil
	})
	if err != nil {
		return err
	}
	slices.SortFunc(actual, func(left, right SourceTreeEntry) int { return strings.Compare(left.Path, right.Path) })
	if !slices.Equal(actual, source.Entries) {
		return errors.New("portable source inventory differs from its manifest-bound provenance")
	}
	return nil
}

func ValidatePortableSourceTreeProvenance(source SourceTreeProvenance) error {
	if err := source.Validate(); err != nil {
		return err
	}
	if source.Dirty || source.StatusDigest != protocol.DigestBytes(nil) {
		return errors.New("portable source-tree provenance must describe a clean Git state")
	}
	for _, entry := range source.Entries {
		if entry.State != "present" || entry.Kind != "file" || entry.LinkTarget != "" || entry.GitMode != "100644" && entry.GitMode != "100755" {
			return fmt.Errorf("portable source entry %q is not a clean regular Git file", entry.Path)
		}
	}
	return nil
}
