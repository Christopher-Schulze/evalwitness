package preprocess

const (
	EvidenceEventScorePolicyVersion      = "evalwitness.evidence-event-score.v1"
	LegacyEvidenceLineScorePolicyVersion = "evalwitness.legacy-evidence-line-score.v1"
	CanonicalEvidenceSelector            = "CanonicalPipeline/ApplyEvidenceBudget"
	CanonicalEvidenceRenderer            = "preprocess.RenderTrajectory"
	LegacyEvidenceSelector               = "Pipeline/EvidenceSlice"
)

type EvidenceSelectionPolicyInventory struct {
	CanonicalSelector    string `json:"canonical_selector"`
	CanonicalScorePolicy string `json:"canonical_score_policy"`
	CanonicalRenderer    string `json:"canonical_renderer"`
	LegacySelector       string `json:"legacy_selector"`
	LegacyScorePolicy    string `json:"legacy_score_policy"`
}

type EvidenceEventScoreInspection struct {
	EventID   string    `json:"event_id"`
	EventKind EventKind `json:"event_kind"`
	Score     int       `json:"score"`
}

type EvidenceLineScoreInspection struct {
	LineDigest string `json:"line_digest"`
	Score      int    `json:"score"`
}

func InspectEvidenceSelectionPolicies() EvidenceSelectionPolicyInventory {
	return EvidenceSelectionPolicyInventory{
		CanonicalSelector: CanonicalEvidenceSelector, CanonicalScorePolicy: EvidenceEventScorePolicyVersion,
		CanonicalRenderer: CanonicalEvidenceRenderer,
		LegacySelector:    LegacyEvidenceSelector, LegacyScorePolicy: LegacyEvidenceLineScorePolicyVersion,
	}
}

func RenderCanonicalEvidenceEvent(event Event) string { return renderEvent(event) }

func InspectEvidenceEventScores(trajectory Trajectory) ([]EvidenceEventScoreInspection, error) {
	if err := trajectory.Validate(); err != nil {
		return nil, err
	}
	result := make([]EvidenceEventScoreInspection, len(trajectory.Events))
	for index, event := range trajectory.Events {
		result[index] = EvidenceEventScoreInspection{
			EventID: event.ID, EventKind: event.Kind,
			Score: evidenceEventScore(event, index, len(trajectory.Events)),
		}
	}
	return result, nil
}

func InspectLegacyEvidenceLineScores(lines []string) []EvidenceLineScoreInspection {
	result := make([]EvidenceLineScoreInspection, len(lines))
	for index, line := range lines {
		result[index] = EvidenceLineScoreInspection{LineDigest: digestBytes([]byte(line)), Score: evidenceLineScore(line)}
	}
	return result
}
