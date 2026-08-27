package study

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/stats"
)

func TestLockAndLifecycleAreCanonicalAndAppendOnly(t *testing.T) {
	manifest := validManifest(t)
	first, err := Lock(manifest, "researcher@example.test")
	if err != nil {
		t.Fatal(err)
	}
	second, err := Lock(manifest, "researcher@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if first.RecordDigest != second.RecordDigest || first.Study.ManifestDigest != second.Study.ManifestDigest {
		t.Fatal("identical manifests did not produce stable identities")
	}
	lockedRoute := first.Study.Manifest.Arms[0].RouteID
	manifest.Arms[0].RouteID = digest("caller-mutation")
	if first.Study.Manifest.Arms[0].RouteID != lockedRoute || first.Validate() != nil {
		t.Fatal("locked study retained mutable manifest aliases")
	}
	authorized, err := Transition(first, StateAuthorized, manifest.Identity.LockedAt.Add(time.Hour), "reviewer@example.test", "approved frozen protocol", []string{digest("b")})
	if err != nil {
		t.Fatal(err)
	}
	if authorized.State != StateAuthorized || authorized.Study.StudyID != first.Study.StudyID || authorized.RecordDigest == first.RecordDigest {
		t.Fatalf("authorized record identity = %#v", authorized)
	}
	if err := authorized.Validate(); err != nil {
		t.Fatal(err)
	}
	first.Events[0].Actor = "mutated caller"
	if authorized.Events[0].Actor == first.Events[0].Actor || authorized.Validate() != nil {
		t.Fatal("lifecycle transition retained mutable record aliases")
	}
	tampered := authorized
	tampered.Events[1].PreviousRecordDigest = digest("c")
	tampered.RecordDigest, _ = tampered.digest()
	if err := tampered.Validate(); err == nil {
		t.Fatal("tampered lifecycle link was accepted")
	}
}

func TestLifecycleRejectsAuthorizationWithoutLockedAttestations(t *testing.T) {
	record, err := Lock(validManifest(t), "researcher")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Transition(record, StateAuthorized, record.Study.Manifest.Identity.LockedAt.Add(time.Hour), "reviewer", "approve", []string{digest("c")}); err == nil {
		t.Fatal("authorization accepted an undeclared route attestation")
	}
	if _, err := Transition(record, StateAuthorized, record.Study.Manifest.Providers[0].AttestationExpiresAt.Add(time.Second), "reviewer", "approve", []string{digest("b")}); err == nil {
		t.Fatal("authorization accepted an expired route attestation")
	}
}

func TestLifecycleSnapshotsNestedEventsAndRestrictsAttestations(t *testing.T) {
	record := authorizedRecord(t)
	running, err := Transition(record, StateRunning, record.Events[1].At.Add(time.Hour), "runner", "start frozen execution", nil)
	if err != nil {
		t.Fatal(err)
	}
	record.Events[1].AttestationDigests[0] = digest("caller-mutation")
	if running.Events[1].AttestationDigests[0] == record.Events[1].AttestationDigests[0] || running.Validate() != nil {
		t.Fatal("lifecycle transition retained a nested event slice alias")
	}
	if _, err := Transition(running, StateComplete, running.Events[2].At.Add(time.Hour), "runner", "complete", []string{digest("b")}); err == nil {
		t.Fatal("non-authorization transition accepted route attestations")
	}
}

func TestSplitGenerationIsDeterministicAndKeepsLineageTogether(t *testing.T) {
	spec := SplitSpec{
		Seed: "frozen-seed", StratificationVariables: []string{"language"},
		Weights: []SplitWeight{{Role: RoleDevelopment, Weight: 6}, {Role: RoleCalibration, Weight: 2}, {Role: RoleTest, Weight: 2}},
	}
	groups := splitGroups(20)
	first, err := GenerateSplit(spec, groups)
	if err != nil {
		t.Fatal(err)
	}
	second, err := GenerateSplit(spec, groups)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest != second.Digest || len(first.Assignments) != len(groups) {
		t.Fatalf("split identity/count = %s/%d", first.Digest, len(first.Assignments))
	}
	for _, assignment := range first.Assignments {
		if len(assignment.TaskIDs) != 2 || len(assignment.PairObservationIDs) != 1 || len(assignment.MutationDescendantDigests) != 1 || len(assignment.EvidenceDescendantDigests) != 1 {
			t.Fatalf("lineage was fragmented: %#v", assignment)
		}
	}
}

func TestSplitValidationRejectsCrossGroupLeakage(t *testing.T) {
	manifest, err := GenerateSplit(SplitSpec{
		Seed: "seed", StratificationVariables: []string{"language"},
		Weights: []SplitWeight{{Role: RoleDevelopment, Weight: 1}, {Role: RoleTest, Weight: 1}},
	}, splitGroups(4))
	if err != nil {
		t.Fatal(err)
	}
	manifest.Assignments[1].TrajectoryDigests = append(manifest.Assignments[1].TrajectoryDigests, manifest.Assignments[0].TrajectoryDigests[0])
	manifest.Digest, _ = manifest.digest()
	if err := manifest.Validate(); err == nil {
		t.Fatal("trajectory lineage leaked across split groups without rejection")
	}
	manifest, err = GenerateSplit(SplitSpec{
		Seed: "seed", StratificationVariables: []string{"language"},
		Weights: []SplitWeight{{Role: RoleDevelopment, Weight: 1}, {Role: RoleTest, Weight: 1}},
	}, splitGroups(4))
	if err != nil {
		t.Fatal(err)
	}
	manifest.Assignments[1].PairObservationIDs = append(manifest.Assignments[1].PairObservationIDs, manifest.Assignments[0].PairObservationIDs[0])
	manifest.Digest, _ = manifest.digest()
	if err := manifest.Validate(); err == nil {
		t.Fatal("pair observation leaked across source-task groups without rejection")
	}
}

func TestSplitValidationRejectsNoncanonicalOrderAndStratumMutation(t *testing.T) {
	manifest, err := GenerateSplit(SplitSpec{
		Seed: "seed", StratificationVariables: []string{"language"},
		Weights: []SplitWeight{{Role: RoleDevelopment, Weight: 1}, {Role: RoleTest, Weight: 1}},
	}, splitGroups(4))
	if err != nil {
		t.Fatal(err)
	}
	manifest.Assignments[0].TaskIDs[0], manifest.Assignments[0].TaskIDs[1] = manifest.Assignments[0].TaskIDs[1], manifest.Assignments[0].TaskIDs[0]
	manifest.Digest, _ = manifest.digest()
	if err := manifest.Validate(); err == nil {
		t.Fatal("noncanonical identity order was accepted")
	}
	manifest, err = GenerateSplit(SplitSpec{
		Seed: "seed", StratificationVariables: []string{"language"},
		Weights: []SplitWeight{{Role: RoleDevelopment, Weight: 1}, {Role: RoleTest, Weight: 1}},
	}, splitGroups(4))
	if err != nil {
		t.Fatal(err)
	}
	manifest.Assignments[0].Stratum[0].Name = "posthoc"
	manifest.Digest, _ = manifest.digest()
	if err := manifest.Validate(); err == nil {
		t.Fatal("split stratum outside the declared variables was accepted")
	}
}

func TestCanonicalIdentityFramingPreventsDelimiterCollisions(t *testing.T) {
	if canonicalStringSetDigest([]string{"a", "b"}) == canonicalStringSetDigest([]string{"a\nb"}) {
		t.Fatal("canonical set digest is ambiguous across embedded delimiters")
	}
	if bytes.Equal(canonicalIdentityBytes("a", "b"), canonicalIdentityBytes("a\x00b")) {
		t.Fatal("canonical identity framing is ambiguous across embedded NUL bytes")
	}
}

func TestSplitRejectsIntegerOverflowingWeights(t *testing.T) {
	maximumInt := int(^uint(0) >> 1)
	_, err := GenerateSplit(SplitSpec{
		Seed: "seed", StratificationVariables: []string{"language"},
		Weights: []SplitWeight{{Role: RoleDevelopment, Weight: maximumInt}, {Role: RoleTest, Weight: maximumInt}},
	}, splitGroups(2))
	if err == nil {
		t.Fatal("integer-overflowing split weights were accepted")
	}
}

func TestManifestRejectsAccessedTestDataAndExploratoryPromotion(t *testing.T) {
	manifest := validManifest(t)
	manifest.Data.Datasets[0].PreviouslyAccessed = true
	if err := manifest.Validate(); err == nil {
		t.Fatal("previously accessed data was accepted as confirmatory test data")
	}
	manifest = validManifest(t)
	manifest.Outcomes.Exploratory = []Endpoint{{
		ID: "posthoc", Metric: "score", Direction: "higher", Question: "superiority", FailureDenominator: "all tasks",
	}}
	manifest.Inference.PrimaryFamily = append(manifest.Inference.PrimaryFamily, "posthoc")
	if err := manifest.Validate(); err == nil {
		t.Fatal("exploratory endpoint was promoted into the primary family")
	}
	manifest = validManifest(t)
	manifest.Outcomes.Exploratory = []Endpoint{{
		ID: "posthoc", Metric: "score", Direction: "higher", Question: "superiority", FailureDenominator: "all tasks",
	}}
	manifest.Publication.AllowedClaimIDs = []string{"posthoc"}
	if err := manifest.Validate(); err == nil {
		t.Fatal("exploratory endpoint was promoted into the publication claim allowlist")
	}
	manifest = validManifest(t)
	manifest.Publication.RegisteredReportTimestamp = manifest.Identity.LockedAt.Add(time.Second)
	if err := manifest.Validate(); err == nil {
		t.Fatal("post-lock registered-report timestamp was accepted")
	}
	manifest = validManifest(t)
	manifest.Data.Datasets[0].Exclusions = append(manifest.Data.Datasets[0].Exclusions, manifest.Data.Datasets[0].Exclusions[0])
	if err := manifest.Validate(); err == nil {
		t.Fatal("duplicate exclusion ID was accepted")
	}
	manifest = validManifest(t)
	manifest.Outcomes.Secondary[0].RiskTarget = math.NaN()
	if err := manifest.Validate(); err == nil {
		t.Fatal("non-finite endpoint value was accepted")
	}
	manifest = validManifest(t)
	manifest.Budget.HardCostUSD = math.Inf(1)
	if err := manifest.Validate(); err == nil {
		t.Fatal("non-finite budget value was accepted")
	}
}

func TestManifestBindsDatasetSplitsAndQuestionSpecificPower(t *testing.T) {
	manifest := validManifest(t)
	manifest.Data.Split.Assignments[0].DatasetID = "undeclared"
	manifest.Data.Split.Digest, _ = manifest.Data.Split.digest()
	if err := manifest.Validate(); err == nil {
		t.Fatal("split assignment for an undeclared dataset was accepted")
	}

	manifest = validManifest(t)
	manifest.Inference.PowerAtMinimumEffect = 0.99
	if err := manifest.Validate(); err == nil {
		t.Fatal("superiority design with false exact power was accepted")
	}

	manifest = validManifest(t)
	manifest.Data.Datasets[0].TaskIDsDigest = digest("wrong-task-set")
	if err := manifest.Validate(); err == nil {
		t.Fatal("dataset task digest not derived from split assignments was accepted")
	}

	manifest = validManifest(t)
	manifest.Outcomes.Primary.Question = string(stats.QuestionNonInferiority)
	manifest.Outcomes.Primary.Margin = 0.05
	manifest.Inference.DesignMethod = "paired_joint_outcome_simulation"
	manifest.Inference.PowerAtMinimumEffect = manifest.Inference.TargetPower
	if err := manifest.Validate(); err != nil {
		t.Fatalf("valid simulation-backed non-inferiority design rejected: %v", err)
	}
	manifest.Inference.DesignMethod = "exact_mcnemar_unconditional"
	if err := manifest.Validate(); err == nil {
		t.Fatal("non-inferiority design reused superiority power calculation")
	}
}

func TestKindSpecificGovernanceIsExclusiveAndComplete(t *testing.T) {
	manifest := validManifest(t)
	manifest.Relations = &ControlledRelations{CorpusVersion: "corpus.v1"}
	if err := manifest.Validate(); err == nil {
		t.Fatal("benchmark accepted an unrelated controlled-relation block")
	}

	manifest = validManifest(t)
	manifest.Identity.Kind = KindControlledRelation
	manifest.Relations = &ControlledRelations{
		CorpusVersion: "corpus.v1", RelationContractVersion: "relations.v1",
		MutationFamilies: []string{"irrelevant_tool_output"}, ExpectedRelations: []string{"invariant"},
		ValidatorDigests: []string{digest("validator")}, AmbiguityPolicy: "exclude before scoring",
		PrimaryDenominator: "all valid source-task relations", ClusterUnit: "source_task",
		ReductionPolicy: "retain source lineage", ClaimType: "invariance",
	}
	manifest.Reliability.RelationCorpusDigest = digest("relation-corpus")
	manifest.Reliability.ValidatorContractDigest = digest("validator-contract")
	if err := manifest.Validate(); err != nil {
		t.Fatalf("complete controlled-relation governance rejected: %v", err)
	}
	manifest.Relations.RelationContractVersion = ""
	if err := manifest.Validate(); err == nil {
		t.Fatal("controlled-relation study accepted an unversioned relation contract")
	}

	manifest = validManifest(t)
	manifest.Identity.Kind = KindEvidenceReliance
	manifest.EvidenceReliance = &EvidenceReliancePlan{
		FactorOntologyDigest: digest("factor-ontology"), AllowedFieldPaths: []string{"evidence.tests"},
		InterventionOperators: []string{"remove"}, EvidenceOnlyFamilies: []string{"tests"},
		IdentificationAssumptions: []string{"randomized intervention"}, Randomization: "within source task",
		MainEffects: []string{"tests"}, Interactions: []string{"tests:model"}, Estimators: []string{"paired contrast"},
		MultiplicityMethod: "holm", MultiplicityFamily: []string{"tests", "tests:model"},
		InvalidCaseHandling: "retain and report", ReductionPolicy: "source-task clustered", RelianceWitness: "counterfactual score change",
	}
	manifest.Reliability.EvidenceFactorDigest = digest("factor")
	manifest.Reliability.InterventionContractDigest = digest("intervention")
	if err := manifest.Validate(); err != nil {
		t.Fatalf("complete evidence-reliance governance rejected: %v", err)
	}
	manifest.EvidenceReliance.MultiplicityFamily = []string{"tests"}
	if err := manifest.Validate(); err == nil {
		t.Fatal("evidence-reliance study accepted an incomplete multiplicity family")
	}
}

func TestExecutionBindingRejectsDirtyBuildExtraInputAndChangedRoute(t *testing.T) {
	record := authorizedRecord(t)
	binding := validBinding(record)
	if err := VerifyExecutionBinding(record, binding); err != nil {
		t.Fatal(err)
	}
	dirty := binding
	dirty.Dirty = true
	if err := VerifyExecutionBinding(record, dirty); err == nil {
		t.Fatal("dirty build was accepted")
	}
	extra := binding
	extra.InputPaths = append(extra.InputPaths, "undeclared.json")
	extra.InputDigests = append(extra.InputDigests, digest("e"))
	if err := VerifyExecutionBinding(record, extra); err == nil {
		t.Fatal("undeclared input was accepted")
	}
	changedRoute := binding
	changedRoute.RouteID = digest("f")
	if err := VerifyExecutionBinding(record, changedRoute); err == nil {
		t.Fatal("undeclared route was accepted")
	}
	changedRetry := binding
	changedRetry.MaxRetries++
	if err := VerifyExecutionBinding(record, changedRetry); err == nil {
		t.Fatal("changed retry policy was accepted")
	}
	changedPacing := binding
	changedPacing.MinDispatchIntervalSeconds++
	if err := VerifyExecutionBinding(record, changedPacing); err == nil {
		t.Fatal("changed dispatch pacing was accepted")
	}
	changedIdentity := binding
	changedIdentity.ExpectedServedModel = "other-model"
	if err := VerifyExecutionBinding(record, changedIdentity); err == nil {
		t.Fatal("changed served identity policy was accepted")
	}
}

func TestManifestAcceptsOnlyCanonicalExactObservedServedModelSets(t *testing.T) {
	manifest := validManifest(t)
	manifest.Providers[0].ServedIdentityPolicy = provider.ServedIdentityPolicyExactObservedSet
	manifest.Providers[0].ExpectedServedModel = ""
	manifest.Providers[0].ExpectedServedModels = []string{"served-model", "served-model-versioned"}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("canonical served-model set rejected: %v", err)
	}

	unsorted := manifest
	unsorted.Providers = append([]ProviderPlan(nil), manifest.Providers...)
	unsorted.Providers[0].ExpectedServedModels = []string{"served-model-versioned", "served-model"}
	if err := unsorted.Validate(); err == nil {
		t.Fatal("unsorted served-model set was accepted")
	}

	unknownPolicy := manifest
	unknownPolicy.Providers = append([]ProviderPlan(nil), manifest.Providers...)
	unknownPolicy.Providers[0].ServedIdentityPolicy = "alias_family"
	if err := unknownPolicy.Validate(); err == nil {
		t.Fatal("unknown served identity policy was accepted")
	}
}

func TestCanonicalPathDigestBindsNamesAndBytesAndRejectsSymlinks(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "inputs")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "a.txt"), []byte("alpha"), 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := CanonicalPathDigest(root, "inputs")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "a.txt"), []byte("beta"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := CanonicalPathDigest(root, "inputs")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("input byte change did not change the canonical path digest")
	}
	if err := os.Mkdir(filepath.Join(directory, "empty"), 0o700); err != nil {
		t.Fatal(err)
	}
	third, err := CanonicalPathDigest(root, "inputs")
	if err != nil {
		t.Fatal(err)
	}
	if second == third {
		t.Fatal("empty directory addition did not change the canonical path digest")
	}
	if err := os.Symlink(filepath.Join(directory, "a.txt"), filepath.Join(directory, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := CanonicalPathDigest(root, "inputs"); err == nil {
		t.Fatal("symlink inside governed input was accepted")
	}
}

func TestStrictCodecRejectsUnknownFieldsAndReportContainsNoResults(t *testing.T) {
	manifest := validManifest(t)
	encoded, err := EncodeIndented(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeManifest(bytes.NewReader(encoded)); err != nil {
		t.Fatal(err)
	}
	unknown := bytes.Replace(encoded, []byte(`"canonical_policy"`), []byte(`"unknown":true,"canonical_policy"`), 1)
	if _, err := DecodeManifest(bytes.NewReader(unknown)); err == nil {
		t.Fatal("unknown manifest field was accepted")
	}
	report, err := ProtocolReport(authorizedRecord(t))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report, "Registered Study Protocol") || strings.Contains(strings.ToLower(report), "observed result") {
		t.Fatalf("protocol report content is invalid:\n%s", report)
	}
}

func TestGeneratedSchemasAreClosedAndVersioned(t *testing.T) {
	for _, document := range []string{"manifest", "record", "split"} {
		schema, err := Schema(document)
		if err != nil {
			t.Fatal(err)
		}
		if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" || schema["additionalProperties"] != false {
			t.Fatalf("%s schema is not a closed Draft 2020-12 schema: %#v", document, schema)
		}
		properties, ok := schema["properties"].(map[string]any)
		if !ok || len(properties) == 0 || properties["schema_version"] == nil {
			t.Fatalf("%s schema does not cover its fields", document)
		}
	}
}

func TestPublishedDevelopmentInventoryIsCompleteAndImmutable(t *testing.T) {
	file, err := os.Open("../../eval/governance/development-inventory.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close development inventory: %v", err)
		}
	}()
	var inventory DevelopmentInventory
	if err := DecodeStrict(file, &inventory); err != nil {
		t.Fatal(err)
	}
	if err := VerifyDevelopmentInventory("../..", inventory); err != nil {
		t.Fatal(err)
	}
	if len(inventory.Datasets) != 2 || inventory.Datasets[0].TaskCount+inventory.Datasets[1].TaskCount != 589 {
		t.Fatalf("inventory does not classify all 589 historical tasks: %#v", inventory.Datasets)
	}
}

func TestDevelopmentInventoryRejectsSymlinkArtifactsAndDuplicateDatasets(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.json")
	if err := os.WriteFile(target, []byte(`{"details":[{"task_name":"one"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "linked.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := readInventoryArtifact(root, "linked.json"); err == nil {
		t.Fatal("development inventory accepted a symbolic-link artifact")
	}
	inventory := DevelopmentInventory{
		SchemaVersion: DevelopmentInventorySchema,
		Policy:        "development only",
		Datasets: []DevelopmentDataset{
			{ID: "duplicate"},
			{ID: "duplicate"},
		},
	}
	if err := VerifyDevelopmentInventory(root, inventory); err == nil || !strings.Contains(err.Error(), "duplicate inventory dataset ID") {
		t.Fatalf("duplicate inventory dataset IDs were not rejected: %v", err)
	}
}

func validManifest(t *testing.T) Manifest {
	t.Helper()
	split, err := GenerateSplit(SplitSpec{
		Seed: "manifest-seed", StratificationVariables: []string{"language"},
		Weights: []SplitWeight{{Role: RoleDevelopment, Weight: 1}, {Role: RoleCalibration, Weight: 1}, {Role: RoleTest, Weight: 1}},
	}, splitGroups(50))
	if err != nil {
		t.Fatal(err)
	}
	var taskIDs []string
	var trajectoryDigests []string
	for _, assignment := range split.Assignments {
		taskIDs = append(taskIDs, assignment.TaskIDs...)
		trajectoryDigests = append(trajectoryDigests, assignment.TrajectoryDigests...)
	}
	created := time.Date(2026, 8, 9, 8, 0, 0, 0, time.UTC)
	return Manifest{
		SchemaVersion: ManifestSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		Identity: Identity{
			Title: "Frozen verifier comparison", ResearchQuestion: "Does the verifier improve task success?",
			Kind: KindBenchmark, Authors: []string{"researcher@example.test"}, CreatedAt: created, LockedAt: created.Add(time.Hour),
		},
		Hypotheses: Hypotheses{PrimaryNull: "paired effect <= 0", PrimaryAlternative: "paired effect > 0", Secondary: []string{"risk target"}, Exploratory: []string{"format strata"}},
		Data: DataPlan{PrimaryUnit: "task", Datasets: []DatasetManifest{{
			ID: "held-out", Source: "public benchmark", Version: "2026.1", License: "Apache-2.0", AcquiredAt: created,
			DatasetDigest: digest("1"), TaskIDsDigest: canonicalStringSetDigest(taskIDs), OutcomeLabelsDigest: digest("3"), TrajectorySetDigest: canonicalStringSetDigest(trajectoryDigests),
			TaskCount: 100, PermittedRoles: []DataRole{RoleDevelopment, RoleCalibration, RoleTest}, Exclusions: []Exclusion{{ID: "invalid-label", Rule: "label unavailable before scoring", Stage: "preflight", Treatment: "retain in denominator as invalid"}},
		}}, Split: split},
		Arms: []Arm{{
			ID: "verifier", Entrypoint: "eval-swebench", RouteID: digest("a"), ProviderID: "provider", RequestedModel: "model",
			PromptDigest: digest("5"), RequestContractDigest: digest("6"), ScorePolicyVersion: "evalwitness.strict-score-policy.v1",
			CalibrationDigest: digest("7"), SelectionMode: "pairwise", Candidates: 3, Repetitions: 1, AttestationDigest: digest("b"),
		}},
		Outcomes: OutcomePlan{
			Primary: Endpoint{
				ID: "task_success", Metric: "paired task success difference", Direction: "higher", Question: "superiority",
				FailureDenominator: "all locked test tasks",
			},
			Secondary: []Endpoint{{
				ID: "risk_target", Metric: "false preference risk", Direction: "lower", Question: "superiority",
				RiskTarget: 0.10, MinimumCoverage: 0.80, FailureDenominator: "all locked test tasks",
			}},
		},
		Inference: InferencePlan{
			Test: "exact_mcnemar", IntervalMethod: "newcombe_paired_score_method_10", DesignMethod: "exact_mcnemar_unconditional", DesignEvidenceDigest: digest("design"), ClusterUnit: "task",
			NominalAlpha: 0.05, TargetPower: 0.80, MinimumEffect: 1.00, DisagreementRate: 1.00,
			DiscordantWinProbability: 1.00, PowerAtMinimumEffect: 1.00, DecidableTasks: 100, MultiplicityMethod: "bonferroni",
			PrimaryFamily: []string{"task_success"}, SecondaryFamilies: [][]string{{"risk_target"}},
			Sequential: SequentialPlan{Method: "fixed_sample", MaximumLooks: 1},
		},
		Failures: FailurePlan{
			MissingScore: "failure", ProviderFailure: "failure", RouteFailure: "failure", Timeout: "failure",
			Abstention: "failure", BudgetExhaustion: "failed study", RetryExhaustion: "failure",
			IncompleteCell: "retain and report", DenominatorPolicy: "all locked test tasks",
		},
		Controls: ControlPlan{RandomSelectionID: "random", TaskIndependentSelector: "first-listed", PositiveControl: "known corruption", PositiveControlSource: RoleDevelopment},
		Providers: []ProviderPlan{{
			ArmID:                 "verifier",
			AttestationObservedAt: created.Add(-time.Hour), AttestationExpiresAt: created.Add(24 * time.Hour),
			ServedIdentityPolicy: "exact_observed", ExpectedServedModel: "served-model",
			CheckpointAssertionPolicy: "exact_observed", RetryPolicyVersion: provider.RetryPolicyVersion,
			MaxRetries: 5, RequestTimeoutSeconds: 120,
		}},
		Budget: BudgetPlan{ExpectedCalls: 100, HardCalls: 120, HardAttempts: 240, HardInputTokens: 1_000_000, HardOutputTokens: 500_000, HardDurationSeconds: 3600, HardConcurrent: 4, HardCostUSD: 10},
		Execution: ExecutionPlan{
			Commit: digest("8"), BinaryDigest: digest("9"), Platform: "darwin/arm64", AnalysisCommand: []string{"evalwitness", "eval-swebench"},
			AnalysisVersion: "evalwitness.evaluation.v2", AnalysisDigest: digest("c"),
			DeclaredInputPaths: []string{"data/held-out.json"}, DeclaredInputDigests: []string{digest("d")}, DeclaredRouteIDs: []string{digest("a")},
		},
		Publication: PublicationPlan{
			CapsuleVisibility: "public", AllowedClaimIDs: []string{"task_success"}, RequiredCaveats: []string{"one provider route"},
			IndependentReproductionGate: true, RegisteredReportTimestamp: created.Add(30 * time.Minute),
		},
		Reliability: ReliabilityContracts{
			ProtocolVersion: "1.2.0", ProtocolCorpusDigest: digest("e"), ProtocolRequestCorpusDigest: digest("request"),
			ProtocolSchemaDigest: digest("schema"), TraceMappingPolicy: "evalwitness.trace-mapping.v1",
			OutcomeContractDigest: digest("f"), AdjudicationContractDigest: digest("0"), ProfileProjectionDigest: digest("1"),
		},
		Adjudication: AdjudicationPlan{
			SampleStrata: []string{"success", "failure", "abstention"}, Blinding: "arm and route hidden", AgreementMetric: "Gwet AC1",
			ConflictResolution: "third blinded adjudicator", LabelRevision: "new manifest revision", SensitivityAnalysis: "original and revised labels reported",
		},
	}
}

func splitGroups(count int) []SplitGroup {
	groups := make([]SplitGroup, count)
	for index := range groups {
		id := fmt.Sprintf("%03d", index)
		language := "go"
		if index%2 == 1 {
			language = "python"
		}
		groups[index] = SplitGroup{
			DatasetID: "held-out", GroupID: "group-" + id, Stratum: []NamedValue{{Name: "language", Value: language}},
			TaskIDs: []string{"task-" + id + "-1", "task-" + id + "-2"}, RepositoryIDs: []string{"repo-" + id},
			CloneFamilyIDs: []string{"clone-" + id}, TrajectoryDigests: []string{digest(id)},
			PairObservationIDs:        []string{"pair-" + id},
			MutationDescendantDigests: []string{digest(id + "1")}, EvidenceDescendantDigests: []string{digest(id + "2")},
			AdjudicationPacketDigests: []string{digest(id + "3")}, CounterexampleDigests: []string{digest(id + "4")},
			CorpusVersionIDs: []string{"corpus-" + id},
		}
	}
	return groups
}

func authorizedRecord(t *testing.T) Record {
	t.Helper()
	manifest := validManifest(t)
	record, err := Lock(manifest, "researcher")
	if err != nil {
		t.Fatal(err)
	}
	record, err = Transition(record, StateAuthorized, manifest.Identity.LockedAt.Add(time.Hour), "reviewer", "approved", []string{digest("b")})
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func validBinding(record Record) ExecutionBinding {
	m := record.Study.Manifest
	arm := m.Arms[0]
	return ExecutionBinding{
		ArmID: arm.ID, Entrypoint: arm.Entrypoint, RouteID: arm.RouteID, RequestContractDigest: arm.RequestContractDigest,
		Commit: m.Execution.Commit, BinaryDigest: m.Execution.BinaryDigest, AnalysisDigest: m.Execution.AnalysisDigest,
		AnalysisCommand: append([]string(nil), m.Execution.AnalysisCommand...), AnalysisVersion: m.Execution.AnalysisVersion,
		InputPaths: append([]string(nil), m.Execution.DeclaredInputPaths...), InputDigests: append([]string(nil), m.Execution.DeclaredInputDigests...),
		ExpectedCalls: m.Budget.ExpectedCalls, HardCalls: m.Budget.HardCalls, HardAttempts: m.Budget.HardAttempts,
		HardInputTokens: m.Budget.HardInputTokens, HardOutputTokens: m.Budget.HardOutputTokens,
		HardDurationSeconds: m.Budget.HardDurationSeconds, HardConcurrent: m.Budget.HardConcurrent, HardCostUSD: m.Budget.HardCostUSD,
		DecidableTasks: m.Inference.DecidableTasks, NominalAlpha: m.Inference.NominalAlpha, TargetPower: m.Inference.TargetPower,
		MinimumEffect: m.Inference.MinimumEffect, DisagreementRate: m.Inference.DisagreementRate,
		DiscordantWinProbability: m.Inference.DiscordantWinProbability, PrimaryFamilySize: len(m.Inference.PrimaryFamily),
		PowerAtMinimumEffect: m.Inference.PowerAtMinimumEffect,
		ServedIdentityPolicy: m.Providers[0].ServedIdentityPolicy, ExpectedServedModel: m.Providers[0].ExpectedServedModel,
		ExpectedServedModels: append([]string(nil), m.Providers[0].ExpectedServedModels...),
		RetryPolicyVersion:   m.Providers[0].RetryPolicyVersion, MaxRetries: m.Providers[0].MaxRetries,
		RequestTimeoutSeconds:      m.Providers[0].RequestTimeoutSeconds,
		MinDispatchIntervalSeconds: m.Providers[0].MinDispatchIntervalSeconds,
	}
}

func digest(seed string) string {
	digest := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(digest[:])
}
