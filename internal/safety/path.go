package safety

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type PathPolicyOptions struct {
	UserHome       string
	RepositoryRoot string
	WorkingDir     string
}

type PathPolicy struct {
	protected []ProtectedPath
}

func CurrentPathPolicy() (*PathPolicy, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, &Error{Kind: ErrorInvalidInput, Operation: OperationValidate, Cause: err}
	}
	workingDir, err := os.Getwd()
	if err != nil {
		return nil, &Error{Kind: ErrorInvalidInput, Operation: OperationValidate, Cause: err}
	}
	return NewPathPolicy(PathPolicyOptions{
		UserHome:       home,
		RepositoryRoot: findRepositoryRoot(workingDir),
		WorkingDir:     workingDir,
	})
}

func NewPathPolicy(options PathPolicyOptions) (*PathPolicy, error) {
	if options.UserHome == "" || options.WorkingDir == "" {
		return nil, &Error{Kind: ErrorInvalidInput, Operation: OperationValidate}
	}
	locations := []ProtectedPath{
		{Location: ProtectedUserHome, Path: options.UserHome},
		{Location: ProtectedWorkingDir, Path: options.WorkingDir},
	}
	if options.RepositoryRoot != "" {
		locations = append(locations, ProtectedPath{Location: ProtectedRepositoryRoot, Path: options.RepositoryRoot})
	}
	return buildPathPolicy(locations)
}

func buildPathPolicy(locations []ProtectedPath) (*PathPolicy, error) {
	root := string(filepath.Separator)
	locations = append(locations, ProtectedPath{Location: ProtectedFilesystemRoot, Path: root})
	policy := &PathPolicy{protected: make([]ProtectedPath, 0, len(locations))}
	for _, location := range locations {
		canonical, err := resolveWithExistingParents(location.Path)
		if err != nil {
			return nil, &Error{Kind: ErrorInvalidInput, Operation: OperationValidate, Path: location.Path, Cause: err}
		}
		policy.protected = append(policy.protected, ProtectedPath{Location: location.Location, Path: canonical})
	}
	return policy, nil
}

func (p *PathPolicy) ProtectedPaths() []ProtectedPath {
	if p == nil {
		return nil
	}
	paths := make([]ProtectedPath, len(p.protected))
	copy(paths, p.protected)
	return paths
}

func (p *PathPolicy) ValidateCacheRoot(path string) (string, error) {
	return p.ValidateMutationRoot(path)
}

func (p *PathPolicy) ValidateMutationRoot(path string) (string, error) {
	if p == nil || path == "" {
		return "", &Error{Kind: ErrorInvalidInput, Operation: OperationValidate, Path: path}
	}
	canonical, err := resolveWithExistingParents(path)
	if err != nil {
		return "", &Error{Kind: ErrorInvalidInput, Operation: OperationValidate, Path: path, Cause: err}
	}
	if isVolumeRoot(canonical) {
		return "", &Error{Kind: ErrorProtectedPath, Operation: OperationValidate, Path: canonical}
	}
	for _, protected := range p.protected {
		if containsOrEqual(canonical, protected.Path) {
			return "", &Error{Kind: ErrorProtectedPath, Operation: OperationValidate, Path: canonical}
		}
	}
	return canonical, nil
}

func resolveWithExistingParents(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	current := filepath.Clean(absolute)
	var missing []string
	for {
		_, err = os.Lstat(current)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
	current, err = filepath.EvalSymlinks(current)
	if err != nil {
		return "", err
	}
	for index := len(missing) - 1; index >= 0; index-- {
		current = filepath.Join(current, missing[index])
	}
	return filepath.Clean(current), nil
}

func containsOrEqual(parent, child string) bool {
	parent = comparisonPath(parent)
	child = comparisonPath(child)
	relative, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func comparisonPath(path string) string {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

func isVolumeRoot(path string) bool {
	clean := filepath.Clean(path)
	volume := filepath.VolumeName(clean)
	if volume != "" && comparisonPath(clean) == comparisonPath(volume+string(filepath.Separator)) {
		return true
	}
	if runtime.GOOS != "darwin" {
		return false
	}
	relative, err := filepath.Rel(string(filepath.Separator)+"Volumes", clean)
	return err == nil && relative != "." && filepath.Dir(relative) == "."
}

func findRepositoryRoot(start string) string {
	current := filepath.Clean(start)
	for {
		if _, err := os.Lstat(filepath.Join(current, ".git")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}
