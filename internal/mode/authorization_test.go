package mode

import (
	"strings"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
)

func authorizationSpecForTest() AuthorizationSpec {
	return AuthorizationSpec{
		Entrypoint:            "cli.verify",
		RouteID:               "route-abc",
		RequestFingerprint:    provider.Fingerprint(strings.Repeat("a", 64)),
		RequestContractDigest: strings.Repeat("b", 64),
		MaxRetries:            2,
		MaxWorkers:            2,
		MaxOutputTokens:       4096,
		ExpectedCalls:         2,
		WorstCalls:            4,
		Limits: BudgetLimits{
			MaxCalls:                4,
			MaxAttempts:             12,
			MaxEstimatedInputTokens: 10000,
			MaxReservedOutputTokens: 49152,
			MaxConcurrent:           2,
			MaxDuration:             time.Minute,
		},
	}
}

func TestAuthorizationDigestBindsEveryExecutionDimension(t *testing.T) {
	base, err := BuildAuthorizationPlan(authorizationSpecForTest())
	if err != nil {
		t.Fatal(err)
	}
	if err := base.Verify(base.AuthorizationDigest); err != nil {
		t.Fatal(err)
	}
	mutations := []func(*AuthorizationSpec){
		func(spec *AuthorizationSpec) { spec.RouteID = "route-other" },
		func(spec *AuthorizationSpec) { spec.RequestFingerprint = provider.Fingerprint(strings.Repeat("c", 64)) },
		func(spec *AuthorizationSpec) { spec.RequestContractDigest = strings.Repeat("d", 64) },
		func(spec *AuthorizationSpec) { spec.MaxRetries++ },
		func(spec *AuthorizationSpec) { spec.MaxWorkers-- },
		func(spec *AuthorizationSpec) { spec.MinDispatchIntervalSeconds++ },
		func(spec *AuthorizationSpec) { spec.MaxOutputTokens-- },
		func(spec *AuthorizationSpec) { spec.Limits.MaxAttempts-- },
		func(spec *AuthorizationSpec) { spec.Limits.MaxDuration -= time.Second },
		func(spec *AuthorizationSpec) { spec.StudyManifestDigest = "manifest" },
	}
	for index, mutate := range mutations {
		spec := authorizationSpecForTest()
		mutate(&spec)
		changed, err := BuildAuthorizationPlan(spec)
		if err != nil {
			t.Fatalf("mutation %d: %v", index, err)
		}
		if changed.AuthorizationDigest == base.AuthorizationDigest {
			t.Fatalf("mutation %d did not change authorization digest", index)
		}
		if err := changed.Verify(base.AuthorizationDigest); err == nil {
			t.Fatalf("mutation %d accepted stale approval", index)
		}
	}
}

func TestAuthorizationRequiresExplicitCompleteLimitsAndApproval(t *testing.T) {
	spec := authorizationSpecForTest()
	spec.Limits.MaxReservedOutputTokens = 0
	if _, err := BuildAuthorizationPlan(spec); err == nil || !strings.Contains(err.Error(), "max output") {
		t.Fatalf("incomplete limits error = %v", err)
	}
	plan, err := BuildAuthorizationPlan(authorizationSpecForTest())
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Verify(""); err == nil || !strings.Contains(err.Error(), "explicit --authorize") {
		t.Fatalf("missing approval error = %v", err)
	}
}

func TestAuthorizationRejectsDurationThatCannotBeRepresented(t *testing.T) {
	spec := authorizationSpecForTest()
	spec.Limits.MaxDuration = time.Millisecond
	if _, err := BuildAuthorizationPlan(spec); err == nil || !strings.Contains(err.Error(), ">= 1s") {
		t.Fatalf("sub-second duration error = %v", err)
	}
}
