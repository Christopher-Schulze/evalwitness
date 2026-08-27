package capsule

import (
	"context"
	"strings"
	"testing"
)

func TestPackageFamilyResolvesExactExternalParents(t *testing.T) {
	base := buildReferencePackage(t)
	child, err := BuildExtensionConformancePackage(base)
	if err != nil {
		t.Fatal(err)
	}
	basePackage := Package(base)
	childPackage := Package(child)
	if _, err := VerifyPackageFamily(
		context.Background(), childPackage, []Package{basePackage},
		VerificationOptions{MaximumVisibility: VisibilityPublic},
	); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyPackageFamily(context.Background(), childPackage, nil, VerificationOptions{}); err == nil {
		t.Fatal("capsule family accepted an unavailable declared parent")
	}

	record := child.Manifest.Components[0]
	parents := append([]ParentRef(nil), record.Parents...)
	parents[0].ComponentID = strings.Repeat("f", 64)
	mutated, normalized, err := BuildComponent(child.Registry, ComponentInput{
		Name: record.Name, TypeID: record.TypeID, Visibility: record.Visibility,
		Payload: child.Payloads[record.Payload.Digest], Parents: parents,
	})
	if err != nil {
		t.Fatal(err)
	}
	components := append([]ComponentRecord(nil), child.Manifest.Components...)
	components[0] = mutated
	roots := append([]string(nil), child.Manifest.ScientificRoots...)
	for index := range roots {
		if roots[index] == record.ComponentID {
			roots[index] = mutated.ComponentID
		}
	}
	manifest, err := BuildManifest(child.Registry, ManifestInput{
		StudyID: child.Manifest.StudyID, CellID: "unresolved-external-parent",
		ParentCapsules: child.Manifest.ParentCapsules, ScientificRoots: roots,
		PresentationRoots: child.Manifest.PresentationRoots, Components: components,
	})
	if err != nil {
		t.Fatal(err)
	}
	payloads := make(map[string][]byte, len(child.Payloads)+1)
	for digest, raw := range child.Payloads {
		payloads[digest] = raw
	}
	payloads[mutated.Payload.Digest] = normalized
	if _, err := VerifyPackageFamily(
		context.Background(), Package{Registry: child.Registry, Manifest: manifest, Payloads: payloads},
		[]Package{basePackage}, VerificationOptions{},
	); err == nil {
		t.Fatal("capsule family accepted an unresolved external component substitution")
	}
}
