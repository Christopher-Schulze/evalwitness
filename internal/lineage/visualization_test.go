package lineage

import (
	"bytes"
	"testing"
)

func TestLossCertificateAndGraphShareCanonicalEvidence(t *testing.T) {
	certificate, err := BuildVerificationLineageLossCertificate("../..")
	if err != nil {
		t.Fatal(err)
	}
	graph, err := BuildVerificationLineageGraph("../..")
	if err != nil {
		t.Fatal(err)
	}
	if graph.LossCertificateDigest != certificate.Digest || graph.Paths[1].EvidenceDigest != certificate.Digest {
		t.Fatal("visual graph diverged from its loss certificate")
	}
}

func TestLineageSVGIsDeterministicAndClaimBounded(t *testing.T) {
	graph, err := BuildVerificationLineageGraph("../..")
	if err != nil {
		t.Fatal(err)
	}
	first, err := RenderVerificationLineageGraphSVG(graph)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RenderVerificationLineageGraphSVG(graph)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) || !bytes.Contains(first, []byte("No empirical claim")) || !bytes.Contains(first, []byte("STOP  first_loss")) {
		t.Fatal("lineage SVG is nondeterministic or lost its claim boundary")
	}
}

func TestLineageGraphDecoderRejectsUnknownAndClaimChangingFields(t *testing.T) {
	graph, err := BuildVerificationLineageGraph("../..")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeIndented(graph)
	if err != nil {
		t.Fatal(err)
	}
	unknown := bytes.Replace(encoded, []byte("\n}"), []byte(",\n  \"unknown\": true\n}"), 1)
	if _, err := DecodeLineageGraph("../..", bytes.NewReader(unknown)); err == nil {
		t.Fatal("lineage graph decoder accepted an unknown field")
	}
	tampered := bytes.Replace(encoded, []byte(`"development_fixtures": 2`), []byte(`"development_fixtures": 3`), 1)
	if _, err := DecodeLineageGraph("../..", bytes.NewReader(tampered)); err == nil {
		t.Fatal("lineage graph decoder accepted a claim-changing fixture count")
	}
}
