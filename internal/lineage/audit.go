package lineage

import "errors"

type StateCount struct {
	State TerminalState `json:"state"`
	Count int           `json:"count"`
}

type LayerFlow struct {
	FromLayer string `json:"from_layer"`
	ToLayer   string `json:"to_layer"`
	Entered   int    `json:"entered"`
	Survived  int    `json:"survived"`
	Lost      int    `json:"lost"`
}

type FormatResult struct {
	Format                  string       `json:"format"`
	ConsideredTaskGroups    int          `json:"considered_task_groups"`
	UniqueTaskGroups        int          `json:"unique_task_groups"`
	ExcludedTaskGroups      int          `json:"excluded_task_groups"`
	SourceSessions          int          `json:"source_sessions"`
	CandidateEvidenceEvents int          `json:"candidate_evidence_events"`
	DirectInvocations       int          `json:"direct_invocations"`
	TerminalCounts          []StateCount `json:"terminal_counts"`
	Flows                   []LayerFlow  `json:"flows"`
	Inferential             bool         `json:"inferential"`
}

type LineageAudit struct {
	Header               ArtifactHeader `json:"header"`
	AuditID              string         `json:"audit_id"`
	AttemptLedgerDigest  string         `json:"attempt_ledger_digest"`
	HoldoutResultsDigest string         `json:"holdout_results_digest"`
	ClaimBoundary        []string       `json:"claim_boundary"`
	ConsideredTaskGroups int            `json:"considered_task_groups"`
	IncludedTaskGroups   int            `json:"included_task_groups"`
	ExcludedTaskGroups   int            `json:"excluded_task_groups"`
	UnresolvedTaskGroups int            `json:"unresolved_task_groups"`
	Formats              []FormatResult `json:"formats"`
	BootstrapSeed        int64          `json:"bootstrap_seed"`
	BootstrapReplicates  int            `json:"bootstrap_replicates"`
	ConservationPassed   bool           `json:"conservation_passed"`
}

func (audit LineageAudit) Validate() error {
	if err := validateHeader(audit.Header, AuditSchemaVersion, []ParentRequirement{
		{Relation: "plan", SchemaVersions: []string{PlanSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true},
		{Relation: "assessment", SchemaVersions: []string{AssessmentSchemaVersion}, Minimum: 1, Maximum: -1, SameTask: true},
		{Relation: "capability", SchemaVersions: []string{CapabilitySchemaVersion}, Minimum: 1, Maximum: -1, SameTask: true},
	}); err != nil {
		return err
	}
	if missing(audit.AuditID) || !validDigest(audit.AttemptLedgerDigest) || !validDigest(audit.HoldoutResultsDigest) ||
		audit.ConsideredTaskGroups < 1 || audit.IncludedTaskGroups < 0 || audit.ExcludedTaskGroups < 0 || audit.UnresolvedTaskGroups < 0 ||
		audit.ConsideredTaskGroups != audit.IncludedTaskGroups+audit.ExcludedTaskGroups ||
		audit.BootstrapSeed != 20260810 || audit.BootstrapReplicates != 10000 || !audit.ConservationPassed || len(audit.Formats) == 0 {
		return errors.New("lineage audit identity, uncertainty, or conservation contract is invalid")
	}
	if err := validateSortedUnique("lineage audit claim boundary", audit.ClaimBoundary, 1); err != nil {
		return err
	}
	total := 0
	considered := 0
	excluded := 0
	unresolved := 0
	previousFormat := ""
	for _, format := range audit.Formats {
		if format.Format <= previousFormat {
			return errors.New("lineage audit format result is incomplete or unsorted")
		}
		if err := validateFormatConservation(format); err != nil {
			return errors.New("lineage audit format result violates conservation: " + err.Error())
		}
		total += format.UniqueTaskGroups
		considered += format.ConsideredTaskGroups
		excluded += format.TerminalCounts[0].Count
		unresolved += format.TerminalCounts[4].Count + format.TerminalCounts[5].Count + format.TerminalCounts[8].Count
		previousFormat = format.Format
	}
	if considered != audit.ConsideredTaskGroups || total != audit.IncludedTaskGroups || excluded != audit.ExcludedTaskGroups || unresolved != audit.UnresolvedTaskGroups {
		return errors.New("lineage audit format and terminal totals do not conserve task groups")
	}
	copy := audit
	copy.Header.Digest = ""
	return validateArtifactDigest(audit.Header.Digest, copy)
}
