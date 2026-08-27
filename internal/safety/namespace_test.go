package safety

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestRouteNamespaceUsesOnlyAHashedPathComponent(t *testing.T) {
	provider := `../provider\\with:separators`
	model := `../../vendor/model:v1 beta`
	namespace, err := NewRouteNamespace(provider, model)
	if err != nil {
		t.Fatal(err)
	}
	if !IsSafeNamespaceID(namespace.ID) {
		t.Fatalf("namespace ID is not safe: %q", namespace.ID)
	}
	for _, raw := range []string{provider, model, "provider", "vendor", "..", "/", `\\`} {
		if strings.Contains(namespace.ID, raw) {
			t.Fatalf("namespace ID %q contains raw identifier fragment %q", namespace.ID, raw)
		}
	}
	if got := namespace.Directory(); got != filepath.Join("routes", namespace.ID) {
		t.Fatalf("directory = %q", got)
	}
	if got := namespace.IdentityPath(); got != filepath.Join("routes", namespace.ID, RouteIdentityFileName) {
		t.Fatalf("identity path = %q", got)
	}
}

func TestRouteNamespaceIsDeterministicAndFieldBounded(t *testing.T) {
	first, err := NewRouteNamespace("ab", "c")
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewRouteNamespace("ab", "c")
	if err != nil {
		t.Fatal(err)
	}
	shifted, err := NewRouteNamespace("a", "bc")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatal("same route produced different namespace IDs")
	}
	if first.ID == shifted.ID {
		t.Fatal("provider/model boundary shift produced the same namespace ID")
	}
	if first.ID == routeNamespaceID("ab\x00c", "") {
		t.Fatal("length-prefix boundary is ineffective")
	}
}

func TestRouteNamespaceMetadataIsInspectableAndSelfAuthenticating(t *testing.T) {
	namespace, err := NewRouteNamespace("company-gateway", "vendor/model")
	if err != nil {
		t.Fatal(err)
	}
	metadata := namespace.Metadata()
	if !metadata.Valid() {
		t.Fatalf("metadata is invalid: %+v", metadata)
	}
	if metadata.Provider != namespace.Provider || metadata.Model != namespace.Model {
		t.Fatalf("metadata lost route identity: %+v", metadata)
	}
	metadata.Model = "different-model"
	if metadata.Valid() {
		t.Fatal("tampered metadata remained valid")
	}
}

func TestRouteNamespaceRejectsMissingOversizedAndInvalidUTF8Identifiers(t *testing.T) {
	invalidUTF8 := string([]byte{utf8.RuneSelf, 0xff})
	tests := []struct {
		provider string
		model    string
	}{
		{"", "model"},
		{"provider", ""},
		{strings.Repeat("p", MaxExternalIdentifierBytes+1), "model"},
		{"provider", strings.Repeat("m", MaxExternalIdentifierBytes+1)},
		{invalidUTF8, "model"},
		{"provider", invalidUTF8},
	}
	for _, test := range tests {
		if _, err := NewRouteNamespace(test.provider, test.model); !IsKind(err, ErrorInvalidInput) {
			t.Fatalf("identifiers %q/%q error = %T %v", test.provider, test.model, err, err)
		}
	}
}

func TestIsSafeNamespaceIDRejectsLookalikes(t *testing.T) {
	for _, identifier := range []string{
		"",
		"route-short",
		"provider-" + strings.Repeat("a", 64),
		RouteNamespacePrefix + strings.Repeat("z", 64),
		RouteNamespacePrefix + strings.Repeat("a", 63) + "/",
	} {
		if IsSafeNamespaceID(identifier) {
			t.Fatalf("unsafe namespace ID passed: %q", identifier)
		}
	}
}

func TestCacheRootPersistsAndVerifiesInspectableRouteIdentity(t *testing.T) {
	policy, home, _, _ := testPathPolicy(t)
	root, err := CreateCacheRoot(policy, filepath.Join(home, ".cache", ProductID))
	if err != nil {
		t.Fatal(err)
	}
	namespace, err := NewRouteNamespace("company-gateway", "vendor/model")
	if err != nil {
		t.Fatal(err)
	}
	if err := root.EnsureRouteIdentity(namespace); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root.Path(), namespace.IdentityPath()))
	if err != nil {
		t.Fatal(err)
	}
	var metadata RouteNamespaceMetadata
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.Provider != namespace.Provider || metadata.Model != namespace.Model || !metadata.Valid() {
		t.Fatalf("stored metadata = %+v", metadata)
	}
	if err := root.EnsureRouteIdentity(namespace); err != nil {
		t.Fatalf("idempotent identity check: %v", err)
	}
}

func TestCacheRootRejectsConflictingRouteIdentityMetadata(t *testing.T) {
	policy, home, _, _ := testPathPolicy(t)
	root, err := CreateCacheRoot(policy, filepath.Join(home, ".cache", ProductID))
	if err != nil {
		t.Fatal(err)
	}
	namespace, err := NewRouteNamespace("provider", "model")
	if err != nil {
		t.Fatal(err)
	}
	conflict := namespace.Metadata()
	conflict.Model = "other"
	raw, err := json.Marshal(conflict)
	if err != nil {
		t.Fatal(err)
	}
	if err := root.PublishSensitive(namespace.IdentityPath(), raw); err != nil {
		t.Fatal(err)
	}
	if err := root.EnsureRouteIdentity(namespace); !IsKind(err, ErrorNameCollision) {
		t.Fatalf("error = %T %v, want name collision", err, err)
	}
}
