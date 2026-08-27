package lineage

import (
	"errors"
	"fmt"
	"html"
	"reflect"
	"slices"
	"strings"
)

const (
	LossCertificateVersion = "evalwitness.verification-lineage-loss-certificate.v1"
	LineageGraphVersion    = "evalwitness.verification-lineage-graph.v1"
)

type LossCertificateLayer struct {
	Layer  string `json:"layer"`
	Status string `json:"status"`
}

type VerificationLineageLossCertificate struct {
	Version               string                 `json:"version"`
	CertificateID         string                 `json:"certificate_id"`
	OfflineProofDigest    string                 `json:"offline_proof_digest"`
	CaseID                string                 `json:"case_id"`
	VectorID              string                 `json:"vector_id"`
	Format                string                 `json:"format"`
	FirstLossState        TerminalState          `json:"first_loss_state"`
	Disposition           StateDisposition       `json:"disposition"`
	Precedence            int                    `json:"precedence"`
	FailabilityResolution FailabilityResolution  `json:"failability_resolution"`
	FailabilityReason     FailabilityReasonCode  `json:"failability_reason"`
	NativeInputDigest     string                 `json:"native_input_digest"`
	OperandsDigest        string                 `json:"operands_digest"`
	Layers                []LossCertificateLayer `json:"layers"`
	PublicSafe            bool                   `json:"public_safe"`
	RestrictedContent     bool                   `json:"restricted_content"`
	SupportedClaim        string                 `json:"supported_claim"`
	UnsupportedClaims     []string               `json:"unsupported_claims"`
	Digest                string                 `json:"digest"`
}

type LineageGraphPath struct {
	PathID         string                 `json:"path_id"`
	Label          string                 `json:"label"`
	TerminalState  TerminalState          `json:"terminal_state"`
	Layers         []LossCertificateLayer `json:"layers"`
	EvidenceDigest string                 `json:"evidence_digest"`
}

type VerificationLineageGraph struct {
	Version               string             `json:"version"`
	GraphID               string             `json:"graph_id"`
	OfflineProofDigest    string             `json:"offline_proof_digest"`
	LossCertificateDigest string             `json:"loss_certificate_digest"`
	Paths                 []LineageGraphPath `json:"paths"`
	DevelopmentFixtures   int                `json:"development_fixtures"`
	Empirical             bool               `json:"empirical"`
	ProviderRanking       bool               `json:"provider_ranking"`
	Digest                string             `json:"digest"`
}

func BuildVerificationLineageLossCertificate(repositoryRoot string) (VerificationLineageLossCertificate, error) {
	proof, err := BuildVerificationLineageOfflineProof(repositoryRoot)
	if err != nil {
		return VerificationLineageLossCertificate{}, err
	}
	certificate, err := lossCertificateFromProof(proof)
	if err != nil {
		return VerificationLineageLossCertificate{}, err
	}
	return certificate, certificate.validateShape()
}

func lossCertificateFromProof(proof VerificationLineageOfflineProof) (VerificationLineageLossCertificate, error) {
	counterexample := proof.Counterexample
	certificate := VerificationLineageLossCertificate{
		Version: LossCertificateVersion, CertificateID: "task_069-same-path-loss-v1", OfflineProofDigest: proof.Digest,
		CaseID: counterexample.CaseID, VectorID: counterexample.VectorID, Format: counterexample.Classification.Format,
		FirstLossState: counterexample.Classification.TerminalState, Disposition: counterexample.Classification.StateDisposition,
		Precedence: counterexample.Classification.PrecedenceApplied, FailabilityResolution: counterexample.Failability.Resolution,
		FailabilityReason: counterexample.Failability.ReasonCode, NativeInputDigest: counterexample.NativeInputDigest,
		OperandsDigest:    counterexample.Failability.Finding.OperandsDigest,
		Layers:            certificateLayers(counterexample.Classification.LayerSurvival),
		PublicSafe:        true,
		SupportedClaim:    "the sealed same-path comparison becomes terminally ineligible at semantic failability and cannot reach retained evidence or a verifier request",
		UnsupportedClaims: []string{"agent behavior prevalence", "provider quality", "task correctness", "universal parser performance"},
	}
	digest, err := lossCertificateDigest(certificate)
	if err != nil {
		return VerificationLineageLossCertificate{}, err
	}
	certificate.Digest = digest
	return certificate, nil
}

func (certificate VerificationLineageLossCertificate) Validate(repositoryRoot string) error {
	if err := certificate.validateShape(); err != nil {
		return err
	}
	expected, err := buildLossCertificateUnchecked(repositoryRoot)
	if err != nil {
		return err
	}
	actualWithoutDigest := certificate
	actualWithoutDigest.Digest = ""
	expectedWithoutDigest := expected
	expectedWithoutDigest.Digest = ""
	if !reflect.DeepEqual(actualWithoutDigest, expectedWithoutDigest) || certificate.Digest != expected.Digest {
		return errors.New("verification-lineage loss certificate differs from the sealed counterexample")
	}
	return nil
}

func (certificate VerificationLineageLossCertificate) validateShape() error {
	if certificate.Version != LossCertificateVersion || certificate.CertificateID != "task_069-same-path-loss-v1" ||
		!validDigest(certificate.OfflineProofDigest) || !validDigest(certificate.NativeInputDigest) || !validDigest(certificate.OperandsDigest) || !validDigest(certificate.Digest) ||
		certificate.CaseID != "same_path_comparison_false_target" || certificate.VectorID != "codex_rollout_jsonl/same_path_comparison_false_target" ||
		certificate.Format != "codex_rollout_jsonl" || certificate.FirstLossState != StateNonFailableVerification || certificate.Disposition != DispositionIneligible ||
		certificate.Precedence != 7 || certificate.FailabilityResolution != FailabilityNonFailable || certificate.FailabilityReason != FailabilityReasonSelfComparison ||
		!certificate.PublicSafe || certificate.RestrictedContent || certificate.SupportedClaim == "" ||
		!slices.Equal(certificate.UnsupportedClaims, []string{"agent behavior prevalence", "provider quality", "task correctness", "universal parser performance"}) {
		return errors.New("verification-lineage loss certificate identity or claim boundary is invalid")
	}
	wantLayers := []LossCertificateLayer{
		{Layer: "runtime_witness", Status: "survived"}, {Layer: "native_export", Status: "survived"},
		{Layer: "canonical_graph", Status: "survived"}, {Layer: "retained_bundle", Status: "first_loss"},
		{Layer: "verifier_request", Status: "not_reached"},
	}
	if !reflect.DeepEqual(certificate.Layers, wantLayers) {
		return errors.New("verification-lineage loss certificate layer projection is invalid")
	}
	digest, err := lossCertificateDigest(certificate)
	if err != nil || certificate.Digest != digest {
		return errors.New("verification-lineage loss certificate digest is invalid")
	}
	return nil
}

func buildLossCertificateUnchecked(repositoryRoot string) (VerificationLineageLossCertificate, error) {
	proof, err := BuildVerificationLineageOfflineProof(repositoryRoot)
	if err != nil {
		return VerificationLineageLossCertificate{}, err
	}
	return lossCertificateFromProof(proof)
}

func BuildVerificationLineageGraph(repositoryRoot string) (VerificationLineageGraph, error) {
	proof, err := BuildVerificationLineageOfflineProof(repositoryRoot)
	if err != nil {
		return VerificationLineageGraph{}, err
	}
	certificate, err := lossCertificateFromProof(proof)
	if err != nil {
		return VerificationLineageGraph{}, err
	}
	graph, err := lineageGraphFromProof(proof, certificate)
	if err != nil {
		return VerificationLineageGraph{}, err
	}
	return graph, graph.validateShape()
}

func lineageGraphFromProof(proof VerificationLineageOfflineProof, certificate VerificationLineageLossCertificate) (VerificationLineageGraph, error) {
	graph := VerificationLineageGraph{
		Version: LineageGraphVersion, GraphID: "task_069-offline-lineage-graph-v1", OfflineProofDigest: proof.Digest,
		LossCertificateDigest: certificate.Digest, DevelopmentFixtures: 2,
		Paths: []LineageGraphPath{
			{PathID: "accepted-positive", Label: "Accepted five-layer fixture", TerminalState: proof.Positive.TerminalState, Layers: certificateLayers(proof.Positive.LayerSurvival), EvidenceDigest: proof.Positive.BOMDigest},
			{PathID: "non-failable-counterexample", Label: "Rejected same-path comparison", TerminalState: proof.Counterexample.Classification.TerminalState, Layers: append([]LossCertificateLayer(nil), certificate.Layers...), EvidenceDigest: certificate.Digest},
		},
	}
	digest, err := lineageGraphDigest(graph)
	if err != nil {
		return VerificationLineageGraph{}, err
	}
	graph.Digest = digest
	return graph, nil
}

func (graph VerificationLineageGraph) Validate(repositoryRoot string) error {
	if err := graph.validateShape(); err != nil {
		return err
	}
	expected, err := buildLineageGraphUnchecked(repositoryRoot)
	if err != nil {
		return err
	}
	actualWithoutDigest := graph
	actualWithoutDigest.Digest = ""
	expectedWithoutDigest := expected
	expectedWithoutDigest.Digest = ""
	if !reflect.DeepEqual(actualWithoutDigest, expectedWithoutDigest) || graph.Digest != expected.Digest {
		return errors.New("verification-lineage graph differs from its canonical evidence")
	}
	return nil
}

func (graph VerificationLineageGraph) validateShape() error {
	if graph.Version != LineageGraphVersion || graph.GraphID != "task_069-offline-lineage-graph-v1" || !validDigest(graph.OfflineProofDigest) ||
		!validDigest(graph.LossCertificateDigest) || !validDigest(graph.Digest) || graph.DevelopmentFixtures != 2 || graph.Empirical || graph.ProviderRanking || len(graph.Paths) != 2 {
		return errors.New("verification-lineage graph identity or claim boundary is invalid")
	}
	if graph.Paths[0].PathID != "accepted-positive" || graph.Paths[0].Label != "Accepted five-layer fixture" || graph.Paths[0].TerminalState != StateDirectVerificationInvocation ||
		graph.Paths[1].PathID != "non-failable-counterexample" || graph.Paths[1].Label != "Rejected same-path comparison" || graph.Paths[1].TerminalState != StateNonFailableVerification ||
		graph.Paths[1].EvidenceDigest != graph.LossCertificateDigest {
		return errors.New("verification-lineage graph paths are incomplete or unordered")
	}
	for _, path := range graph.Paths {
		if path.Label == "" || !validDigest(path.EvidenceDigest) || len(path.Layers) != 5 {
			return errors.New("verification-lineage graph path is invalid")
		}
	}
	positiveLayers := []LossCertificateLayer{
		{Layer: "runtime_witness", Status: "survived"}, {Layer: "native_export", Status: "survived"},
		{Layer: "canonical_graph", Status: "survived"}, {Layer: "retained_bundle", Status: "survived"},
		{Layer: "verifier_request", Status: "survived"},
	}
	negativeLayers := []LossCertificateLayer{
		{Layer: "runtime_witness", Status: "survived"}, {Layer: "native_export", Status: "survived"},
		{Layer: "canonical_graph", Status: "survived"}, {Layer: "retained_bundle", Status: "first_loss"},
		{Layer: "verifier_request", Status: "not_reached"},
	}
	if !reflect.DeepEqual(graph.Paths[0].Layers, positiveLayers) || !reflect.DeepEqual(graph.Paths[1].Layers, negativeLayers) {
		return errors.New("verification-lineage graph layer projection is invalid")
	}
	digest, err := lineageGraphDigest(graph)
	if err != nil || graph.Digest != digest {
		return errors.New("verification-lineage graph digest is invalid")
	}
	return nil
}

func buildLineageGraphUnchecked(repositoryRoot string) (VerificationLineageGraph, error) {
	proof, err := BuildVerificationLineageOfflineProof(repositoryRoot)
	if err != nil {
		return VerificationLineageGraph{}, err
	}
	certificate, err := lossCertificateFromProof(proof)
	if err != nil {
		return VerificationLineageGraph{}, err
	}
	return lineageGraphFromProof(proof, certificate)
}

func RenderVerificationLineageGraphSVG(graph VerificationLineageGraph) ([]byte, error) {
	if err := graph.validateShape(); err != nil {
		return nil, err
	}
	layers := []string{"Runtime witness", "Native export", "Canonical graph", "Retained bundle", "Verifier request"}
	var builder strings.Builder
	writef := func(format string, args ...any) error {
		_, err := fmt.Fprintf(&builder, format, args...)
		return err
	}
	builder.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="430" viewBox="0 0 1200 430" role="img" aria-labelledby="title desc">`)
	builder.WriteString(`<title id="title">EvalWitness verification lineage proof</title><desc id="desc">Two development fixtures derived from canonical JSON: one accepted five-layer chain and one same-path comparison rejected at semantic failability.</desc>`)
	builder.WriteString(`<rect width="1200" height="430" fill="#0b1020"/><text x="64" y="58" fill="#f8fafc" font-family="ui-monospace, SFMono-Regular, Menlo, monospace" font-size="26" font-weight="700">Verification lineage, without the hand-waving</text>`)
	builder.WriteString(`<text x="64" y="88" fill="#94a3b8" font-family="ui-monospace, SFMono-Regular, Menlo, monospace" font-size="14">Two sealed development fixtures. No provider call. No empirical claim.</text>`)
	for index, label := range layers {
		x := 74 + index*220
		if err := writef(`<text x="%d" y="135" fill="#cbd5e1" font-family="ui-monospace, SFMono-Regular, Menlo, monospace" font-size="13">%s</text>`, x, html.EscapeString(label)); err != nil {
			return nil, err
		}
	}
	for pathIndex, path := range graph.Paths {
		y := 205 + pathIndex*120
		if err := writef(`<text x="64" y="%d" fill="#f8fafc" font-family="ui-monospace, SFMono-Regular, Menlo, monospace" font-size="15" font-weight="700">%s</text>`, y-30, html.EscapeString(path.Label)); err != nil {
			return nil, err
		}
		for layerIndex, layer := range path.Layers {
			x := 74 + layerIndex*220
			fill, stroke, mark := graphLayerStyle(layer.Status)
			if err := writef(`<rect x="%d" y="%d" width="176" height="48" rx="8" fill="%s" stroke="%s" stroke-width="2"/>`, x, y, fill, stroke); err != nil {
				return nil, err
			}
			if err := writef(`<text x="%d" y="%d" fill="#f8fafc" font-family="ui-monospace, SFMono-Regular, Menlo, monospace" font-size="13">%s  %s</text>`, x+14, y+29, mark, html.EscapeString(layer.Status)); err != nil {
				return nil, err
			}
			if layerIndex < len(path.Layers)-1 {
				if err := writef(`<path d="M %d %d H %d" stroke="#475569" stroke-width="2"/>`, x+176, y+24, x+220); err != nil {
					return nil, err
				}
			}
		}
	}
	if err := writef(`<text x="64" y="400" fill="#64748b" font-family="ui-monospace, SFMono-Regular, Menlo, monospace" font-size="12">graph %s · source %s</text>`, html.EscapeString(graph.Digest[:12]), html.EscapeString(graph.OfflineProofDigest[:12])); err != nil {
		return nil, err
	}
	builder.WriteString(`</svg>`)
	return []byte(builder.String() + "\n"), nil
}

func certificateLayers(survival []bool) []LossCertificateLayer {
	layers := []string{"runtime_witness", "native_export", "canonical_graph", "retained_bundle", "verifier_request"}
	result := make([]LossCertificateLayer, len(layers))
	firstLoss := -1
	for index, survived := range survival {
		if !survived && firstLoss < 0 {
			firstLoss = index
		}
	}
	for index, layer := range layers {
		status := "survived"
		if firstLoss >= 0 && index == firstLoss {
			status = "first_loss"
		} else if firstLoss >= 0 && index > firstLoss {
			status = "not_reached"
		}
		result[index] = LossCertificateLayer{Layer: layer, Status: status}
	}
	return result
}

func graphLayerStyle(status string) (string, string, string) {
	switch status {
	case "survived":
		return "#123c35", "#34d399", "PASS"
	case "first_loss":
		return "#4a1d2b", "#fb7185", "STOP"
	default:
		return "#1e293b", "#475569", "WAIT"
	}
}

func lossCertificateDigest(certificate VerificationLineageLossCertificate) (string, error) {
	certificate.Digest = ""
	return digestJSON(certificate)
}

func lineageGraphDigest(graph VerificationLineageGraph) (string, error) {
	graph.Digest = ""
	return digestJSON(graph)
}
