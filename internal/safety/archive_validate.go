package safety

import (
	"archive/tar"
	"path"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

type archiveNameIndex struct {
	exact        map[string]struct{}
	folded       map[string]string
	requiredDirs map[string]struct{}
	kinds        map[string]archiveEntryKind
}

func newArchiveNameIndex() *archiveNameIndex {
	return &archiveNameIndex{
		exact:        map[string]struct{}{},
		folded:       map[string]string{},
		requiredDirs: map[string]struct{}{},
		kinds:        map[string]archiveEntryKind{},
	}
}

func validateArchiveHeader(header *tar.Header, limits ArchiveLimits, expectedRoots map[string]struct{}, names *archiveNameIndex, location string) (archiveEntry, error) {
	kind, err := archiveKind(header, location)
	if err != nil {
		return archiveEntry{}, err
	}
	name, err := normalizeArchiveName(header.Name, kind, limits, location)
	if err != nil {
		return archiveEntry{}, err
	}
	if len(expectedRoots) > 0 {
		root := strings.SplitN(name, "/", 2)[0]
		if _, ok := expectedRoots[root]; !ok {
			return archiveEntry{}, &Error{Kind: ErrorArtifactPolicyViolation, Operation: OperationValidate, Path: location}
		}
	}
	entry := archiveEntry{Name: name, Kind: kind, Size: header.Size}
	if err := validateArchiveEntrySize(entry, limits, location); err != nil {
		return archiveEntry{}, err
	}
	if names != nil {
		if err := names.add(entry, location); err != nil {
			return archiveEntry{}, err
		}
	}
	return entry, nil
}

func archiveKind(header *tar.Header, location string) (archiveEntryKind, error) {
	if header.Linkname != "" || hasArchiveExtendedAttributes(header.PAXRecords) {
		return "", &Error{Kind: ErrorUnsupportedFileType, Operation: OperationValidate, Path: location}
	}
	switch header.Typeflag {
	case tar.TypeReg, 0:
		return archiveRegularFile, nil
	case tar.TypeDir:
		return archiveDirectory, nil
	default:
		return "", &Error{Kind: ErrorUnsupportedFileType, Operation: OperationValidate, Path: location}
	}
}

func hasArchiveExtendedAttributes(records map[string]string) bool {
	for key := range records {
		if strings.HasPrefix(key, "SCHILY.xattr.") || strings.HasPrefix(key, "LIBARCHIVE.xattr.") {
			return true
		}
	}
	return false
}

func normalizeArchiveName(raw string, kind archiveEntryKind, limits ArchiveLimits, location string) (string, error) {
	if kind == archiveDirectory {
		raw = strings.TrimSuffix(raw, "/")
	}
	if len(raw) > limits.MaxPathBytes {
		return "", &Error{Kind: ErrorResourceLimit, Operation: OperationValidate, Path: location}
	}
	if raw == "" || !utf8.ValidString(raw) || strings.ContainsAny(raw, "\\\x00") {
		return "", &Error{Kind: ErrorInvalidInput, Operation: OperationValidate, Path: location}
	}
	if path.IsAbs(raw) || hasWindowsVolumePrefix(raw) || path.Clean(raw) != raw || norm.NFC.String(raw) != raw {
		return "", &Error{Kind: ErrorContainmentViolation, Operation: OperationValidate, Path: location}
	}
	parts := strings.Split(raw, "/")
	if len(parts) > limits.MaxDepth || strings.HasPrefix(parts[0], ".evalwitness-") {
		return "", &Error{Kind: ErrorResourceLimit, Operation: OperationValidate, Path: location}
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", &Error{Kind: ErrorContainmentViolation, Operation: OperationValidate, Path: location}
		}
	}
	return raw, nil
}

func hasWindowsVolumePrefix(name string) bool {
	return len(name) >= 2 && name[1] == ':' &&
		((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z'))
}

func validateArchiveEntrySize(entry archiveEntry, limits ArchiveLimits, location string) error {
	if entry.Size < 0 || entry.Size > limits.MaxEntryBytes {
		return &Error{Kind: ErrorResourceLimit, Operation: OperationValidate, Path: location}
	}
	if entry.Kind == archiveDirectory && entry.Size != 0 {
		return &Error{Kind: ErrorUnsupportedFileType, Operation: OperationValidate, Path: location}
	}
	return nil
}

func (n *archiveNameIndex) add(entry archiveEntry, location string) error {
	if _, duplicate := n.exact[entry.Name]; duplicate {
		return &Error{Kind: ErrorNameCollision, Operation: OperationValidate, Path: location}
	}
	folded := cases.Fold().String(entry.Name)
	if existing, collision := n.folded[folded]; collision && existing != entry.Name {
		return &Error{Kind: ErrorNameCollision, Operation: OperationValidate, Path: location}
	}
	if entry.Kind == archiveRegularFile {
		if _, requiredAsDirectory := n.requiredDirs[entry.Name]; requiredAsDirectory {
			return &Error{Kind: ErrorNameCollision, Operation: OperationValidate, Path: location}
		}
	}
	for _, ancestor := range archiveAncestors(entry.Name) {
		if n.kinds[ancestor] == archiveRegularFile {
			return &Error{Kind: ErrorNameCollision, Operation: OperationValidate, Path: location}
		}
		n.requiredDirs[ancestor] = struct{}{}
	}
	n.exact[entry.Name] = struct{}{}
	n.folded[folded] = entry.Name
	n.kinds[entry.Name] = entry.Kind
	return nil
}

func archiveAncestors(name string) []string {
	parts := strings.Split(name, "/")
	ancestors := make([]string, 0, len(parts)-1)
	for index := 1; index < len(parts); index++ {
		ancestors = append(ancestors, strings.Join(parts[:index], "/"))
	}
	return ancestors
}
