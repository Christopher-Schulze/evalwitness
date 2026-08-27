package outcome

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"
)

type AgreementPair struct {
	PacketID    string `json:"packet_id"`
	TaskGroupID string `json:"task_group_id"`
	Left        State  `json:"left"`
	Right       State  `json:"right"`
}

type Interval struct {
	Lower float64 `json:"lower"`
	Upper float64 `json:"upper"`
}

type StateCount struct {
	State State `json:"state"`
	Count int   `json:"count"`
}

type AgreementReport struct {
	SchemaVersion          string       `json:"schema_version"`
	CanonicalPolicy        string       `json:"canonical_policy"`
	Pairs                  int          `json:"pairs"`
	TaskGroups             int          `json:"task_groups"`
	RawAgreement           float64      `json:"raw_agreement"`
	RawAgreementInterval   Interval     `json:"raw_agreement_interval"`
	CohenKappa             float64      `json:"cohen_kappa"`
	CohenKappaDefined      bool         `json:"cohen_kappa_defined"`
	CohenKappaInterval     Interval     `json:"cohen_kappa_interval"`
	BootstrapIterations    int          `json:"bootstrap_iterations"`
	DefinedKappaReplicates int          `json:"defined_kappa_replicates"`
	BootstrapSeed          string       `json:"bootstrap_seed"`
	LabelPrevalence        []StateCount `json:"label_prevalence"`
	Digest                 string       `json:"digest"`
}

func ComputeAgreement(pairs []AgreementPair, iterations int, seed string) (AgreementReport, error) {
	if len(pairs) < 2 || iterations < 10_000 || missing(seed) {
		return AgreementReport{}, errors.New("agreement requires at least two pairs, 10000 bootstrap iterations, and a seed")
	}
	seenPackets := make(map[string]struct{}, len(pairs))
	groups := make(map[string][]AgreementPair)
	prevalence := make(map[State]int)
	for index, pair := range pairs {
		if !validOpaquePacketID(pair.PacketID) || missing(pair.TaskGroupID) || !adjudicatableState(pair.Left) || !adjudicatableState(pair.Right) {
			return AgreementReport{}, fmt.Errorf("agreement pair %d is invalid", index)
		}
		if _, duplicate := seenPackets[pair.PacketID]; duplicate {
			return AgreementReport{}, fmt.Errorf("agreement packet %q appears more than once", pair.PacketID)
		}
		seenPackets[pair.PacketID] = struct{}{}
		groups[pair.TaskGroupID] = append(groups[pair.TaskGroupID], pair)
		prevalence[pair.Left]++
		prevalence[pair.Right]++
	}
	raw, kappa, defined := agreementPoint(pairs)
	groupIDs := make([]string, 0, len(groups))
	for groupID := range groups {
		groupIDs = append(groupIDs, groupID)
	}
	sort.Strings(groupIDs)
	digest := sha256.Sum256([]byte(seed))
	random := rand.New(rand.NewSource(int64(binary.BigEndian.Uint64(digest[:8]))))
	rawReplicates := make([]float64, 0, iterations)
	kappaReplicates := make([]float64, 0, iterations)
	for iteration := 0; iteration < iterations; iteration++ {
		var sampled []AgreementPair
		for range groupIDs {
			sampled = append(sampled, groups[groupIDs[random.Intn(len(groupIDs))]]...)
		}
		sampledRaw, sampledKappa, sampledDefined := agreementPoint(sampled)
		rawReplicates = append(rawReplicates, sampledRaw)
		if sampledDefined {
			kappaReplicates = append(kappaReplicates, sampledKappa)
		}
	}
	report := AgreementReport{
		Pairs: len(pairs), TaskGroups: len(groups), RawAgreement: raw, RawAgreementInterval: percentileInterval(rawReplicates),
		CohenKappa: kappa, CohenKappaDefined: defined, BootstrapIterations: iterations,
		DefinedKappaReplicates: len(kappaReplicates), BootstrapSeed: seed,
	}
	if len(kappaReplicates) > 0 {
		report.CohenKappaInterval = percentileInterval(kappaReplicates)
	}
	states := make([]string, 0, len(prevalence))
	for state := range prevalence {
		states = append(states, string(state))
	}
	sort.Strings(states)
	for _, state := range states {
		report.LabelPrevalence = append(report.LabelPrevalence, StateCount{State: State(state), Count: prevalence[State(state)]})
	}
	return SealAgreementReport(report)
}

func SealAgreementReport(report AgreementReport) (AgreementReport, error) {
	report.SchemaVersion = AgreementSchemaVersion
	report.CanonicalPolicy = CanonicalPolicy
	report.Digest = ""
	digest, err := agreementReportDigest(report)
	if err != nil {
		return AgreementReport{}, err
	}
	report.Digest = digest
	return report, report.Validate()
}

func (report AgreementReport) Validate() error {
	if report.SchemaVersion != AgreementSchemaVersion || report.CanonicalPolicy != CanonicalPolicy || report.Pairs < 2 || report.TaskGroups < 1 ||
		report.TaskGroups > report.Pairs || report.BootstrapIterations < 10_000 || missing(report.BootstrapSeed) ||
		!boundedProbability(report.RawAgreement) || !validInterval(report.RawAgreementInterval, 0, 1) ||
		report.DefinedKappaReplicates < 0 || report.DefinedKappaReplicates > report.BootstrapIterations {
		return errors.New("outcome agreement identity, counts, raw agreement, bootstrap, or interval is invalid")
	}
	if report.CohenKappaDefined && (!finite(report.CohenKappa) || report.CohenKappa < -1 || report.CohenKappa > 1) {
		return errors.New("defined Cohen kappa is outside [-1,1]")
	}
	if report.DefinedKappaReplicates > 0 && !validInterval(report.CohenKappaInterval, -1, 1) {
		return errors.New("cohen kappa interval is invalid")
	}
	total := 0
	for index, item := range report.LabelPrevalence {
		if !adjudicatableState(item.State) || item.Count < 1 || index > 0 && report.LabelPrevalence[index-1].State >= item.State {
			return errors.New("outcome label prevalence must be positive, unique, and sorted")
		}
		total += item.Count
	}
	if total != report.Pairs*2 {
		return errors.New("outcome label prevalence denominator does not match paired labels")
	}
	expected, err := agreementReportDigest(report)
	if err != nil || report.Digest != expected {
		return errors.New("outcome agreement digest is invalid")
	}
	return nil
}

func agreementPoint(pairs []AgreementPair) (float64, float64, bool) {
	agree := 0
	left, right := make(map[State]int), make(map[State]int)
	for _, pair := range pairs {
		if pair.Left == pair.Right {
			agree++
		}
		left[pair.Left]++
		right[pair.Right]++
	}
	observed := float64(agree) / float64(len(pairs))
	expected := 0.0
	for state, leftCount := range left {
		expected += float64(leftCount*right[state]) / float64(len(pairs)*len(pairs))
	}
	if expected == 1 {
		return observed, 0, false
	}
	return observed, (observed - expected) / (1 - expected), true
}

func percentileInterval(values []float64) Interval {
	values = append([]float64(nil), values...)
	sort.Float64s(values)
	if len(values) == 0 {
		return Interval{}
	}
	lower := int(math.Floor(0.025 * float64(len(values)-1)))
	upper := int(math.Ceil(0.975 * float64(len(values)-1)))
	return Interval{Lower: values[lower], Upper: values[upper]}
}

func boundedProbability(value float64) bool { return finite(value) && value >= 0 && value <= 1 }
func finite(value float64) bool             { return !math.IsNaN(value) && !math.IsInf(value, 0) }
func validInterval(interval Interval, minimum, maximum float64) bool {
	return finite(interval.Lower) && finite(interval.Upper) && interval.Lower >= minimum && interval.Lower <= interval.Upper && interval.Upper <= maximum
}

func agreementReportDigest(value AgreementReport) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
