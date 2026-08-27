package claim

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestClaimSurfacesAreDeterministicLedgerDerivedViews(t *testing.T) {
	pack := buildClaimReferencePackage(t)
	ledger, err := DefaultLedger(pack.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	views, err := BuildSurfaceViews(context.Background(), pack.Registry, pack.Manifest, pack.Payloads, ledger)
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != len(SurfaceIDs()) || len(views[SurfaceDocumentation].Claims) != 34 ||
		len(views[SurfaceREADME].Claims) != 5 || len(views[SurfaceFindings].Claims) != 20 ||
		len(views[SurfaceRelease].Claims) != 4 || len(views[SurfaceResult].Claims) != 17 ||
		len(views[SurfaceSkill].Claims) != 17 {
		t.Fatalf("claim surface coverage = %+v", views)
	}
	readme := views[SurfaceREADME]
	if readme.Claims[0].ClaimID != "CLM-001" || readme.Claims[0].Status != StatusSupported ||
		readme.Claims[3].ClaimID != "CLM-031" || readme.Claims[3].Status != StatusUnsupported ||
		readme.Claims[4].ClaimID != "CLM-032" || readme.Claims[4].Status != StatusUnsupported {
		t.Fatal("readme verification card omitted supported mechanism claims or explicit non-claims")
	}

	for _, surface := range SurfaceIDs() {
		view := views[surface]
		if err := view.Validate(); err != nil {
			t.Fatalf("surface %q: %v", surface, err)
		}
		raw, err := EncodeSurfaceView(view)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeSurfaceView(raw)
		if err != nil || !reflect.DeepEqual(decoded, view) {
			t.Fatalf("surface %q canonical round trip failed: %v", surface, err)
		}
		markdown, err := RenderSurfaceMarkdown(view)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Count(markdown, []byte(SurfaceBeginMarker(surface))) != 1 ||
			bytes.Count(markdown, []byte(SurfaceEndMarker(surface))) != 1 ||
			!strings.Contains(string(markdown), view.Digest) {
			t.Fatalf("surface %q renderer omitted its closed markers or digest", surface)
		}
	}

	rebuilt, err := BuildSurfaceViews(context.Background(), pack.Registry, pack.Manifest, pack.Payloads, ledger)
	if err != nil || !reflect.DeepEqual(rebuilt, views) {
		t.Fatalf("claim surfaces are not deterministic: %v", err)
	}
	mutated := views[SurfaceResult]
	mutated.Claims = append([]SurfaceClaim(nil), mutated.Claims...)
	mutated.Claims[0].Value.Value = "999"
	if err := mutated.Validate(); err == nil {
		t.Fatal("claim surface accepted a value substitution without a matching ledger-derived digest")
	}
}
