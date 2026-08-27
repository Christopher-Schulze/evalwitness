package claim

import (
	"context"
	"reflect"
	"slices"
	"testing"
)

func TestClaimAutopsyRecomputesMethodAndTransportEvidence(t *testing.T) {
	pack := buildClaimReferencePackage(t)
	ledger, err := DefaultLedger(pack.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	autopsy, err := BuildAutopsy(context.Background(), pack.Registry, pack.Manifest, pack.Payloads, ledger)
	if err != nil {
		t.Fatal(err)
	}

	if got := autopsy.MethodIntegrity.Generations; len(got) != 3 || got[0].State != "falsified" || got[1].State != "superseded" || got[2].State != "admitted_development" {
		t.Fatalf("method generations = %+v", got)
	}
	assertAutopsyCounts(t, autopsy.MethodIntegrity.Generations[0].FrozenDenominators, map[string]int{
		"corrected_rejections":     3,
		"fixtures":                 3,
		"legacy_false_acceptances": 3,
	})
	assertAutopsyCounts(t, autopsy.MethodIntegrity.Generations[1].FrozenDenominators, map[string]int{
		"challenge_cases":       14,
		"positive_controls":     6,
		"shared_guards":         3,
		"v2_false_acceptances":  5,
		"v3_repaired_negatives": 5,
	})
	assertAutopsyCounts(t, autopsy.MethodIntegrity.Generations[2].FrozenDenominators, map[string]int{
		"inferential_core_cases":    280,
		"natural_applied_attempts":  689,
		"natural_rejected_attempts": 250,
		"natural_selected_cases":    283,
		"natural_source_tasks":      100,
		"natural_sources":           200,
		"natural_total_attempts":    939,
		"release_cases":             283,
		"scarcity_admitted":         3,
		"scarcity_attempted":        198,
		"scarcity_rejected":         195,
		"scarcity_shortfall":        37,
		"scarcity_target":           40,
		"scarcity_test_role":        0,
	})

	wantLayers := []string{"runtime_witness", "native_export", "canonical_graph", "retained_bundle", "verifier_request"}
	gotLayers := make([]string, len(autopsy.ClaimTransport.Layers))
	for index, layer := range autopsy.ClaimTransport.Layers {
		gotLayers[index] = layer.Layer
	}
	if !slices.Equal(gotLayers, wantLayers) || autopsy.ClaimTransport.ProviderCalls != 0 || !autopsy.ClaimTransport.ReleaseVerified {
		t.Fatalf("claim transport = %+v", autopsy.ClaimTransport)
	}
	if len(autopsy.CurrentClaimIDs) != 15 || len(autopsy.HistoricalClaimIDs) != 19 {
		t.Fatalf("claim lifecycle = %d current, %d historical", len(autopsy.CurrentClaimIDs), len(autopsy.HistoricalClaimIDs))
	}

	raw, err := EncodeAutopsy(autopsy)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeAutopsy(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, autopsy) {
		t.Fatal("claim autopsy codec changed the canonical artifact")
	}
	rebuilt, err := BuildAutopsy(context.Background(), pack.Registry, pack.Manifest, pack.Payloads, ledger)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rebuilt, autopsy) {
		t.Fatal("claim autopsy is not deterministic")
	}

	t.Run("frozen denominator substitution", func(t *testing.T) {
		mutated := autopsy
		mutated.MethodIntegrity.Generations = cloneMethodGenerations(autopsy.MethodIntegrity.Generations)
		mutated.MethodIntegrity.Generations[2].FrozenDenominators[0].Value++
		mutated.Digest, err = autopsyDigest(mutated)
		if err != nil {
			t.Fatal(err)
		}
		if err := VerifyAutopsy(context.Background(), pack.Registry, pack.Manifest, pack.Payloads, ledger, mutated); err == nil {
			t.Fatal("claim autopsy accepted a substituted frozen denominator")
		}
	})

	t.Run("request lineage collapse", func(t *testing.T) {
		mutated := autopsy
		mutated.ClaimTransport.RequestFingerprint = mutated.ClaimTransport.CanonicalRequestLineageDigest
		mutated.Digest, err = autopsyDigest(mutated)
		if err != nil {
			t.Fatal(err)
		}
		if err := VerifyAutopsy(context.Background(), pack.Registry, pack.Manifest, pack.Payloads, ledger, mutated); err == nil {
			t.Fatal("claim autopsy accepted a collapsed request identity and lineage digest")
		}
	})

	t.Run("method transition evidence substitution", func(t *testing.T) {
		mutated := autopsy
		mutated.MethodIntegrity.Transitions = slices.Clone(autopsy.MethodIntegrity.Transitions)
		mutated.MethodIntegrity.Transitions[1].EvidenceComponentIDs = []string{pack.Manifest.ManifestDigest}
		mutated.Digest, err = autopsyDigest(mutated)
		if err != nil {
			t.Fatal(err)
		}
		if err := VerifyAutopsy(context.Background(), pack.Registry, pack.Manifest, pack.Payloads, ledger, mutated); err == nil {
			t.Fatal("claim autopsy accepted substituted method-transition evidence")
		}
	})
}

func assertAutopsyCounts(t *testing.T, counts []AutopsyCount, expected map[string]int) {
	t.Helper()
	if len(counts) != len(expected) {
		t.Fatalf("autopsy counts = %+v, want %+v", counts, expected)
	}
	for _, count := range counts {
		want, found := expected[count.Name]
		if !found || want != count.Value {
			t.Fatalf("autopsy count %q = %d, want %d (declared=%t)", count.Name, count.Value, want, found)
		}
	}
}

func cloneMethodGenerations(values []MethodGeneration) []MethodGeneration {
	cloned := slices.Clone(values)
	for index := range cloned {
		cloned[index].FrozenDenominators = slices.Clone(cloned[index].FrozenDenominators)
	}
	return cloned
}
