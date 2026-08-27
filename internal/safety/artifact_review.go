package safety

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"sort"
	"strconv"
)

const (
	ArtifactReviewSchema   = 1
	maxArtifactReviewBytes = 4 << 20
)

type ArtifactReviewManifest struct {
	SchemaVersion int               `json:"schema_version"`
	ArtifactID    string            `json:"artifact_id"`
	Findings      []ArtifactFinding `json:"reviewed_findings"`
}

func LoadArtifactReviewManifest(path string) (ArtifactReviewManifest, error) {
	if path == "" {
		return ArtifactReviewManifest{}, &Error{Kind: ErrorInvalidInput, Operation: OperationRead}
	}
	info, err := os.Lstat(path)
	if err != nil {
		return ArtifactReviewManifest{}, &Error{Kind: ErrorInvalidInput, Operation: OperationRead, Cause: err}
	}
	if !info.Mode().IsRegular() {
		return ArtifactReviewManifest{}, &Error{Kind: ErrorUnsupportedFileType, Operation: OperationRead}
	}
	if info.Size() < 0 || info.Size() > maxArtifactReviewBytes || info.Mode().Perm()&0o022 != 0 {
		return ArtifactReviewManifest{}, &Error{Kind: ErrorArtifactPolicyViolation, Operation: OperationRead}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ArtifactReviewManifest{}, &Error{Kind: ErrorInvalidInput, Operation: OperationRead, Cause: err}
	}
	var manifest ArtifactReviewManifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return ArtifactReviewManifest{}, &Error{Kind: ErrorInvalidInput, Operation: OperationRead, Cause: err}
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return ArtifactReviewManifest{}, &Error{Kind: ErrorInvalidInput, Operation: OperationRead, Cause: err}
	}
	if !manifest.Valid() {
		return ArtifactReviewManifest{}, &Error{Kind: ErrorArtifactPolicyViolation, Operation: OperationValidate}
	}
	return manifest, nil
}

func (m ArtifactReviewManifest) Valid() bool {
	if m.SchemaVersion != ArtifactReviewSchema || m.ArtifactID == "" || len(m.Findings) == 0 {
		return false
	}
	previous := ""
	for _, finding := range m.Findings {
		if !reviewableArtifactFinding(finding) {
			return false
		}
		key := artifactFindingKey(finding)
		if previous != "" && key <= previous {
			return false
		}
		previous = key
	}
	return true
}

func ReviewManifestFromReport(artifactID string, report ArtifactScanReport) (ArtifactReviewManifest, error) {
	manifest := ArtifactReviewManifest{SchemaVersion: ArtifactReviewSchema, ArtifactID: artifactID}
	if artifactID == "" || len(report.Findings) == 0 {
		return manifest, &Error{Kind: ErrorInvalidInput, Operation: OperationPublish}
	}
	manifest.Findings = append([]ArtifactFinding(nil), report.Findings...)
	for _, finding := range manifest.Findings {
		if !reviewableArtifactFinding(finding) {
			return ArtifactReviewManifest{}, &Error{Kind: ErrorArtifactPolicyViolation, Operation: OperationPublish}
		}
	}
	sort.Slice(manifest.Findings, func(left, right int) bool {
		return artifactFindingKey(manifest.Findings[left]) < artifactFindingKey(manifest.Findings[right])
	})
	if !manifest.Valid() {
		return ArtifactReviewManifest{}, &Error{Kind: ErrorArtifactPolicyViolation, Operation: OperationPublish}
	}
	return manifest, nil
}

func WriteArtifactReviewCandidate(path string, manifest ArtifactReviewManifest) error {
	if path == "" || !manifest.Valid() {
		return &Error{Kind: ErrorInvalidInput, Operation: OperationPublish}
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return &Error{Kind: ErrorInvalidInput, Operation: OperationPublish, Cause: err}
	}
	raw = append(raw, '\n')
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, PublicFileMode)
	if err != nil {
		return &Error{Kind: ErrorConcurrentMutation, Operation: OperationPublish, Cause: err}
	}
	committed := false
	defer func() {
		_ = file.Close()
		if !committed {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(raw); err != nil {
		return &Error{Kind: ErrorConcurrentMutation, Operation: OperationPublish, Cause: err}
	}
	if err := file.Sync(); err != nil {
		return &Error{Kind: ErrorConcurrentMutation, Operation: OperationPublish, Cause: err}
	}
	if err := file.Close(); err != nil {
		return &Error{Kind: ErrorConcurrentMutation, Operation: OperationPublish, Cause: err}
	}
	committed = true
	return nil
}

func ApplyArtifactReviewManifest(report ArtifactScanReport, manifest ArtifactReviewManifest) (ArtifactScanReport, error) {
	if !manifest.Valid() {
		return report, &Error{Kind: ErrorArtifactPolicyViolation, Operation: OperationValidate}
	}
	reviewed := make(map[string]ArtifactFinding, len(manifest.Findings))
	for _, finding := range manifest.Findings {
		reviewed[artifactFindingKey(finding)] = finding
	}
	remaining := make([]ArtifactFinding, 0, len(report.Findings))
	accepted := make([]ArtifactFinding, 0, len(manifest.Findings))
	for _, finding := range report.Findings {
		key := artifactFindingKey(finding)
		if _, ok := reviewed[key]; ok {
			accepted = append(accepted, finding)
			delete(reviewed, key)
			continue
		}
		remaining = append(remaining, finding)
	}
	report.Findings = remaining
	report.ReviewedFindings = accepted
	if len(reviewed) != 0 {
		return report, &Error{Kind: ErrorArtifactPolicyViolation, Operation: OperationValidate}
	}
	return report, artifactScanError(report.Findings)
}

func artifactFindingKey(finding ArtifactFinding) string {
	return finding.Path + "\x00" + finding.Rule + "\x00" + strconv.Itoa(finding.Line) + "\x00" + finding.FileSHA256
}

func reviewableArtifactFinding(finding ArtifactFinding) bool {
	if finding.Rule == "" || finding.Path == "" || finding.Line <= 0 || len(finding.FileSHA256) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(finding.FileSHA256)
	return err == nil
}
