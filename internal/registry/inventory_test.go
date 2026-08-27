package registry

import "testing"

func TestProviderEvidenceInventoryIsConfiguredOnly(t *testing.T) {
	inventory, err := ProviderEvidenceInventory()
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Presets) == 0 || inventory.CommunityValidated || inventory.IndependentlyReproduced {
		t.Fatalf("inventory = %+v", inventory)
	}
	for _, preset := range inventory.Presets {
		if preset.CapabilityState != "configured" {
			t.Fatalf("preset %s capability = %q", preset.Name, preset.CapabilityState)
		}
	}
}

func TestRenderPublicDerivativeOmitsPrivateSurfaces(t *testing.T) {
	derivative, err := RenderPublicDerivative(validEntry("deriv"))
	if err != nil {
		t.Fatal(err)
	}
	if derivative.EvidenceCeiling != PublicDerivativeCeiling || derivative.CommunityValidated {
		t.Fatalf("derivative = %+v", derivative)
	}
	if len(derivative.Omitted) == 0 || derivative.CapsuleDigest == "" {
		t.Fatalf("derivative = %+v", derivative)
	}
}
