package verifier

type DecisionState string

const (
	DecisionSelected        DecisionState = "selected"
	DecisionTied            DecisionState = "tied"
	DecisionAbstained       DecisionState = "abstained"
	DecisionInvalid         DecisionState = "invalid"
	DecisionBudgetExhausted DecisionState = "budget_exhausted"
	DecisionProviderFailed  DecisionState = "provider_failed"
)

type AbstentionReason string

const (
	AbstentionNone                AbstentionReason = ""
	AbstentionInsufficientMargin  AbstentionReason = "insufficient_margin"
	AbstentionUnstableOrder       AbstentionReason = "unstable_presentation_order"
	AbstentionEvidenceCeiling     AbstentionReason = "evidence_ceiling_reached"
	AbstentionInsufficientSupport AbstentionReason = "insufficient_support"
)

type EvidenceStrength struct {
	ExtractionMode        ExtractionMode `json:"extraction_mode"`
	Observations          int            `json:"observations"`
	ExtractedObservations int            `json:"extracted_observations"`
	MinimumReturnedTopK   int            `json:"minimum_returned_top_k"`
	MinimumVisibleMass    float64        `json:"minimum_visible_probability_mass"`
	MeanVisibleMass       float64        `json:"mean_visible_probability_mass"`
	MinimumValidMass      float64        `json:"minimum_valid_score_mass"`
	MeanValidMass         float64        `json:"mean_valid_score_mass"`
}

func SummarizeEvidenceStrength(evidence []ScoreEvidence) EvidenceStrength {
	strength := EvidenceStrength{
		MinimumReturnedTopK: int(^uint(0) >> 1),
		MinimumVisibleMass:  1,
		MinimumValidMass:    1,
	}
	if len(evidence) == 0 {
		strength.MinimumReturnedTopK = 0
		strength.MinimumVisibleMass = 0
		strength.MinimumValidMass = 0
		return strength
	}
	strength.ExtractionMode = evidence[0].ExtractionMode
	for _, item := range evidence {
		strength.Observations++
		if item.Extracted {
			strength.ExtractedObservations++
		}
		if item.ExtractionMode != strength.ExtractionMode {
			strength.ExtractionMode = ExtractionModeMixed
		}
		if item.ReturnedTopK < strength.MinimumReturnedTopK {
			strength.MinimumReturnedTopK = item.ReturnedTopK
		}
		if item.VisibleProbabilityMass < strength.MinimumVisibleMass {
			strength.MinimumVisibleMass = item.VisibleProbabilityMass
		}
		if item.ValidScoreMass < strength.MinimumValidMass {
			strength.MinimumValidMass = item.ValidScoreMass
		}
		strength.MeanVisibleMass += item.VisibleProbabilityMass
		strength.MeanValidMass += item.ValidScoreMass
	}
	denominator := float64(strength.Observations)
	strength.MeanVisibleMass /= denominator
	strength.MeanValidMass /= denominator
	return strength
}
