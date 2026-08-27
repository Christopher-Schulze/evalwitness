package explorer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	claimledger "github.com/Christopher-Schulze/evalwitness/internal/claim"
	"github.com/Christopher-Schulze/evalwitness/internal/replay"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const identicalResponseBaseRoot = "evalwitness-capsule-3b26fefb5174cc63d03f47f2be5543878c287e978155116b0da6dce85d9ebf19"

func TestIdenticalResponseExplorerUsesVerifiedCanonicalEvidence(t *testing.T) {
	repositoryRoot := filepath.Join("..", "..")
	basePath := extractIdenticalResponseBase(t, repositoryRoot)
	base, err := replay.LoadResponseBundlePackage(context.Background(), basePath)
	if err != nil {
		t.Fatal(err)
	}
	child, err := replay.LoadIdenticalResponseCapsulePackage(
		context.Background(),
		filepath.Join(repositoryRoot, "eval", "governance", "identical-response-capsule-v5-outer"),
		base.Registry,
	)
	if err != nil {
		t.Fatal(err)
	}
	ledger := decodeIdenticalResponseFixture[claimledger.Ledger](t, filepath.Join(repositoryRoot, "eval", "governance", "identical-response-claim-ledger-v5.json"))
	pack := decodeIdenticalResponseFixture[claimledger.ChallengePack](t, filepath.Join(repositoryRoot, "eval", "governance", "identical-response-claim-challenge-pack-v5.json"))
	reproductionPath := filepath.Join(repositoryRoot, identicalResponseReproductionPath)
	reproductionRaw, err := os.ReadFile(reproductionPath)
	if err != nil {
		t.Fatal(err)
	}
	view, err := buildIdenticalResponseView(context.Background(), base, child, ledger, pack, reproductionRaw)
	if err != nil {
		t.Fatal(err)
	}
	if view.Comparison.TaskGroups != 60 || view.Comparison.Disagreements != 7 || len(view.Failures) != 7 {
		t.Fatalf("identical-response comparison = %+v / failures=%d", view.Comparison, len(view.Failures))
	}
	if view.Challenges.Total != 34 || len(view.Claims) != 5 || view.Reproduction.ArtifactsVerified != 14 {
		t.Fatalf("identical-response evidence = challenges=%d claims=%d artifacts=%d", view.Challenges.Total, len(view.Claims), view.Reproduction.ArtifactsVerified)
	}
	if err := view.Validate(); err != nil {
		t.Fatal(err)
	}
	testIdenticalResponseExplorerRejectsDetachedSummaries(t, view)
}

func testIdenticalResponseExplorerRejectsDetachedSummaries(t *testing.T, source IdenticalResponseView) {
	t.Helper()
	tests := []struct {
		name   string
		mutate func(*IdenticalResponseView)
	}{
		{name: "rate", mutate: func(view *IdenticalResponseView) { view.Comparison.DisagreementRate = "0.5" }},
		{name: "class count", mutate: func(view *IdenticalResponseView) { view.Challenges.ClassCounts[0].Value++ }},
		{name: "unknown claim", mutate: func(view *IdenticalResponseView) { view.Challenges.Receipts[0].ClaimID = "CLM-999" }},
		{name: "source schema", mutate: func(view *IdenticalResponseView) { view.Source.SchemaVersion = "detached" }},
		{name: "decision margin", mutate: func(view *IdenticalResponseView) { view.Failures[0].ChosenToken.Margin = "-1" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			view := cloneIdenticalResponseView(t, source)
			test.mutate(&view)
			var err error
			view.Digest, err = identicalResponseViewDigest(view)
			if err != nil {
				t.Fatal(err)
			}
			if err := view.Validate(); err == nil {
				t.Fatal("mutated identical-response explorer view passed validation")
			}
		})
	}
}

func cloneIdenticalResponseView(t *testing.T, source IdenticalResponseView) IdenticalResponseView {
	t.Helper()
	raw, err := protocol.CanonicalMarshal(source)
	if err != nil {
		t.Fatal(err)
	}
	var clone IdenticalResponseView
	if err := protocol.DecodeStrict(raw, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func extractIdenticalResponseBase(t *testing.T, repositoryRoot string) string {
	t.Helper()
	destination := filepath.Join(t.TempDir(), "base")
	_, err := safety.ExtractTarGzip(context.Background(), safety.ArchiveExtractRequest{
		Sources:     []string{filepath.Join(repositoryRoot, "eval", "governance", "identical-response-capsule-v5.tar.gz")},
		Destination: destination, ExpectedRoots: []string{identicalResponseBaseRoot},
		Limits: safety.DefaultArchiveLimits(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(destination, identicalResponseBaseRoot)
}

func decodeIdenticalResponseFixture[T any](t *testing.T, path string) T {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value T
	if err := protocol.DecodeStrict(bytes.TrimSpace(raw), &value); err != nil {
		t.Fatal(err)
	}
	return value
}
