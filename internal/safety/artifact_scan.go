package safety

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type ArtifactScanLimits struct {
	MaxFiles      int
	MaxFileBytes  int64
	MaxTotalBytes int64
	MaxDepth      int
	MaxPathBytes  int
	MaxFindings   int
}

func DefaultArtifactScanLimits() ArtifactScanLimits {
	return ArtifactScanLimits{
		MaxFiles: 250_000, MaxFileBytes: 64 << 20, MaxTotalBytes: 8 << 30,
		MaxDepth: 64, MaxPathBytes: 4096, MaxFindings: 10_000,
	}
}

func (l ArtifactScanLimits) Valid() bool {
	return l.MaxFiles > 0 && l.MaxFileBytes > 0 && l.MaxTotalBytes >= l.MaxFileBytes &&
		l.MaxDepth > 0 && l.MaxPathBytes > 0 && l.MaxFindings > 0
}

type ArtifactScanRequest struct {
	Roots            []string
	Class            ArtifactClass
	KnownSecrets     []string
	Limits           ArtifactScanLimits
	ReviewedFindings []ArtifactFinding
}

type ArtifactFinding struct {
	Rule       string `json:"rule"`
	Path       string `json:"path"`
	Line       int    `json:"line,omitempty"`
	FileSHA256 string `json:"file_sha256,omitempty"`
}

func (f ArtifactFinding) reviewKey() string {
	return f.Rule + ":" + f.FileSHA256 + ":" + strconv.Itoa(f.Line)
}

type ArtifactScanReport struct {
	Class            ArtifactClass     `json:"class"`
	Files            int               `json:"files"`
	TextFiles        int               `json:"text_files"`
	OpaqueFiles      int               `json:"opaque_files"`
	Bytes            int64             `json:"bytes"`
	Findings         []ArtifactFinding `json:"findings"`
	ReviewedFindings []ArtifactFinding `json:"reviewed_findings,omitempty"`
}

var (
	environmentDumpPattern = regexp.MustCompile(`(?m)^[\t ]*(export[\t ]+)?[A-Z][A-Z0-9_]*(KEY|TOKEN|SECRET|PASSWORD|COOKIE|AUTH)[A-Z0-9_]*[\t ]*=[^\r\n]*`)
	privatePathPatterns    = []secretPattern{
		{rule: "private_home_path", expression: regexp.MustCompile(`(?:/Users/|/home/)[^/\s]+/[^\s"']*`)},
		{rule: "private_windows_path", expression: regexp.MustCompile(`[A-Za-z]:\\Users\\[^\\\s]+\\[^\s"']*`)},
		{rule: "private_temp_path", expression: regexp.MustCompile(`/private/var/folders/[^\s"']+`)},
	}
)

func ScanArtifacts(request ArtifactScanRequest) (ArtifactScanReport, error) {
	report := ArtifactScanReport{Class: request.Class, Findings: make([]ArtifactFinding, 0)}
	if len(request.Roots) == 0 || !request.Class.Valid() || !request.Limits.Valid() {
		return report, &Error{Kind: ErrorInvalidInput, Operation: OperationScan}
	}
	state := artifactScanState{request: request, report: &report, findingKeys: make(map[string]struct{})}
	for rootIndex, root := range request.Roots {
		if err := state.scanRoot(rootIndex, root); err != nil {
			return report, err
		}
	}
	sort.Slice(report.Findings, func(left, right int) bool {
		if report.Findings[left].Path != report.Findings[right].Path {
			return report.Findings[left].Path < report.Findings[right].Path
		}
		if report.Findings[left].Line != report.Findings[right].Line {
			return report.Findings[left].Line < report.Findings[right].Line
		}
		return report.Findings[left].Rule < report.Findings[right].Rule
	})
	if len(request.ReviewedFindings) > 0 {
		reviewed := make(map[string]struct{}, len(request.ReviewedFindings)*2)
		for _, finding := range request.ReviewedFindings {
			reviewed[finding.reviewKey()] = struct{}{}
		}
		remaining := report.Findings[:0]
		for _, finding := range report.Findings {
			if _, ok := reviewed[finding.reviewKey()]; ok {
				report.ReviewedFindings = append(report.ReviewedFindings, finding)
			} else {
				remaining = append(remaining, finding)
			}
		}
		report.Findings = remaining
	}
	return report, artifactScanError(report.Findings)
}

type artifactScanState struct {
	request        ArtifactScanRequest
	report         *ArtifactScanReport
	findingKeys    map[string]struct{}
	currentFileSHA string
	newlines       []int
}

func (s *artifactScanState) scanRoot(rootIndex int, root string) (returnErr error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return &Error{Kind: ErrorInvalidInput, Operation: OperationScan, Cause: err}
	}
	rootInfo, err := os.Lstat(absolute)
	if err != nil {
		return &Error{Kind: ErrorInvalidInput, Operation: OperationScan, Cause: err}
	}
	if !rootInfo.IsDir() {
		filesystemRoot, err := os.OpenRoot(filepath.Dir(absolute))
		if err != nil {
			return &Error{Kind: ErrorInvalidInput, Operation: OperationScan, Cause: err}
		}
		defer func() {
			if closeErr := filesystemRoot.Close(); closeErr != nil && returnErr == nil {
				returnErr = &Error{Kind: ErrorConcurrentMutation, Operation: OperationScan, Path: absolute, Cause: closeErr}
			}
		}()
		return s.scanEntry(rootIndex, absolute, absolute, filesystemRoot, filepath.Base(absolute), rootInfo)
	}
	filesystemRoot, err := os.OpenRoot(absolute)
	if err != nil {
		return &Error{Kind: ErrorInvalidInput, Operation: OperationScan, Cause: err}
	}
	defer func() {
		if closeErr := filesystemRoot.Close(); closeErr != nil && returnErr == nil {
			returnErr = &Error{Kind: ErrorConcurrentMutation, Operation: OperationScan, Path: absolute, Cause: closeErr}
		}
	}()
	return fs.WalkDir(filesystemRoot.FS(), ".", func(relative string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return &Error{Kind: ErrorInvalidInput, Operation: OperationScan, Cause: walkErr}
		}
		info, err := entry.Info()
		if err != nil {
			return &Error{Kind: ErrorInvalidInput, Operation: OperationScan, Cause: err}
		}
		path := filepath.Join(absolute, filepath.FromSlash(relative))
		return s.scanEntry(rootIndex, absolute, path, filesystemRoot, filepath.FromSlash(relative), info)
	})
}

func (s *artifactScanState) scanEntry(rootIndex int, root, path string, filesystemRoot *os.Root, storageRelative string, info fs.FileInfo) error {
	reportPath := s.artifactRelativePath(rootIndex, root, path)
	if len(reportPath) > s.request.Limits.MaxPathBytes || artifactPathDepth(reportPath) > s.request.Limits.MaxDepth {
		return &Error{Kind: ErrorResourceLimit, Operation: OperationScan}
	}
	if finding := modeFinding(s.request.Class, reportPath, info); finding != nil {
		if err := s.addFinding(*finding); err != nil {
			return err
		}
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return s.addFinding(ArtifactFinding{Rule: "symlink", Path: reportPath})
	}
	if info.IsDir() {
		return nil
	}
	if !info.Mode().IsRegular() {
		return s.addFinding(ArtifactFinding{Rule: "unsupported_file_type", Path: reportPath})
	}
	return s.scanFile(reportPath, filesystemRoot, storageRelative, info)
}

func (s *artifactScanState) scanFile(reportPath string, filesystemRoot *os.Root, relative string, walkedInfo fs.FileInfo) (returnErr error) {
	if s.report.Files >= s.request.Limits.MaxFiles || walkedInfo.Size() < 0 || walkedInfo.Size() > s.request.Limits.MaxFileBytes || walkedInfo.Size() > s.request.Limits.MaxTotalBytes-s.report.Bytes {
		return &Error{Kind: ErrorResourceLimit, Operation: OperationScan}
	}
	file, err := filesystemRoot.Open(relative)
	if err != nil {
		return &Error{Kind: ErrorConcurrentMutation, Operation: OperationScan, Cause: err}
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && returnErr == nil {
			returnErr = &Error{Kind: ErrorConcurrentMutation, Operation: OperationScan, Path: reportPath, Cause: closeErr}
		}
	}()
	openedInfo, err := file.Stat()
	if err != nil || !openedInfo.Mode().IsRegular() || !os.SameFile(walkedInfo, openedInfo) {
		return &Error{Kind: ErrorConcurrentMutation, Operation: OperationScan, Cause: err}
	}
	remaining := s.request.Limits.MaxTotalBytes - s.report.Bytes
	readLimit := min(s.request.Limits.MaxFileBytes, remaining)
	data, err := io.ReadAll(io.LimitReader(file, readLimit+1))
	if err != nil {
		return &Error{Kind: ErrorInvalidInput, Operation: OperationScan, Cause: err}
	}
	if int64(len(data)) > readLimit {
		return &Error{Kind: ErrorResourceLimit, Operation: OperationScan}
	}
	finalInfo, err := file.Stat()
	pathInfo, pathErr := filesystemRoot.Lstat(relative)
	if err != nil || pathErr != nil || !pathInfo.Mode().IsRegular() ||
		!os.SameFile(openedInfo, finalInfo) || !os.SameFile(finalInfo, pathInfo) ||
		openedInfo.Size() != finalInfo.Size() || !openedInfo.ModTime().Equal(finalInfo.ModTime()) {
		return &Error{Kind: ErrorConcurrentMutation, Operation: OperationScan, Cause: errors.Join(err, pathErr)}
	}
	s.report.Files++
	s.report.Bytes += int64(len(data))
	digest := sha256.Sum256(data)
	s.currentFileSHA = hex.EncodeToString(digest[:])
	s.newlines = newlineOffsets(data)
	defer func() {
		s.currentFileSHA = ""
		s.newlines = nil
	}()
	if bytes.IndexByte(data, 0) >= 0 {
		s.report.OpaqueFiles++
		return s.scanKnownSecrets(reportPath, data)
	}
	s.report.TextFiles++
	for _, match := range findSecretPatternBytes(data) {
		if placeholderMatch(data[match.Start:match.End]) {
			continue
		}
		if err := s.addContentFinding(match.Rule, reportPath, match.Start); err != nil {
			return err
		}
	}
	if err := s.scanAdditionalPatterns(reportPath, data); err != nil {
		return err
	}
	return s.scanKnownSecrets(reportPath, data)
}

func (s *artifactScanState) scanAdditionalPatterns(relative string, data []byte) error {
	for _, position := range environmentDumpPattern.FindAllIndex(data, -1) {
		if placeholderMatch(data[position[0]:position[1]]) {
			continue
		}
		if err := s.addContentFinding("environment_dump", relative, position[0]); err != nil {
			return err
		}
	}
	if s.request.Class != ArtifactPublic {
		return nil
	}
	for _, pattern := range privatePathPatterns {
		for _, position := range pattern.expression.FindAllIndex(data, -1) {
			if err := s.addContentFinding(pattern.rule, relative, position[0]); err != nil {
				return err
			}
		}
	}
	return nil
}

func placeholderMatch(match []byte) bool {
	value := strings.ToLower(strings.TrimSpace(string(match)))
	if separator := strings.IndexByte(value, '='); separator >= 0 {
		value = strings.Trim(strings.TrimSpace(value[separator+1:]), `"'`)
	}
	return value == "" || strings.Contains(value, "${") || strings.Contains(value, "<") ||
		strings.Contains(value, "...") || strings.Contains(value, "[redacted]") ||
		strings.Contains(value, "example") || strings.Contains(value, "notreal")
}

func (s *artifactScanState) scanKnownSecrets(relative string, data []byte) error {
	for _, secret := range s.request.KnownSecrets {
		if len(secret) < 4 {
			continue
		}
		for offset := 0; ; {
			index := bytes.Index(data[offset:], []byte(secret))
			if index < 0 {
				break
			}
			position := offset + index
			if err := s.addContentFinding("registered_secret", relative, position); err != nil {
				return err
			}
			offset = position + len(secret)
		}
	}
	return nil
}

func (s *artifactScanState) addContentFinding(rule, path string, offset int) error {
	line := sort.SearchInts(s.newlines, offset) + 1
	return s.addFinding(ArtifactFinding{Rule: rule, Path: path, Line: line, FileSHA256: s.currentFileSHA})
}

func newlineOffsets(data []byte) []int {
	newlines := make([]int, 0, bytes.Count(data, []byte{'\n'}))
	for offset := 0; ; {
		index := bytes.IndexByte(data[offset:], '\n')
		if index < 0 {
			return newlines
		}
		position := offset + index
		newlines = append(newlines, position)
		offset = position + 1
	}
}

func (s *artifactScanState) addFinding(finding ArtifactFinding) error {
	key := artifactFindingKey(finding)
	if _, duplicate := s.findingKeys[key]; duplicate {
		return nil
	}
	if len(s.report.Findings) >= s.request.Limits.MaxFindings {
		return &Error{Kind: ErrorResourceLimit, Operation: OperationScan}
	}
	s.findingKeys[key] = struct{}{}
	s.report.Findings = append(s.report.Findings, finding)
	return nil
}

func modeFinding(class ArtifactClass, path string, info fs.FileInfo) *ArtifactFinding {
	permissions := info.Mode().Perm()
	if class == ArtifactSensitive && permissions&0o077 != 0 {
		return &ArtifactFinding{Rule: "unsafe_sensitive_mode", Path: path}
	}
	if class == ArtifactPublic && permissions&0o022 != 0 {
		return &ArtifactFinding{Rule: "writable_public_mode", Path: path}
	}
	return nil
}

func (s *artifactScanState) artifactRelativePath(rootIndex int, root, path string) string {
	base := filepath.Base(root)
	if base == "." || base == string(filepath.Separator) || s.unsafeReportPath(base) {
		base = "root-" + strconv.Itoa(rootIndex)
	}
	if root == path && !isDirectoryPath(root) {
		return filepath.ToSlash(base)
	}
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == "." {
		return filepath.ToSlash(base)
	}
	return filepath.ToSlash(filepath.Join(base, relative))
}

func (s *artifactScanState) unsafeReportPath(path string) bool {
	if len(FindSecretPatterns(path)) > 0 {
		return true
	}
	for _, secret := range s.request.KnownSecrets {
		if len(secret) >= 4 && strings.Contains(path, secret) {
			return true
		}
	}
	return false
}

func isDirectoryPath(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.IsDir()
}

func artifactPathDepth(path string) int {
	return len(strings.Split(filepath.ToSlash(path), "/"))
}

func artifactScanError(findings []ArtifactFinding) error {
	if len(findings) == 0 {
		return nil
	}
	for _, finding := range findings {
		if finding.Rule == "unsafe_sensitive_mode" || finding.Rule == "writable_public_mode" || finding.Rule == "symlink" || finding.Rule == "unsupported_file_type" {
			continue
		}
		return &Error{Kind: ErrorSecretDetected, Operation: OperationScan}
	}
	return &Error{Kind: ErrorArtifactPolicyViolation, Operation: OperationScan}
}
