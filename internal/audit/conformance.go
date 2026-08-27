package audit

import (
	"fmt"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
)

// EntrypointFingerprintCheck is one row of the TASK 039 cross-entrypoint
// conformance harness: the same canonical request built through two different
// shipped entrypoints must produce the identical TASK 043 request fingerprint
// before any decision comparison is meaningful.
type EntrypointFingerprintCheck struct {
	TaskID       string               `json:"task_id"`
	EntrypointA  string               `json:"entrypoint_a"`
	EntrypointB  string               `json:"entrypoint_b"`
	FingerprintA provider.Fingerprint `json:"fingerprint_a"`
	FingerprintB provider.Fingerprint `json:"fingerprint_b"`
	Equal        bool                 `json:"equal"`
}

// BuildEntryEnvelope constructs a canonical RequestEnvelope exactly as an
// entrypoint does for a task prompt — via the same TASK 043 NewRequestEnvelope
// constructor every shipped entrypoint uses, so defaults (thinking mode,
// response format, temperature normalization) cannot drift per surface.
const conformanceMaxOutputTokens = 4096

func BuildEntryEnvelope(providerID, baseURL, requestedModel, prompt string) (provider.RequestEnvelope, error) {
	if providerID == "" || requestedModel == "" || prompt == "" {
		return provider.RequestEnvelope{}, fmt.Errorf("conformance: provider, model, and prompt are required")
	}
	return provider.NewRequestEnvelope(provider.RequestOptions{
		ProviderID:      providerID,
		BaseURL:         baseURL,
		RequestedModel:  requestedModel,
		Messages:        []provider.Message{{Role: "user", Content: prompt}},
		MaxOutputTokens: conformanceMaxOutputTokens,
	})
}

// CheckEntrypointFingerprints compares fingerprints across entrypoints per
// task and returns every divergence as a failure with both digests named.
// Divergence before decision comparison means a product defect or an
// explicitly declared adapter effect — never silently ignored.
func CheckEntrypointFingerprints(checks []EntrypointFingerprintCheck) []string {
	var fails []string
	for _, check := range checks {
		if !check.Equal {
			fails = append(fails, fmt.Sprintf(
				"fingerprint mismatch on task %s between %s (%s) and %s (%s): surfaces are not semantically equivalent",
				check.TaskID, check.EntrypointA, shortDigest(string(check.FingerprintA)), check.EntrypointB, shortDigest(string(check.FingerprintB))))
		}
	}
	return fails
}

// CompareEnvelopes computes fingerprints for two envelopes and classifies the
// check result.
func CompareEnvelopes(taskID, entrypointA, entrypointB string, a, b provider.RequestEnvelope) (EntrypointFingerprintCheck, error) {
	fa, err := a.Fingerprint()
	if err != nil {
		return EntrypointFingerprintCheck{}, fmt.Errorf("conformance: fingerprint %s: %w", entrypointA, err)
	}
	fb, err := b.Fingerprint()
	if err != nil {
		return EntrypointFingerprintCheck{}, fmt.Errorf("conformance: fingerprint %s: %w", entrypointB, err)
	}
	return EntrypointFingerprintCheck{
		TaskID: taskID, EntrypointA: entrypointA, EntrypointB: entrypointB,
		FingerprintA: fa, FingerprintB: fb, Equal: fa == fb,
	}, nil
}
