import { describe, expect, it } from "vitest";

import type { StressView } from "@/lib/report";
import {
  defaultStressStepIndex,
  formatReductionPercent,
  selectedStressStep,
  stressOrderPresentation,
} from "@/lib/stress-presentation";

const digest = "a".repeat(64);

describe("stress presentation", () => {
  const stress = {
    original_order: ["trajectory-a", "trajectory-b"],
    transformed_order: ["trajectory-b", "trajectory-a"],
    original_selected: "trajectory-a",
    transformed_selected: "trajectory-b",
    final_witness: [
      {
        trajectory_id: "trajectory-a",
        unit_id: "trajectory-a-line-013",
        fixture_path: "a.txt",
        line: 13,
        content: "5",
      },
      {
        trajectory_id: "trajectory-b",
        unit_id: "trajectory-b-line-013",
        fixture_path: "b.txt",
        line: 13,
        content: "-1",
      },
    ],
    steps: [
      {
        index: 0,
        unit_id: "trajectory-a-line-001",
        decision: "accepted",
        relation_revalidated: true,
        privacy_revalidated: true,
        violation_preserved: true,
        before_digest: digest,
        candidate_digest: "b".repeat(64),
        after_digest: "b".repeat(64),
        observation_digest: digest,
      },
      {
        index: 1,
        unit_id: "trajectory-a-line-013",
        decision: "rejected",
        relation_revalidated: false,
        privacy_revalidated: true,
        violation_preserved: false,
        before_digest: "b".repeat(64),
        candidate_digest: "c".repeat(64),
        after_digest: "b".repeat(64),
        observation_digest: digest,
      },
    ],
  } as StressView;

  it("switches the real candidate order and selected output", () => {
    expect(stressOrderPresentation(stress, "original")).toMatchObject({
      order: ["trajectory-a", "trajectory-b"],
      selected: "trajectory-a",
      candidates: [
        { output: "5", selected: true },
        { output: "-1", selected: false },
      ],
    });
    expect(stressOrderPresentation(stress, "transformed")).toMatchObject({
      order: ["trajectory-b", "trajectory-a"],
      selected: "trajectory-b",
      candidates: [
        { output: "-1", selected: true },
        { output: "5", selected: false },
      ],
    });
  });

  it("selects the first protected unit and formats the exact reduction", () => {
    expect(defaultStressStepIndex(stress)).toBe(1);
    expect(selectedStressStep(stress, 999).unit_id).toBe("trajectory-a-line-013");
    expect(formatReductionPercent(9375)).toBe("93.75%");
  });
});
