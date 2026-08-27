package explorer

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

func assertExplorerRenderContract(t *testing.T, report Report) {
	t.Helper()
	first, firstMetadata, err := RenderHTML(report)
	if err != nil {
		t.Fatal(err)
	}
	second, secondMetadata, err := RenderHTML(report)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) || firstMetadata != secondMetadata {
		t.Fatal("evidence explorer rendering is not deterministic")
	}
	reportRaw, err := EncodeReport(report)
	if err != nil {
		t.Fatal(err)
	}
	assertRenderedDocumentBindings(t, first, reportRaw, firstMetadata)
	malicious := `</meta><script>globalThis.injected=true</script><img src=https://invalid.example>`
	report.Claim.FailableProperty = malicious
	report.Digest = ""
	report, err = sealReport(report)
	if err != nil {
		t.Fatal(err)
	}
	html, _, err := RenderHTML(report)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(html, []byte(malicious)) || bytes.Contains(html, []byte("globalThis.injected=true")) {
		t.Fatal("rendered HTML contains untrusted report text outside its base64 payload")
	}
	reportRaw, err = EncodeReport(report)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(html, []byte(base64.StdEncoding.EncodeToString(reportRaw))) {
		t.Fatal("rendered HTML omitted the encoded malicious-text report")
	}
}

func assertRenderedDocumentBindings(t *testing.T, html []byte, reportRaw []byte, metadata RenderMetadata) {
	t.Helper()
	document := string(html)
	if metadata.Bytes != len(html) || metadata.HTMLSHA256 != protocol.DigestBytes(html) ||
		metadata.ReportPayloadSHA256 != protocol.DigestBytes(reportRaw) {
		t.Fatal("render metadata differs from the rendered bytes")
	}
	for _, required := range []string{
		`default-src 'none'`, `connect-src 'none'`, `script-src-attr 'none'`,
		`meta name="evalwitness-report" content="` + base64.StdEncoding.EncodeToString(reportRaw) + `"`,
		`meta name="evalwitness-report-sha256" content="` + protocol.DigestBytes(reportRaw) + `"`,
		`meta name="evalwitness-renderer-sha256" content="` + metadata.RendererDigest + `"`,
	} {
		if !strings.Contains(document, required) {
			t.Fatalf("rendered HTML omits %q", required)
		}
	}
	structure, err := renderedDocumentStructure(document)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(structure, `<script src=`) || strings.Contains(structure, `<link `) ||
		strings.Contains(structure, `<iframe`) || strings.Contains(structure, `<form`) {
		t.Fatal("rendered HTML contains an external-load or submission surface")
	}
}

func renderedDocumentStructure(document string) (string, error) {
	styleStart := strings.Index(document, "<style>")
	styleEnd := strings.Index(document, "</style>")
	scriptStart := strings.Index(document[styleEnd+len("</style>"):], "<script>")
	if scriptStart >= 0 {
		scriptStart += styleEnd + len("</style>")
	}
	scriptEnd := strings.LastIndex(document, "</script>")
	if styleStart < 0 || styleEnd <= styleStart || scriptStart <= styleEnd || scriptEnd <= scriptStart {
		return "", errors.New("rendered HTML inline asset boundaries are invalid")
	}
	return document[:styleStart] + "<style></style>" +
		document[styleEnd+len("</style>"):scriptStart] + "<script></script>" +
		document[scriptEnd+len("</script>"):], nil
}
