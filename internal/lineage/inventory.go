package lineage

import "errors"

const SchemaInventoryVersion = "evalwitness.verification-lineage-schema-inventory.v1"

type SchemaInventoryEntry struct {
	DocumentType       string              `json:"document_type"`
	SchemaVersion      string              `json:"schema_version"`
	SchemaID           string              `json:"schema_id"`
	SchemaDigest       string              `json:"schema_digest"`
	ParentRequirements []ParentRequirement `json:"parent_requirements"`
}

type SchemaInventory struct {
	SchemaVersion   string                 `json:"schema_version"`
	CanonicalPolicy string                 `json:"canonical_policy"`
	ProtocolVersion string                 `json:"protocol_version"`
	PlanDigest      string                 `json:"plan_digest"`
	Documents       []SchemaInventoryEntry `json:"documents"`
	Digest          string                 `json:"digest"`
}

func DefaultSchemaInventory() (SchemaInventory, error) {
	inventory := SchemaInventory{
		SchemaVersion: SchemaInventoryVersion, CanonicalPolicy: CanonicalPolicy, ProtocolVersion: ProtocolVersion,
		PlanDigest: LockedPlanDigest,
	}
	for _, spec := range documentSpecs() {
		schema, err := Schema(spec.Name)
		if err != nil {
			return SchemaInventory{}, err
		}
		schemaDigest, err := digestJSON(schema)
		if err != nil {
			return SchemaInventory{}, err
		}
		inventory.Documents = append(inventory.Documents, SchemaInventoryEntry{
			DocumentType: spec.Name, SchemaVersion: spec.SchemaVersion,
			SchemaID: schemaID(spec.SchemaVersion), SchemaDigest: schemaDigest,
			ParentRequirements: spec.Parents,
		})
	}
	digest, err := inventoryDigest(inventory)
	if err != nil {
		return SchemaInventory{}, err
	}
	inventory.Digest = digest
	return inventory, inventory.Validate()
}

func (inventory SchemaInventory) Validate() error {
	if inventory.SchemaVersion != SchemaInventoryVersion || inventory.CanonicalPolicy != CanonicalPolicy ||
		inventory.ProtocolVersion != ProtocolVersion || inventory.PlanDigest != LockedPlanDigest || len(inventory.Documents) != 10 || !validDigest(inventory.Digest) {
		return errors.New("verification-lineage schema inventory identity is invalid")
	}
	specs := documentSpecs()
	for index, entry := range inventory.Documents {
		spec := specs[index]
		schema, err := Schema(spec.Name)
		if err != nil {
			return err
		}
		schemaDigest, err := digestJSON(schema)
		if err != nil {
			return err
		}
		if entry.DocumentType != spec.Name || entry.SchemaVersion != spec.SchemaVersion ||
			entry.SchemaID != schemaID(spec.SchemaVersion) || entry.SchemaDigest != schemaDigest ||
			!parentRequirementsEqual(entry.ParentRequirements, spec.Parents) {
			return errors.New("verification-lineage schema inventory differs from the closed ten-schema DAG")
		}
	}
	if err := validateInventoryDAG(inventory.Documents); err != nil {
		return err
	}
	expected, err := inventoryDigest(inventory)
	if err != nil {
		return err
	}
	if inventory.Digest != expected {
		return errors.New("verification-lineage schema inventory digest is invalid")
	}
	return nil
}

func validateInventoryDAG(entries []SchemaInventoryEntry) error {
	parentsBySchema := make(map[string][]string, len(entries))
	for _, entry := range entries {
		if _, duplicate := parentsBySchema[entry.SchemaVersion]; duplicate {
			return errors.New("schema inventory contains a duplicate schema version")
		}
		parentsBySchema[entry.SchemaVersion] = nil
	}
	for _, entry := range entries {
		for _, requirement := range entry.ParentRequirements {
			for _, parentSchema := range requirement.SchemaVersions {
				if _, known := parentsBySchema[parentSchema]; !known {
					return errors.New("schema inventory references an unknown parent schema")
				}
				parentsBySchema[entry.SchemaVersion] = append(parentsBySchema[entry.SchemaVersion], parentSchema)
			}
		}
	}
	visiting := make(map[string]bool, len(entries))
	visited := make(map[string]bool, len(entries))
	var visit func(string) error
	visit = func(schema string) error {
		if visiting[schema] {
			return errors.New("schema inventory parent graph contains a cycle")
		}
		if visited[schema] {
			return nil
		}
		visiting[schema] = true
		for _, parent := range parentsBySchema[schema] {
			if err := visit(parent); err != nil {
				return err
			}
		}
		visiting[schema] = false
		visited[schema] = true
		return nil
	}
	for schema := range parentsBySchema {
		if err := visit(schema); err != nil {
			return err
		}
	}
	return nil
}

func parentRequirementsEqual(left, right []ParentRequirement) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Relation != right[index].Relation || left[index].Minimum != right[index].Minimum ||
			left[index].Maximum != right[index].Maximum || left[index].SameTask != right[index].SameTask ||
			left[index].SameTaskGroup != right[index].SameTaskGroup || len(left[index].SchemaVersions) != len(right[index].SchemaVersions) {
			return false
		}
		for schemaIndex := range left[index].SchemaVersions {
			if left[index].SchemaVersions[schemaIndex] != right[index].SchemaVersions[schemaIndex] {
				return false
			}
		}
	}
	return true
}

func inventoryDigest(inventory SchemaInventory) (string, error) {
	inventory.Digest = ""
	return digestJSON(inventory)
}
