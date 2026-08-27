package outcome

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"time"
)

type QualificationCase struct {
	ID                  string       `json:"id"`
	Packet              BlindPacket  `json:"packet"`
	ExpectedOutcome     State        `json:"expected_outcome"`
	RequiredReasonCodes []ReasonCode `json:"required_reason_codes"`
	Explanation         string       `json:"explanation"`
}

type QualificationSet struct {
	SchemaVersion   string              `json:"schema_version"`
	CanonicalPolicy string              `json:"canonical_policy"`
	RubricVersion   string              `json:"rubric_version"`
	PassingScore    float64             `json:"passing_score"`
	Cases           []QualificationCase `json:"cases"`
	Digest          string              `json:"digest"`
}

type QualificationCaseResult struct {
	CaseID               string `json:"case_id"`
	PacketID             string `json:"packet_id"`
	OutcomeCorrect       bool   `json:"outcome_correct"`
	RequiredReasonsFound bool   `json:"required_reasons_found"`
	Passed               bool   `json:"passed"`
}

type QualificationReport struct {
	SchemaVersion    string                    `json:"schema_version"`
	CanonicalPolicy  string                    `json:"canonical_policy"`
	QualificationSet string                    `json:"qualification_set_digest"`
	RubricVersion    string                    `json:"rubric_version"`
	AdjudicatorAlias string                    `json:"adjudicator_alias"`
	Cases            int                       `json:"cases"`
	PassedCases      int                       `json:"passed_cases"`
	Score            float64                   `json:"score"`
	PassingScore     float64                   `json:"passing_score"`
	Qualified        bool                      `json:"qualified"`
	QualifiedAt      string                    `json:"qualified_at"`
	CaseResults      []QualificationCaseResult `json:"case_results"`
	Digest           string                    `json:"digest"`
}

func DefaultQualificationSet() (QualificationSet, error) {
	type fixture struct {
		id          string
		content     string
		outcome     State
		reasons     []ReasonCode
		explanation string
	}
	fixtures := []fixture{
		{id: "qualification-01", content: "Task requirement: add an input validator. Patch evidence: validator rejects malformed input and preserves valid input. Independent pinned tests: all required tests passed with exit code 0.", outcome: StateSolved, reasons: []ReasonCode{ReasonIndependentTestsPass, ReasonTaskSatisfied}, explanation: "Independent task-aligned tests and patch evidence support a solved outcome."},
		{id: "qualification-02", content: "Task requirement: preserve backward compatibility. Independent pinned tests: the compatibility suite failed with exit code 1 after the patch. The failure is in the changed interface.", outcome: StateUnsolved, reasons: []ReasonCode{ReasonIndependentTestsFail, ReasonTaskUnsatisfied}, explanation: "A task-critical independent test failure makes the outcome unsolved."},
		{id: "qualification-03", content: "Task requirement: repair the parser. The disposable runner terminated at its resource deadline before tests completed. No task-level assertion was observed.", outcome: StateEnvironmentFail, reasons: []ReasonCode{ReasonEnvironmentFailure}, explanation: "A bounded runner failure is infrastructure evidence, not an unsolved task label."},
		{id: "qualification-04", content: "Task requirement A: the output must contain exactly one line. Task requirement B: the output must contain at least two nonempty lines. No precedence or exception is specified.", outcome: StateInvalidTask, reasons: []ReasonCode{ReasonInvalidTask}, explanation: "The supplied task requirements are mutually inconsistent."},
		{id: "qualification-05", content: "The trajectory claims that all checks passed. A later rerun reports a failure, but its environment revision and validator contract are absent. No complete executable log is available.", outcome: StateIndeterminate, reasons: []ReasonCode{ReasonEvidenceConflict, ReasonEvidenceInsufficient}, explanation: "Conflicting evidence without a valid independent execution contract remains indeterminate."},
	}
	key := []byte("evalwitness-public-qualification-key")
	cases := make([]QualificationCase, 0, len(fixtures))
	for _, item := range fixtures {
		request := BlindBuildRequest{
			SchemaVersion: BlindBuildSchemaVersion, PlanDigest: digestText("qualification-plan-v1"), TaskAlias: item.id,
			Evidence: []PacketEvidence{{Slot: "source-evidence", Kind: "synthetic_qualification_evidence", Content: item.content, ContentDigest: digestText(item.content), License: "MIT", Limitation: "synthetic rubric qualification case; not empirical study data"}},
			RubricQuestions: []string{
				"Are harmful side effects evident?", "Does the implementation satisfy the task?", "Is the available evidence sufficient?",
				"Is the implementation technically correct?", "Is verification adequate?",
			},
			PrivacyClass: "public_qualification", PublicReleasable: true, SourceCaseDigest: digestText(item.id),
			Condition: "qualification_fixture", ExpectedRelation: string(item.outcome),
			SlotMappings:  []SlotMapping{{Slot: "source-evidence", SourceDigest: digestText(item.id + "-source")}},
			BlindingKeyID: "public-qualification-v1", ForbiddenValues: []string{"evalwitness_verifier", "study_hypothesis"},
		}
		packet, _, err := BuildBlindedPacketFromRequest(request, key)
		if err != nil {
			return QualificationSet{}, err
		}
		cases = append(cases, QualificationCase{ID: item.id, Packet: packet, ExpectedOutcome: item.outcome, RequiredReasonCodes: item.reasons, Explanation: item.explanation})
	}
	return SealQualificationSet(QualificationSet{RubricVersion: "evalwitness.outcome-rubric.v1", PassingScore: 0.80, Cases: cases})
}

func SealQualificationSet(set QualificationSet) (QualificationSet, error) {
	set.SchemaVersion = QualificationSchemaVersion
	set.CanonicalPolicy = CanonicalPolicy
	set.Digest = ""
	digest, err := qualificationSetDigest(set)
	if err != nil {
		return QualificationSet{}, err
	}
	set.Digest = digest
	return set, set.Validate()
}

func (set QualificationSet) Validate() error {
	if set.SchemaVersion != QualificationSchemaVersion || set.CanonicalPolicy != CanonicalPolicy ||
		set.RubricVersion != "evalwitness.outcome-rubric.v1" || set.PassingScore != 0.80 || len(set.Cases) < 5 {
		return errors.New("outcome qualification set identity, rubric, threshold, or case count is invalid")
	}
	packetIDs := make(map[string]struct{}, len(set.Cases))
	for index, item := range set.Cases {
		if missing(item.ID, item.Explanation) || !adjudicatableState(item.ExpectedOutcome) || len(item.RequiredReasonCodes) == 0 ||
			index > 0 && set.Cases[index-1].ID >= item.ID {
			return errors.New("outcome qualification cases must be complete, unique, and sorted")
		}
		if err := item.Packet.Validate(); err != nil {
			return fmt.Errorf("outcome qualification case %q packet: %w", item.ID, err)
		}
		if item.Packet.PrivacyClass != "public_qualification" || !item.Packet.PublicReleasable {
			return fmt.Errorf("outcome qualification case %q packet is not public qualification material", item.ID)
		}
		if err := validReasonCodes(item.RequiredReasonCodes); err != nil {
			return fmt.Errorf("outcome qualification case %q: %w", item.ID, err)
		}
		if _, duplicate := packetIDs[item.Packet.PacketID]; duplicate {
			return errors.New("outcome qualification set repeats a packet")
		}
		packetIDs[item.Packet.PacketID] = struct{}{}
	}
	expected, err := qualificationSetDigest(set)
	if err != nil || set.Digest != expected {
		return errors.New("outcome qualification set digest is invalid")
	}
	return nil
}

func ScoreQualification(set QualificationSet, labels []Label) (QualificationReport, error) {
	if err := set.Validate(); err != nil {
		return QualificationReport{}, err
	}
	if len(labels) != len(set.Cases) {
		return QualificationReport{}, errors.New("qualification requires exactly one label per case")
	}
	labelsByPacket := make(map[string]Label, len(labels))
	adjudicator := ""
	var qualifiedAt time.Time
	for _, label := range labels {
		if err := label.Validate(); err != nil {
			return QualificationReport{}, err
		}
		if label.QualificationDigest != set.Digest || label.ReviewerSlot != 1 {
			return QualificationReport{}, errors.New("qualification label does not bind the set or reviewer slot one")
		}
		if adjudicator == "" {
			adjudicator = label.AdjudicatorAlias
		} else if adjudicator != label.AdjudicatorAlias {
			return QualificationReport{}, errors.New("qualification labels must belong to one adjudicator")
		}
		if _, duplicate := labelsByPacket[label.PacketID]; duplicate {
			return QualificationReport{}, errors.New("qualification labels repeat a packet")
		}
		submittedAt, err := time.Parse(time.RFC3339, label.SubmittedAt)
		if err != nil {
			return QualificationReport{}, errors.New("qualification label submission time must be RFC3339")
		}
		if submittedAt.After(qualifiedAt) {
			qualifiedAt = submittedAt
		}
		labelsByPacket[label.PacketID] = label
	}
	report := QualificationReport{
		QualificationSet: set.Digest, RubricVersion: set.RubricVersion, AdjudicatorAlias: adjudicator, Cases: len(set.Cases), PassingScore: set.PassingScore,
		QualifiedAt: qualifiedAt.UTC().Format(time.RFC3339),
	}
	for _, item := range set.Cases {
		label, exists := labelsByPacket[item.Packet.PacketID]
		if !exists {
			return QualificationReport{}, fmt.Errorf("qualification label missing packet %q", item.Packet.PacketID)
		}
		outcomeCorrect := label.PrimaryOutcome == item.ExpectedOutcome
		reasonsFound := true
		for _, required := range item.RequiredReasonCodes {
			reasonsFound = reasonsFound && slices.Contains(label.ReasonCodes, required)
		}
		passed := outcomeCorrect && reasonsFound
		if passed {
			report.PassedCases++
		}
		report.CaseResults = append(report.CaseResults, QualificationCaseResult{
			CaseID: item.ID, PacketID: item.Packet.PacketID, OutcomeCorrect: outcomeCorrect,
			RequiredReasonsFound: reasonsFound, Passed: passed,
		})
	}
	report.Score = float64(report.PassedCases) / float64(report.Cases)
	report.Qualified = report.Score >= report.PassingScore
	return SealQualificationReport(report)
}

func SealQualificationReport(report QualificationReport) (QualificationReport, error) {
	report.SchemaVersion = QualificationReportSchemaVersion
	report.CanonicalPolicy = CanonicalPolicy
	report.Digest = ""
	digest, err := qualificationReportDigest(report)
	if err != nil {
		return QualificationReport{}, err
	}
	report.Digest = digest
	return report, report.Validate()
}

func (report QualificationReport) Validate() error {
	if report.SchemaVersion != QualificationReportSchemaVersion || report.CanonicalPolicy != CanonicalPolicy ||
		!validDigest(report.QualificationSet) || report.RubricVersion != "evalwitness.outcome-rubric.v1" || missing(report.AdjudicatorAlias, report.QualifiedAt) || report.Cases < 5 || report.PassedCases < 0 || report.PassedCases > report.Cases ||
		report.Score != float64(report.PassedCases)/float64(report.Cases) || report.PassingScore != 0.80 || report.Qualified != (report.Score >= report.PassingScore) || len(report.CaseResults) != report.Cases {
		return errors.New("outcome qualification report identity, counts, score, threshold, or status is invalid")
	}
	if _, err := time.Parse(time.RFC3339, report.QualifiedAt); err != nil {
		return errors.New("outcome qualification report time must be RFC3339")
	}
	for index, item := range report.CaseResults {
		if missing(item.CaseID) || !validOpaquePacketID(item.PacketID) || item.Passed != (item.OutcomeCorrect && item.RequiredReasonsFound) ||
			index > 0 && report.CaseResults[index-1].CaseID >= item.CaseID {
			return errors.New("outcome qualification results must be valid, unique, and sorted")
		}
	}
	expected, err := qualificationReportDigest(report)
	if err != nil || report.Digest != expected {
		return errors.New("outcome qualification report digest is invalid")
	}
	return nil
}

func ValidateReviewerQualification(label Label, report QualificationReport) error {
	if err := label.Validate(); err != nil {
		return err
	}
	if err := report.Validate(); err != nil {
		return err
	}
	if !report.Qualified || label.AdjudicatorAlias != report.AdjudicatorAlias || label.QualificationDigest != report.Digest {
		return errors.New("outcome label is not bound to a passing qualification report for its adjudicator")
	}
	return nil
}

func DecodeQualificationSet(reader io.Reader) (QualificationSet, error) {
	var value QualificationSet
	if err := decodeStrict(reader, &value); err != nil {
		return QualificationSet{}, fmt.Errorf("decode outcome qualification set: %w", err)
	}
	return value, value.Validate()
}

func DecodeQualificationReport(reader io.Reader) (QualificationReport, error) {
	var value QualificationReport
	if err := decodeStrict(reader, &value); err != nil {
		return QualificationReport{}, fmt.Errorf("decode outcome qualification report: %w", err)
	}
	return value, value.Validate()
}

func DecodeQualificationLabels(reader io.Reader) ([]Label, error) {
	var values []Label
	if err := decodeStrict(reader, &values); err != nil {
		return nil, fmt.Errorf("decode outcome qualification labels: %w", err)
	}
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return nil, fmt.Errorf("qualification label %d: %w", index, err)
		}
	}
	sort.Slice(values, func(left, right int) bool { return values[left].PacketID < values[right].PacketID })
	return values, nil
}

func qualificationSetDigest(value QualificationSet) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}

func qualificationReportDigest(value QualificationReport) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
