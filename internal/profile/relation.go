package profile

// RelationDimension preserves TASK 068 raw pair-axis agreement, formal-human support, and admissibility.
type RelationDimension struct {
	Family        string `json:"family"`
	Agreement     string `json:"agreement"`
	FormalHuman   string `json:"formal_human"`
	Admissibility string `json:"admissibility"`
	Status        Status `json:"status"`
}

// RelationDimensions returns TASK 068 controlled-relation dimensions.
func RelationDimensions() []RelationDimension {
	return []RelationDimension{
		{Family: "candidate_order", Agreement: "pair_axis", FormalHuman: "unresolved", Admissibility: "068 admitted only", Status: StatusNotMeasured},
		{Family: "omitted_evidence", Agreement: "pair_axis", FormalHuman: "formal_only", Admissibility: "sentinel descriptive", Status: StatusNotMeasured},
	}
}
