package safety

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestArtifactScannerReportsLocationsWithoutSecretValues(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, PublicDirectoryMode); err != nil {
		t.Fatal(err)
	}
	const registered = "registered-value-7c9f"
	content := strings.Join([]string{
		"safe first line",
		"OPENAI_API_KEY=tiny",
		"Authorization: Bearer abcdefghijklmnopqrstuvwxyz",
		"local=/Users/alice/private/result.json",
		"known=" + registered,
	}, "\n")
	path := filepath.Join(root, "report.json")
	if err := os.WriteFile(path, []byte(content), PublicFileMode); err != nil {
		t.Fatal(err)
	}
	report, err := ScanArtifacts(ArtifactScanRequest{
		Roots: []string{root}, Class: ArtifactPublic, KnownSecrets: []string{registered}, Limits: DefaultArtifactScanLimits(),
	})
	if !IsKind(err, ErrorSecretDetected) {
		t.Fatalf("error = %T %v, want secret detected", err, err)
	}
	rules := map[string]bool{}
	for _, finding := range report.Findings {
		rules[finding.Rule] = true
		if finding.Path == "" || finding.Line == 0 {
			t.Fatalf("finding has no location: %+v", finding)
		}
	}
	for _, rule := range []string{"environment_dump", "authorization_bearer", "private_home_path", "registered_secret"} {
		if !rules[rule] {
			t.Fatalf("missing rule %q in %+v", rule, report.Findings)
		}
	}
	raw, marshalErr := json.Marshal(report)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	for _, secret := range []string{"tiny", "abcdefghijklmnopqrstuvwxyz", "/Users/alice", registered} {
		if strings.Contains(string(raw), secret) || strings.Contains(err.Error(), secret) {
			t.Fatalf("scanner output disclosed %q: %s / %v", secret, raw, err)
		}
	}
}

func TestArtifactScannerAcceptsSafePublicAndSensitiveTrees(t *testing.T) {
	publicRoot := t.TempDir()
	if err := os.Chmod(publicRoot, PublicDirectoryMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(publicRoot, "report.json"), []byte(`{"score":0.9,"input_tokens":42}`), PublicFileMode); err != nil {
		t.Fatal(err)
	}
	publicReport, err := ScanArtifacts(ArtifactScanRequest{
		Roots: []string{publicRoot}, Class: ArtifactPublic, Limits: DefaultArtifactScanLimits(),
	})
	if err != nil || len(publicReport.Findings) != 0 || publicReport.Files != 1 {
		t.Fatalf("public report = %+v, %v", publicReport, err)
	}
	if publicReport.TextFiles != 1 || publicReport.OpaqueFiles != 0 {
		t.Fatalf("public content classes = %+v", publicReport)
	}

	sensitiveRoot := t.TempDir()
	if err := os.Chmod(sensitiveRoot, SensitiveDirectoryMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sensitiveRoot, "capture.jsonl"), []byte("local=/Users/alice/private\n"), SensitiveFileMode); err != nil {
		t.Fatal(err)
	}
	sensitiveReport, err := ScanArtifacts(ArtifactScanRequest{
		Roots: []string{sensitiveRoot}, Class: ArtifactSensitive, Limits: DefaultArtifactScanLimits(),
	})
	if err != nil || len(sensitiveReport.Findings) != 0 {
		t.Fatalf("sensitive report = %+v, %v", sensitiveReport, err)
	}
}

func TestArtifactScannerTreatsOpaqueBinariesAsKnownSecretOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, PublicDirectoryMode); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "program")
	data := []byte("detector /Users/alice/path Authorization: Bearer abcdefghijklmnopqrstuvwxyz\x00safe")
	if err := os.WriteFile(path, data, PublicFileMode); err != nil {
		t.Fatal(err)
	}
	report, err := ScanArtifacts(ArtifactScanRequest{
		Roots: []string{root}, Class: ArtifactPublic, Limits: DefaultArtifactScanLimits(),
	})
	if err != nil || len(report.Findings) != 0 || report.Files != 1 || report.TextFiles != 0 || report.OpaqueFiles != 1 {
		t.Fatalf("opaque report = %+v, %v", report, err)
	}

	const registered = "registered-binary-secret"
	if err := os.WriteFile(path, append(data, []byte(registered)...), PublicFileMode); err != nil {
		t.Fatal(err)
	}
	report, err = ScanArtifacts(ArtifactScanRequest{
		Roots: []string{root}, Class: ArtifactPublic, KnownSecrets: []string{registered}, Limits: DefaultArtifactScanLimits(),
	})
	if !IsKind(err, ErrorSecretDetected) || len(report.Findings) != 1 || report.Findings[0].Rule != "registered_secret" {
		t.Fatalf("opaque known-secret report = %+v, %v", report, err)
	}
}

func TestArtifactScannerEnforcesModeAndSymlinkPolicy(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, PublicDirectoryMode); err != nil {
		t.Fatal(err)
	}
	writable := filepath.Join(root, "writable.json")
	if err := os.WriteFile(writable, []byte("safe"), PublicFileMode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(writable, 0o666); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		outside := filepath.Join(t.TempDir(), "outside")
		if err := os.WriteFile(outside, []byte("sk-proj-OUTSIDESECRETNOTSCANNED000000"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
			t.Fatal(err)
		}
	}
	report, err := ScanArtifacts(ArtifactScanRequest{
		Roots: []string{root}, Class: ArtifactPublic, Limits: DefaultArtifactScanLimits(),
	})
	if !IsKind(err, ErrorArtifactPolicyViolation) {
		t.Fatalf("error = %T %v, want artifact policy", err, err)
	}
	rules := map[string]bool{}
	for _, finding := range report.Findings {
		rules[finding.Rule] = true
	}
	if !rules["writable_public_mode"] {
		t.Fatalf("mode finding missing: %+v", report.Findings)
	}
	if runtime.GOOS != "windows" && !rules["symlink"] {
		t.Fatalf("symlink finding missing: %+v", report.Findings)
	}
}

func TestArtifactScannerRejectsSymlinkSwapBeforeOpen(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated privileges")
	}
	root := t.TempDir()
	target := filepath.Join(root, "target.json")
	replacement := filepath.Join(root, "replacement.json")
	if err := os.WriteFile(target, []byte(`{"safe":true}`), PublicFileMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(replacement, []byte(`{"secret":"different"}`), PublicFileMode); err != nil {
		t.Fatal(err)
	}
	walkedInfo, err := os.Lstat(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(target, filepath.Join(root, "original.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(replacement), target); err != nil {
		t.Fatal(err)
	}
	filesystemRoot, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := filesystemRoot.Close(); err != nil {
			t.Errorf("close artifact scan root: %v", err)
		}
	}()
	report := ArtifactScanReport{Class: ArtifactPublic, Findings: make([]ArtifactFinding, 0)}
	state := artifactScanState{
		request: ArtifactScanRequest{Class: ArtifactPublic, Limits: DefaultArtifactScanLimits()},
		report:  &report, findingKeys: make(map[string]struct{}),
	}
	if err := state.scanFile("root/target.json", filesystemRoot, "target.json", walkedInfo); !IsKind(err, ErrorConcurrentMutation) {
		t.Fatalf("swap error = %T %v", err, err)
	}
	if report.Files != 0 || report.Bytes != 0 || len(report.Findings) != 0 {
		t.Fatalf("swapped file contributed to report: %+v", report)
	}
}

func TestArtifactScannerEnforcesResourceBounds(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, PublicDirectoryMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "one"), []byte("1234"), PublicFileMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "two"), []byte("5678"), PublicFileMode); err != nil {
		t.Fatal(err)
	}
	tests := []func(*ArtifactScanLimits){
		func(l *ArtifactScanLimits) { l.MaxFiles = 1 },
		func(l *ArtifactScanLimits) { l.MaxFileBytes = 3; l.MaxTotalBytes = 8 },
		func(l *ArtifactScanLimits) { l.MaxTotalBytes = 7; l.MaxFileBytes = 4 },
		func(l *ArtifactScanLimits) { l.MaxDepth = 1 },
		func(l *ArtifactScanLimits) { l.MaxPathBytes = 2 },
	}
	for index, mutate := range tests {
		limits := DefaultArtifactScanLimits()
		mutate(&limits)
		_, err := ScanArtifacts(ArtifactScanRequest{Roots: []string{root}, Class: ArtifactPublic, Limits: limits})
		if !IsKind(err, ErrorResourceLimit) {
			t.Fatalf("case %d error = %T %v", index, err, err)
		}
	}
}

func TestArtifactScannerRejectsInvalidRequestAndMissingRoot(t *testing.T) {
	if _, err := ScanArtifacts(ArtifactScanRequest{}); !IsKind(err, ErrorInvalidInput) {
		t.Fatalf("invalid request error = %T %v", err, err)
	}
	missing := filepath.Join(t.TempDir(), "missing")
	_, err := ScanArtifacts(ArtifactScanRequest{
		Roots: []string{missing}, Class: ArtifactPublic, Limits: DefaultArtifactScanLimits(),
	})
	if !IsKind(err, ErrorInvalidInput) || errors.Is(err, os.ErrNotExist) && strings.Contains(err.Error(), missing) {
		t.Fatalf("missing root error = %T %v", err, err)
	}
}

func TestSharedSecretPatternsDriveRedactionAndScanning(t *testing.T) {
	text := "Authorization: Basic dXNlcjp0aW55"
	matches := FindSecretPatterns(text)
	if len(matches) == 0 || matches[0].Rule == "" {
		t.Fatalf("no classified match: %+v", matches)
	}
	redacted := RedactSecretPatterns(text)
	if strings.Contains(redacted, "dXNlcjp0aW55") || !strings.Contains(redacted, "REDACTED") {
		t.Fatalf("redacted text = %q", redacted)
	}
}

func TestArtifactScannerDeduplicatesSameRuleAndLine(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, PublicDirectoryMode); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "result.txt")
	if err := os.WriteFile(path, []byte("/Users/alice/one /Users/alice/two\n"), PublicFileMode); err != nil {
		t.Fatal(err)
	}
	report, err := ScanArtifacts(ArtifactScanRequest{
		Roots: []string{root}, Class: ArtifactPublic, Limits: DefaultArtifactScanLimits(),
	})
	if !IsKind(err, ErrorSecretDetected) {
		t.Fatalf("error = %T %v, want secret detected", err, err)
	}
	if len(report.Findings) != 1 || report.Findings[0].Rule != "private_home_path" {
		t.Fatalf("findings = %+v", report.Findings)
	}
}
