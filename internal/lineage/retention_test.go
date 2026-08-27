package lineage

import "testing"

func TestRetentionAnalysisReportsOnlyTheFirstLossPerFieldAndChannel(t *testing.T) {
	_, _, _, candidate, _, _, _, _ := completeBOMInputs(t)
	fields := []string{"call_id", "exit_status", "stderr", "stdout"}
	channels := []string{"exit_status", "stderr", "stdout"}
	mutated := candidate
	mutated.Layers = cloneLayerBindings(candidate.Layers)
	mutated.Layers[2].RequiredFields = []string{"call_id", "exit_status", "stdout"}
	mutated.Layers[3].RequiredFields = []string{"call_id", "exit_status", "stdout"}
	mutated.Layers[1].DecisiveChannels = []string{"exit_status", "stdout"}
	mutated.Layers[2].DecisiveChannels = []string{"exit_status", "stdout"}
	mutated.Header.Digest = sealCandidateForTest(t, mutated)
	analysis, err := AnalyzeRetention(mutated, fields, channels)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Complete || len(analysis.Losses) != 2 || len(analysis.TruncatedRequiredFields) != 1 || analysis.TruncatedRequiredFields[0] != "stderr" {
		t.Fatalf("unexpected retention losses: %#v", analysis)
	}
	if analysis.Losses[0] != (RetentionLoss{Layer: "native_export", Kind: RetentionDecisiveChannelLoss, Name: "stderr", Reason: "decisive_channel_absent"}) ||
		analysis.Losses[1] != (RetentionLoss{Layer: "canonical_graph", Kind: RetentionRequiredFieldLoss, Name: "stderr", Reason: "required_field_absent"}) {
		t.Fatalf("retention analysis did not preserve earliest loss layers: %#v", analysis.Losses)
	}
}

func TestRetentionAnalysisDistinguishesStructuredAndSemanticLoss(t *testing.T) {
	_, _, _, candidate, _, _, _, _ := completeBOMInputs(t)
	mutated := candidate
	mutated.Layers = cloneLayerBindings(candidate.Layers)
	mutated.Layers[3].StructuredPresence = false
	mutated.Layers[3].SemanticSufficiency = false
	mutated.Header.Digest = sealCandidateForTest(t, mutated)
	analysis, err := AnalyzeRetention(mutated, []string{"exit_status"}, []string{"exit_status"})
	if err != nil {
		t.Fatal(err)
	}
	if len(analysis.Losses) != 2 || analysis.Losses[0].Reason != "structured_presence_lost" || analysis.Losses[1].Reason != "semantic_sufficiency_lost" {
		t.Fatalf("structured and semantic losses were collapsed: %#v", analysis.Losses)
	}
}

func TestRetentionAnalysisDigestRejectsTampering(t *testing.T) {
	_, _, _, candidate, _, _, _, _ := completeBOMInputs(t)
	analysis, err := AnalyzeRetention(candidate, []string{"exit_status"}, []string{"exit_status"})
	if err != nil {
		t.Fatal(err)
	}
	analysis.Complete = false
	if err := analysis.Validate(); err == nil {
		t.Fatal("tampered retention analysis was accepted")
	}
	analysis, err = AnalyzeRetention(candidate, []string{"exit_status"}, []string{"exit_status"})
	if err != nil {
		t.Fatal(err)
	}
	analysis.Complete = false
	analysis.Losses = []RetentionLoss{{Layer: "retained_bundle", Kind: RetentionRequiredFieldLoss, Name: "exit_status", Reason: "invented_reason"}}
	analysis.TruncatedRequiredFields = []string{"exit_status"}
	analysis.Digest, err = retentionAnalysisDigest(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if err := analysis.Validate(); err == nil {
		t.Fatal("self-consistent invented retention reason was accepted")
	}
}
