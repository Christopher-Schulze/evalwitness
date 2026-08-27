package registry

import (
	"github.com/Christopher-Schulze/evalwitness/internal/config"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const ProviderInventorySchemaVersion = "evalwitness.registry-provider-inventory.v1"

type ProviderInventory struct {
	SchemaVersion           string                 `json:"schema_version"`
	Presets                 []config.PresetSummary `json:"presets"`
	CommunityValidated      bool                   `json:"community_validated"`
	IndependentlyReproduced bool                   `json:"independently_reproduced"`
	Limitations             []string               `json:"limitations"`
	Digest                  string                 `json:"digest"`
}

func ProviderEvidenceInventory() (ProviderInventory, error) {
	inventory := ProviderInventory{
		SchemaVersion:           ProviderInventorySchemaVersion,
		Presets:                 config.PresetSummaries(),
		CommunityValidated:      false,
		IndependentlyReproduced: false,
		Limitations: []string{
			"configured local presets only; capability_state=configured is not an attestation",
			"no live provider call, no key values, no community ranking",
			"provider terms and redistribution remain owner decisions",
		},
	}
	digest, err := protocol.Digest(unsignedProviderInventory(inventory))
	if err != nil {
		return ProviderInventory{}, err
	}
	inventory.Digest = digest
	return inventory, nil
}

func unsignedProviderInventory(inventory ProviderInventory) ProviderInventory {
	inventory.Digest = ""
	return inventory
}
