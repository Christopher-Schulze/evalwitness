import type { StressReductionStep, StressView } from "@/lib/report";

export type StressOrderView = "original" | "transformed";

export interface StressCandidatePresentation {
  id: string;
  selected: boolean;
  output: string;
  fixturePath: string;
  line: number;
}

export interface StressOrderPresentation {
  order: string[];
  selected: string;
  candidates: StressCandidatePresentation[];
}

export function defaultStressStepIndex(stress: StressView): number {
  return stress.steps.find((step) => step.decision === "rejected")?.index ?? 0;
}

export function stressOrderPresentation(
  stress: StressView,
  view: StressOrderView,
): StressOrderPresentation {
  const order = view === "original" ? stress.original_order : stress.transformed_order;
  const selected = view === "original" ? stress.original_selected : stress.transformed_selected;
  return {
    order,
    selected,
    candidates: order.map((id) => {
      const witness = stress.final_witness.find((line) => line.trajectory_id === id);
      if (witness === undefined) {
        throw new Error(`Stress witness is missing trajectory ${id}.`);
      }
      return {
        id,
        selected: id === selected,
        output: witness.content,
        fixturePath: witness.fixture_path,
        line: witness.line,
      };
    }),
  };
}

export function selectedStressStep(stress: StressView, index: number): StressReductionStep {
  return stress.steps[index] ?? stress.steps[defaultStressStepIndex(stress)]!;
}

export function formatReductionPercent(basisPoints: number): string {
  return `${(basisPoints / 100).toFixed(2)}%`;
}
