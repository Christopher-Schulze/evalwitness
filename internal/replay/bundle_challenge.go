package replay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const ResponseBundleChallengeReceiptSchemaVersion = "evalwitness.response-bundle-challenge-receipt.v1"

type ResponseBundleChallengeReceipt struct {
	SchemaVersion            string `json:"schema_version"`
	ChallengeID              string `json:"challenge_id"`
	Guard                    string `json:"guard"`
	ExpectedFailureSubstring string `json:"expected_failure_substring"`
	Failed                   bool   `json:"failed"`
	Error                    string `json:"error,omitempty"`
	SourcePayloadSHA256      string `json:"source_payload_sha256"`
	Digest                   string `json:"digest"`
}

type ResponseBundleChallengeSuite struct {
	SchemaVersion       string                           `json:"schema_version"`
	SourcePayloadSHA256 string                           `json:"source_payload_sha256"`
	Receipts            []ResponseBundleChallengeReceipt `json:"receipts"`
	Digest              string                           `json:"digest"`
}

func ChallengeResponseBundle(
	build ResponseBundleBuild,
	policy ResponseBundlePolicy,
	evidence ResponseBundleProducerEvidence,
	redistributionEvidencePath string,
	alternateCapturePath string,
) (ResponseBundleChallengeSuite, error) {
	if len(build.Index.Captures) == 0 {
		return ResponseBundleChallengeSuite{}, errors.New("response bundle challenge requires a capture")
	}
	sourceDigest := build.Index.Captures[0].PayloadSHA256
	receipts := make([]ResponseBundleChallengeReceipt, 0, 7)
	receipts = append(receipts, challengeVerifyMutation(build, "response", "does not match identity", sourceDigest, func(payloads map[string][]byte) error {
		raw, err := payloadByType(build, ResponseCaptureComponentSchemaVersion)
		if err != nil {
			return err
		}
		mutated := slices.Clone(raw)
		mutated[0] ^= 1
		payloads[protocol.DigestBytes(raw)] = mutated
		return nil
	}))
	receipts = append(receipts, challengeVerifyMutation(build, "token-evidence", "does not match identity", sourceDigest, func(payloads map[string][]byte) error {
		return mutateCaptureRecord(build, payloads, func(entry *fixtureEntry) {
			if len(entry.Response.OrderedTokenEvidence) > 0 {
				entry.Response.OrderedTokenEvidence[0].Token = entry.Response.OrderedTokenEvidence[0].Token + "-mutated"
				return
			}
			entry.Response.RawText += "-mutated"
		})
	}))
	receipts = append(receipts, challengeVerifyMutation(build, "request-lineage", "does not match identity", sourceDigest, func(payloads map[string][]byte) error {
		return mutateCaptureRecord(build, payloads, func(entry *fixtureEntry) {
			if entry.Request.Lineage.SourceTraceHash == "" {
				entry.Request.Lineage.SourceTraceHash = strings.Repeat("a", 64)
				return
			}
			raw := []byte(entry.Request.Lineage.SourceTraceHash)
			raw[0] ^= 1
			entry.Request.Lineage.SourceTraceHash = string(raw)
		})
	}))
	receipts = append(receipts, challengeBuildFailure(
		"request-corpus", "differ", sourceDigest, policy, evidence, redistributionEvidencePath,
		map[string]string{"primary": alternateCapturePath},
	))
	receipts = append(receipts, challengeVerifyMutation(build, "redistribution-evidence", "does not match identity", sourceDigest, func(payloads map[string][]byte) error {
		raw, err := payloadByType(build, ResponseBundleRedistributionSchemaVersion)
		if err != nil {
			return err
		}
		mutated := slices.Clone(raw)
		mutated[len(mutated)-1] ^= 1
		payloads[protocol.DigestBytes(raw)] = mutated
		return nil
	}))
	receipts = append(receipts, challengeVerifyMutation(build, "component-omission", "payload", sourceDigest, func(payloads map[string][]byte) error {
		raw, err := payloadByType(build, ResponseCaptureComponentSchemaVersion)
		if err != nil {
			return err
		}
		delete(payloads, protocol.DigestBytes(raw))
		return nil
	}))
	receipts = append(receipts, challengeVerifyMutation(build, "stale-attestation", "does not match identity", sourceDigest, func(payloads map[string][]byte) error {
		return mutateCaptureRecord(build, payloads, func(entry *fixtureEntry) {
			entry.Response.CapabilityAttestationID = "att-" + strings.Repeat("f", 64)
		})
	}))
	suite := ResponseBundleChallengeSuite{
		SchemaVersion:       ResponseBundleChallengeReceiptSchemaVersion,
		SourcePayloadSHA256: sourceDigest,
		Receipts:            receipts,
	}
	digest, err := protocol.Digest(unsignedResponseBundleChallengeSuite(suite))
	if err != nil {
		return ResponseBundleChallengeSuite{}, err
	}
	suite.Digest = digest
	return suite, nil
}

func challengeBuildFailure(
	challengeID, expected, sourceDigest string,
	policy ResponseBundlePolicy,
	evidence ResponseBundleProducerEvidence,
	redistributionEvidencePath string,
	sources map[string]string,
) ResponseBundleChallengeReceipt {
	_, err := BuildResponseBundle(policy, evidence, redistributionEvidencePath, sources, nil, []safety.ArtifactFinding{})
	return sealChallengeReceipt(challengeID, expected, expected, sourceDigest, err)
}

func challengeVerifyMutation(
	build ResponseBundleBuild,
	challengeID, expected, sourceDigest string,
	mutate func(map[string][]byte) error,
) ResponseBundleChallengeReceipt {
	payloads := clonePayloads(build.Package.Payloads)
	if err := mutate(payloads); err != nil {
		return sealChallengeReceipt(challengeID, expected, expected, sourceDigest, err)
	}
	_, err := capsule.VerifyPackage(
		context.Background(),
		build.Package.Registry,
		build.Package.Manifest,
		payloads,
		capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic},
	)
	return sealChallengeReceipt(challengeID, expected, expected, sourceDigest, err)
}

func mutateCaptureRecord(build ResponseBundleBuild, payloads map[string][]byte, mutate func(*fixtureEntry)) error {
	raw, err := payloadByType(build, ResponseCaptureComponentSchemaVersion)
	if err != nil {
		return err
	}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return errors.New("capture payload is not newline-terminated JSONL")
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte("\n"))
	if len(lines) == 0 {
		return errors.New("capture payload has no records")
	}
	var entry fixtureEntry
	if err := json.Unmarshal(lines[0], &entry); err != nil {
		return err
	}
	mutate(&entry)
	encoded, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	lines[0] = encoded
	mutated := append(bytes.Join(lines, []byte("\n")), '\n')
	payloads[protocol.DigestBytes(raw)] = mutated
	return nil
}

func payloadByType(build ResponseBundleBuild, typeID string) ([]byte, error) {
	for _, component := range build.Package.Manifest.Components {
		if component.TypeID != typeID {
			continue
		}
		payload, ok := build.Package.Payloads[component.Payload.Digest]
		if !ok {
			return nil, fmt.Errorf("missing payload for %s", typeID)
		}
		return payload, nil
	}
	return nil, fmt.Errorf("response bundle has no component %s", typeID)
}

func clonePayloads(source map[string][]byte) map[string][]byte {
	cloned := make(map[string][]byte, len(source))
	for digest, payload := range source {
		cloned[digest] = slices.Clone(payload)
	}
	return cloned
}

func sealChallengeReceipt(challengeID, guard, expected, sourceDigest string, err error) ResponseBundleChallengeReceipt {
	receipt := ResponseBundleChallengeReceipt{
		SchemaVersion:            ResponseBundleChallengeReceiptSchemaVersion,
		ChallengeID:              challengeID,
		Guard:                    guard,
		ExpectedFailureSubstring: expected,
		Failed:                   err != nil,
		SourcePayloadSHA256:      sourceDigest,
	}
	if err != nil {
		receipt.Error = err.Error()
	}
	if receipt.Failed && expected != "" && !strings.Contains(strings.ToLower(receipt.Error), strings.ToLower(expected)) {
		receipt.Failed = false
		if receipt.Error == "" {
			receipt.Error = "challenge did not fail"
		}
		receipt.Error = "guard mismatch: " + receipt.Error
	}
	digest, digestErr := protocol.Digest(unsignedResponseBundleChallengeReceipt(receipt))
	if digestErr == nil {
		receipt.Digest = digest
	}
	return receipt
}

func unsignedResponseBundleChallengeReceipt(receipt ResponseBundleChallengeReceipt) ResponseBundleChallengeReceipt {
	receipt.Digest = ""
	return receipt
}

func unsignedResponseBundleChallengeSuite(suite ResponseBundleChallengeSuite) ResponseBundleChallengeSuite {
	suite.Digest = ""
	return suite
}
