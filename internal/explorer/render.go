package explorer

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	renderTemplateVersion       = "evalwitness.evidence-explorer-html.v1"
	RenderMetadataSchemaVersion = "evalwitness.evidence-explorer-render-metadata.v1"
	maximumRenderedHTMLBytes    = 8 << 20
)

type RenderMetadata struct {
	SchemaVersion       string `json:"schema_version"`
	ReportDigest        string `json:"report_digest"`
	ReportPayloadSHA256 string `json:"report_payload_sha256"`
	RendererDigest      string `json:"renderer_digest"`
	HTMLSHA256          string `json:"html_sha256"`
	Bytes               int    `json:"bytes"`
}

func RenderHTML(report Report) ([]byte, RenderMetadata, error) {
	reportRaw, err := EncodeReport(report)
	if err != nil {
		return nil, RenderMetadata{}, err
	}
	assets, err := loadExplorerAssets()
	if err != nil {
		return nil, RenderMetadata{}, err
	}
	html := renderDocument(reportRaw, assets)
	if len(html) > maximumRenderedHTMLBytes {
		return nil, RenderMetadata{}, errors.New("rendered evidence explorer exceeds its size bound")
	}
	metadata := RenderMetadata{
		SchemaVersion: RenderMetadataSchemaVersion, ReportDigest: report.Digest,
		ReportPayloadSHA256: protocol.DigestBytes(reportRaw), RendererDigest: assets.rendererDigest,
		HTMLSHA256: protocol.DigestBytes(html), Bytes: len(html),
	}
	if err := metadata.Validate(); err != nil {
		return nil, RenderMetadata{}, err
	}
	return html, metadata, nil
}

func (metadata RenderMetadata) Validate() error {
	if metadata.SchemaVersion != RenderMetadataSchemaVersion || !validDigest(metadata.ReportDigest) ||
		!validDigest(metadata.ReportPayloadSHA256) || !validDigest(metadata.RendererDigest) ||
		!validDigest(metadata.HTMLSHA256) || metadata.Bytes < 1 || metadata.Bytes > maximumRenderedHTMLBytes {
		return errors.New("evidence explorer render metadata is invalid")
	}
	return nil
}

func renderDocument(reportRaw []byte, assets explorerAssetBundle) []byte {
	reportPayload := base64.StdEncoding.EncodeToString(reportRaw)
	styleHash := contentSecurityHash(assets.stylesheet)
	scriptHash := contentSecurityHash(assets.javascript)
	csp := renderContentSecurityPolicy(styleHash, scriptHash)
	document := strings.Join([]string{
		"<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\">",
		"<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">",
		"<meta name=\"color-scheme\" content=\"light\"><meta name=\"referrer\" content=\"no-referrer\">",
		"<meta http-equiv=\"Content-Security-Policy\" content=\"", csp,
		"\"><meta name=\"evalwitness-report\" content=\"", reportPayload,
		"\"><meta name=\"evalwitness-report-sha256\" content=\"", protocol.DigestBytes(reportRaw),
		"\"><meta name=\"evalwitness-renderer-sha256\" content=\"", assets.rendererDigest,
		"\"><title>EvalWitness Evidence Explorer</title><style>", string(assets.stylesheet),
		"</style></head><body><div id=\"root\"></div>",
		"<noscript>This verified evidence report requires local JavaScript; it performs no network requests.</noscript><script>",
		string(assets.javascript), "</script></body></html>\n",
	}, "")
	return []byte(document)
}

func contentSecurityHash(raw []byte) string {
	digest := sha256.Sum256(raw)
	return base64.StdEncoding.EncodeToString(digest[:])
}

func renderContentSecurityPolicy(styleHash string, scriptHash string) string {
	return fmt.Sprintf(
		"default-src 'none'; base-uri 'none'; connect-src 'none'; font-src 'none'; form-action 'none'; frame-src 'none'; img-src 'none'; manifest-src 'none'; media-src 'none'; object-src 'none'; script-src 'sha256-%s'; script-src-attr 'none'; style-src 'sha256-%s'; style-src-attr 'unsafe-inline'; worker-src 'none'",
		scriptHash, styleHash,
	)
}
