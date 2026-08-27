package registry

import (
	"fmt"
	"os"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const SeedCatalogRelativePath = "eval/governance/registry-seed-catalog-v1.json"

func LoadSeedCatalog(path string) ([]IntakeEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("seed catalog: %w", err)
	}
	var catalog []IntakeEntry
	if err := protocol.DecodeStrict(raw, &catalog); err != nil {
		return nil, fmt.Errorf("seed catalog: %w", err)
	}
	if len(catalog) < 2 {
		return nil, fmt.Errorf("seed catalog: need at least two contrasting development entries")
	}
	contracts := map[string]bool{}
	for _, entry := range catalog {
		if err := ValidateIntake(entry); err != nil {
			return nil, fmt.Errorf("seed catalog %s: %w", entry.EntryID, err)
		}
		if entry.CommunityValidated {
			return nil, fmt.Errorf("seed catalog %s: community_validated must stay false", entry.EntryID)
		}
		contracts[entry.RequestContract+":"+entry.EndpointKind] = true
	}
	if len(contracts) < 2 {
		return nil, fmt.Errorf("seed catalog: entries must use contrasting request contracts")
	}
	return catalog, nil
}
