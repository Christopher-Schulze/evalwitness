package relation

import (
	"regexp"
	"strings"
	"testing"
)

func TestRenderPilotLaunchBriefMarkdownIsPublicSafeAndDeterministic(t *testing.T) {
	t.Parallel()

	dossier := validPilotLaunchDossierForTest(t)
	first, err := RenderPilotLaunchBriefMarkdown(dossier)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RenderPilotLaunchBriefMarkdown(dossier)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("RenderPilotLaunchBriefMarkdown() is not deterministic")
	}
	for _, required := range []string{
		"Maximum total review actions | 64",
		"All eight pilot packets are non-public and restricted reference-only.",
		"Every row is a non-binding proposal.",
		"`authorship_and_labor_credit` | `owner_decision_required`",
		"`compensation_or_volunteer_terms` | `owner_decision_required`",
		"`publication` | `not_authorized`",
		"human support for any controlled relation or construct-validity conclusion",
	} {
		if !strings.Contains(first, required) {
			t.Fatalf("RenderPilotLaunchBriefMarkdown() omitted %q", required)
		}
	}
	if regexp.MustCompile(`[[:xdigit:]]{64}`).MatchString(first) {
		t.Fatal("RenderPilotLaunchBriefMarkdown() exposed a cryptographic digest")
	}
	for _, disclosure := range dossier.PacketDisclosures {
		if strings.Contains(first, disclosure.PacketID) || strings.Contains(first, disclosure.PacketDigest) || strings.Contains(first, disclosure.TaskRequirementDigest) {
			t.Fatal("RenderPilotLaunchBriefMarkdown() exposed private packet identity")
		}
	}
	for _, forbidden := range []string{
		dossier.Digest,
		dossier.PlanDigest,
		dossier.PilotSampleDigest,
		dossier.BundleDigest,
		dossier.ReadinessDigest,
		dossier.QualificationSetDigest,
		dossier.HandbookDigest,
		dossier.MappingCommitmentDigest,
	} {
		if strings.Contains(first, forbidden) {
			t.Fatal("RenderPilotLaunchBriefMarkdown() exposed a governed digest")
		}
	}
}

func TestRenderPilotLaunchBriefMarkdownRejectsInvalidDossier(t *testing.T) {
	t.Parallel()

	dossier := validPilotLaunchDossierForTest(t)
	dossier.ExternalActions[0].Status = ExternalActionStatus("authorized")
	if _, err := RenderPilotLaunchBriefMarkdown(dossier); err == nil {
		t.Fatal("RenderPilotLaunchBriefMarkdown() accepted unauthorized external action")
	}
}
