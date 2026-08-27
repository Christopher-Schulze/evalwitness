package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLegacyCacheCensusIsDeterministicPublicSafeAndReadOnly(t *testing.T) {
	firstRoot := buildLegacyCensusFixture(t, "first operational secret")
	secondRoot := buildLegacyCensusFixture(t, "other operational secret")
	first, err := CensusLegacyCache(firstRoot, "opencode-go-cn")
	if err != nil {
		t.Fatal(err)
	}
	second, err := CensusLegacyCache(secondRoot, "opencode-go-cn")
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest != second.Digest || first.ScientificInventoryDigest != second.ScientificInventoryDigest ||
		first.OperationalMetadataDigest != second.OperationalMetadataDigest {
		t.Fatal("same scientific bytes and operational metadata produced different census identities")
	}
	if first.TotalFiles != 4 || first.ResponseFiles != 2 || first.CapabilityFiles != 1 ||
		first.OperationalFiles != 1 || first.PublishedNamespace.Files != 1 || len(first.Providers) != 2 ||
		first.ExactAdmissibleEntries != 0 || first.LegacyOnlyEntries != 2 || !first.ReadOnly || first.ProviderCalls != 0 {
		t.Fatalf("legacy cache census = %+v", first)
	}
	for _, identity := range []string{"parser_contract_identity", "response_record_identity", "score_evidence_identity"} {
		if !slices.Contains(first.MissingIdentities, identity) {
			t.Fatalf("legacy cache census omitted missing identity %q", identity)
		}
	}
	raw, err := EncodeLegacyCacheCensus(first)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{firstRoot, "private response text", "operational secret", strings.Repeat("a", 64) + ".json"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("public census leaked %q", forbidden)
		}
	}
	decoded, err := DecodeLegacyCacheCensus(raw)
	if err != nil || decoded.Digest != first.Digest {
		t.Fatalf("census round trip = %+v, %v", decoded, err)
	}
}

func TestLegacyCacheCensusRejectsMalformedUnexpectedAndSymlinkEntries(t *testing.T) {
	tests := []struct {
		name  string
		build func(*testing.T, string)
		want  string
	}{
		{
			name: "schema", want: "not schema 1",
			build: func(t *testing.T, root string) {
				writeLegacyCensusFile(t, root, "opencode-go-cn/aa/"+strings.Repeat("a", 64)+".json", []byte(`{"schema_version":2}`))
			},
		},
		{
			name: "unexpected", want: "unexpected file",
			build: func(t *testing.T, root string) {
				writeLegacyCensusResponse(t, root, "opencode-go-cn", "aa", "safe")
				writeLegacyCensusFile(t, root, "unknown.txt", []byte("x"))
			},
		},
		{
			name: "symlink", want: "symlink",
			build: func(t *testing.T, root string) {
				writeLegacyCensusResponse(t, root, "opencode-go-cn", "aa", "safe")
				target := filepath.Join(root, "target")
				if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, filepath.Join(root, "linked")); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			test.build(t, root)
			_, err := CensusLegacyCache(root, "opencode-go-cn")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("census error = %v, want %q", err, test.want)
			}
		})
	}
}

func buildLegacyCensusFixture(t *testing.T, operational string) string {
	t.Helper()
	root := t.TempDir()
	writeLegacyCensusResponse(t, root, "opencode-go-cn", "aa", "private response text")
	writeLegacyCensusResponse(t, root, "openrouter", "bb", "second response")
	writeLegacyCensusFile(t, root, "caps/opencode-go-cn-caps.json", []byte(`{"logprobs":true}`))
	writeLegacyCensusFile(t, root, "supervisor/task-031/run.log", []byte(operational))
	return root
}

func writeLegacyCensusResponse(t *testing.T, root, provider, prefix, rawText string) {
	t.Helper()
	hash := prefix + strings.Repeat(prefix[:1], 62)
	entry, err := json.Marshal(LegacyEntry{SchemaVersion: 1, RawText: rawText, CreatedAt: 1})
	if err != nil {
		t.Fatal(err)
	}
	writeLegacyCensusFile(t, root, filepath.ToSlash(filepath.Join(provider, prefix, hash+".json")), entry)
}

func writeLegacyCensusFile(t *testing.T, root, relative string, raw []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}
