package replay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
)

func digestForTest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

type mockProvider struct {
	name, model string
	servedModel string
	caps        provider.Capabilities
	calls       int
}

func (m *mockProvider) Name() string                        { return m.name }
func (m *mockProvider) Capabilities() provider.Capabilities { return m.caps }
func (m *mockProvider) Score(_ context.Context, request provider.RequestEnvelope) (provider.ResponseRecord, error) {
	m.calls++
	response, err := exactResponse(request)
	if err != nil {
		return provider.ResponseRecord{}, err
	}
	response.ProviderRequestID = "req-1"
	if m.servedModel != "" {
		response.ServedModel = m.servedModel
	}
	return provider.FinalizeResponse(request, response)
}

func TestCaptureAndReplay(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "fixture.jsonl")

	mock := &mockProvider{name: "mock", model: "m1"}
	request := exactRequest(t, mock.name, mock.model, "test prompt")
	want, err := exactResponse(request)
	if err != nil {
		t.Fatal(err)
	}

	cap, err := WrapCapture(mock, "m1", fixture, false)
	if err != nil {
		t.Fatalf("WrapCapture: %v", err)
	}
	ctx := context.Background()
	if _, err := cap.Score(ctx, request); err != nil {
		t.Fatal(err)
	}
	if err := cap.Close(); err != nil {
		t.Fatal(err)
	}

	rep, err := LoadReplay(fixture, "mock", "m1", provider.Capabilities{})
	if err != nil {
		t.Fatalf("LoadReplay: %v", err)
	}
	got, err := rep.Score(ctx, request)
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if got.RawText != want.RawText {
		t.Errorf("RawText = %q, want %q", got.RawText, want.RawText)
	}
	if got.Usage.Input != 120 {
		t.Errorf("input tokens = %d, want 120", got.Usage.Input)
	}
	if got.Distributions["<score_A>"]["B"] != 0.9 {
		t.Errorf("Distribution roundtrip failed")
	}

	if _, err := rep.Score(ctx, exactRequest(t, mock.name, mock.model, "unknown")); err == nil {
		t.Errorf("expected miss error for unknown prompt")
	}
}

func TestResearchCaptureBindsAttestationBeforeFinalization(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "research.jsonl")
	mock := &mockProvider{name: "mock", model: "m1"}
	request := exactRequest(t, mock.name, mock.model, "research prompt")
	request.Lineage = provider.RequestLineage{
		CriterionID: "generic", SamplingSlot: "generic", Entrypoint: "eval-terminal",
		AuditCaseID: "group-1", SourceTraceHash: digestForTest("source"), TraceMapHash: digestForTest("map"),
		MutationID: "paired_task_group_disagreement", StudyCellID: "distribution-aware-vs-chosen-token",
		PolicyHash: digestForTest("policy"),
	}
	request.EvidenceBindings = []provider.EvidenceBinding{{
		InputSlot: "trajectory_0", SourceDigest: digestForTest("source-evidence"),
		CanonicalDigest: digestForTest("canonical"), IngestionDigest: digestForTest("ingestion"),
		TraceEnvelopeDigest: digestForTest("envelope"), MappingReportDigest: digestForTest("mapping"),
		MappingPolicyVersion: "evalwitness.trace-mapping.v1",
	}}

	capture, err := WrapResearchCapture(mock, "m1", fixture, false, CaptureMetadata{
		CapabilityAttestationID: "att-" + digestForTest("attestation"),
		ServedIdentityPolicy:    provider.ServedIdentityPolicyExactObserved,
		ExpectedServedModel:     "served-name",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := capture.Score(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if err := capture.Close(); err != nil {
		t.Fatal(err)
	}
	inspection, err := InspectCaptureFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.CompleteResearchEntries != 1 || len(inspection.CapabilityAttestationIDs) != 1 {
		t.Fatalf("research inspection = %+v", inspection)
	}
}

func TestResearchCaptureAcceptsOnlyTheLockedServedModelSet(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "research-set.jsonl")
	mock := &mockProvider{name: "mock", model: "m1", servedModel: "deepseek-v4-flash"}
	capture, err := WrapResearchCapture(mock, "m1", fixture, false, CaptureMetadata{
		CapabilityAttestationID: "att-" + digestForTest("attestation-set"),
		ServedIdentityPolicy:    provider.ServedIdentityPolicyExactObservedSet,
		ExpectedServedModels:    []string{"deepseek-v4-flash", "deepseek-v4-flash-202605"},
	})
	if err != nil {
		t.Fatal(err)
	}

	for index, servedModel := range []string{"deepseek-v4-flash", "deepseek-v4-flash-202605"} {
		mock.servedModel = servedModel
		request := exactRequest(t, mock.name, mock.model, fmt.Sprintf("research prompt %d", index))
		if _, err := capture.Score(context.Background(), request); err != nil {
			t.Fatalf("allowed served model %q rejected: %v", servedModel, err)
		}
	}
	mock.servedModel = "deepseek-v4-flash-unknown"
	if _, err := capture.Score(context.Background(), exactRequest(t, mock.name, mock.model, "unknown response route")); err == nil {
		t.Fatal("unknown served model was admitted")
	}
	if err := capture.Close(); err != nil {
		t.Fatal(err)
	}
	inspection, err := InspectCaptureFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Entries != 2 || len(inspection.ServedModels) != 2 {
		t.Fatalf("served-model set capture = %+v", inspection)
	}
}
