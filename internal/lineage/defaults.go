package lineage

func defaultResearchQuestions() []ResearchQuestion {
	return []ResearchQuestion{
		{ID: "RQ1", Question: "At which layer does verification lineage first become unprovable?", PrimaryObservable: "earliest typed loss state per task group", ForbiddenInterpretation: "missing export text means the agent did not verify"},
		{ID: "RQ2", Question: "Which trace formats can represent each required lineage field?", PrimaryObservable: "preregistered format capability vector from specification and golden vectors", ForbiddenInterpretation: "one observed trace proves format-wide capability"},
		{ID: "RQ3", Question: "How much proven verification survives native export, canonicalization, slicing, and request construction?", PrimaryObservable: "paired task-cluster survival proportions and exact flow counts", ForbiddenInterpretation: "survival proves verification quality or task success"},
		{ID: "RQ4", Question: "Which apparent checks are non-failable, provenance-contaminated, stale, or evidence-preserving no-ops?", PrimaryObservable: "closed semantic assessment states and counterexamples", ForbiddenInterpretation: "text matching establishes executable verification"},
		{ID: "RQ5", Question: "Does a frozen parser transfer to unseen formats or wrapper patterns?", PrimaryObservable: "format-holdout and syntax-family-holdout false acceptance, false rejection, and unresolved counts", ForbiddenInterpretation: "development-vector performance is generalization evidence"},
		{ID: "RQ6", Question: "Is a balanced future omitted-evidence panel feasible without weakening the construct?", PrimaryObservable: "at least 20 calibration and 20 test task groups after all lineage gates", ForbiddenInterpretation: "scarcity may be repaired by changing thresholds after observation"},
	}
}

func defaultUnitPlan() UnitPlan {
	return UnitPlan{
		PrimaryUnit: PrimaryUnit, ClusterUnit: ClusterUnit,
		NestedUnits: []string{"call", "result_chunk", "retry", "wrapper_child"},
		RetryPolicy: "nest_retries_under_original_task_group", DenominatorPolicy: "one_terminal_state_per_included_task_group",
	}
}

func defaultSourceClasses() []SourceClassPlan {
	return []SourceClassPlan{
		{ID: "checked_in_controls", Basis: "public synthetic golden vectors and frozen historical negative controls", PermittedRoles: []DataRole{RoleAdapterDevelopment, RoleAdversarialChallenge}, SourceManifestRequired: true, RedistributionRequired: true, PreviouslyInspectedUse: "development_or_challenge_only"},
		{ID: "explicitly_authorized_owner_captures", Basis: "local captures with explicit owner authorization and consent", PermittedRoles: []DataRole{RoleAdapterDevelopment, RoleCaptureCalibration, RoleLockedTest}, SourceManifestRequired: true, ExplicitConsentRequired: true, PreviouslyInspectedUse: "role_assigned_before_content_inspection"},
		{ID: "public_licensed_native_exports", Basis: "public source with verified license and redistribution terms", PermittedRoles: []DataRole{RoleAdapterDevelopment, RoleCaptureCalibration, RoleLockedTest}, SourceManifestRequired: true, RedistributionRequired: true, PreviouslyInspectedUse: "role_assigned_before_content_inspection"},
	}
}

func defaultClusterPlan() ClusterPlan {
	return ClusterPlan{
		Dimensions:    []string{"lineage_id", "near_duplicate_id", "repository_id", "source_session_id", "task_id"},
		IsolatedRoles: canonicalRoles(), CrossRolePolicy: "forbid_any_shared_cluster_dimension",
		PairedViewPolicy:  "paired_format_views_share_one_role_and_task_group",
		NearDuplicateRule: "normalized_task_and_repository_similarity_declared_before_role_assignment",
	}
}

func defaultRolePlans() []RolePlan {
	return []RolePlan{
		{Role: RoleAdapterDevelopment, Purpose: "implement adapters, schemas, and diagnostics", ParserChangesPermitted: true},
		{Role: RoleCaptureCalibration, Purpose: "calibrate frozen capture alignment and uncertainty without parser changes", MinimumEligibleTaskGroups: 20},
		{Role: RoleLockedTest, Purpose: "single locked confirmatory evaluation", MinimumEligibleTaskGroups: 20, ConfirmatoryClaims: true},
		{Role: RoleAdversarialChallenge, Purpose: "falsify parser, provenance, failability, and claim-boundary assumptions", MinimumEligibleTaskGroups: 2},
	}
}

func defaultMissingnessPlan() MissingnessPlan {
	return MissingnessPlan{
		TerminalStates:     terminalStateContract(),
		ClassificationRule: "assign_first_proven_terminal_state_by_precedence",
		ConservationRule:   "included_equals_eligible_plus_all_terminal_losses_and_ineligible_states",
	}
}

func terminalStateContract() []TerminalStateRule {
	return []TerminalStateRule{
		{State: StateInvalidCapture, Precedence: 1, Disposition: DispositionExcluded, ProofRequirement: "capture manifest or completeness invariant failed before analysis"},
		{State: StateBehaviorAbsent, Precedence: 2, Disposition: DispositionLoss, ProofRequirement: "authoritative execution witness proves the closed capture boundary and no in-scope invocation"},
		{State: StateExportObservabilityAbsent, Precedence: 3, Disposition: DispositionLoss, ProofRequirement: "execution witness proves invocation while complete native export lacks it"},
		{State: StateAdapterMappingLoss, Precedence: 4, Disposition: DispositionLoss, ProofRequirement: "native field exists and field-accounting proves canonical loss"},
		{State: StateUnsupportedShell, Precedence: 5, Disposition: DispositionIneligible, ProofRequirement: "execution is present but frozen parser explicitly rejects its syntax"},
		{State: StateAmbiguousLineage, Precedence: 6, Disposition: DispositionIneligible, ProofRequirement: "required call, result, state, or task edges are incomplete or conflicting"},
		{State: StateNonFailableVerification, Precedence: 7, Disposition: DispositionIneligible, ProofRequirement: "bound invocation cannot falsify the claimed property"},
		{State: StateClaimSpecificEvidenceNotWeakened, Precedence: 8, Disposition: DispositionIneligible, ProofRequirement: "an equivalent decisive evidence channel survives the transformation"},
		{State: StateFreshnessUnresolved, Precedence: 9, Disposition: DispositionIneligible, ProofRequirement: "observation state or later invalidation boundary cannot be proven"},
		{State: StateDirectVerificationInvocation, Precedence: 10, Disposition: DispositionEligible, ProofRequirement: "failable bound invocation survives all five layers with current state lineage"},
	}
}

func defaultExclusions() []ExclusionRule {
	return []ExclusionRule{
		{ID: "compatibility_mode", Stage: "adapter_admission", Rule: "unversioned compatibility recovery cannot enter confirmatory roles", Treatment: "exclude_before_primary_denominator"},
		{ID: "cross_role_cluster_leakage", Stage: "role_assignment", Rule: "any shared cluster dimension across roles invalidates the assignment", Treatment: "exclude_before_primary_denominator"},
		{ID: "duplicate_or_near_duplicate", Stage: "role_assignment", Rule: "duplicate and prespecified near-duplicate groups remain in one role", Treatment: "exclude_before_primary_denominator"},
		{ID: "invalid_capture", Stage: "capture_admission", Rule: "capture completeness or authoritative-boundary invariant failed", Treatment: "exclude_before_primary_denominator"},
		{ID: "license_or_redistribution_unresolved", Stage: "source_admission", Rule: "source use or public projection lacks verified terms", Treatment: "exclude_before_primary_denominator"},
		{ID: "post_lock_source_change", Stage: "source_admission", Rule: "source bytes or manifest changed after role lock", Treatment: "exclude_before_primary_denominator"},
		{ID: "privacy_policy_failure", Stage: "source_admission", Rule: "secret, private reasoning, path, account, or undeclared omission policy failed", Treatment: "exclude_before_primary_denominator"},
		{ID: "source_not_authorized", Stage: "source_admission", Rule: "source lacks public basis or explicit owner authorization", Treatment: "exclude_before_primary_denominator"},
	}
}

func defaultReplacementPlan() ReplacementPlan {
	return ReplacementPlan{
		PermittedStage: "before_content_or_outcome_inspection",
		MatchingRule:   "same_preregistered_source_class_format_and_role_stratum",
		LineageRule:    "new_source_identity_and_recorded_replacement_edge",
		ShortfallRule:  "retain_shortfall_when_no_prespecified_replacement_exists",
	}
}

func defaultStoppingPlan() StoppingPlan {
	return StoppingPlan{
		MaximumAdmittedTaskGroups: 240, MaximumSourceSessions: 120, MaximumAcquisitionDays: 30,
		StopRule: "stop_at_first_hard_limit_or_exhausted_preregistered_inventory_never_on_observed_result",
	}
}

func defaultMinimumSupportPlan() MinimumSupportPlan {
	return MinimumSupportPlan{
		NativeTraceFormats: 3, IndependentAgentEcosystems: 2, TaskGroupsPerInferentialFormat: 20,
		CalibrationTaskGroups: 20, TestTaskGroups: 20, PairedWitnessRequired: true,
	}
}

func defaultUncertaintyPlan() UncertaintyPlan {
	return UncertaintyPlan{
		ConfidenceLevel: 0.95, ProportionInterval: "clopper_pearson_exact_task_cluster",
		PairedDifferenceInterval: "task_cluster_percentile_bootstrap", BootstrapReplicates: 10000, BootstrapSeed: 20260810,
		MultiplicityPolicy:      "none_descriptive_primary_estimands",
		PrimaryEstimands:        []string{"adjacent_layer_survival_proportion_by_format", "terminal_loss_state_proportion_by_format"},
		RawEventCountsSecondary: true,
	}
}

func defaultHoldouts() []HoldoutPlan {
	recovery := "forbidden_results_remain_immutable_recovery_requires_protocol_v2"
	return []HoldoutPlan{
		{ID: "format-holdout-v1", Kind: HoldoutFormat, SelectionSeed: "task_069-format-holdout-20260810-v1", SelectionRule: "rank format IDs by SHA-256(seed NUL format_id) and hold out the greatest digest", MinimumHeldOutUnits: 1, FrozenSurface: "shared generic lineage extractor and all non-held-out mappings", PostResultRecovery: recovery},
		{ID: "syntax-family-holdout-v1", Kind: HoldoutSyntaxFamily, SelectionSeed: "task_069-syntax-holdout-20260810-v1", SelectionRule: "rank syntax family IDs by SHA-256(seed NUL family_id) and hold out the greatest digest", MinimumHeldOutUnits: 1, FrozenSurface: "invocation parser vocabulary and wrapper semantics", PostResultRecovery: recovery},
	}
}

func defaultClaimPlan() ClaimPlan {
	return ClaimPlan{
		Allowed: []ClaimCeiling{
			{ID: "corpus_feasibility", MaximumScope: "locked TASK-069 source population and 20-calibration/20-test threshold", RequiredEvidence: "sealed acquisition funnel and role-isolated eligibility audit"},
			{ID: "held_out_parser_transfer", MaximumScope: "frozen held-out format or syntax families only", RequiredEvidence: "immutable holdout predictions and labeled outcomes"},
			{ID: "observed_capture_fields", MaximumScope: "admitted observed captures by pinned source format", RequiredEvidence: "source manifests and exact field-accounting reports"},
			{ID: "pinned_format_representability", MaximumScope: "pinned specification commit and normative vectors", RequiredEvidence: "capability vector and conformance evidence"},
			{ID: "task_cluster_lineage_survival", MaximumScope: "admitted task groups by format and frozen parser", RequiredEvidence: "paired five-layer audit with exact conservation"},
			{ID: "terminal_loss_distribution", MaximumScope: "admitted task groups by format and source population", RequiredEvidence: "exclusive terminal-state audit and uncertainty contract"},
		},
		Forbidden: []string{
			"agent_did_not_verify_from_missing_trace", "causal_performance_impact", "format_is_lossless", "hidden_reasoning",
			"human_validated", "provider_quality", "universal_verification_frequency",
		},
	}
}

func defaultAcquisitionPlan() AcquisitionPlan {
	return AcquisitionPlan{
		State: AcquisitionNotStarted, CountInspectionState: CountsNotInspectedBeforePlanLock,
		ExternalActionStatus: ExternalActionNotAuthorized, OwnerAuthorizationRequired: true,
		PreAcquisitionRequirements: []string{
			"adapter_and_parser_freeze", "capability_vectors_sealed", "golden_vectors_sealed",
			"source_manifests_sealed", "ten_schema_inventory_sealed",
		},
		LiveCaptureCommandStatus: "undeclared_until_explicit_owner_authorization",
	}
}

func canonicalRoles() []DataRole {
	return []DataRole{RoleAdapterDevelopment, RoleAdversarialChallenge, RoleCaptureCalibration, RoleLockedTest}
}
