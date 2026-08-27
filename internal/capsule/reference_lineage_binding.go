package capsule

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Christopher-Schulze/evalwitness/internal/lineage"
)

const lineageBindingValidatorID = "evalwitness.validator.lineage-bindings.v1"

type lineageArtifactEnvelope struct {
	Header lineage.ArtifactHeader `json:"header"`
}

type lineagePlanEnvelope struct {
	SchemaVersion string `json:"schema_version"`
	Identity      struct {
		PlanID string `json:"plan_id"`
		TaskID string `json:"task_id"`
	} `json:"identity"`
	Digest string `json:"digest"`
}

type lineageBindingIdentity struct {
	SchemaVersion string
	ObjectID      string
	TaskID        string
	TaskGroupID   string
	Digest        string
}

func lineageBindingValidators() map[string]BindingValidator {
	return map[string]BindingValidator{lineageBindingValidatorID: validateLineageBindings}
}

func validateLineageBindings(context BindingContext) error {
	if context.Component.TypeID == lineage.PlanSchemaVersion {
		if len(context.Parents) != 0 {
			return errors.New("lineage plan must not have capsule parents")
		}
		return nil
	}
	var child lineageArtifactEnvelope
	if err := json.Unmarshal(context.Payload, &child); err != nil {
		return fmt.Errorf("decode lineage child header: %w", err)
	}
	if len(child.Header.Parents) != len(context.Parents) {
		return errors.New("lineage payload and capsule graph have different parent counts")
	}
	matched := make(map[int]struct{}, len(context.Parents))
	for _, parent := range context.Parents {
		if parent.Reference.Resolution != ParentInternal || len(parent.Payload) == 0 {
			return errors.New("lineage reference components require internal payload-bearing parents")
		}
		identity, err := decodeLineageBindingIdentity(parent.Record.TypeID, parent.Payload)
		if err != nil {
			return err
		}
		found := -1
		for index, reference := range child.Header.Parents {
			if _, used := matched[index]; used {
				continue
			}
			if reference.SchemaVersion == identity.SchemaVersion && reference.ObjectID == identity.ObjectID {
				found = index
				if reference.Digest != identity.Digest || reference.TaskID != identity.TaskID ||
					identity.TaskGroupID != "" && reference.TaskGroupID != identity.TaskGroupID {
					return fmt.Errorf("lineage parent %q payload identity differs from its capsule parent", reference.ObjectID)
				}
				break
			}
		}
		if found < 0 {
			return fmt.Errorf("lineage capsule parent %q is absent from the child payload", identity.ObjectID)
		}
		matched[found] = struct{}{}
	}
	return nil
}

func decodeLineageBindingIdentity(typeID string, payload []byte) (lineageBindingIdentity, error) {
	if typeID == lineage.PlanSchemaVersion {
		var plan lineagePlanEnvelope
		if err := json.Unmarshal(payload, &plan); err != nil {
			return lineageBindingIdentity{}, fmt.Errorf("decode lineage plan identity: %w", err)
		}
		return lineageBindingIdentity{
			SchemaVersion: plan.SchemaVersion, ObjectID: plan.Identity.PlanID, TaskID: plan.Identity.TaskID, Digest: plan.Digest,
		}, nil
	}
	var artifact lineageArtifactEnvelope
	if err := json.Unmarshal(payload, &artifact); err != nil {
		return lineageBindingIdentity{}, fmt.Errorf("decode lineage artifact identity: %w", err)
	}
	return lineageBindingIdentity{
		SchemaVersion: artifact.Header.SchemaVersion, ObjectID: artifact.Header.ObjectID,
		TaskID: artifact.Header.TaskID, TaskGroupID: artifact.Header.TaskGroupID, Digest: artifact.Header.Digest,
	}, nil
}
