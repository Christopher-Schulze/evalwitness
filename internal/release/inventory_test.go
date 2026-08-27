package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

func TestReleaseManifestRoundTripAndExactAssetVerification(t *testing.T) {
	root := releaseAssetFixture(t)
	manifest, err := BuildManifest(root, "0.2.0", strings.Repeat("1", 40), releaseFixtureSourceSHA256(t), "2026-08-12T10:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := EncodeManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyManifestAssets(root, decoded); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "evidence", "result.json"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyManifestAssets(root, decoded); err == nil {
		t.Fatal("release verifier accepted a modified asset")
	}
}

func TestReleaseManifestAllowsOnlyExplicitTagPublicationAuthorization(t *testing.T) {
	root := releaseAssetFixture(t)
	manifest, err := BuildManifest(root, "0.2.0", strings.Repeat("1", 40), releaseFixtureSourceSHA256(t), "2026-08-12T10:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	manifest.Truth.ExternalPublication = "authorized_by_tag"
	if _, err := EncodeManifest(manifest); err != nil {
		t.Fatalf("tag-authorized release manifest was rejected: %v", err)
	}
	manifest.Truth.ExternalPublication = "published"
	if _, err := EncodeManifest(manifest); err == nil {
		t.Fatal("release manifest accepted an ungoverned publication claim")
	}
}

func TestReleaseManifestRejectsUnexpectedAndLinkedAssets(t *testing.T) {
	root := releaseAssetFixture(t)
	if err := os.WriteFile(filepath.Join(root, "unexpected.txt"), []byte("unexpected"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildManifest(root, "0.2.0", strings.Repeat("1", 40), releaseFixtureSourceSHA256(t), "2026-08-12T10:00:00Z"); err == nil {
		t.Fatal("release manifest accepted an asset without a canonical role")
	}
	if err := os.Remove(filepath.Join(root, "unexpected.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "source", "evalwitness-0.2.0-source.tar.gz"), filepath.Join(root, "evidence", "linked.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildManifest(root, "0.2.0", strings.Repeat("1", 40), releaseFixtureSourceSHA256(t), "2026-08-12T10:00:00Z"); err == nil {
		t.Fatal("release manifest accepted a symlink")
	}
}

func TestReleaseManifestRejectsUnboundSourceAndModuleProxyIndex(t *testing.T) {
	root := releaseAssetFixture(t)
	if _, err := BuildManifest(root, "0.2.0", strings.Repeat("1", 40), strings.Repeat("2", 64), "2026-08-12T10:00:00Z"); err == nil {
		t.Fatal("release manifest accepted a source digest that differs from the exact archive asset")
	}
	indexPath := filepath.Join(root, "source", "go-proxy", "index.json")
	indexRaw, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	index, err := DecodeGoModuleProxyIndex(indexRaw)
	if err != nil {
		t.Fatal(err)
	}
	index.Files[0].SHA256 = strings.Repeat("f", 64)
	tampered, err := protocol.CanonicalMarshal(index)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexPath, tampered, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildManifest(root, "0.2.0", strings.Repeat("1", 40), releaseFixtureSourceSHA256(t), "2026-08-12T10:00:00Z"); err == nil {
		t.Fatal("release manifest accepted a module-proxy index that differs from its asset records")
	}
}

func TestReleaseManifestRejectsProxyIdentityAndVersionListDrift(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, string, *GoModuleProxyIndex)
	}{
		{
			name: "module path substitution",
			mutate: func(t *testing.T, _ string, index *GoModuleProxyIndex) {
				t.Helper()
				index.Modules[0].Path = "example.invalid/substituted"
			},
		},
		{
			name: "version list substitution",
			mutate: func(t *testing.T, root string, index *GoModuleProxyIndex) {
				t.Helper()
				listPath := "example.invalid/module/@v/list"
				body := []byte("v2.0.0\n")
				if err := os.WriteFile(filepath.Join(root, "source", "go-proxy", filepath.FromSlash(listPath)), body, 0o600); err != nil {
					t.Fatal(err)
				}
				fileIndex := slices.IndexFunc(index.Files, func(file GoModuleProxyFile) bool { return file.Path == listPath })
				if fileIndex < 0 {
					t.Fatal("fixture module version list is missing")
				}
				index.Files[fileIndex].Bytes = int64(len(body))
				index.Files[fileIndex].SHA256 = protocol.DigestBytes(body)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := releaseAssetFixture(t)
			indexPath := filepath.Join(root, "source", "go-proxy", "index.json")
			indexRaw, err := os.ReadFile(indexPath)
			if err != nil {
				t.Fatal(err)
			}
			index, err := DecodeGoModuleProxyIndex(indexRaw)
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(t, root, &index)
			mutated, err := protocol.CanonicalMarshal(index)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(indexPath, mutated, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := BuildManifest(root, "0.2.0", strings.Repeat("1", 40), releaseFixtureSourceSHA256(t), "2026-08-12T10:00:00Z"); err == nil {
				t.Fatal("release manifest accepted a semantically false module proxy index")
			}
		})
	}
}

func TestReleaseManifestRejectsFalseSourceArchiveReportAndNonUSTARArchive(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, string)
	}{
		{
			name: "false report count",
			mutate: func(t *testing.T, root string) {
				t.Helper()
				reportPath := filepath.Join(root, filepath.FromSlash(sourceArchiveReportPath))
				raw, err := os.ReadFile(reportPath)
				if err != nil {
					t.Fatal(err)
				}
				report, err := DecodeSourceArchiveReport(raw)
				if err != nil {
					t.Fatal(err)
				}
				report.Files++
				writeReleaseFixtureSourceReport(t, root, report)
			},
		},
		{
			name: "PAX archive",
			mutate: func(t *testing.T, root string) {
				t.Helper()
				archiveRaw := releaseFixtureSourceArchiveWithFormat(t, tar.FormatPAX)
				archivePath := filepath.Join(root, "source", "evalwitness-0.2.0-source.tar.gz")
				if err := os.WriteFile(archivePath, archiveRaw, 0o600); err != nil {
					t.Fatal(err)
				}
				reportRaw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(sourceArchiveReportPath)))
				if err != nil {
					t.Fatal(err)
				}
				report, err := DecodeSourceArchiveReport(reportRaw)
				if err != nil {
					t.Fatal(err)
				}
				report.SHA256 = protocol.DigestBytes(archiveRaw)
				report.Bytes = int64(len(archiveRaw))
				writeReleaseFixtureSourceReport(t, root, report)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := releaseAssetFixture(t)
			test.mutate(t, root)
			archiveRaw, err := os.ReadFile(filepath.Join(root, "source", "evalwitness-0.2.0-source.tar.gz"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := BuildManifest(root, "0.2.0", strings.Repeat("1", 40), protocol.DigestBytes(archiveRaw), "2026-08-12T10:00:00Z"); err == nil {
				t.Fatal("release manifest accepted a false canonical-source claim")
			}
		})
	}
}

func releaseAssetFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, role := range canonicalRoles {
		if err := os.Mkdir(filepath.Join(root, role), 0o700); err != nil {
			t.Fatal(err)
		}
		if role == "binary" {
			for _, path := range canonicalBinaryPaths {
				if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte(path), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			continue
		}
		name := role + ".txt"
		body := []byte(role)
		if role == "source" {
			name = "evalwitness-0.2.0-source.tar.gz"
			body = releaseFixtureSourceArchive(t)
		}
		if err := os.WriteFile(filepath.Join(root, role, name), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	proxyRoot := filepath.Join(root, "source", "go-proxy")
	moduleDirectory := "example.invalid/module/@v"
	if err := os.MkdirAll(filepath.Join(proxyRoot, filepath.FromSlash(moduleDirectory)), 0o700); err != nil {
		t.Fatal(err)
	}
	proxyBodies := map[string][]byte{
		moduleDirectory + "/v1.0.0.info": []byte(`{"Version":"v1.0.0","Time":"2026-08-12T10:00:00Z"}`),
		moduleDirectory + "/v1.0.0.mod":  []byte("module example.invalid/module\n"),
		moduleDirectory + "/v1.0.0.zip":  []byte("fixture zip bytes"),
		moduleDirectory + "/list":        []byte("v1.0.0\n"),
	}
	proxyFiles := make([]GoModuleProxyFile, 0, len(proxyBodies))
	for filePath, body := range proxyBodies {
		if err := os.WriteFile(filepath.Join(proxyRoot, filepath.FromSlash(filePath)), body, 0o600); err != nil {
			t.Fatal(err)
		}
		proxyFiles = append(proxyFiles, GoModuleProxyFile{Path: filePath, Bytes: int64(len(body)), SHA256: protocol.DigestBytes(body)})
	}
	slices.SortFunc(proxyFiles, func(left, right GoModuleProxyFile) int { return strings.Compare(left.Path, right.Path) })
	fixtureSum := "h1:" + base64.StdEncoding.EncodeToString(make([]byte, 32))
	proxyIndex := GoModuleProxyIndex{
		SchemaVersion: GoModuleProxySchemaVersion, ModuleCount: 1, FileCount: len(proxyFiles),
		Modules: []GoModuleProxyModule{{
			Path: "example.invalid/module", Version: "v1.0.0", Sum: fixtureSum, GoModSum: fixtureSum,
			Files: []string{moduleDirectory + "/v1.0.0.info", moduleDirectory + "/v1.0.0.mod", moduleDirectory + "/v1.0.0.zip"},
		}},
		Files: proxyFiles,
	}
	indexRaw, err := protocol.CanonicalMarshal(proxyIndex)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proxyRoot, "index.json"), indexRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	archiveRaw, err := os.ReadFile(filepath.Join(root, "source", "evalwitness-0.2.0-source.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	source := capsule.SourceTreeProvenance{
		SchemaVersion: capsule.SourceTreeProvenanceSchemaVersion,
		Algorithm:     capsule.SourceTreeAlgorithm,
		VCS:           "git",
		Commit:        strings.Repeat("1", 40),
		StatusDigest:  protocol.DigestBytes(nil),
		Files:         1,
		Bytes:         int64(len("source")),
		Entries: []capsule.SourceTreeEntry{{
			Path: "README.md", Kind: "file", GitMode: "100644", State: "present",
			Bytes: int64(len("source")), SHA256: protocol.DigestBytes([]byte("source")),
		}},
	}
	source.Digest, err = protocol.Digest(source)
	if err != nil {
		t.Fatal(err)
	}
	sourceRaw, err := capsule.EncodeSourceTreeProvenance(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(sourceTreeProvenancePath)), sourceRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	writeReleaseFixtureSourceReport(t, root, SourceArchiveReport{
		SchemaVersion:    SourceArchiveReportSchemaVersion,
		Product:          ProductName,
		Version:          "0.2.0",
		GitCommit:        strings.Repeat("1", 40),
		SourceTreeDigest: source.Digest,
		ArchiveRoot:      "evalwitness-0.2.0",
		Format:           sourceArchiveFormat,
		SHA256:           protocol.DigestBytes(archiveRaw),
		Bytes:            int64(len(archiveRaw)),
		ExpandedBytes:    int64(len("source")),
		Files:            1,
		Directories:      1,
		Deterministic:    true,
	})
	return root
}

func writeReleaseFixtureSourceReport(t *testing.T, root string, report SourceArchiveReport) {
	t.Helper()
	reportRaw, err := EncodeSourceArchiveReport(report)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(sourceArchiveReportPath)), reportRaw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func releaseFixtureSourceSHA256(t *testing.T) string {
	t.Helper()
	return protocol.DigestBytes(releaseFixtureSourceArchive(t))
}

func releaseFixtureSourceArchive(t *testing.T) []byte {
	t.Helper()
	return releaseFixtureSourceArchiveWithFormat(t, tar.FormatUSTAR)
}

func releaseFixtureSourceArchiveWithFormat(t *testing.T, format tar.Format) []byte {
	t.Helper()
	var buffer bytes.Buffer
	compressed, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	compressed.ModTime = time.Unix(0, 0).UTC()
	compressed.OS = 255
	archive := tar.NewWriter(compressed)
	headers := []*tar.Header{
		{Typeflag: tar.TypeDir, Name: "evalwitness-0.2.0/", Mode: 0o755, ModTime: time.Unix(0, 0).UTC(), Format: format},
		{Typeflag: tar.TypeReg, Name: "evalwitness-0.2.0/README.md", Mode: 0o644, Size: int64(len("source")), ModTime: time.Unix(0, 0).UTC(), Format: format},
	}
	if format == tar.FormatPAX {
		headers[1].PAXRecords = map[string]string{"EVALWITNESS.test": "pax"}
	}
	for index, header := range headers {
		if err := archive.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if index == 1 {
			if _, err := archive.Write([]byte("source")); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
