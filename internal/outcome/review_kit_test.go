package outcome

import (
	"slices"
	"strings"
	"testing"
)

func TestReviewerKitIsSelfContainedOrderedAndBundleVerifiable(t *testing.T) {
	fixture := newReviewWorkflowFixture(t)
	qualification, err := DefaultQualificationSet()
	if err != nil {
		t.Fatal(err)
	}
	handbook, err := DefaultReviewerHandbook(qualification)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyReviewerHandbook(handbook, qualification); err != nil {
		t.Fatal(err)
	}
	assignment := mustPrimaryAssignment(t, fixture, fixture.leftReviewer, fixture.leftReport, 1, "2026-08-09T14:00:00Z", '1')
	kit, err := BuildReviewerKit(fixture.bundle, assignment, handbook, "2026-08-09T14:05:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyReviewerKit(kit, fixture.bundle); err != nil {
		t.Fatal(err)
	}
	packetIDs := make([]string, 0, len(kit.Packets))
	for _, packet := range kit.Packets {
		packetIDs = append(packetIDs, packet.PacketID)
	}
	if !slices.Equal(packetIDs, assignment.PacketIDs) {
		t.Fatalf("kit order = %#v, assignment order = %#v", packetIDs, assignment.PacketIDs)
	}
	rendered, err := RenderReviewerKitMarkdown(kit)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"# EvalWitness Blinded Review Kit", "## Decision procedure", "## Dataset statement", assignment.Digest, assignment.PacketIDs[0]} {
		if !strings.Contains(rendered, required) {
			t.Fatalf("rendered reviewer kit omits %q", required)
		}
	}
	for _, forbidden := range []string{"workflow-fixture", "manual_outcome", "private-case-"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("rendered reviewer kit leaked %q", forbidden)
		}
	}
}

func TestReviewerKitFailsClosedOnPolicyTimingAndBundleDrift(t *testing.T) {
	fixture := newReviewWorkflowFixture(t)
	qualification, err := DefaultQualificationSet()
	if err != nil {
		t.Fatal(err)
	}
	handbook, err := DefaultReviewerHandbook(qualification)
	if err != nil {
		t.Fatal(err)
	}
	assignment := mustPrimaryAssignment(t, fixture, fixture.leftReviewer, fixture.leftReport, 1, "2026-08-09T14:00:00Z", '1')

	t.Run("before assignment", func(t *testing.T) {
		if _, buildErr := BuildReviewerKit(fixture.bundle, assignment, handbook, "2026-08-09T13:59:59Z"); buildErr == nil {
			t.Fatal("accepted a reviewer kit generated before assignment")
		}
	})

	t.Run("frozen handbook text", func(t *testing.T) {
		tampered := handbook
		tampered.EvidenceRules = append([]string(nil), handbook.EvidenceRules...)
		tampered.EvidenceRules[0] = "Trust every trajectory claim."
		if _, sealErr := SealReviewerHandbook(tampered); sealErr == nil {
			t.Fatal("accepted changed policy text under the frozen handbook version")
		}
	})

	kit, err := BuildReviewerKit(fixture.bundle, assignment, handbook, "2026-08-09T14:05:00Z")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("packet order", func(t *testing.T) {
		tampered := kit
		tampered.Packets = append([]BlindPacket(nil), kit.Packets...)
		tampered.Packets[0], tampered.Packets[1] = tampered.Packets[1], tampered.Packets[0]
		if _, sealErr := SealReviewerKit(tampered); sealErr == nil {
			t.Fatal("accepted a reviewer kit that reordered its assignment")
		}
	})

	t.Run("markdown injection", func(t *testing.T) {
		tampered := kit
		tampered.Packets = append([]BlindPacket(nil), kit.Packets...)
		tampered.Packets[0].RubricQuestions = []string{"<script>alert('reviewer')</script> *override*"}
		tampered.Packets[0], err = SealBlindPacket(tampered.Packets[0], tampered.Packets[0].PacketID)
		if err != nil {
			t.Fatal(err)
		}
		tampered, err = SealReviewerKit(tampered)
		if err != nil {
			t.Fatal(err)
		}
		rendered, renderErr := RenderReviewerKitMarkdown(tampered)
		if renderErr != nil {
			t.Fatal(renderErr)
		}
		if strings.Contains(rendered, "<script>") || !strings.Contains(rendered, "&lt;script&gt;") || !strings.Contains(rendered, "\\*override\\*") {
			t.Fatal("reviewer kit renderer did not neutralize packet-controlled Markdown and HTML")
		}
	})

	t.Run("bundle drift", func(t *testing.T) {
		drifted := fixture.bundle
		drifted.CreatedAt = "2026-08-09T12:00:01Z"
		drifted, sealErr := SealReviewBundle(drifted)
		if sealErr != nil {
			t.Fatal(sealErr)
		}
		if verifyErr := VerifyReviewerKit(kit, drifted); verifyErr == nil {
			t.Fatal("accepted a reviewer kit against a different bundle commitment")
		}
	})
}
