package outcome

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"time"
)

const OutcomeSourceAuditSchemaVersion = "evalwitness.outcome-source-audit.v1"

type OutcomeSourceSummary struct {
	Kind             EvidenceKind `json:"kind"`
	Observations     int          `json:"observations"`
	StateDefined     bool         `json:"state_defined"`
	State            State        `json:"state,omitempty"`
	InternalConflict bool         `json:"internal_conflict"`
	StatePrevalence  []StateCount `json:"state_prevalence"`
	EvidenceDigests  []string     `json:"evidence_digests"`
}

type OutcomeSourcePair struct {
	Left  EvidenceKind `json:"left"`
	Right EvidenceKind `json:"right"`
}

type OutcomeSourceItem struct {
	PacketID             string                 `json:"packet_id"`
	TaskGroupID          string                 `json:"task_group_id"`
	MappingDigest        string                 `json:"mapping_digest"`
	SourceRecordDigest   string                 `json:"source_record_digest"`
	SourceEvidenceDigest string                 `json:"source_evidence_digest"`
	SourceEvidence       []Evidence             `json:"source_evidence"`
	HumanResolution      Resolution             `json:"human_resolution"`
	Sources              []OutcomeSourceSummary `json:"sources"`
	ConflictingPairs     []OutcomeSourcePair    `json:"conflicting_pairs"`
	BenchmarkComparable  bool                   `json:"benchmark_comparable"`
	BenchmarkChanged     bool                   `json:"benchmark_changed"`
}

type OutcomeSourceCoverageMetric struct {
	Kind                    EvidenceKind `json:"kind"`
	Packets                 int          `json:"packets"`
	PacketsWithSource       int          `json:"packets_with_source"`
	CoverageRate            float64      `json:"coverage_rate"`
	CoverageInterval        Interval     `json:"coverage_interval"`
	Observations            int          `json:"observations"`
	InternalConflictPackets int          `json:"internal_conflict_packets"`
	StatePrevalence         []StateCount `json:"state_prevalence"`
}

type OutcomeSourcePairMetric struct {
	Left                   EvidenceKind `json:"left"`
	Right                  EvidenceKind `json:"right"`
	Packets                int          `json:"packets"`
	CasesWithBothSources   int          `json:"cases_with_both_sources"`
	MissingCases           int          `json:"missing_cases"`
	AmbiguousCases         int          `json:"ambiguous_cases"`
	ComparableCases        int          `json:"comparable_cases"`
	Agreements             int          `json:"agreements"`
	Disagreements          int          `json:"disagreements"`
	AgreementRate          float64      `json:"agreement_rate"`
	AgreementInterval      Interval     `json:"agreement_interval"`
	CohenKappa             float64      `json:"cohen_kappa"`
	CohenKappaDefined      bool         `json:"cohen_kappa_defined"`
	CohenKappaInterval     Interval     `json:"cohen_kappa_interval"`
	DefinedKappaReplicates int          `json:"defined_kappa_replicates"`
}

type OutcomeTransitionCount struct {
	From  State `json:"from"`
	To    State `json:"to"`
	Count int   `json:"count"`
}

type OutcomeSourceAudit struct {
	SchemaVersion               string                        `json:"schema_version"`
	CanonicalPolicy             string                        `json:"canonical_policy"`
	BundleDigest                string                        `json:"bundle_digest"`
	MappingRevealDigest         string                        `json:"mapping_reveal_digest"`
	AdjudicationLedgerDigest    string                        `json:"adjudication_ledger_digest"`
	RubricAmbiguityDigest       string                        `json:"rubric_ambiguity_digest"`
	BlindingAnalysisDigest      string                        `json:"blinding_analysis_digest"`
	DataRole                    ReviewDataRole                `json:"data_role"`
	Visibility                  ReviewVisibility              `json:"visibility"`
	Packets                     int                           `json:"packets"`
	SourceEvidenceObservations  int                           `json:"source_evidence_observations"`
	PacketsWithInternalConflict int                           `json:"packets_with_internal_conflict"`
	PacketsWithAnyDisagreement  int                           `json:"packets_with_any_disagreement"`
	AnyDisagreementRate         float64                       `json:"any_disagreement_rate"`
	AnyDisagreementInterval     Interval                      `json:"any_disagreement_interval"`
	BenchmarkComparablePackets  int                           `json:"benchmark_comparable_packets"`
	BenchmarkChangedPackets     int                           `json:"benchmark_changed_packets"`
	BenchmarkChangedRate        float64                       `json:"benchmark_changed_rate"`
	BenchmarkChangedInterval    Interval                      `json:"benchmark_changed_interval"`
	SourceCoverage              []OutcomeSourceCoverageMetric `json:"source_coverage"`
	PairMetrics                 []OutcomeSourcePairMetric     `json:"pair_metrics"`
	BenchmarkTransitions        []OutcomeTransitionCount      `json:"benchmark_transitions"`
	Items                       []OutcomeSourceItem           `json:"items"`
	BootstrapIterations         int                           `json:"bootstrap_iterations"`
	BootstrapSeed               string                        `json:"bootstrap_seed"`
	AnalyzedAt                  string                        `json:"analyzed_at"`
	Limitations                 []string                      `json:"limitations"`
	Digest                      string                        `json:"digest"`
}

func BuildOutcomeSourceAudit(bundle ReviewBundle, reveal MappingReveal, ledger AdjudicationLedger, mappings []PrivateMapping, records []Record, resolutions []Resolution, analyzedAt string, bootstrapIterations int, bootstrapSeed string) (OutcomeSourceAudit, error) {
	if err := validateOutcomeSourceAuditInputs(bundle, reveal, ledger, mappings, records, resolutions, analyzedAt, bootstrapIterations, bootstrapSeed); err != nil {
		return OutcomeSourceAudit{}, err
	}
	recordByTask := make(map[string]Record, len(records))
	for _, record := range records {
		recordByTask[record.TaskAlias] = record
	}
	resolutionByPacket := resolutionsByPacket(resolutions)
	mappingByPacket := privateMappingsByPacket(mappings)
	items := make([]OutcomeSourceItem, 0, len(bundle.Items))
	for _, reviewItem := range bundle.Items {
		mapping := mappingByPacket[reviewItem.Packet.PacketID]
		record := recordByTask[mapping.SourceTaskAlias]
		item, err := buildOutcomeSourceItem(reviewItem, mapping, record, resolutionByPacket[reviewItem.Packet.PacketID])
		if err != nil {
			return OutcomeSourceAudit{}, err
		}
		items = append(items, item)
	}
	audit, err := summarizeOutcomeSourceItems(items, bootstrapIterations, bootstrapSeed)
	if err != nil {
		return OutcomeSourceAudit{}, err
	}
	audit.BundleDigest, audit.MappingRevealDigest, audit.AdjudicationLedgerDigest = bundle.Digest, reveal.Digest, ledger.Digest
	audit.RubricAmbiguityDigest, audit.BlindingAnalysisDigest = ledger.RubricAmbiguityDigest, ledger.BlindingAnalysisDigest
	audit.DataRole, audit.Visibility = bundle.DataRole, bundle.Visibility
	audit.AnalyzedAt, audit.Limitations = analyzedAt, outcomeSourceAuditLimitations()
	return SealOutcomeSourceAudit(audit)
}

func SealOutcomeSourceAudit(audit OutcomeSourceAudit) (OutcomeSourceAudit, error) {
	audit.SchemaVersion, audit.CanonicalPolicy, audit.Digest = OutcomeSourceAuditSchemaVersion, CanonicalPolicy, ""
	digest, err := outcomeSourceAuditDigest(audit)
	if err != nil {
		return OutcomeSourceAudit{}, err
	}
	audit.Digest = digest
	return audit, audit.Validate()
}

func (audit OutcomeSourceAudit) Validate() error {
	if audit.SchemaVersion != OutcomeSourceAuditSchemaVersion || audit.CanonicalPolicy != CanonicalPolicy ||
		!validDigest(audit.BundleDigest) || !validDigest(audit.MappingRevealDigest) || !validDigest(audit.AdjudicationLedgerDigest) ||
		!validDigest(audit.RubricAmbiguityDigest) || !validDigest(audit.BlindingAnalysisDigest) || !validReviewDataRole(audit.DataRole) || !validReviewVisibility(audit.Visibility) ||
		audit.Packets < 1 || len(audit.Items) != audit.Packets || audit.BootstrapIterations < 10_000 || missing(audit.BootstrapSeed, audit.AnalyzedAt) {
		return errors.New("outcome source audit identity, bindings, visibility, sample, bootstrap, or time are invalid")
	}
	if _, err := time.Parse(time.RFC3339, audit.AnalyzedAt); err != nil {
		return errors.New("outcome source audit timestamp must be RFC3339")
	}
	if !equalStrings(audit.Limitations, outcomeSourceAuditLimitations()) {
		return errors.New("outcome source audit limitations are invalid")
	}
	if err := validateOutcomeSourceItems(audit.Items); err != nil {
		return err
	}
	recomputed, err := summarizeOutcomeSourceItems(audit.Items, audit.BootstrapIterations, audit.BootstrapSeed)
	if err != nil {
		return err
	}
	if !equalOutcomeSourceAuditSummary(audit, recomputed) || !boundedProbability(audit.AnyDisagreementRate) ||
		!validInterval(audit.AnyDisagreementInterval, 0, 1) || !boundedProbability(audit.BenchmarkChangedRate) ||
		(audit.BenchmarkComparablePackets > 0 && !validInterval(audit.BenchmarkChangedInterval, 0, 1)) {
		return errors.New("outcome source audit statistics are inconsistent with item evidence")
	}
	expected, err := outcomeSourceAuditDigest(audit)
	if err != nil || audit.Digest != expected {
		return errors.New("outcome source audit digest is invalid")
	}
	return nil
}

func DecodeOutcomeSourceAudit(reader io.Reader) (OutcomeSourceAudit, error) {
	var value OutcomeSourceAudit
	if err := decodeStrict(reader, &value); err != nil {
		return OutcomeSourceAudit{}, fmt.Errorf("decode outcome source audit: %w", err)
	}
	return value, value.Validate()
}

func DecodeRecords(reader io.Reader) ([]Record, error) {
	var values []Record
	if err := decodeStrict(reader, &values); err != nil {
		return nil, fmt.Errorf("decode outcome records: %w", err)
	}
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return nil, fmt.Errorf("outcome record %d: %w", index, err)
		}
	}
	return values, nil
}

func DecodeResolutions(reader io.Reader) ([]Resolution, error) {
	var values []Resolution
	if err := decodeStrict(reader, &values); err != nil {
		return nil, fmt.Errorf("decode outcome resolutions: %w", err)
	}
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return nil, fmt.Errorf("outcome resolution %d: %w", index, err)
		}
	}
	return values, nil
}

func validateOutcomeSourceAuditInputs(bundle ReviewBundle, reveal MappingReveal, ledger AdjudicationLedger, mappings []PrivateMapping, records []Record, resolutions []Resolution, analyzedAt string, bootstrapIterations int, bootstrapSeed string) error {
	if err := bundle.Validate(); err != nil {
		return err
	}
	if err := reveal.Validate(); err != nil {
		return err
	}
	if err := ledger.Validate(); err != nil {
		return err
	}
	if ledger.BundleDigest != bundle.Digest || ledger.MappingRevealDigest != reveal.Digest || reveal.BundleDigest != bundle.Digest {
		return errors.New("outcome source audit bundle, reveal, and ledger bindings differ")
	}
	if bootstrapIterations < 10_000 || missing(bootstrapSeed) {
		return errors.New("outcome source audit requires at least 10000 bootstrap iterations and a seed")
	}
	if err := requireStrictlyAfter("outcome source audit", analyzedAt, ledger.CompletedAt); err != nil {
		return err
	}
	if err := validateSourceAuditMappings(bundle, reveal, mappings); err != nil {
		return err
	}
	if err := validateSourceAuditRecords(mappings, records); err != nil {
		return err
	}
	return validateSourceAuditResolutions(bundle, ledger, resolutions)
}

func validateSourceAuditMappings(bundle ReviewBundle, reveal MappingReveal, mappings []PrivateMapping) error {
	references, err := mappingReferences(bundle, mappings)
	if err != nil {
		return err
	}
	if !equalMappingReferences(references, reveal.Mappings) {
		return errors.New("outcome source audit mappings do not reproduce the committed reveal references")
	}
	return nil
}

func validateSourceAuditRecords(mappings []PrivateMapping, records []Record) error {
	if len(records) != len(mappings) {
		return errors.New("outcome source audit requires exactly one source record per mapping")
	}
	recordByTask := make(map[string]Record, len(records))
	for index, record := range records {
		if err := record.Validate(); err != nil {
			return fmt.Errorf("outcome source record %d: %w", index, err)
		}
		if _, duplicate := recordByTask[record.TaskAlias]; duplicate {
			return errors.New("outcome source audit records must have unique task aliases")
		}
		for _, evidence := range record.Evidence {
			if evidence.Kind == EvidenceHumanLabel {
				return errors.New("outcome source records cannot pre-embed human adjudication evidence")
			}
		}
		recordByTask[record.TaskAlias] = record
	}
	seen := make(map[string]struct{}, len(mappings))
	for _, mapping := range mappings {
		if _, exists := recordByTask[mapping.SourceTaskAlias]; !exists {
			return errors.New("outcome source audit mapping has no matching source record")
		}
		if _, duplicate := seen[mapping.SourceTaskAlias]; duplicate {
			return errors.New("outcome source audit mappings reuse one source task")
		}
		seen[mapping.SourceTaskAlias] = struct{}{}
	}
	return nil
}

func validateSourceAuditResolutions(bundle ReviewBundle, ledger AdjudicationLedger, resolutions []Resolution) error {
	if len(resolutions) != len(bundle.Items) {
		return errors.New("outcome source audit requires exactly one resolution per packet")
	}
	digests := make([]string, 0, len(resolutions))
	packetIDs := make([]string, 0, len(resolutions))
	for _, resolution := range resolutions {
		if err := resolution.Validate(); err != nil {
			return err
		}
		digests = append(digests, resolution.Digest)
		packetIDs = append(packetIDs, resolution.PacketID)
	}
	sort.Strings(digests)
	if !equalStrings(digests, ledger.ResolutionDigests) {
		return errors.New("outcome source audit resolutions do not reproduce the terminal ledger")
	}
	return exactPacketCoverage(reviewBundlePacketIDs(bundle), packetIDs)
}

func buildOutcomeSourceItem(reviewItem ReviewItem, mapping PrivateMapping, record Record, resolution Resolution) (OutcomeSourceItem, error) {
	evidence := append([]Evidence(nil), record.Evidence...)
	evidenceDigest, err := digestJSON(evidence)
	if err != nil {
		return OutcomeSourceItem{}, err
	}
	item := OutcomeSourceItem{
		PacketID: reviewItem.Packet.PacketID, TaskGroupID: reviewItem.TaskGroupID, MappingDigest: mapping.Digest,
		SourceRecordDigest: record.Digest, SourceEvidenceDigest: evidenceDigest, SourceEvidence: evidence, HumanResolution: resolution,
	}
	item.Sources = summarizeItemSources(item.SourceEvidence, item.HumanResolution)
	item.ConflictingPairs = conflictingSourcePairs(item.Sources)
	benchmark := sourceSummaryByKind(item.Sources)[EvidenceBenchmarkReward]
	item.BenchmarkComparable = benchmark.StateDefined
	item.BenchmarkChanged = benchmark.StateDefined && benchmark.State != resolution.State
	return item, validateOutcomeSourceItem(item)
}

func validateOutcomeSourceItems(items []OutcomeSourceItem) error {
	seenMappings, seenRecords, seenResolutions := make(map[string]struct{}), make(map[string]struct{}), make(map[string]struct{})
	for index, item := range items {
		if err := validateOutcomeSourceItem(item); err != nil {
			return fmt.Errorf("outcome source audit item %d: %w", index, err)
		}
		if index > 0 && items[index-1].PacketID >= item.PacketID {
			return errors.New("outcome source audit items must be unique and sorted by packet")
		}
		if _, duplicate := seenMappings[item.MappingDigest]; duplicate {
			return errors.New("outcome source audit reuses a mapping digest")
		}
		if _, duplicate := seenRecords[item.SourceRecordDigest]; duplicate {
			return errors.New("outcome source audit reuses a source record digest")
		}
		if _, duplicate := seenResolutions[item.HumanResolution.Digest]; duplicate {
			return errors.New("outcome source audit reuses a human resolution digest")
		}
		seenMappings[item.MappingDigest], seenRecords[item.SourceRecordDigest], seenResolutions[item.HumanResolution.Digest] = struct{}{}, struct{}{}, struct{}{}
	}
	return nil
}

func validateOutcomeSourceItem(item OutcomeSourceItem) error {
	if !validOpaquePacketID(item.PacketID) || !validTaskGroupID(item.TaskGroupID) || !validDigest(item.MappingDigest) ||
		!validDigest(item.SourceRecordDigest) || !validDigest(item.SourceEvidenceDigest) || len(item.SourceEvidence) == 0 {
		return errors.New("outcome source item identity, record, mapping, or evidence is invalid")
	}
	for index, evidence := range item.SourceEvidence {
		if err := evidence.Validate(); err != nil {
			return err
		}
		if evidence.Kind == EvidenceHumanLabel || index > 0 && item.SourceEvidence[index-1].ID >= evidence.ID {
			return errors.New("outcome source evidence must be non-human, unique, and sorted by ID")
		}
	}
	evidenceDigest, err := digestJSON(item.SourceEvidence)
	if err != nil || evidenceDigest != item.SourceEvidenceDigest {
		return errors.New("outcome source evidence digest is invalid")
	}
	if err := item.HumanResolution.Validate(); err != nil {
		return err
	}
	if item.HumanResolution.PacketID != item.PacketID {
		return errors.New("outcome source human resolution does not bind the packet")
	}
	expectedSources := summarizeItemSources(item.SourceEvidence, item.HumanResolution)
	if !equalOutcomeSourceSummaries(item.Sources, expectedSources) || !equalOutcomeSourcePairs(item.ConflictingPairs, conflictingSourcePairs(expectedSources)) {
		return errors.New("outcome source summaries or contradictions do not reproduce source evidence")
	}
	benchmark := sourceSummaryByKind(expectedSources)[EvidenceBenchmarkReward]
	if item.BenchmarkComparable != benchmark.StateDefined || item.BenchmarkChanged != (benchmark.StateDefined && benchmark.State != item.HumanResolution.State) {
		return errors.New("outcome source benchmark transition does not reproduce source evidence")
	}
	return nil
}

func summarizeItemSources(evidence []Evidence, resolution Resolution) []OutcomeSourceSummary {
	byKind := make(map[EvidenceKind][]Evidence, len(outcomeSourceKinds()))
	for _, item := range evidence {
		byKind[item.Kind] = append(byKind[item.Kind], item)
	}
	values := make([]OutcomeSourceSummary, 0, len(outcomeSourceKinds()))
	for _, kind := range outcomeSourceKinds() {
		if kind == EvidenceHumanLabel {
			values = append(values, OutcomeSourceSummary{
				Kind: kind, Observations: 1, StateDefined: true, State: resolution.State,
				StatePrevalence: []StateCount{{State: resolution.State, Count: 1}}, EvidenceDigests: []string{resolution.Digest},
			})
			continue
		}
		values = append(values, summarizeEvidenceKind(kind, byKind[kind]))
	}
	return values
}

func summarizeEvidenceKind(kind EvidenceKind, evidence []Evidence) OutcomeSourceSummary {
	value := OutcomeSourceSummary{Kind: kind, Observations: len(evidence), StatePrevalence: []StateCount{}, EvidenceDigests: []string{}}
	counts := make(map[State]int)
	for _, item := range evidence {
		counts[item.State]++
		value.EvidenceDigests = append(value.EvidenceDigests, item.Digest)
	}
	sort.Strings(value.EvidenceDigests)
	value.StatePrevalence = sortedStateCounts(counts)
	value.StateDefined = false
	value.InternalConflict = len(counts) > 1
	if len(counts) == 1 {
		for state := range counts {
			if adjudicatableState(state) {
				value.StateDefined = true
				value.State = state
			}
		}
	}
	return value
}

func summarizeOutcomeSourceItems(items []OutcomeSourceItem, bootstrapIterations int, bootstrapSeed string) (OutcomeSourceAudit, error) {
	audit := OutcomeSourceAudit{Packets: len(items), Items: append([]OutcomeSourceItem(nil), items...), BootstrapIterations: bootstrapIterations, BootstrapSeed: bootstrapSeed}
	coverage := make(map[EvidenceKind]*OutcomeSourceCoverageMetric, len(outcomeSourceKinds()))
	for _, kind := range outcomeSourceKinds() {
		coverage[kind] = &OutcomeSourceCoverageMetric{Kind: kind, Packets: len(items), StatePrevalence: []StateCount{}}
	}
	transitionCounts := make(map[string]int)
	for _, item := range items {
		anyInternalConflict := false
		for _, source := range item.Sources {
			metric := coverage[source.Kind]
			metric.Observations += source.Observations
			if source.Observations > 0 {
				metric.PacketsWithSource++
			}
			if source.InternalConflict {
				metric.InternalConflictPackets++
				anyInternalConflict = true
			}
			metric.StatePrevalence = mergeStateCounts(metric.StatePrevalence, source.StatePrevalence)
			if source.Kind != EvidenceHumanLabel {
				audit.SourceEvidenceObservations += source.Observations
			}
		}
		if anyInternalConflict {
			audit.PacketsWithInternalConflict++
		}
		if len(item.ConflictingPairs) > 0 {
			audit.PacketsWithAnyDisagreement++
		}
		if item.BenchmarkComparable {
			audit.BenchmarkComparablePackets++
			if item.BenchmarkChanged {
				audit.BenchmarkChangedPackets++
			}
			benchmark := sourceSummaryByKind(item.Sources)[EvidenceBenchmarkReward]
			key := string(benchmark.State) + "\x00" + string(item.HumanResolution.State)
			transitionCounts[key]++
		}
	}
	audit.AnyDisagreementRate = ratio(audit.PacketsWithAnyDisagreement, audit.Packets)
	audit.AnyDisagreementInterval = wilsonInterval(audit.PacketsWithAnyDisagreement, audit.Packets)
	audit.BenchmarkChangedRate = ratio(audit.BenchmarkChangedPackets, audit.BenchmarkComparablePackets)
	audit.BenchmarkChangedInterval = wilsonInterval(audit.BenchmarkChangedPackets, audit.BenchmarkComparablePackets)
	for _, kind := range outcomeSourceKinds() {
		metric := coverage[kind]
		metric.CoverageRate = ratio(metric.PacketsWithSource, metric.Packets)
		metric.CoverageInterval = wilsonInterval(metric.PacketsWithSource, metric.Packets)
		sort.Slice(metric.StatePrevalence, func(left, right int) bool {
			return metric.StatePrevalence[left].State < metric.StatePrevalence[right].State
		})
		audit.SourceCoverage = append(audit.SourceCoverage, *metric)
	}
	for _, pair := range outcomeSourcePairs() {
		metric, err := summarizeOutcomeSourcePair(items, pair, bootstrapIterations, bootstrapSeed)
		if err != nil {
			return OutcomeSourceAudit{}, err
		}
		audit.PairMetrics = append(audit.PairMetrics, metric)
	}
	for key, count := range transitionCounts {
		separator := 0
		for index := range key {
			if key[index] == 0 {
				separator = index
				break
			}
		}
		audit.BenchmarkTransitions = append(audit.BenchmarkTransitions, OutcomeTransitionCount{From: State(key[:separator]), To: State(key[separator+1:]), Count: count})
	}
	sort.Slice(audit.BenchmarkTransitions, func(left, right int) bool {
		if audit.BenchmarkTransitions[left].From == audit.BenchmarkTransitions[right].From {
			return audit.BenchmarkTransitions[left].To < audit.BenchmarkTransitions[right].To
		}
		return audit.BenchmarkTransitions[left].From < audit.BenchmarkTransitions[right].From
	})
	return audit, nil
}

func summarizeOutcomeSourcePair(items []OutcomeSourceItem, pair OutcomeSourcePair, bootstrapIterations int, bootstrapSeed string) (OutcomeSourcePairMetric, error) {
	metric := OutcomeSourcePairMetric{Left: pair.Left, Right: pair.Right, Packets: len(items)}
	pairs := make([]AgreementPair, 0, len(items))
	for _, item := range items {
		sources := sourceSummaryByKind(item.Sources)
		left, right := sources[pair.Left], sources[pair.Right]
		if left.Observations == 0 || right.Observations == 0 {
			continue
		}
		metric.CasesWithBothSources++
		if !left.StateDefined || !right.StateDefined {
			metric.AmbiguousCases++
			continue
		}
		metric.ComparableCases++
		if left.State == right.State {
			metric.Agreements++
		} else {
			metric.Disagreements++
		}
		pairs = append(pairs, AgreementPair{PacketID: item.PacketID, TaskGroupID: item.TaskGroupID, Left: left.State, Right: right.State})
	}
	metric.MissingCases = metric.Packets - metric.CasesWithBothSources
	metric.AgreementRate = ratio(metric.Agreements, metric.ComparableCases)
	metric.AgreementInterval = wilsonInterval(metric.Agreements, metric.ComparableCases)
	if len(pairs) < 2 {
		return metric, nil
	}
	report, err := ComputeAgreement(pairs, bootstrapIterations, bootstrapSeed+"|"+string(pair.Left)+"|"+string(pair.Right))
	if err != nil {
		return OutcomeSourcePairMetric{}, err
	}
	metric.AgreementRate, metric.AgreementInterval = report.RawAgreement, report.RawAgreementInterval
	metric.CohenKappa, metric.CohenKappaDefined, metric.CohenKappaInterval = report.CohenKappa, report.CohenKappaDefined, report.CohenKappaInterval
	metric.DefinedKappaReplicates = report.DefinedKappaReplicates
	return metric, nil
}

func conflictingSourcePairs(sources []OutcomeSourceSummary) []OutcomeSourcePair {
	byKind := sourceSummaryByKind(sources)
	values := make([]OutcomeSourcePair, 0)
	for _, pair := range outcomeSourcePairs() {
		left, right := byKind[pair.Left], byKind[pair.Right]
		if left.StateDefined && right.StateDefined && left.State != right.State {
			values = append(values, pair)
		}
	}
	return values
}

func outcomeSourceKinds() []EvidenceKind {
	return []EvidenceKind{EvidenceClaimedTest, EvidenceBenchmarkReward, EvidenceIndependentRun, EvidenceFormalRelation, EvidenceHumanLabel}
}

func outcomeSourcePairs() []OutcomeSourcePair {
	kinds := outcomeSourceKinds()
	values := make([]OutcomeSourcePair, 0, len(kinds)*(len(kinds)-1)/2)
	for left := 0; left < len(kinds); left++ {
		for right := left + 1; right < len(kinds); right++ {
			values = append(values, OutcomeSourcePair{Left: kinds[left], Right: kinds[right]})
		}
	}
	return values
}

func sourceSummaryByKind(values []OutcomeSourceSummary) map[EvidenceKind]OutcomeSourceSummary {
	result := make(map[EvidenceKind]OutcomeSourceSummary, len(values))
	for _, value := range values {
		result[value.Kind] = value
	}
	return result
}

func privateMappingsByPacket(values []PrivateMapping) map[string]PrivateMapping {
	result := make(map[string]PrivateMapping, len(values))
	for _, value := range values {
		result[value.PacketID] = value
	}
	return result
}

func resolutionsByPacket(values []Resolution) map[string]Resolution {
	result := make(map[string]Resolution, len(values))
	for _, value := range values {
		result[value.PacketID] = value
	}
	return result
}

func mergeStateCounts(values []StateCount, additions []StateCount) []StateCount {
	counts := make(map[State]int, len(values)+len(additions))
	for _, value := range values {
		counts[value.State] += value.Count
	}
	for _, value := range additions {
		counts[value.State] += value.Count
	}
	return sortedStateCounts(counts)
}

func outcomeSourceAuditLimitations() []string {
	return []string{
		"human outcomes are terminal commitments and source reconciliation cannot consult verifier scores or selections",
		"pairwise source agreement describes consistency and does not establish which source is correct",
		"real reviewer evidence is required before synthetic workflow fixtures can support an empirical validity claim",
		"within-source state conflicts remain ambiguous and are excluded from pairwise comparable denominators",
	}
}

func equalOutcomeSourceSummaries(left, right []OutcomeSourceSummary) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Kind != right[index].Kind || left[index].Observations != right[index].Observations ||
			left[index].StateDefined != right[index].StateDefined || left[index].State != right[index].State ||
			left[index].InternalConflict != right[index].InternalConflict || !equalStateCounts(left[index].StatePrevalence, right[index].StatePrevalence) ||
			!equalStrings(left[index].EvidenceDigests, right[index].EvidenceDigests) {
			return false
		}
	}
	return true
}

func equalOutcomeSourcePairs(left, right []OutcomeSourcePair) bool {
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

func equalOutcomeSourceCoverage(left, right []OutcomeSourceCoverageMetric) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Kind != right[index].Kind || left[index].Packets != right[index].Packets ||
			left[index].PacketsWithSource != right[index].PacketsWithSource || left[index].CoverageRate != right[index].CoverageRate ||
			left[index].CoverageInterval != right[index].CoverageInterval || left[index].Observations != right[index].Observations ||
			left[index].InternalConflictPackets != right[index].InternalConflictPackets || !equalStateCounts(left[index].StatePrevalence, right[index].StatePrevalence) {
			return false
		}
	}
	return true
}

func equalOutcomeSourcePairMetrics(left, right []OutcomeSourcePairMetric) bool {
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

func equalOutcomeTransitions(left, right []OutcomeTransitionCount) bool {
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

func equalOutcomeSourceAuditSummary(left, right OutcomeSourceAudit) bool {
	return left.Packets == right.Packets && left.SourceEvidenceObservations == right.SourceEvidenceObservations &&
		left.PacketsWithInternalConflict == right.PacketsWithInternalConflict && left.PacketsWithAnyDisagreement == right.PacketsWithAnyDisagreement &&
		left.AnyDisagreementRate == right.AnyDisagreementRate && left.AnyDisagreementInterval == right.AnyDisagreementInterval &&
		left.BenchmarkComparablePackets == right.BenchmarkComparablePackets && left.BenchmarkChangedPackets == right.BenchmarkChangedPackets &&
		left.BenchmarkChangedRate == right.BenchmarkChangedRate && left.BenchmarkChangedInterval == right.BenchmarkChangedInterval &&
		equalOutcomeSourceCoverage(left.SourceCoverage, right.SourceCoverage) && equalOutcomeSourcePairMetrics(left.PairMetrics, right.PairMetrics) &&
		equalOutcomeTransitions(left.BenchmarkTransitions, right.BenchmarkTransitions)
}

func outcomeSourceAuditDigest(value OutcomeSourceAudit) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
