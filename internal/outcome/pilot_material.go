package outcome

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

const (
	OutcomePilotSourceBindingSchemaVersion    = "evalwitness.outcome-pilot-source-binding.v2"
	OutcomePilotPrivateMaterialsSchemaVersion = "evalwitness.outcome-pilot-private-materials.v1"
	OutcomePilotEvidenceSelectorVersion       = "evalwitness.outcome-evidence-selector.v2"
	OutcomePilotEvidenceBudgetTokens          = 16_000
)

type OutcomePilotSourceBinding struct {
	SchemaVersion          string          `json:"schema_version"`
	CanonicalPolicy        string          `json:"canonical_policy"`
	PilotSampleDigest      string          `json:"pilot_sample_digest"`
	NaturalInventoryDigest string          `json:"natural_inventory_digest"`
	Objective              ReviewObjective `json:"objective"`
	Stratum                string          `json:"stratum"`
	Suite                  string          `json:"suite"`
	TaskID                 string          `json:"task_id"`
	SelectedIndex          int             `json:"selected_index"`
	SelectedReward         int             `json:"selected_reward"`
	SourceLocation         string          `json:"source_location"`
	SourceRevision         string          `json:"source_revision"`
	SourceDigest           string          `json:"source_digest"`
	TrajectoryDigest       string          `json:"trajectory_digest"`
	RetainedDigest         string          `json:"retained_digest"`
	SourceEvents           int             `json:"source_events"`
	RetainedEvents         int             `json:"retained_events"`
	RedactionHits          int             `json:"redaction_hits"`
	EvidenceBudgetTokens   int             `json:"evidence_budget_tokens"`
	EvidenceSelector       string          `json:"evidence_selector"`
	DecisionAnchorKind     string          `json:"decision_anchor_kind"`
	DecisionAnchorDigest   string          `json:"decision_anchor_digest"`
	RetainedMessages       int             `json:"retained_messages"`
	RetainedActions        int             `json:"retained_actions"`
	RetainedResults        int             `json:"retained_results"`
	LicenseSPDX            string          `json:"license_spdx"`
	SourceURL              string          `json:"source_url"`
	Redistribution         string          `json:"redistribution"`
	PacketID               string          `json:"packet_id"`
	MappingDigest          string          `json:"mapping_digest"`
	Digest                 string          `json:"digest"`
}

type OutcomePilotMaterials struct {
	Items    []ReviewItem
	Mappings []PrivateMapping
	Bindings []OutcomePilotSourceBinding
}

type OutcomePilotPrivateMaterials struct {
	SchemaVersion        string                      `json:"schema_version"`
	CanonicalPolicy      string                      `json:"canonical_policy"`
	PilotSampleDigest    string                      `json:"pilot_sample_digest"`
	Mappings             []PrivateMapping            `json:"mappings"`
	SourceBindings       []OutcomePilotSourceBinding `json:"source_bindings"`
	MappingsDigest       string                      `json:"mappings_digest"`
	SourceBindingsDigest string                      `json:"source_bindings_digest"`
	Digest               string                      `json:"digest"`
}

type outcomePilotLicense struct {
	SPDX           string
	SourceURL      string
	SourceRevision string
	Redistribution string
}

type outcomePilotRawSource struct {
	Raw            []byte
	Location       string
	RouteIdentity  []string
	License        outcomePilotLicense
	SelectedReward int
}

type outcomePilotSWEItem struct {
	InstanceID   string  `json:"instance_id"`
	ModelName    string  `json:"model_name"`
	Reward       float64 `json:"reward"`
	TrajectoryID string  `json:"trajectory_id"`
}

var (
	outcomePilotSourceReferencePattern = regexp.MustCompile(`\bsource=([^\s|]+)`)
	outcomePilotCallReferencePattern   = regexp.MustCompile(`\bcall_id=([^\s\]]+)`)
)

func BuildOutcomePilotMaterials(root string, plan Plan, request NaturalInventoryRequest, inventory NaturalInventory, pilot OutcomePilotSampleCommitment, blindingKeyID string, key []byte, evidenceBudgetTokens int) (OutcomePilotMaterials, error) {
	if err := plan.Validate(); err != nil {
		return OutcomePilotMaterials{}, err
	}
	if err := request.Validate(); err != nil {
		return OutcomePilotMaterials{}, err
	}
	if err := inventory.Validate(); err != nil {
		return OutcomePilotMaterials{}, err
	}
	if err := pilot.Validate(); err != nil {
		return OutcomePilotMaterials{}, err
	}
	if pilot.PlanDigest != plan.Digest || inventory.PlanDigest != plan.Digest || pilot.NaturalInventoryDigest != inventory.Digest || inventory.RequestDigest != request.Digest {
		return OutcomePilotMaterials{}, errors.New("outcome pilot material inputs do not bind one plan, request, inventory, and sample")
	}
	if evidenceBudgetTokens != OutcomePilotEvidenceBudgetTokens || len(key) != 32 || missing(blindingKeyID) {
		return OutcomePilotMaterials{}, errors.New("outcome pilot materialization requires the frozen 16000-token budget, a 32-byte key, and a key ID")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return OutcomePilotMaterials{}, err
	}
	candidates, err := resolveOutcomePilotCandidates(root, request, inventory)
	if err != nil {
		return OutcomePilotMaterials{}, err
	}
	materials := OutcomePilotMaterials{
		Items: make([]ReviewItem, 0, len(pilot.Cases)), Mappings: make([]PrivateMapping, 0, len(pilot.Cases)), Bindings: make([]OutcomePilotSourceBinding, 0, len(pilot.Cases)),
	}
	for _, pilotCase := range pilot.Cases {
		candidate, exists := candidates[pilotCase.Stratum+"\x00"+pilotCase.SourceCaseDigest]
		if !exists {
			return OutcomePilotMaterials{}, fmt.Errorf("outcome pilot case %s/%s is absent from the reproduced natural candidates", pilotCase.Stratum, pilotCase.SourceCaseDigest)
		}
		rawSource, err := loadOutcomePilotRawSource(root, candidate)
		if err != nil {
			return OutcomePilotMaterials{}, fmt.Errorf("materialize natural case %s: %w", pilotCase.Stratum, err)
		}
		trajectory, err := preprocess.IngestReader(bytes.NewReader(rawSource.Raw), preprocess.FrozenCanonicalizationV1IngestOptions())
		if err != nil {
			return OutcomePilotMaterials{}, fmt.Errorf("ingest natural case %s: %w", pilotCase.Stratum, err)
		}
		decisionAnchor, err := outcomePilotDecisionAnchor(trajectory)
		if err != nil {
			return OutcomePilotMaterials{}, fmt.Errorf("select natural case %s decision anchor: %w", pilotCase.Stratum, err)
		}
		retained, err := preprocess.ApplyEvidenceBudgetWithRequiredEvents(trajectory, evidenceBudgetTokens, []string{decisionAnchor.ID})
		if err != nil {
			return OutcomePilotMaterials{}, fmt.Errorf("slice natural case %s: %w", pilotCase.Stratum, err)
		}
		messages, actions, results := outcomePilotEvidenceCoverage(retained)
		if actions < 1 || results < 1 {
			return OutcomePilotMaterials{}, fmt.Errorf("natural case %s evidence lacks action or result coverage", pilotCase.Stratum)
		}
		taskRequirement, err := outcomePilotTaskRequirement(trajectory)
		if err != nil {
			return OutcomePilotMaterials{}, fmt.Errorf("extract natural case %s requirement: %w", pilotCase.Stratum, err)
		}
		privateValues := append([]string{candidate.TaskID}, rawSource.RouteIdentity...)
		taskContent := redactOutcomePilotIdentities(taskRequirement, privateValues)
		trajectoryContent := pseudonymizeOutcomePilotRouteTokens(redactOutcomePilotIdentities(preprocess.RenderTrajectory(retained), privateValues))
		license := rawSource.License.SPDX + "; " + rawSource.License.Redistribution + "; source revision " + rawSource.License.SourceRevision
		buildRequest := BlindBuildRequest{
			SchemaVersion: BlindBuildSchemaVersion, PlanDigest: plan.Digest, TaskAlias: candidate.TaskID,
			Evidence: []PacketEvidence{
				{Slot: "task-requirement", Kind: "task_requirement", Content: taskContent, ContentDigest: digestText(taskContent), License: license, Limitation: "Task text is identity-redacted; source location and raw artifact remain owner-only."},
				{Slot: "trajectory-evidence", Kind: "trajectory_evidence", Content: trajectoryContent, ContentDigest: digestText(trajectoryContent), License: license, Limitation: outcomePilotTrajectoryLimitation(trajectory, retained)},
			},
			RubricQuestions: outcomePilotRubricQuestions(), PrivacyClass: "restricted_reference_only", PublicReleasable: false,
			SourceCaseDigest: pilotCase.SourceCaseDigest, Condition: "natural." + pilotCase.Stratum, ExpectedRelation: OutcomePilotExpectedRelation,
			SlotMappings: []SlotMapping{
				{Slot: "task-requirement", SourceDigest: digestText(taskRequirement)},
				{Slot: "trajectory-evidence", SourceDigest: trajectory.Digest},
			},
			BlindingKeyID:   blindingKeyID,
			ForbiddenValues: sortedUniqueStrings(append(privateValues, "natural."+pilotCase.Stratum, OutcomePilotExpectedRelation, blindingKeyID, pilotCase.SourceCaseDigest)),
		}
		packet, mapping, err := BuildBlindedPacketFromRequest(buildRequest, key)
		if err != nil {
			return OutcomePilotMaterials{}, fmt.Errorf("blind natural case %s: %w", pilotCase.Stratum, err)
		}
		binding, err := SealOutcomePilotSourceBinding(OutcomePilotSourceBinding{
			PilotSampleDigest: pilot.Digest, NaturalInventoryDigest: inventory.Digest, Objective: ReviewObjectiveOutcome, Stratum: pilotCase.Stratum,
			Suite: candidate.Suite, TaskID: candidate.TaskID, SelectedIndex: selectedIndex(candidate.VerifierData), SelectedReward: rawSource.SelectedReward,
			SourceLocation: rawSource.Location, SourceRevision: rawSource.License.SourceRevision, SourceDigest: trajectory.SourceDigest,
			TrajectoryDigest: trajectory.Digest, RetainedDigest: retained.Digest, SourceEvents: len(trajectory.Events), RetainedEvents: len(retained.Events),
			RedactionHits: trajectory.Report.RedactionHits, EvidenceBudgetTokens: evidenceBudgetTokens, EvidenceSelector: OutcomePilotEvidenceSelectorVersion,
			DecisionAnchorKind: string(decisionAnchor.Kind), DecisionAnchorDigest: decisionAnchor.ContentDigest,
			RetainedMessages: messages, RetainedActions: actions, RetainedResults: results, LicenseSPDX: rawSource.License.SPDX,
			SourceURL: rawSource.License.SourceURL, Redistribution: rawSource.License.Redistribution, PacketID: packet.PacketID, MappingDigest: mapping.Digest,
		})
		if err != nil {
			return OutcomePilotMaterials{}, err
		}
		materials.Items = append(materials.Items, ReviewItem{TaskGroupID: pilotCase.TaskGroupID, Packet: packet})
		materials.Mappings = append(materials.Mappings, mapping)
		materials.Bindings = append(materials.Bindings, binding)
	}
	sort.Slice(materials.Items, func(left, right int) bool {
		return materials.Items[left].Packet.PacketID < materials.Items[right].Packet.PacketID
	})
	sort.Slice(materials.Mappings, func(left, right int) bool {
		return materials.Mappings[left].PacketID < materials.Mappings[right].PacketID
	})
	sort.Slice(materials.Bindings, func(left, right int) bool {
		return materials.Bindings[left].PacketID < materials.Bindings[right].PacketID
	})
	return materials, nil
}

func SealOutcomePilotSourceBinding(binding OutcomePilotSourceBinding) (OutcomePilotSourceBinding, error) {
	binding.SchemaVersion, binding.CanonicalPolicy, binding.Digest = OutcomePilotSourceBindingSchemaVersion, CanonicalPolicy, ""
	digest, err := outcomePilotSourceBindingDigest(binding)
	if err != nil {
		return OutcomePilotSourceBinding{}, err
	}
	binding.Digest = digest
	return binding, binding.Validate()
}

func (binding OutcomePilotSourceBinding) Validate() error {
	if binding.SchemaVersion != OutcomePilotSourceBindingSchemaVersion || binding.CanonicalPolicy != CanonicalPolicy || binding.Objective != ReviewObjectiveOutcome ||
		!validDigest(binding.PilotSampleDigest) || !validDigest(binding.NaturalInventoryDigest) || missing(binding.Stratum, binding.Suite, binding.TaskID, binding.SourceLocation, binding.SourceRevision) ||
		binding.SelectedIndex < 0 || binding.SelectedReward < 0 || binding.SelectedReward > 1 || !validDigest(binding.SourceDigest) || !validDigest(binding.TrajectoryDigest) ||
		!validDigest(binding.RetainedDigest) || binding.SourceEvents < 1 || binding.RetainedEvents < 1 || binding.RetainedEvents > binding.SourceEvents || binding.RedactionHits < 0 ||
		binding.EvidenceBudgetTokens != OutcomePilotEvidenceBudgetTokens || binding.EvidenceSelector != OutcomePilotEvidenceSelectorVersion ||
		!slices.Contains([]string{string(preprocess.EventFileChange), string(preprocess.EventMessage)}, binding.DecisionAnchorKind) || !validDigest(binding.DecisionAnchorDigest) ||
		binding.RetainedMessages < 0 || binding.RetainedActions < 1 || binding.RetainedResults < 1 || missing(binding.LicenseSPDX, binding.SourceURL) || binding.Redistribution != "reference_only" ||
		!validOpaquePacketID(binding.PacketID) || !validDigest(binding.MappingDigest) {
		return errors.New("outcome pilot source binding identity, provenance, selection, retention, license, or packet binding is invalid")
	}
	expected, err := outcomePilotSourceBindingDigest(binding)
	if err != nil || binding.Digest != expected {
		return errors.New("outcome pilot source binding digest is invalid")
	}
	return nil
}

func DecodeOutcomePilotSourceBindings(reader io.Reader) ([]OutcomePilotSourceBinding, error) {
	var values []OutcomePilotSourceBinding
	if err := decodeStrict(reader, &values); err != nil {
		return nil, fmt.Errorf("decode outcome pilot source bindings: %w", err)
	}
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return nil, fmt.Errorf("outcome pilot source binding %d: %w", index, err)
		}
	}
	return values, nil
}

func DecodeOutcomePilotSourceBinding(reader io.Reader) (OutcomePilotSourceBinding, error) {
	var value OutcomePilotSourceBinding
	if err := decodeStrict(reader, &value); err != nil {
		return OutcomePilotSourceBinding{}, fmt.Errorf("decode outcome pilot source binding: %w", err)
	}
	return value, value.Validate()
}

func SealOutcomePilotPrivateMaterials(pilotSampleDigest string, mappings []PrivateMapping, bindings []OutcomePilotSourceBinding) (OutcomePilotPrivateMaterials, error) {
	materials := OutcomePilotPrivateMaterials{
		SchemaVersion: OutcomePilotPrivateMaterialsSchemaVersion, CanonicalPolicy: CanonicalPolicy, PilotSampleDigest: pilotSampleDigest,
		Mappings: append([]PrivateMapping(nil), mappings...), SourceBindings: append([]OutcomePilotSourceBinding(nil), bindings...),
	}
	sort.Slice(materials.Mappings, func(left, right int) bool {
		return materials.Mappings[left].PacketID < materials.Mappings[right].PacketID
	})
	sort.Slice(materials.SourceBindings, func(left, right int) bool {
		return materials.SourceBindings[left].PacketID < materials.SourceBindings[right].PacketID
	})
	var err error
	materials.MappingsDigest, err = digestJSON(materials.Mappings)
	if err != nil {
		return OutcomePilotPrivateMaterials{}, err
	}
	materials.SourceBindingsDigest, err = digestJSON(materials.SourceBindings)
	if err != nil {
		return OutcomePilotPrivateMaterials{}, err
	}
	materials.Digest, err = outcomePilotPrivateMaterialsDigest(materials)
	if err != nil {
		return OutcomePilotPrivateMaterials{}, err
	}
	return materials, materials.Validate()
}

func (materials OutcomePilotPrivateMaterials) Validate() error {
	if materials.SchemaVersion != OutcomePilotPrivateMaterialsSchemaVersion || materials.CanonicalPolicy != CanonicalPolicy ||
		!validDigest(materials.PilotSampleDigest) || len(materials.Mappings) < 1 || len(materials.Mappings) != len(materials.SourceBindings) ||
		!validDigest(materials.MappingsDigest) || !validDigest(materials.SourceBindingsDigest) || !validDigest(materials.Digest) {
		return errors.New("outcome pilot private materials identity or coverage is invalid")
	}
	for index := range materials.Mappings {
		mapping, binding := materials.Mappings[index], materials.SourceBindings[index]
		if err := mapping.Validate(); err != nil {
			return err
		}
		if err := binding.Validate(); err != nil {
			return err
		}
		if mapping.PacketID != binding.PacketID || mapping.Digest != binding.MappingDigest || binding.PilotSampleDigest != materials.PilotSampleDigest ||
			index > 0 && materials.Mappings[index-1].PacketID >= mapping.PacketID ||
			index > 0 && materials.SourceBindings[index-1].PacketID >= binding.PacketID {
			return errors.New("outcome pilot private mappings and source bindings must align one-to-one in canonical packet order")
		}
	}
	expectedMappings, err := digestJSON(materials.Mappings)
	if err != nil || materials.MappingsDigest != expectedMappings {
		return errors.New("outcome pilot private mapping digest is invalid")
	}
	expectedBindings, err := digestJSON(materials.SourceBindings)
	if err != nil || materials.SourceBindingsDigest != expectedBindings {
		return errors.New("outcome pilot private source-binding digest is invalid")
	}
	expected, err := outcomePilotPrivateMaterialsDigest(materials)
	if err != nil || materials.Digest != expected {
		return errors.New("outcome pilot private materials digest is invalid")
	}
	return nil
}

func DecodeOutcomePilotPrivateMaterials(reader io.Reader) (OutcomePilotPrivateMaterials, error) {
	var value OutcomePilotPrivateMaterials
	if err := decodeStrict(reader, &value); err != nil {
		return OutcomePilotPrivateMaterials{}, fmt.Errorf("decode outcome pilot private materials: %w", err)
	}
	return value, value.Validate()
}

func resolveOutcomePilotCandidates(root string, request NaturalInventoryRequest, inventory NaturalInventory) (map[string]naturalCandidate, error) {
	artifacts := make(map[string]NaturalSourceArtifact, len(inventory.SourceArtifacts))
	for _, artifact := range inventory.SourceArtifacts {
		artifacts[artifact.Suite+"\x00"+artifact.Role] = artifact
	}
	candidates := make(map[string][]naturalCandidate)
	for _, pair := range request.SourcePairs {
		verifier, verifierArtifact, err := readLegacyResult(pair.Suite, "verifier", filepath.Join(root, filepath.FromSlash(pair.VerifierPath)))
		if err != nil {
			return nil, err
		}
		judge, judgeArtifact, err := readLegacyResult(pair.Suite, "judge", filepath.Join(root, filepath.FromSlash(pair.JudgePath)))
		if err != nil {
			return nil, err
		}
		for _, observed := range []NaturalSourceArtifact{verifierArtifact, judgeArtifact} {
			expected, exists := artifacts[observed.Suite+"\x00"+observed.Role]
			if !exists || expected.Digest != observed.Digest || expected.TaskCount != observed.TaskCount {
				return nil, fmt.Errorf("natural %s/%s source no longer reproduces the governed inventory", observed.Suite, observed.Role)
			}
		}
		if err := addNaturalCandidates(candidates, pair.Suite, verifier, judge, request.HighConfidenceErrorThreshold); err != nil {
			return nil, err
		}
	}
	resolved := make(map[string]naturalCandidate)
	for stratum, values := range candidates {
		for _, candidate := range values {
			resolved[stratum+"\x00"+candidate.CaseDigest] = candidate
		}
	}
	return resolved, nil
}

func loadOutcomePilotRawSource(root string, candidate naturalCandidate) (outcomePilotRawSource, error) {
	selected := selectedIndex(candidate.VerifierData)
	expectedReward := candidate.VerifierData.Rewards[selected]
	switch candidate.Suite {
	case "terminal-bench":
		pattern := filepath.Join(root, "eval", "trajectories", "terminal_trajs", "forge_gpt54", candidate.TaskID, "*_trajectory.json")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return outcomePilotRawSource{}, err
		}
		sort.Strings(matches)
		if len(matches) != len(candidate.VerifierData.Rewards) || selected >= len(matches) {
			return outcomePilotRawSource{}, errors.New("Terminal-Bench raw trajectory order does not reproduce the governed reward vector")
		}
		raw, err := os.ReadFile(matches[selected])
		if err != nil {
			return outcomePilotRawSource{}, err
		}
		var metadata struct {
			TaskName  string `json:"task_name"`
			TrialName string `json:"trial_name"`
			TrialID   string `json:"trial_id"`
			Reward    int    `json:"reward"`
		}
		if err := json.Unmarshal(raw, &metadata); err != nil {
			return outcomePilotRawSource{}, err
		}
		if metadata.TaskName != candidate.TaskID || metadata.Reward != expectedReward || missing(metadata.TrialName, metadata.TrialID) {
			return outcomePilotRawSource{}, errors.New("Terminal-Bench raw trajectory identity or reward disagrees with the governed selection")
		}
		relative, err := filepath.Rel(root, matches[selected])
		if err != nil {
			return outcomePilotRawSource{}, err
		}
		return outcomePilotRawSource{
			Raw: raw, Location: filepath.ToSlash(relative), RouteIdentity: []string{
				metadata.TrialName, metadata.TrialID, "forge_gpt54", "Forge agent (Claude Opus 4.6)", "Claude Opus 4.6", "Forge agent", "terminal_bench.trajectory", "ATIF-v1.5",
			}, SelectedReward: metadata.Reward,
			License: outcomePilotLicense{SPDX: "Apache-2.0", SourceURL: "https://huggingface.co/datasets/harborframework/terminal-bench-2-leaderboard/tree/11e0eb7f6b1cca7b4aee5f3ef39ede09c5d99f60", SourceRevision: "11e0eb7f6b1cca7b4aee5f3ef39ede09c5d99f60", Redistribution: "reference_only"},
		}, nil
	case "swebench":
		runsRoot := filepath.Join(root, "eval", "trajectories", "swebench_verified_trajs")
		entries, err := os.ReadDir(runsRoot)
		if err != nil {
			return outcomePilotRawSource{}, err
		}
		var runs []string
		for _, entry := range entries {
			if entry.IsDir() {
				runs = append(runs, entry.Name())
			}
		}
		sort.Strings(runs)
		if len(runs) != len(candidate.VerifierData.Rewards) || selected >= len(runs) {
			return outcomePilotRawSource{}, errors.New("SWE-bench run order does not reproduce the governed reward vector")
		}
		raw, item, location, err := readOutcomePilotSWEItem(root, filepath.Join(runsRoot, runs[selected]), candidate.TaskID)
		if err != nil {
			return outcomePilotRawSource{}, err
		}
		if item.Reward != float64(expectedReward) || item.InstanceID != candidate.TaskID || missing(item.TrajectoryID, item.ModelName) {
			return outcomePilotRawSource{}, errors.New("SWE-bench raw trajectory identity or reward disagrees with the governed selection")
		}
		return outcomePilotRawSource{
			Raw: raw, Location: location, RouteIdentity: []string{runs[selected], item.ModelName, item.TrajectoryID}, SelectedReward: expectedReward,
			License: outcomePilotLicense{SPDX: "MIT", SourceURL: "https://github.com/SWE-bench/SWE-bench", SourceRevision: "evalwitness-release-data-v0.2.0", Redistribution: "reference_only"},
		}, nil
	default:
		return outcomePilotRawSource{}, fmt.Errorf("unsupported natural suite %q", candidate.Suite)
	}
}

func readOutcomePilotSWEItem(root, runDir, taskID string) ([]byte, outcomePilotSWEItem, string, error) {
	single := filepath.Join(runDir, "data_cache.json")
	files := []string{single}
	if _, err := os.Stat(single); err != nil {
		matches, globErr := filepath.Glob(filepath.Join(runDir, "data_cache*.json"))
		if globErr != nil {
			return nil, outcomePilotSWEItem{}, "", globErr
		}
		files = matches
	}
	sort.Strings(files)
	for _, path := range files {
		file, err := os.Open(path)
		if err != nil {
			return nil, outcomePilotSWEItem{}, "", err
		}
		decoder := json.NewDecoder(file)
		start, err := decoder.Token()
		if err != nil || start != json.Delim('[') {
			_ = file.Close()
			return nil, outcomePilotSWEItem{}, "", fmt.Errorf("SWE-bench cache %q is not a JSON array", path)
		}
		itemIndex := 0
		for decoder.More() {
			var raw json.RawMessage
			if err := decoder.Decode(&raw); err != nil {
				_ = file.Close()
				return nil, outcomePilotSWEItem{}, "", err
			}
			if len(raw) > maximumLegacyResultBytes {
				_ = file.Close()
				return nil, outcomePilotSWEItem{}, "", errors.New("SWE-bench pilot source item exceeds 64 MiB")
			}
			var item outcomePilotSWEItem
			if err := json.Unmarshal(raw, &item); err != nil {
				_ = file.Close()
				return nil, outcomePilotSWEItem{}, "", err
			}
			if item.InstanceID == taskID {
				if err := file.Close(); err != nil {
					return nil, outcomePilotSWEItem{}, "", err
				}
				relative, err := filepath.Rel(root, path)
				if err != nil {
					return nil, outcomePilotSWEItem{}, "", err
				}
				return append([]byte(nil), raw...), item, filepath.ToSlash(relative) + "#/" + strconv.Itoa(itemIndex), nil
			}
			itemIndex++
		}
		if _, err := decoder.Token(); err != nil {
			_ = file.Close()
			return nil, outcomePilotSWEItem{}, "", err
		}
		if err := file.Close(); err != nil {
			return nil, outcomePilotSWEItem{}, "", err
		}
	}
	return nil, outcomePilotSWEItem{}, "", fmt.Errorf("SWE-bench task %q is absent from selected run", taskID)
}

func outcomePilotTaskRequirement(trajectory preprocess.Trajectory) (string, error) {
	for _, event := range trajectory.Events {
		if event.Message == nil || !strings.EqualFold(event.Message.Role, "user") {
			continue
		}
		var parts []string
		for _, part := range event.Message.Parts {
			if part.Kind == preprocess.ContentText && strings.TrimSpace(part.Text) != "" {
				parts = append(parts, strings.TrimSpace(part.Text))
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n\n"), nil
		}
	}
	return "", errors.New("canonical trajectory has no nonempty user task requirement")
}

func outcomePilotDecisionAnchor(trajectory preprocess.Trajectory) (preprocess.Event, error) {
	for index := len(trajectory.Events) - 1; index >= 0; index-- {
		event := trajectory.Events[index]
		if event.FileChange != nil && strings.TrimSpace(event.FileChange.Diff) != "" {
			return event, nil
		}
	}
	for index := len(trajectory.Events) - 1; index >= 0; index-- {
		event := trajectory.Events[index]
		if event.Message == nil || !strings.EqualFold(event.Message.Role, "assistant") {
			continue
		}
		for _, part := range event.Message.Parts {
			if part.Kind == preprocess.ContentText && strings.TrimSpace(part.Text) != "" {
				return event, nil
			}
		}
	}
	return preprocess.Event{}, errors.New("canonical trajectory has no nonempty final patch or assistant narrative")
}

func outcomePilotEvidenceCoverage(trajectory preprocess.Trajectory) (int, int, int) {
	var messages, actions, results int
	for _, event := range trajectory.Events {
		switch event.Kind {
		case preprocess.EventMessage:
			messages++
		case preprocess.EventToolCall, preprocess.EventCommand, preprocess.EventFileChange:
			actions++
		case preprocess.EventToolResult, preprocess.EventOutput, preprocess.EventError:
			results++
		}
	}
	return messages, actions, results
}

func redactOutcomePilotIdentities(value string, privateValues []string) string {
	for _, private := range sortedUniqueStrings(privateValues) {
		private = strings.TrimSpace(private)
		if private == "" {
			continue
		}
		for {
			index := strings.Index(strings.ToLower(value), strings.ToLower(private))
			if index < 0 {
				break
			}
			value = value[:index] + "[ROUTE_ID_REDACTED]" + value[index+len(private):]
		}
	}
	return value
}

func pseudonymizeOutcomePilotRouteTokens(value string) string {
	value = outcomePilotSourceReferencePattern.ReplaceAllStringFunc(value, func(match string) string {
		raw := strings.TrimPrefix(match, "source=")
		return "source=source-ref-" + digestText(raw)[:16]
	})
	return outcomePilotCallReferencePattern.ReplaceAllStringFunc(value, func(match string) string {
		raw := strings.TrimPrefix(match, "call_id=")
		return "call_id=call-ref-" + digestText(raw)[:16]
	})
}

func outcomePilotRubricQuestions() []string {
	return []string{
		"Does available evidence support a decisive outcome?",
		"Does implementation introduce task-defeating harmful side effects?",
		"Does trajectory evidence support task satisfaction?",
		"Is implementation technically correct?",
		"Is verification sufficient for the selected outcome?",
	}
}

func outcomePilotTrajectoryLimitation(source, retained preprocess.Trajectory) string {
	return fmt.Sprintf("Deterministic secret redaction and a %d-token anchored whole-event evidence budget retained %d of %d canonical events with a final-patch or assistant-narrative decision anchor plus action and result coverage; omitted evidence may change the judgment.", OutcomePilotEvidenceBudgetTokens, len(retained.Events), len(source.Events))
}

func sortedUniqueStrings(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			set[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func outcomePilotSourceBindingDigest(value OutcomePilotSourceBinding) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}

func outcomePilotPrivateMaterialsDigest(value OutcomePilotPrivateMaterials) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
