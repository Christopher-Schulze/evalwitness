package audit

import (
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
)

func TestBuildEntryEnvelopeAndFingerprintDeterminism(t *testing.T) {
	a, err := BuildEntryEnvelope("reference-adapter", "http://localhost/v1/chat", "model-x", "fix the bug")
	if err != nil {
		t.Fatal(err)
	}
	b, err := BuildEntryEnvelope("reference-adapter", "http://localhost/v1/chat", "model-x", "fix the bug")
	if err != nil {
		t.Fatal(err)
	}
	fa, err := a.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	fb, err := b.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if fa != fb {
		t.Fatalf("identical canonical inputs must fingerprint identically: %s vs %s", fa, fb)
	}
	// Empty prompt must be rejected: a one-line --task is still content.
	if _, err := BuildEntryEnvelope("p", "http://o/e", "m", ""); err == nil {
		t.Fatal("empty prompt must fail")
	}
}

func TestCompareEnvelopesClassifiesDivergence(t *testing.T) {
	envA, _ := BuildEntryEnvelope("reference-adapter", "http://localhost/v1/chat", "model-x", "fix the bug")
	envB, _ := BuildEntryEnvelope("reference-adapter", "http://localhost/v1/chat", "model-x", "fix the bug")
	check, err := CompareEnvelopes("task-1", "cli", "mcp", envA, envB)
	if err != nil {
		t.Fatal(err)
	}
	if !check.Equal {
		t.Fatalf("identical envelopes must match: %+v", check)
	}
	envC, _ := BuildEntryEnvelope("reference-adapter", "http://localhost/v1/chat", "model-y", "fix the bug")
	divergent, err := CompareEnvelopes("task-1", "cli", "eval", envA, envC)
	if err != nil {
		t.Fatal(err)
	}
	if divergent.Equal {
		t.Fatal("different models must diverge")
	}
	fails := CheckEntrypointFingerprints([]EntrypointFingerprintCheck{divergent})
	if len(fails) != 1 || !strings.Contains(fails[0], "fingerprint mismatch on task task-1 between cli") || !strings.Contains(fails[0], "eval") {
		t.Fatalf("divergence must be named with both entrypoints: %v", fails)
	}
	clean := CheckEntrypointFingerprints([]EntrypointFingerprintCheck{check})
	if len(clean) != 0 {
		t.Fatalf("matching fingerprints must not fail: %v", clean)
	}
}

// Real envelope shape guard: the harness must use provider.RequestEnvelope
// (TASK 043), not a parallel structure.
func TestCompareEnvelopesUsesTask043Shape(t *testing.T) {
	envelope, err := BuildEntryEnvelope("p", "http://localhost/v1/chat", "m", "task")
	if err != nil {
		t.Fatal(err)
	}
	var asFingerprint interface {
		Fingerprint() (provider.Fingerprint, error)
	} = envelope
	if _, err := asFingerprint.Fingerprint(); err != nil {
		t.Fatalf("envelope must expose TASK 043 Fingerprint(): %v", err)
	}
	if envelope.SchemaVersion != provider.RequestSchemaVersion || envelope.EndpointKind != provider.EndpointOpenAIChatCompletions {
		t.Fatalf("envelope drifted from TASK 043 shape: schema %d kind %q", envelope.SchemaVersion, envelope.EndpointKind)
	}
}
