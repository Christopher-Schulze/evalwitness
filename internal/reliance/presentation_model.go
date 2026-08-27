package reliance

const PresentationOrderPlanSchemaVersion = "evalwitness.presentation-order-plan.v1"

type PresentationOrderStatus string

const (
	PresentationOrderAvailable   PresentationOrderStatus = "available"
	PresentationOrderUnsupported PresentationOrderStatus = "unsupported"
)

type PresentationOrderReason string

const (
	PresentationReasonDependencyCycle PresentationOrderReason = "dependency_cycle"
	PresentationReasonNarrativeAbsent PresentationOrderReason = "narrative_factor_absent"
	PresentationReasonNoOrderContrast PresentationOrderReason = "no_dependency_valid_order_contrast"
)

type PresentationOrderPlan struct {
	SchemaVersion            string                  `json:"schema_version"`
	CanonicalPolicy          string                  `json:"canonical_policy"`
	PolicyVersion            string                  `json:"policy_version"`
	CellDigest               string                  `json:"cell_digest"`
	AssignmentSetDigest      string                  `json:"assignment_set_digest"`
	TrajectoryDigest         string                  `json:"trajectory_digest"`
	NarrativeEventIDs        []string                `json:"narrative_event_ids"`
	NarrativeFirstEventIDs   []string                `json:"narrative_first_event_ids"`
	NarrativeLastEventIDs    []string                `json:"narrative_last_event_ids"`
	NarrativeFirstTextDigest string                  `json:"narrative_first_text_digest,omitempty"`
	NarrativeLastTextDigest  string                  `json:"narrative_last_text_digest,omitempty"`
	Status                   PresentationOrderStatus `json:"status"`
	Reason                   PresentationOrderReason `json:"reason,omitempty"`
	ProviderCalls            int                     `json:"provider_calls"`
	NetworkRequired          bool                    `json:"network_required"`
	Digest                   string                  `json:"digest"`
}
