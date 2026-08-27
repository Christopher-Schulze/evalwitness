import { describe, expect, it } from "vitest";

import { inspectGeneration, inspectLayer, inspectReceipt } from "@/lib/inspection";
import type {
  ChallengeReceipt,
  MethodGeneration,
  TransportLayer,
  TransportPath,
} from "@/lib/report";

const digest = "1".repeat(64);

describe("inspection projections", () => {
  it("keeps exact verified challenge receipt state and replay data", () => {
    const receipt = challengeReceipt();
    const inspection = inspectReceipt(receipt);

    expect(inspection.tone).toBe("rejected");
    expect(inspection.deepLink).toBe("#challenge/CLM-011/clm-011.denominator-deletion");
    expect(inspection.fields).toContainEqual({
      label: "Expected guard",
      value: "expression-denominator-missing",
    });
    expect(inspection.fields).toContainEqual({
      label: "Observed guard",
      value: "expression-denominator-missing",
    });
    expect(inspection.fields).toContainEqual({ label: "Sealed source", value: digest });
  });

  it("keeps unsupported transport detail separate from first loss", () => {
    const layer: TransportLayer = {
      layer: "retained_bundle",
      status: "first_loss",
      object_digest: null,
      object_digest_availability: "not_applicable",
      required_fields: null,
      decisive_channels: null,
      detail_availability: "not_applicable",
      deep_link: "#autopsy/transport/claim/path/retained_bundle",
    };
    const path = transportPath(layer);
    const inspection = inspectLayer(path, layer);

    expect(inspection.tone).toBe("rejected");
    expect(inspection.fields).toContainEqual({ label: "Object digest", value: "not applicable" });
    expect(inspection.fields).toContainEqual({
      label: "Decisive channels",
      value: "not applicable",
    });
  });

  it("preserves every method denominator and non-claim", () => {
    const generation: MethodGeneration = {
      generation: "v3",
      state: "admitted_development",
      evidence: [artifact()],
      frozen_denominators: [
        { name: "scarcity_admitted", value: 3 },
        { name: "scarcity_target", value: 40 },
      ],
      observations: ["development observation"],
      non_claims: ["held-out validity"],
      guarding_tests: ["TestGuard"],
      deep_link: "#autopsy/method/v3",
    };
    const inspection = inspectGeneration(generation);

    expect(inspection.fields).toContainEqual({ label: "scarcity admitted", value: "3" });
    expect(inspection.fields).toContainEqual({ label: "scarcity target", value: "40" });
    expect(inspection.fields).toContainEqual({ label: "Non-claims", value: "held-out validity" });
  });
});

function artifact() {
  return {
    kind: "capsule_component" as const,
    id: digest,
    schema_version: "example.v1",
    payload_sha256: digest,
    artifact_digest: digest,
  };
}

function transportPath(layer: TransportLayer): TransportPath {
  return {
    path_id: "path",
    label: "Rejected path",
    terminal_state: "non_failable_verification",
    evidence: artifact(),
    layers: [layer],
    deep_link: "#autopsy/transport/claim/path",
  };
}

function challengeReceipt(): ChallengeReceipt {
  return {
    schema_version: "evalwitness.claim-challenge-receipt.v1",
    verifier: "evalwitness.claim-verifier.v1",
    claim_id: "CLM-011",
    source_capsule_id: digest,
    source_manifest_digest: digest,
    source_ledger_digest: digest,
    challenge_id: "clm-011.denominator-deletion",
    class: "denominator-deletion",
    mutation: "remove the declared ratio denominator from the ephemeral expression state",
    expected_guard: "expression-denominator-missing",
    observed_guard: "expression-denominator-missing",
    before_state: "verified:assertable",
    after_state: "rejected:expression-denominator-missing",
    sealed_source_digest: digest,
    after_sealed_source_digest: digest,
    passed: true,
    digest,
    replay_command:
      'evalwitness claim challenge --capsule "$CAPSULE" --ledger "$LEDGER" --claim CLM-011 --challenge clm-011.denominator-deletion',
  };
}
