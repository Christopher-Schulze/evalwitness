package capsule

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/safety"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	manifestName  = "capsule.json"
	registryName  = "registry.json"
	checksumsName = "checksums.txt"
)

type PayloadIdentityError struct {
	ComponentName string
	PayloadDigest string
}

func (failure PayloadIdentityError) Error() string {
	return fmt.Sprintf("capsule payload for %q does not match identity %q", failure.ComponentName, failure.PayloadDigest)
}

func WriteDirectory(ctx context.Context, destination string, registry *Registry, manifest Manifest, payloads map[string][]byte) error {
	if ctx == nil || registry == nil || destination == "" {
		return errors.New("capsule directory write requires context, destination, and registry")
	}
	if err := manifest.Validate(registry); err != nil {
		return err
	}
	if err := validatePayloadSet(ctx, registry, manifest, payloads, DefaultVerificationLimits()); err != nil {
		return err
	}
	path, err := validateNewDestination(destination)
	if err != nil {
		return err
	}
	fileMode, directoryMode := capsuleModes(manifest)
	return publishDirectory(ctx, path, registry, manifest, payloads, fileMode, directoryMode)
}

// VerifyPackage verifies an already loaded capsule without reading a filesystem
// inventory. Callers that receive a directory or archive must use
// VerifyDirectory so undeclared files and non-canonical packaging are checked as
// well.
func VerifyPackage(ctx context.Context, registry *Registry, manifest Manifest, payloads map[string][]byte, options VerificationOptions) (VerificationReport, error) {
	if ctx == nil || registry == nil {
		return VerificationReport{}, errors.New("capsule package verification requires context and registry")
	}
	limits := options.Limits
	if !limits.Valid() {
		limits = DefaultVerificationLimits()
	}
	if len(manifest.Components) > limits.MaxComponents {
		return VerificationReport{}, errors.New("capsule component count exceeds verification limit")
	}
	if err := manifest.Validate(registry); err != nil {
		return VerificationReport{}, err
	}
	if err := enforceMaximumVisibility(manifest, options.MaximumVisibility); err != nil {
		return VerificationReport{}, err
	}
	cloned := make(map[string][]byte, len(payloads))
	for digest, raw := range payloads {
		cloned[digest] = slices.Clone(raw)
	}
	if err := validatePayloadSet(ctx, registry, manifest, cloned, limits); err != nil {
		return VerificationReport{}, err
	}
	return verificationReport(manifest), nil
}

// LoadDirectory returns a verified in-memory package. The returned payloads are
// detached copies, and the directory must still satisfy the exact deterministic
// inventory required by VerifyDirectory.
func LoadDirectory(ctx context.Context, root string, registry *Registry, options VerificationOptions) (Manifest, map[string][]byte, error) {
	if _, err := VerifyDirectory(ctx, root, registry, options); err != nil {
		return Manifest{}, nil, err
	}
	limits := options.Limits
	if !limits.Valid() {
		limits = DefaultVerificationLimits()
	}
	manifest, err := loadCapsuleDocuments(root, registry, limits)
	if err != nil {
		return Manifest{}, nil, err
	}
	payloads, err := loadPayloads(ctx, root, manifest, limits)
	if err != nil {
		return Manifest{}, nil, err
	}
	for digest, raw := range payloads {
		payloads[digest] = slices.Clone(raw)
	}
	return manifest, payloads, nil
}

func VerifyDirectory(ctx context.Context, root string, registry *Registry, options VerificationOptions) (VerificationReport, error) {
	if ctx == nil || registry == nil || root == "" {
		return VerificationReport{}, errors.New("capsule verification requires context, root, and registry")
	}
	limits := options.Limits
	if !limits.Valid() {
		limits = DefaultVerificationLimits()
	}
	manifest, err := loadCapsuleDocuments(root, registry, limits)
	if err != nil {
		return VerificationReport{}, err
	}
	if err := enforceMaximumVisibility(manifest, options.MaximumVisibility); err != nil {
		return VerificationReport{}, err
	}
	payloads, err := loadPayloads(ctx, root, manifest, limits)
	if err != nil {
		return VerificationReport{}, err
	}
	if err := validatePayloadSet(ctx, registry, manifest, payloads, limits); err != nil {
		return VerificationReport{}, err
	}
	if err := verifyDirectoryInventory(root, registry, manifest, payloads); err != nil {
		return VerificationReport{}, err
	}
	return verificationReport(manifest), nil
}

func loadCapsuleDocuments(root string, registry *Registry, limits VerificationLimits) (Manifest, error) {
	if err := requireDirectory(root); err != nil {
		return Manifest{}, err
	}
	registryRaw, err := readRegularFile(filepath.Join(root, registryName), limits.MaxRegistryBytes)
	if err != nil {
		return Manifest{}, err
	}
	if err := verifyRegistryDocument(registryRaw, registry); err != nil {
		return Manifest{}, err
	}
	manifestRaw, err := readRegularFile(filepath.Join(root, manifestName), limits.MaxManifestBytes)
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := protocol.DecodeStrict(manifestRaw, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode capsule manifest: %w", err)
	}
	canonical, err := protocol.CanonicalMarshal(manifest)
	if err != nil || !bytes.Equal(canonical, manifestRaw) {
		return Manifest{}, errors.New("capsule manifest is not canonical JSON")
	}
	if len(manifest.Components) > limits.MaxComponents {
		return Manifest{}, errors.New("capsule component count exceeds verification limit")
	}
	if err := manifest.Validate(registry); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func verifyRegistryDocument(raw []byte, expected *Registry) error {
	var document RegistryDocument
	if err := protocol.DecodeStrict(raw, &document); err != nil {
		return fmt.Errorf("decode capsule registry: %w", err)
	}
	canonical, err := protocol.CanonicalMarshal(document)
	if err != nil || !bytes.Equal(canonical, raw) {
		return errors.New("capsule registry is not canonical JSON")
	}
	if err := document.Validate(); err != nil {
		return err
	}
	expectedDocument := expected.Document()
	expectedRaw, err := protocol.CanonicalMarshal(expectedDocument)
	if err != nil || !bytes.Equal(expectedRaw, raw) {
		return errors.New("capsule registry does not match the verifier registry")
	}
	return nil
}

func loadPayloads(ctx context.Context, root string, manifest Manifest, limits VerificationLimits) (map[string][]byte, error) {
	payloads := make(map[string][]byte)
	var total int64
	for _, record := range manifest.Components {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, loaded := payloads[record.Payload.Digest]; loaded {
			continue
		}
		if record.Payload.Bytes > limits.MaxPayloadBytes || record.Payload.Bytes > limits.MaxTotalBytes-total {
			return nil, errors.New("capsule payload exceeds verification limit")
		}
		path := filepath.Join(root, payloadRelativePath(record.Payload.Digest))
		raw, err := readRegularFile(path, record.Payload.Bytes)
		if err != nil {
			return nil, err
		}
		if int64(len(raw)) != record.Payload.Bytes {
			return nil, fmt.Errorf("capsule payload %q byte count is invalid", record.Payload.Digest)
		}
		payloads[record.Payload.Digest] = raw
		total += int64(len(raw))
	}
	return payloads, nil
}

func validatePayloadSet(ctx context.Context, registry *Registry, manifest Manifest, payloads map[string][]byte, limits VerificationLimits) error {
	expected := make(map[string]PayloadIdentity)
	components := make(map[string]ComponentRecord, len(manifest.Components))
	var total int64
	for _, record := range manifest.Components {
		if err := ctx.Err(); err != nil {
			return err
		}
		if previous, found := expected[record.Payload.Digest]; found && previous != record.Payload {
			return fmt.Errorf("capsule payload %q has conflicting identities", record.Payload.Digest)
		}
		expected[record.Payload.Digest] = record.Payload
		raw, found := payloads[record.Payload.Digest]
		if !found {
			return fmt.Errorf("capsule payload %q is missing", record.Payload.Digest)
		}
		if err := verifyPayload(registry, record, raw); err != nil {
			return err
		}
		components[record.ComponentID] = record
	}
	for digest, raw := range payloads {
		identity, found := expected[digest]
		if !found {
			return fmt.Errorf("capsule payload %q is undeclared", digest)
		}
		if int64(len(raw)) > limits.MaxPayloadBytes || int64(len(raw)) > limits.MaxTotalBytes-total || int64(len(raw)) != identity.Bytes {
			return fmt.Errorf("capsule payload %q violates size limits", digest)
		}
		total += int64(len(raw))
	}
	for _, record := range manifest.Components {
		if err := ctx.Err(); err != nil {
			return err
		}
		parents := make([]BoundParent, 0, len(record.Parents))
		for _, reference := range record.Parents {
			parent := ComponentRecord{
				ComponentID: reference.ComponentID, TypeID: reference.TypeID, Role: reference.Role, Visibility: reference.Visibility,
			}
			var parentPayload []byte
			if reference.Resolution == ParentInternal {
				parent = components[reference.ComponentID]
				parentPayload = slices.Clone(payloads[parent.Payload.Digest])
			}
			parents = append(parents, BoundParent{Reference: reference, Record: parent, Payload: parentPayload})
		}
		if err := registry.ValidateBindings(BindingContext{Component: record, Payload: slices.Clone(payloads[record.Payload.Digest]), Parents: parents}); err != nil {
			return err
		}
	}
	return nil
}

func verifyPayload(registry *Registry, record ComponentRecord, raw []byte) error {
	if int64(len(raw)) != record.Payload.Bytes || protocol.DigestBytes(raw) != record.Payload.Digest {
		return PayloadIdentityError{ComponentName: record.Name, PayloadDigest: record.Payload.Digest}
	}
	if record.Payload.Canonicalization == PayloadCanonicalJSON {
		canonical, err := protocol.CanonicalizeJSON(raw)
		if err != nil || !bytes.Equal(canonical, raw) {
			return fmt.Errorf("capsule payload for %q is not canonical JSON", record.Name)
		}
	}
	return registry.ValidatePayload(record.TypeID, raw)
}

func publishDirectory(ctx context.Context, destination string, registry *Registry, manifest Manifest, payloads map[string][]byte, fileMode, directoryMode fs.FileMode) error {
	parent := filepath.Dir(destination)
	if err := os.MkdirAll(parent, directoryMode); err != nil {
		return fmt.Errorf("create capsule parent: %w", err)
	}
	staging, err := os.MkdirTemp(parent, "."+filepath.Base(destination)+".candidate-*")
	if err != nil {
		return fmt.Errorf("create capsule candidate: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(staging)
		}
	}()
	if err := writeCapsuleFiles(ctx, staging, registry, manifest, payloads, fileMode, directoryMode); err != nil {
		return err
	}
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		return errors.New("capsule destination appeared during publication")
	}
	if err := RenameDirectoryNoReplace(staging, destination); err != nil {
		return fmt.Errorf("publish capsule directory: %w", err)
	}
	committed = true
	return syncDirectory(parent)
}

func writeCapsuleFiles(ctx context.Context, root string, registry *Registry, manifest Manifest, payloads map[string][]byte, fileMode, directoryMode fs.FileMode) error {
	if err := os.Chmod(root, directoryMode); err != nil {
		return err
	}
	componentRoot := filepath.Join(root, "components", DigestAlgorithm)
	if err := os.MkdirAll(componentRoot, directoryMode); err != nil {
		return err
	}
	files, err := encodedCapsuleFiles(registry, manifest, payloads)
	if err != nil {
		return err
	}
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := writeExclusiveFile(filepath.Join(root, filepath.FromSlash(file.Path)), file.Data, fileMode); err != nil {
			return err
		}
	}
	return syncCapsuleDirectories(root, componentRoot)
}

type capsuleFile struct {
	Path string
	Data []byte
}

func encodedCapsuleFiles(registry *Registry, manifest Manifest, payloads map[string][]byte) ([]capsuleFile, error) {
	manifestRaw, err := protocol.CanonicalMarshal(manifest)
	if err != nil {
		return nil, err
	}
	registryRaw, err := protocol.CanonicalMarshal(registry.Document())
	if err != nil {
		return nil, err
	}
	files := []capsuleFile{{Path: manifestName, Data: manifestRaw}, {Path: registryName, Data: registryRaw}}
	for digest, raw := range payloads {
		files = append(files, capsuleFile{Path: payloadRelativePath(digest), Data: slices.Clone(raw)})
	}
	slices.SortFunc(files, func(left, right capsuleFile) int { return strings.Compare(left.Path, right.Path) })
	checksums := checksumDocument(files)
	files = append(files, capsuleFile{Path: checksumsName, Data: checksums})
	return files, nil
}

func checksumDocument(files []capsuleFile) []byte {
	var output strings.Builder
	for _, file := range files {
		fmt.Fprintf(&output, "%s  %s\n", protocol.DigestBytes(file.Data), file.Path)
	}
	return []byte(output.String())
}

func verifyDirectoryInventory(root string, registry *Registry, manifest Manifest, payloads map[string][]byte) error {
	expectedFiles, err := encodedCapsuleFiles(registry, manifest, payloads)
	if err != nil {
		return err
	}
	expected := make(map[string][]byte, len(expectedFiles))
	for _, file := range expectedFiles {
		expected[file.Path] = file.Data
	}
	seen := make(map[string]struct{}, len(expected))
	expectedDirectories := map[string]struct{}{".": {}, "components": {}, "components/" + DigestAlgorithm: {}}
	err = fs.WalkDir(os.DirFS(root), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if _, found := expectedDirectories[filepath.ToSlash(path)]; !found {
				return fmt.Errorf("capsule contains undeclared directory %q", path)
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return fmt.Errorf("capsule contains unsupported file %q", path)
		}
		wanted, found := expected[filepath.ToSlash(path)]
		if !found {
			return fmt.Errorf("capsule contains undeclared file %q", path)
		}
		actual, err := readRegularFile(filepath.Join(root, path), int64(len(wanted)))
		if err != nil || !bytes.Equal(actual, wanted) {
			return fmt.Errorf("capsule file %q differs from its deterministic form", path)
		}
		seen[filepath.ToSlash(path)] = struct{}{}
		return nil
	})
	if err != nil {
		return err
	}
	if len(seen) != len(expected) {
		return errors.New("capsule directory is missing a declared file")
	}
	return nil
}

func enforceMaximumVisibility(manifest Manifest, maximum Visibility) error {
	if maximum == "" {
		return nil
	}
	if !maximum.Valid() {
		return errors.New("capsule maximum visibility is invalid")
	}
	for _, record := range manifest.Components {
		if record.Visibility.rank() > maximum.rank() {
			return fmt.Errorf("capsule component %q exceeds %q visibility", record.Name, maximum)
		}
	}
	return nil
}

func verificationReport(manifest Manifest) VerificationReport {
	report := VerificationReport{
		SchemaVersion: VerificationSchemaVersion, CapsuleID: manifest.CapsuleID, ManifestDigest: manifest.ManifestDigest,
		RegistryDigest: manifest.RegistryDigest, Components: len(manifest.Components), VisibilityCounts: map[string]int{},
		Offline: true, Valid: true,
	}
	for _, record := range manifest.Components {
		report.VisibilityCounts[string(record.Visibility)]++
		if record.Role == RolePresentation {
			report.PresentationComponents++
		} else {
			report.ScientificComponents++
		}
	}
	return report
}

func validateNewDestination(destination string) (string, error) {
	policy, err := safety.CurrentPathPolicy()
	if err != nil {
		return "", err
	}
	path, err := policy.ValidateMutationRoot(destination)
	if err != nil {
		return "", err
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		return "", errors.New("capsule destination already exists")
	}
	return path, nil
}

func requireDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("capsule root is not a regular directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("capsule root is not a regular directory")
	}
	return nil
}

func readRegularFile(path string, maximum int64) ([]byte, error) {
	if maximum < 0 {
		return nil, errors.New("capsule file size limit is invalid")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > maximum {
		return nil, fmt.Errorf("capsule path %q is not a bounded regular file", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, maximum+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || int64(len(raw)) > maximum {
		return nil, errors.Join(readErr, closeErr, errors.New("capsule file read failed or exceeded its limit"))
	}
	return raw, nil
}

func writeExclusiveFile(path string, data []byte, mode fs.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func syncCapsuleDirectories(root, componentRoot string) error {
	for _, path := range []string{componentRoot, filepath.Dir(componentRoot), root} {
		if err := syncDirectory(path); err != nil {
			return err
		}
	}
	return nil
}

func syncDirectory(path string) error {
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

func payloadRelativePath(digest string) string {
	return "components/" + DigestAlgorithm + "/" + digest
}

func capsuleModes(manifest Manifest) (fs.FileMode, fs.FileMode) {
	for _, record := range manifest.Components {
		if record.Visibility != VisibilityPublic {
			return safety.SensitiveFileMode, safety.SensitiveDirectoryMode
		}
	}
	return safety.PublicFileMode, safety.PublicDirectoryMode
}
