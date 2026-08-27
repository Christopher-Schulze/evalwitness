package agentstudy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/study"
)

const (
	SchemaVersion      = "evalwitness.agent-only-study.v1"
	CanonicalPolicy    = "evalwitness.agent-only-canonical-json.v1"
	SelectionAlgorithm = "family-quota-hash-order.v1"
	PrimaryAVersion    = "formal-release-validator-a.v1"
	PrimaryBVersion    = "formal-release-validator-b.v1"
	TieBreakVersion    = "formal-release-tie-break.v1"
	LabelAccepted      = "accepted"
	LabelRejected      = "rejected"
	LabelUnresolved    = "unresolved"
	ResolutionAgreed   = "primary_agreement"
	ResolutionTie      = "automated_tie_break"
	ResolutionOpen     = "unresolved_conflict"
)

type BuildInputs struct {
	PlanDigest       string
	AuditDigest      string
	Release          mutation.CorpusReleaseV3
	CalibrationCases int
	TestCases        int
	Seed             string
}

type MachineCheck struct {
	Name     string `json:"name"`
	Expected string `json:"expected"`
	Observed string `json:"observed"`
	Passed   bool   `json:"passed"`
}

type ValidatorResult struct {
	ValidatorID    string         `json:"validator_id"`
	Version        string         `json:"version"`
	Label          string         `json:"label"`
	Checks         []MachineCheck `json:"checks"`
	EvidenceDigest string         `json:"evidence_digest"`
}

type SourceAudit struct {
	SourceIDs                    []string       `json:"source_ids"`
	Sources                      []SourceRecord `json:"sources"`
	ManifestSourceDigest         string         `json:"manifest_source_digest"`
	ManifestTrajectoryDigest     string         `json:"manifest_trajectory_digest"`
	ManifestOutcomeWitnessDigest string         `json:"manifest_outcome_witness_digest"`
	SourceDigests                []string       `json:"source_digests"`
	TrajectoryDigests            []string       `json:"trajectory_digests"`
	OutcomeWitnessDigests        []string       `json:"outcome_witness_digests"`
	SourceSetDigest              string         `json:"source_set_digest"`
}

type SourceRecord struct {
	ID                   string `json:"id"`
	TaskID               string `json:"task_id"`
	RepositoryID         string `json:"repository_id"`
	SourceFamily         string `json:"source_family"`
	SourceFormat         string `json:"source_format"`
	SplitGroupID         string `json:"split_group_id"`
	Split                string `json:"split"`
	SourceDigest         string `json:"source_digest"`
	TrajectoryDigest     string `json:"trajectory_digest"`
	OutcomeWitnessDigest string `json:"outcome_witness_digest"`
}

type CaseResult struct {
	CaseID            string            `json:"case_id"`
	Family            mutation.Family   `json:"family"`
	Split             study.DataRole    `json:"split"`
	ExpectedRelation  mutation.Relation `json:"expected_relation"`
	ManifestDigest    string            `json:"manifest_digest"`
	FirewallDigest    string            `json:"firewall_digest"`
	BlindPacketDigest string            `json:"blind_packet_digest"`
	SourceAudit       SourceAudit       `json:"source_audit"`
	PrimaryA          ValidatorResult   `json:"primary_a"`
	PrimaryB          ValidatorResult   `json:"primary_b"`
	TieBreak          *ValidatorResult  `json:"tie_break,omitempty"`
	PrimaryAgreement  bool              `json:"primary_agreement"`
	Resolution        string            `json:"resolution"`
	FinalLabel        string            `json:"final_label"`
}

type Selection struct {
	Algorithm         string `json:"algorithm"`
	Seed              string `json:"seed"`
	CalibrationTarget int    `json:"calibration_target"`
	TestTarget        int    `json:"test_target"`
	CalibrationChosen int    `json:"calibration_chosen"`
	TestChosen        int    `json:"test_chosen"`
	CalibrationGroups int    `json:"calibration_unique_split_groups"`
	TestGroups        int    `json:"test_unique_split_groups"`
	CrossSplitGroups  int    `json:"cross_split_group_overlap"`
}

type Counts struct {
	Cases             int `json:"cases"`
	Calibration       int `json:"calibration"`
	Test              int `json:"test"`
	Accepted          int `json:"accepted"`
	Rejected          int `json:"rejected"`
	Unresolved        int `json:"unresolved"`
	PrimaryAgreements int `json:"primary_agreements"`
	TieBreaks         int `json:"tie_breaks"`
}

type Study struct {
	SchemaVersion   string       `json:"schema_version"`
	CanonicalPolicy string       `json:"canonical_policy"`
	StudyID         string       `json:"study_id"`
	Title           string       `json:"title"`
	AgentOnly       bool         `json:"agent_only"`
	PlanDigest      string       `json:"plan_digest"`
	AuditDigest     string       `json:"audit_digest"`
	ReleaseDigest   string       `json:"release_digest"`
	Selection       Selection    `json:"selection"`
	PrimaryRoles    []string     `json:"primary_roles"`
	TieBreakRole    string       `json:"tie_break_role"`
	Cases           []CaseResult `json:"cases"`
	Counts          Counts       `json:"counts"`
	ProviderCalls   int          `json:"provider_calls"`
	HumanReviewers  int          `json:"human_reviewers"`
	ClaimBoundary   []string     `json:"claim_boundary"`
	Digest          string       `json:"digest"`
}

func Build(inputs BuildInputs) (Study, error) {
	if err := validateBuildInputs(inputs); err != nil {
		return Study{}, err
	}
	selected, err := selectCases(inputs.Release.Cases, inputs.Seed, inputs.CalibrationCases, inputs.TestCases)
	if err != nil {
		return Study{}, err
	}
	sources := sourceIndex(inputs.Release.Sources)
	results := make([]CaseResult, 0, len(selected))
	for _, item := range selected {
		audit, auditErr := buildSourceAudit(item, sources)
		if auditErr != nil {
			return Study{}, fmt.Errorf("case %s source audit: %w", item.ID, auditErr)
		}
		primaryA := runPrimaryA(item, sources)
		primaryB := runPrimaryB(item, sources)
		result := resolveCase(item, audit, primaryA, primaryB)
		results = append(results, result)
	}
	selection := selectionRecord(selected, inputs.Seed, inputs.CalibrationCases, inputs.TestCases)
	result := Study{
		SchemaVersion: SchemaVersion, CanonicalPolicy: CanonicalPolicy,
		StudyID: "agent-only-controlled-relation-v1", Title: "Coding-agent-only formal controlled-relation audit",
		AgentOnly: true, PlanDigest: inputs.PlanDigest, AuditDigest: inputs.AuditDigest,
		ReleaseDigest: inputs.Release.Digest, Selection: selection,
		PrimaryRoles: []string{"primary_a/formal-release-validator-a", "primary_b/formal-release-validator-b"},
		TieBreakRole: "tie_break/formal-release-tie-break", Cases: results,
		ProviderCalls: 0, HumanReviewers: 0,
		ClaimBoundary: []string{
			"supported: reproducible formal relation validity on the selected frozen v3 release cases",
			"supported: independent machine validators and disagreement-only automated tie-break execution",
			"unsupported: human semantic adjudication, provider quality, transfer, prevalence, or universal generalization",
			"unsupported: claims beyond the frozen controlled-corruption v3 release and its declared source lineage",
		},
	}
	result.Counts = countResults(results)
	sealed, err := Seal(result)
	if err != nil {
		return Study{}, err
	}
	if err := sealed.ValidateAgainstRelease(inputs.Release); err != nil {
		return Study{}, err
	}
	return sealed, nil
}

func validateBuildInputs(inputs BuildInputs) error {
	if !validDigest(inputs.PlanDigest) || !validDigest(inputs.AuditDigest) || inputs.Release.Digest == "" || !validDigest(inputs.Release.Digest) {
		return errors.New("agent-only study requires valid plan, audit, and release digests")
	}
	if strings.TrimSpace(inputs.Seed) == "" || inputs.CalibrationCases <= 0 || inputs.TestCases <= 0 {
		return errors.New("agent-only study requires a seed and positive calibration/test targets")
	}
	return nil
}

func selectCases(cases []mutation.CorpusCaseV3, seed string, calibrationTarget, testTarget int) ([]mutation.CorpusCaseV3, error) {
	byRole := map[study.DataRole][]mutation.CorpusCaseV3{study.RoleCalibration: {}, study.RoleTest: {}}
	for _, item := range cases {
		if item.Family == mutation.FamilyTestEvidenceOmitted {
			continue
		}
		if _, ok := byRole[item.Split]; ok {
			byRole[item.Split] = append(byRole[item.Split], item)
		}
	}
	calibration, err := selectRole(byRole[study.RoleCalibration], seed, calibrationTarget)
	if err != nil {
		return nil, fmt.Errorf("calibration selection: %w", err)
	}
	test, err := selectRole(byRole[study.RoleTest], seed, testTarget)
	if err != nil {
		return nil, fmt.Errorf("test selection: %w", err)
	}
	selected := append(calibration, test...)
	sort.Slice(selected, func(i, j int) bool { return selected[i].ID < selected[j].ID })
	return selected, nil
}

func selectRole(cases []mutation.CorpusCaseV3, seed string, target int) ([]mutation.CorpusCaseV3, error) {
	if len(cases) < target {
		return nil, fmt.Errorf("only %d eligible cases available, need %d", len(cases), target)
	}
	byFamily := make(map[mutation.Family][]mutation.CorpusCaseV3)
	for _, item := range cases {
		byFamily[item.Family] = append(byFamily[item.Family], item)
	}
	families := make([]mutation.Family, 0, len(byFamily))
	for family := range byFamily {
		families = append(families, family)
	}
	sort.Slice(families, func(i, j int) bool { return families[i] < families[j] })
	if len(families) == 0 {
		return nil, errors.New("no eligible relation families")
	}
	base, remainder := target/len(families), target%len(families)
	selected := make([]mutation.CorpusCaseV3, 0, target)
	for index, family := range families {
		items := byFamily[family]
		sort.Slice(items, func(i, j int) bool { return selectionKey(seed, items[i].ID) < selectionKey(seed, items[j].ID) })
		quota := base
		if index < remainder {
			quota++
		}
		if len(items) < quota {
			return nil, fmt.Errorf("family %q has %d eligible cases, need %d", family, len(items), quota)
		}
		selected = append(selected, items[:quota]...)
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].ID < selected[j].ID })
	return selected, nil
}

func selectionKey(seed, caseID string) string {
	digest := sha256.Sum256([]byte(seed + "\x00" + caseID))
	return hex.EncodeToString(digest[:]) + "\x00" + caseID
}

func sourceIndex(sources []mutation.CorpusSource) map[string]mutation.CorpusSource {
	index := make(map[string]mutation.CorpusSource, len(sources))
	for _, source := range sources {
		index[source.ID] = source
	}
	return index
}

func buildSourceAudit(item mutation.CorpusCaseV3, sources map[string]mutation.CorpusSource) (SourceAudit, error) {
	if len(item.SourceIDs) == 0 {
		return SourceAudit{}, errors.New("case has no source IDs")
	}
	audit := SourceAudit{
		SourceIDs:                    append([]string(nil), item.SourceIDs...),
		ManifestSourceDigest:         item.Manifest.Source.SourceDigest,
		ManifestTrajectoryDigest:     item.Manifest.Source.TrajectoryDigest,
		ManifestOutcomeWitnessDigest: item.Manifest.Source.Outcome.WitnessDigest,
	}
	for _, id := range item.SourceIDs {
		source, ok := sources[id]
		if !ok {
			return SourceAudit{}, fmt.Errorf("source %q is missing", id)
		}
		audit.Sources = append(audit.Sources, SourceRecord{
			ID: source.ID, TaskID: source.TaskID, RepositoryID: source.RepositoryID,
			SourceFamily: source.SourceFamily, SourceFormat: string(source.SourceFormat),
			SplitGroupID: source.SplitGroupID, Split: string(source.Split),
			SourceDigest: source.SourceDigest, TrajectoryDigest: source.TrajectoryDigest,
			OutcomeWitnessDigest: source.Outcome.WitnessDigest,
		})
		audit.SourceDigests = append(audit.SourceDigests, source.SourceDigest)
		audit.TrajectoryDigests = append(audit.TrajectoryDigests, source.TrajectoryDigest)
		audit.OutcomeWitnessDigests = append(audit.OutcomeWitnessDigests, source.Outcome.WitnessDigest)
	}
	audit.SourceSetDigest = digestValue(struct {
		IDs, Sources, Trajectories, Outcomes []string
	}{audit.SourceIDs, audit.SourceDigests, audit.TrajectoryDigests, audit.OutcomeWitnessDigests})
	return audit, nil
}

func runPrimaryA(item mutation.CorpusCaseV3, sources map[string]mutation.CorpusSource) ValidatorResult {
	checks := []MachineCheck{
		checkError("manifest_validate", item.Manifest.Validate()),
		checkError("blind_packet_validate", item.BlindPacket.Validate()),
		checkError("construct_firewall_validate", item.ConstructFirewall.Validate()),
		checkBool("case_identity", item.ID == item.Manifest.MutationID, item.ID, item.Manifest.MutationID),
		checkBool("relation_binding", item.Manifest.ExpectedRelation == item.Manifest.Witness.Relation, string(item.Manifest.ExpectedRelation), string(item.Manifest.Witness.Relation)),
		checkBool("witness_proven", item.Manifest.Witness.LabelState == mutation.LabelProven, string(mutation.LabelProven), string(item.Manifest.Witness.LabelState)),
		checkBool("witness_checks_pass", checksPass(item.Manifest.Witness.Checks), "all_passed", "failed_check"),
		checkBool("packet_binding", item.BlindPacket.Digest == item.Manifest.Review.BlindPacketDigest && item.BlindPacket.OriginalDigest == item.Manifest.OriginalTrajectoryDigest && item.BlindPacket.MutatedDigest == item.Manifest.MutatedTrajectoryDigest, "linked", "unlinked"),
		checkBool("firewall_binding", item.Manifest.ConstructFirewallDigest == item.ConstructFirewall.Digest, item.Manifest.ConstructFirewallDigest, item.ConstructFirewall.Digest),
		checkSourceBindingA(item, sources),
		checkOutcomeBindingA(item, sources),
	}
	return sealValidatorResult("primary_a", PrimaryAVersion, checks)
}

func runPrimaryB(item mutation.CorpusCaseV3, sources map[string]mutation.CorpusSource) ValidatorResult {
	manifest := item.Manifest
	witness := manifest.Witness
	packet := item.BlindPacket
	firewall := item.ConstructFirewall
	checks := []MachineCheck{
		checkBool("manifest_digest_recomputed", digestManifest(manifest) == manifest.Digest, digestManifest(manifest), manifest.Digest),
		checkBool("witness_digest_recomputed", digestWitness(witness) == witness.Digest, digestWitness(witness), witness.Digest),
		checkBool("packet_digest_recomputed", digestPacket(packet) == packet.Digest, digestPacket(packet), packet.Digest),
		checkBool("firewall_digest_recomputed", digestFirewall(firewall) == firewall.Digest, digestFirewall(firewall), firewall.Digest),
		checkBool("manifest_packet_link", packet.Digest == manifest.Review.BlindPacketDigest && packet.OriginalDigest == manifest.OriginalTrajectoryDigest && packet.MutatedDigest == manifest.MutatedTrajectoryDigest, "linked", "unlinked"),
		checkBool("manifest_firewall_link", firewall.Digest == manifest.ConstructFirewallDigest && firewall.Family == item.Family && firewall.Status == mutation.ConstructApplied, "linked", "unlinked"),
		checkBool("case_manifest_identity", item.ID == manifest.MutationID && manifest.Program.Family == item.Family, "case_bound", "case_unbound"),
		checkBool("expected_relation_registered", manifest.ExpectedRelation == item.Manifest.Witness.Relation && manifest.Witness.LabelState == mutation.LabelProven, "registered_proven", "unproven"),
		checkBool("all_witness_checks_pass", checksPass(manifest.Witness.Checks), "all_passed", "failed_check"),
		checkSourceBindingB(item, sources),
		checkOutcomeBindingB(item, sources),
	}
	return sealValidatorResult("primary_b", PrimaryBVersion, checks)
}

func checkSourceBindingA(item mutation.CorpusCaseV3, sources map[string]mutation.CorpusSource) MachineCheck {
	if len(item.SourceIDs) == 1 {
		source, ok := sources[item.SourceIDs[0]]
		return checkBool("source_binding", ok && source.Split == item.Split && source.SplitGroupID == item.Manifest.SplitGroupID && source.SourceDigest == item.Manifest.Source.SourceDigest && source.TrajectoryDigest == item.Manifest.Source.TrajectoryDigest, "source_bound", "source_unbound")
	}
	if len(item.SourceIDs) != 2 {
		return checkBool("source_binding", false, "pair", fmt.Sprintf("%d sources", len(item.SourceIDs)))
	}
	first, firstOK := sources[item.SourceIDs[0]]
	second, secondOK := sources[item.SourceIDs[1]]
	bound := firstOK && secondOK && first.Split == item.Split && second.Split == item.Split && first.SplitGroupID == item.Manifest.SplitGroupID && second.SplitGroupID == item.Manifest.SplitGroupID && len(item.Manifest.Source.PairedTrajectoryDigests) == 2 && item.Manifest.Source.PairedTrajectoryDigests[0] == first.TrajectoryDigest && item.Manifest.Source.PairedTrajectoryDigests[1] == second.TrajectoryDigest
	return checkBool("source_binding", bound, "ordered_pair_bound", "ordered_pair_unbound")
}

func checkSourceBindingB(item mutation.CorpusCaseV3, sources map[string]mutation.CorpusSource) MachineCheck {
	if len(item.SourceIDs) == 1 {
		source, ok := sources[item.SourceIDs[0]]
		if !ok {
			return checkBool("source_digest_binding", false, "source", "missing")
		}
		observed := source.ID + ":" + source.SourceDigest + ":" + source.TrajectoryDigest
		expected := item.SourceIDs[0] + ":" + item.Manifest.Source.SourceDigest + ":" + item.Manifest.Source.TrajectoryDigest
		return checkBool("source_digest_binding", source.SourceDigest == item.Manifest.Source.SourceDigest && source.TrajectoryDigest == item.Manifest.Source.TrajectoryDigest, expected, observed)
	}
	if len(item.SourceIDs) != 2 {
		return checkBool("source_digest_binding", false, "pair", fmt.Sprintf("%d sources", len(item.SourceIDs)))
	}
	first, firstOK := sources[item.SourceIDs[0]]
	second, secondOK := sources[item.SourceIDs[1]]
	observed := first.TrajectoryDigest + "," + second.TrajectoryDigest
	expected := strings.Join(item.Manifest.Source.PairedTrajectoryDigests, ",")
	return checkBool("source_digest_binding", firstOK && secondOK && observed == expected, expected, observed)
}

func checkOutcomeBindingA(item mutation.CorpusCaseV3, sources map[string]mutation.CorpusSource) MachineCheck {
	if len(item.SourceIDs) == 1 {
		source, ok := sources[item.SourceIDs[0]]
		return checkBool("outcome_witness_binding", ok && source.Outcome.WitnessDigest == item.Manifest.Source.Outcome.WitnessDigest, "witness_bound", "witness_unbound")
	}
	return checkBool("outcome_witness_binding", validDigest(item.Manifest.Source.Outcome.WitnessDigest), "witness_digest", item.Manifest.Source.Outcome.WitnessDigest)
}

func checkOutcomeBindingB(item mutation.CorpusCaseV3, sources map[string]mutation.CorpusSource) MachineCheck {
	if len(item.SourceIDs) == 1 {
		source, ok := sources[item.SourceIDs[0]]
		if !ok {
			return checkBool("outcome_witness_binding", false, "source", "missing")
		}
		return checkBool("outcome_witness_binding", source.Outcome.WitnessDigest == item.Manifest.Source.Outcome.WitnessDigest && validDigest(source.Outcome.WitnessDigest), source.Outcome.WitnessDigest, item.Manifest.Source.Outcome.WitnessDigest)
	}
	valid := validDigest(item.Manifest.Source.Outcome.WitnessDigest)
	for _, id := range item.SourceIDs {
		if source, ok := sources[id]; !ok || !validDigest(source.Outcome.WitnessDigest) {
			valid = false
		}
	}
	return checkBool("outcome_witness_binding", valid, "witness_digests", item.Manifest.Source.Outcome.WitnessDigest)
}

func resolveCase(item mutation.CorpusCaseV3, audit SourceAudit, primaryA, primaryB ValidatorResult) CaseResult {
	result := CaseResult{CaseID: item.ID, Family: item.Family, Split: item.Split, ExpectedRelation: item.Manifest.ExpectedRelation, ManifestDigest: item.Manifest.Digest, FirewallDigest: item.ConstructFirewall.Digest, BlindPacketDigest: item.BlindPacket.Digest, SourceAudit: audit, PrimaryA: primaryA, PrimaryB: primaryB}
	result.PrimaryAgreement = primaryA.Label == primaryB.Label
	if result.PrimaryAgreement {
		result.Resolution, result.FinalLabel = ResolutionAgreed, primaryA.Label
		return result
	}
	tie := runTieBreak(item, audit)
	result.TieBreak, result.Resolution, result.FinalLabel = &tie, ResolutionTie, tie.Label
	if tie.Label == LabelUnresolved {
		result.Resolution = ResolutionOpen
	}
	return result
}

func runTieBreak(item mutation.CorpusCaseV3, audit SourceAudit) ValidatorResult {
	checks := []MachineCheck{
		checkBool("release_case_digest_present", validDigest(item.Manifest.Digest), "digest", item.Manifest.Digest),
		checkBool("source_audit_digest_present", validDigest(audit.SourceSetDigest), "digest", audit.SourceSetDigest),
		checkBool("formal_witness_proven", item.Manifest.Witness.LabelState == mutation.LabelProven && checksPass(item.Manifest.Witness.Checks), "proven", string(item.Manifest.Witness.LabelState)),
		checkBool("firewall_applied", item.ConstructFirewall.Status == mutation.ConstructApplied, string(mutation.ConstructApplied), string(item.ConstructFirewall.Status)),
	}
	return sealValidatorResult("tie_break", TieBreakVersion, checks)
}

func sealValidatorResult(id, version string, checks []MachineCheck) ValidatorResult {
	result := ValidatorResult{ValidatorID: id, Version: version, Label: labelForChecks(checks), Checks: checks}
	result.EvidenceDigest = digestValue(struct {
		ValidatorID string         `json:"validator_id"`
		Version     string         `json:"version"`
		Label       string         `json:"label"`
		Checks      []MachineCheck `json:"checks"`
	}{result.ValidatorID, result.Version, result.Label, result.Checks})
	return result
}

func labelForChecks(checks []MachineCheck) string {
	for _, check := range checks {
		if !check.Passed {
			return LabelRejected
		}
	}
	return LabelAccepted
}

func checksPass(checks []mutation.Check) bool {
	if len(checks) == 0 {
		return false
	}
	for _, check := range checks {
		if !check.Passed {
			return false
		}
	}
	return true
}

func checkError(name string, err error) MachineCheck {
	if err == nil {
		return checkBool(name, true, "valid", "valid")
	}
	return checkBool(name, false, "valid", err.Error())
}

func checkBool(name string, passed bool, expected, observed string) MachineCheck {
	return MachineCheck{Name: name, Expected: expected, Observed: observed, Passed: passed}
}

func digestManifest(value mutation.Manifest) string {
	value.Digest, value.MutationID = "", ""
	return digestValue(value)
}

func digestWitness(value mutation.Witness) string {
	value.Digest = ""
	return digestValue(value)
}

func digestPacket(value mutation.BlindReviewPacket) string {
	value.PacketID, value.Digest = "", ""
	return digestValue(value)
}

func digestFirewall(value mutation.ConstructFirewallReportV2) string {
	value.Digest = ""
	return digestValue(value)
}

func selectionRecord(cases []mutation.CorpusCaseV3, seed string, calibrationTarget, testTarget int) Selection {
	groups := map[study.DataRole]map[string]struct{}{study.RoleCalibration: {}, study.RoleTest: {}}
	for _, item := range cases {
		groups[item.Split][item.Manifest.SplitGroupID] = struct{}{}
	}
	overlap := 0
	for group := range groups[study.RoleCalibration] {
		if _, ok := groups[study.RoleTest][group]; ok {
			overlap++
		}
	}
	return Selection{Algorithm: SelectionAlgorithm, Seed: seed, CalibrationTarget: calibrationTarget, TestTarget: testTarget, CalibrationChosen: countRole(cases, study.RoleCalibration), TestChosen: countRole(cases, study.RoleTest), CalibrationGroups: len(groups[study.RoleCalibration]), TestGroups: len(groups[study.RoleTest]), CrossSplitGroups: overlap}
}

func countRole(cases []mutation.CorpusCaseV3, role study.DataRole) int {
	count := 0
	for _, item := range cases {
		if item.Split == role {
			count++
		}
	}
	return count
}

func countResults(results []CaseResult) Counts {
	counts := Counts{Cases: len(results)}
	for _, result := range results {
		switch result.Split {
		case study.RoleCalibration:
			counts.Calibration++
		case study.RoleTest:
			counts.Test++
		}
		switch result.FinalLabel {
		case LabelAccepted:
			counts.Accepted++
		case LabelRejected:
			counts.Rejected++
		case LabelUnresolved:
			counts.Unresolved++
		}
		if result.PrimaryAgreement {
			counts.PrimaryAgreements++
		}
		if result.TieBreak != nil {
			counts.TieBreaks++
		}
	}
	return counts
}

func Seal(value Study) (Study, error) {
	value.SchemaVersion, value.CanonicalPolicy, value.Digest = SchemaVersion, CanonicalPolicy, ""
	value.Digest = digestValue(value)
	if err := value.Validate(); err != nil {
		return Study{}, err
	}
	return value, nil
}

func (value Study) Validate() error {
	if value.SchemaVersion != SchemaVersion || value.CanonicalPolicy != CanonicalPolicy || !value.AgentOnly || value.StudyID == "" || value.Title == "" || !validDigest(value.PlanDigest) || !validDigest(value.AuditDigest) || !validDigest(value.ReleaseDigest) || value.ProviderCalls != 0 || value.HumanReviewers != 0 || len(value.Cases) == 0 || len(value.PrimaryRoles) != 2 || value.TieBreakRole == "" || len(value.ClaimBoundary) != 4 {
		return errors.New("agent-only study identity or boundary is invalid")
	}
	if value.Selection.Algorithm != SelectionAlgorithm || strings.TrimSpace(value.Selection.Seed) == "" || value.Selection.CalibrationTarget <= 0 || value.Selection.TestTarget <= 0 || value.Selection.CalibrationChosen != value.Selection.CalibrationTarget || value.Selection.TestChosen != value.Selection.TestTarget || value.Counts.Cases != len(value.Cases) || value.Counts.Calibration != value.Selection.CalibrationChosen || value.Counts.Test != value.Selection.TestChosen {
		return errors.New("agent-only study selection or counts are invalid")
	}
	if value.Selection.CrossSplitGroups != 0 {
		return errors.New("agent-only calibration/test split groups overlap")
	}
	if value.PrimaryRoles[0] != "primary_a/formal-release-validator-a" || value.PrimaryRoles[1] != "primary_b/formal-release-validator-b" || value.TieBreakRole != "tie_break/formal-release-tie-break" {
		return errors.New("agent-only validator roles are invalid")
	}
	if err := validateResults(value.Cases); err != nil {
		return err
	}
	if value.Counts != countResults(value.Cases) {
		return errors.New("agent-only study result counts do not reproduce")
	}
	copyValue := value
	copyValue.Digest = ""
	if value.Digest != digestValue(copyValue) {
		return errors.New("agent-only study digest is invalid")
	}
	return nil
}

func (value Study) ValidateAgainstRelease(release mutation.CorpusReleaseV3) error {
	if err := value.Validate(); err != nil {
		return err
	}
	if value.ReleaseDigest != release.Digest {
		return errors.New("agent-only study release digest does not match corpus release")
	}
	byID := make(map[string]mutation.CorpusCaseV3, len(release.Cases))
	for _, item := range release.Cases {
		byID[item.ID] = item
	}
	selected, err := selectCases(release.Cases, value.Selection.Seed, value.Selection.CalibrationTarget, value.Selection.TestTarget)
	if err != nil {
		return err
	}
	if len(selected) != len(value.Cases) {
		return errors.New("agent-only selected-case count does not reproduce")
	}
	selection := selectionRecord(selected, value.Selection.Seed, value.Selection.CalibrationTarget, value.Selection.TestTarget)
	if selection != value.Selection {
		return errors.New("agent-only selection metadata does not reproduce")
	}
	for index, item := range selected {
		if item.ID != value.Cases[index].CaseID {
			return fmt.Errorf("agent-only selection differs at index %d", index)
		}
	}
	sources := sourceIndex(release.Sources)
	for _, result := range value.Cases {
		item, ok := byID[result.CaseID]
		if !ok || item.Split != result.Split || item.Family != result.Family || item.Manifest.Digest != result.ManifestDigest || item.ConstructFirewall.Digest != result.FirewallDigest || item.BlindPacket.Digest != result.BlindPacketDigest || item.Manifest.ExpectedRelation != result.ExpectedRelation {
			return fmt.Errorf("agent-only case %q does not bind the frozen release", result.CaseID)
		}
		expectedAudit, auditErr := buildSourceAudit(item, sources)
		if auditErr != nil || digestValue(expectedAudit) != digestValue(result.SourceAudit) {
			return fmt.Errorf("agent-only case %q source audit does not reproduce", result.CaseID)
		}
	}
	return nil
}

func validateResults(results []CaseResult) error {
	seen := make(map[string]struct{}, len(results))
	for index, result := range results {
		if result.CaseID == "" || result.Family == mutation.FamilyTestEvidenceOmitted || (result.Split != study.RoleCalibration && result.Split != study.RoleTest) {
			return fmt.Errorf("agent-only case %d identity or split is invalid", index)
		}
		if _, ok := seen[result.CaseID]; ok {
			return fmt.Errorf("duplicate agent-only case %q", result.CaseID)
		}
		seen[result.CaseID] = struct{}{}
		if index > 0 && results[index-1].CaseID >= result.CaseID {
			return errors.New("agent-only cases must be identity-sorted")
		}
		if !validDigest(result.ManifestDigest) || !validDigest(result.FirewallDigest) || !validDigest(result.BlindPacketDigest) || !validDigest(result.SourceAudit.SourceSetDigest) {
			return fmt.Errorf("agent-only case %q digest binding is invalid", result.CaseID)
		}
		if err := validateSourceAudit(result.SourceAudit); err != nil {
			return fmt.Errorf("agent-only case %q source audit: %w", result.CaseID, err)
		}
		if err := validateValidatorResult(result.PrimaryA, "primary_a", PrimaryAVersion); err != nil {
			return err
		}
		if err := validateValidatorResult(result.PrimaryB, "primary_b", PrimaryBVersion); err != nil {
			return err
		}
		if result.PrimaryAgreement != (result.PrimaryA.Label == result.PrimaryB.Label) {
			return fmt.Errorf("agent-only case %q agreement is inconsistent", result.CaseID)
		}
		if result.PrimaryAgreement {
			if result.TieBreak != nil || result.Resolution != ResolutionAgreed || result.FinalLabel != result.PrimaryA.Label {
				return fmt.Errorf("agent-only case %q agreement resolution is invalid", result.CaseID)
			}
		} else {
			if result.TieBreak == nil || result.Resolution != ResolutionTie && result.Resolution != ResolutionOpen || result.FinalLabel != result.TieBreak.Label {
				return fmt.Errorf("agent-only case %q tie-break resolution is invalid", result.CaseID)
			}
			if err := validateValidatorResult(*result.TieBreak, "tie_break", TieBreakVersion); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateValidatorResult(value ValidatorResult, id, version string) error {
	if value.ValidatorID != id || value.Version != version || (value.Label != LabelAccepted && value.Label != LabelRejected && value.Label != LabelUnresolved) || len(value.Checks) == 0 || !validDigest(value.EvidenceDigest) {
		return fmt.Errorf("validator %s result is invalid", id)
	}
	if value.Label != labelForChecks(value.Checks) {
		return fmt.Errorf("validator %s label does not reproduce checks", id)
	}
	seen := make(map[string]struct{}, len(value.Checks))
	for _, check := range value.Checks {
		if check.Name == "" || check.Expected == "" || check.Observed == "" {
			return fmt.Errorf("validator %s contains an incomplete check", id)
		}
		if _, ok := seen[check.Name]; ok {
			return fmt.Errorf("validator %s contains a duplicate check", id)
		}
		seen[check.Name] = struct{}{}
	}
	expected := digestValue(struct {
		ValidatorID string         `json:"validator_id"`
		Version     string         `json:"version"`
		Label       string         `json:"label"`
		Checks      []MachineCheck `json:"checks"`
	}{value.ValidatorID, value.Version, value.Label, value.Checks})
	if value.EvidenceDigest != expected {
		return fmt.Errorf("validator %s evidence digest is invalid", id)
	}
	return nil
}

func validateSourceAudit(value SourceAudit) error {
	if len(value.SourceIDs) == 0 || len(value.SourceIDs) != len(value.Sources) || len(value.SourceIDs) != len(value.SourceDigests) || len(value.SourceIDs) != len(value.TrajectoryDigests) || len(value.SourceIDs) != len(value.OutcomeWitnessDigests) {
		return errors.New("source audit arrays are incomplete")
	}
	for index, source := range value.Sources {
		if source.ID != value.SourceIDs[index] || source.TaskID == "" || source.RepositoryID == "" || source.SourceFamily == "" || source.SourceFormat == "" || source.SplitGroupID == "" || source.Split == "" || !validDigest(source.SourceDigest) || !validDigest(source.TrajectoryDigest) || !validDigest(source.OutcomeWitnessDigest) || source.SourceDigest != value.SourceDigests[index] || source.TrajectoryDigest != value.TrajectoryDigests[index] || source.OutcomeWitnessDigest != value.OutcomeWitnessDigests[index] {
			return fmt.Errorf("source audit record %d is not bound", index)
		}
	}
	for _, digest := range []string{value.ManifestSourceDigest, value.ManifestTrajectoryDigest, value.ManifestOutcomeWitnessDigest} {
		if !validDigest(digest) {
			return errors.New("manifest source audit digest is invalid")
		}
	}
	return nil
}

func Decode(reader io.Reader) (Study, error) {
	decoder := json.NewDecoder(io.LimitReader(reader, 16*1024*1024))
	decoder.DisallowUnknownFields()
	var value Study
	if err := decoder.Decode(&value); err != nil {
		return Study{}, fmt.Errorf("decode agent-only study: %w", err)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Study{}, errors.New("decode agent-only study: trailing JSON is forbidden")
		}
		return Study{}, fmt.Errorf("decode agent-only study: trailing data: %w", err)
	}
	if err := value.Validate(); err != nil {
		return Study{}, err
	}
	return value, nil
}

func EncodeIndented(value any) ([]byte, error) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func Schema() (map[string]any, error) {
	schema := schemaForType(reflect.TypeOf(Study{}))
	schema["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	schema["$id"] = "https://evalwitness.dev/schemas/agent-only-study.v1.json"
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return nil, errors.New("agent-only study schema root is not an object")
	}
	properties["schema_version"] = map[string]any{"const": SchemaVersion}
	properties["canonical_policy"] = map[string]any{"const": CanonicalPolicy}
	properties["agent_only"] = map[string]any{"const": true}
	return schema, nil
}

func schemaForType(value reflect.Type) map[string]any {
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.Struct:
		properties := make(map[string]any)
		required := make([]string, 0, value.NumField())
		for index := 0; index < value.NumField(); index++ {
			field := value.Field(index)
			parts := strings.Split(field.Tag.Get("json"), ",")
			name := parts[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = field.Name
			}
			properties[name] = schemaForType(field.Type)
			if !containsSchemaTag(parts[1:], "omitempty") {
				required = append(required, name)
			}
		}
		return map[string]any{"type": "object", "additionalProperties": false, "properties": properties, "required": required}
	case reflect.Slice, reflect.Array:
		return map[string]any{"type": "array", "items": schemaForType(value.Elem())}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return map[string]any{"type": "integer"}
	case reflect.String:
		return map[string]any{"type": "string"}
	default:
		return map[string]any{}
	}
}

func containsSchemaTag(tags []string, expected string) bool {
	for _, tag := range tags {
		if tag == expected {
			return true
		}
	}
	return false
}

func digestValue(value any) string {
	encoded, _ := json.Marshal(value)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func validDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
