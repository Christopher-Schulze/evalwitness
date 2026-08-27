package study

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// IdenticalResponseProtocolSchemaVersion is the frozen schema identity for the
// TASK 070 identical-response study protocol. It states the counterfactual,
// primary endpoint, aggregation, missingness, multiplicity, stopping, minimum
// support, and exact uncertainty procedure before any provider response exists.
const IdenticalResponseProtocolSchemaVersion = "evalwitness.identical-response-study-protocol.v1"

// IdenticalResponseProtocol is the frozen study protocol. It binds the four
// prior frozen artifacts by digest so any drift in the design, inventory, or
// redistribution evidence changes this protocol's own digest.
type IdenticalResponseProtocol struct {
	SchemaVersion                 string   `json:"schema_version"`
	Counterfactual                string   `json:"counterfactual"`
	PrimaryEndpoint               string   `json:"primary_endpoint"`
	TaskGroupUnit                 string   `json:"task_group_unit"`
	TaskGroupAggregation          string   `json:"task_group_aggregation"`
	IndependentOutcomeSensitivity string   `json:"independent_outcome_sensitivity"`
	MissingnessPolicy             string   `json:"missingness_policy"`
	MultiplicityFamily            []string `json:"multiplicity_family"`
	StoppingRule                  string   `json:"stopping_rule"`
	MinimumTaskGroups             int      `json:"minimum_task_groups"`
	EligibleTaskGroups            int      `json:"eligible_task_groups"`
	UncertaintyProcedure          string   `json:"uncertainty_procedure"`
	DesignSpecDigest              string   `json:"design_spec_digest"`
	DesignReportDigest            string   `json:"design_report_digest"`
	InventoryDigest               string   `json:"inventory_digest"`
	RedistributionRightDigest     string   `json:"redistribution_right_digest"`
	SupportedClaim                string   `json:"supported_claim"`
	UnsupportedClaims             []string `json:"unsupported_claims"`
	Digest                        string   `json:"digest"`
}

const (
	protocolCounterfactual                = "distribution_aware_vs_chosen_token"
	protocolPrimaryEndpoint               = "paired source-task-group disagreement: for each task group, whether the distribution-aware decision differs from the chosen-token decision derived from the same immutable completion"
	protocolTaskGroupUnit                 = "source_task_group"
	protocolTaskGroupAggregation          = "one terminal decision per extraction arm per task group; a task group is discordant when the two arms disagree, concordant otherwise; unresolved outcomes are retained separately, never pooled with agreement"
	protocolIndependentOutcomeSensitivity = "any correctness statement is analyzed separately against independent benchmark outcome evidence and reports coverage and abstention separately; descriptive rows are never promoted to confirmatory findings"
	protocolMissingnessPolicy             = "invalid, missing, abstention, and route-failure task groups remain in the denominator and are never imputed, dropped, or replaced by majority vote"
	protocolStoppingRule                  = "fixed sample; stop only after every eligible task group reaches a terminal state; no sequential efficacy stopping"
	protocolUncertaintyProcedure          = "exact conditional McNemar paired inference at the source-task-group level with the prespecified adjusted alpha and one-sided exact paired confidence bound"
	protocolSupportedClaim                = "for the frozen development population, how often and by how much the distribution-aware decision differs from the chosen-token decision derived from the same immutable completion"
)

var protocolUnsupportedClaims = []string{
	"correctness or accuracy",
	"calibration",
	"model quality",
	"provider transfer",
	"population inference beyond the frozen development inventory",
}

// BuildIdenticalResponseProtocol reproduces the frozen protocol from the four
// committed TASK 070 artifacts. It performs zero provider calls and zero agent
// launches; the binding digests are read from the committed artifacts.
func BuildIdenticalResponseProtocol(repositoryRoot string) (IdenticalResponseProtocol, error) {
	designSpecDigest, err := rawArtifactSHA256(repositoryRoot, "identical-response-design-spec-v1.json")
	if err != nil {
		return IdenticalResponseProtocol{}, err
	}
	designReportDigest, err := artifactFieldDigest(repositoryRoot, "identical-response-design-report-v1.json", "design_digest")
	if err != nil {
		return IdenticalResponseProtocol{}, err
	}
	inventoryDigest, err := artifactFieldDigest(repositoryRoot, "identical-response-eligible-inventory-v1.json", "digest")
	if err != nil {
		return IdenticalResponseProtocol{}, err
	}
	redistributionDigest, err := artifactFieldDigest(repositoryRoot, "identical-response-redistribution-right-v1.json", "digest")
	if err != nil {
		return IdenticalResponseProtocol{}, err
	}
	protocol := IdenticalResponseProtocol{
		SchemaVersion:                 IdenticalResponseProtocolSchemaVersion,
		Counterfactual:                protocolCounterfactual,
		PrimaryEndpoint:               protocolPrimaryEndpoint,
		TaskGroupUnit:                 protocolTaskGroupUnit,
		TaskGroupAggregation:          protocolTaskGroupAggregation,
		IndependentOutcomeSensitivity: protocolIndependentOutcomeSensitivity,
		MissingnessPolicy:             protocolMissingnessPolicy,
		MultiplicityFamily:            []string{"paired_task_group_disagreement"},
		StoppingRule:                  protocolStoppingRule,
		MinimumTaskGroups:             IdenticalResponseMinimumTaskGroups,
		EligibleTaskGroups:            IdenticalResponseMinimumTaskGroups,
		UncertaintyProcedure:          protocolUncertaintyProcedure,
		DesignSpecDigest:              designSpecDigest,
		DesignReportDigest:            designReportDigest,
		InventoryDigest:               inventoryDigest,
		RedistributionRightDigest:     redistributionDigest,
		SupportedClaim:                protocolSupportedClaim,
		UnsupportedClaims:             append([]string(nil), protocolUnsupportedClaims...),
	}
	inventory, err := BuildIdenticalResponseEligibleInventory(repositoryRoot)
	if err != nil {
		return IdenticalResponseProtocol{}, err
	}
	protocol.EligibleTaskGroups = inventory.EligibleTaskGroups
	protocol.Digest, err = identicalResponseProtocolDigest(protocol)
	if err != nil {
		return IdenticalResponseProtocol{}, err
	}
	if err := protocol.Validate(); err != nil {
		return IdenticalResponseProtocol{}, err
	}
	return protocol, nil
}

// Validate enforces the frozen protocol invariants.
func (protocol IdenticalResponseProtocol) Validate() error {
	if protocol.SchemaVersion != IdenticalResponseProtocolSchemaVersion || !validDigest(protocol.Digest) {
		return errors.New("identical-response protocol schema identity or digest is invalid")
	}
	if protocol.Counterfactual != protocolCounterfactual || protocol.PrimaryEndpoint != protocolPrimaryEndpoint ||
		protocol.TaskGroupUnit != protocolTaskGroupUnit || protocol.TaskGroupAggregation != protocolTaskGroupAggregation ||
		protocol.IndependentOutcomeSensitivity != protocolIndependentOutcomeSensitivity || protocol.MissingnessPolicy != protocolMissingnessPolicy ||
		protocol.StoppingRule != protocolStoppingRule || protocol.UncertaintyProcedure != protocolUncertaintyProcedure ||
		protocol.SupportedClaim != protocolSupportedClaim {
		return errors.New("identical-response protocol wording drifted")
	}
	if protocol.MinimumTaskGroups != IdenticalResponseMinimumTaskGroups || protocol.EligibleTaskGroups < protocol.MinimumTaskGroups {
		return errors.New("identical-response protocol minimum or eligible task-group count is invalid")
	}
	if !validDigest(protocol.DesignSpecDigest) || !validDigest(protocol.DesignReportDigest) || !validDigest(protocol.InventoryDigest) || !validDigest(protocol.RedistributionRightDigest) {
		return errors.New("identical-response protocol binding digests are invalid")
	}
	if !sameStringSlice(protocol.MultiplicityFamily, []string{"paired_task_group_disagreement"}) {
		return errors.New("identical-response protocol multiplicity family changed")
	}
	if !sameStringSlice(protocol.UnsupportedClaims, protocolUnsupportedClaims) {
		return errors.New("identical-response protocol unsupported claims changed")
	}
	expected, err := identicalResponseProtocolDigest(protocol)
	if err != nil {
		return err
	}
	if protocol.Digest != expected {
		return errors.New("identical-response protocol digest is invalid")
	}
	return nil
}

func rawArtifactSHA256(repositoryRoot, name string) (string, error) {
	path := filepath.Join(repositoryRoot, "eval", "governance", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func artifactFieldDigest(repositoryRoot, name, field string) (string, error) {
	path := filepath.Join(repositoryRoot, "eval", "governance", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(raw, &document); err != nil {
		return "", err
	}
	var digest string
	if err := json.Unmarshal(document[field], &digest); err != nil {
		return "", err
	}
	if !validDigest(digest) {
		return "", errors.New("bound artifact " + name + " has an invalid " + field)
	}
	return digest, nil
}

func sameStringSlice(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func identicalResponseProtocolDigest(protocol IdenticalResponseProtocol) (string, error) {
	protocol.Digest = ""
	encoded, err := json.Marshal(protocol)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
