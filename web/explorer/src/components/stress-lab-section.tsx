import { ArrowRight, FileSearch, RotateCcw, ShieldCheck, Shrink, X } from "lucide-react";

import { StatusBadge } from "@/components/status-badge";
import { Button } from "@/components/ui/button";
import type { InspectionItem } from "@/lib/inspection";
import { inspectArtifact } from "@/lib/inspection";
import type { EvidenceReport, StressReductionStep } from "@/lib/report";
import {
  formatReductionPercent,
  selectedStressStep,
  stressOrderPresentation,
  type StressOrderView,
} from "@/lib/stress-presentation";
import { humanize, shortDigest } from "@/lib/utils";

interface StressLabSectionProps {
  report: EvidenceReport;
  orderView: StressOrderView;
  selectedStepIndex: number;
  onOrderChange: (value: StressOrderView) => void;
  onStepChange: (index: number) => void;
  onInspect: (item: InspectionItem) => void;
}

export function StressLabSection({
  report,
  orderView,
  selectedStepIndex,
  onOrderChange,
  onStepChange,
  onInspect,
}: StressLabSectionProps) {
  const stress = report.stress;
  const order = stressOrderPresentation(stress, orderView);
  const selectedStep = selectedStressStep(stress, selectedStepIndex);
  return (
    <section
      aria-labelledby="stress-title"
      className="scroll-mt-4 border-y border-white/10 bg-graphite py-12 text-white sm:py-16"
      id="stress"
    >
      <div className="mx-auto max-w-[94rem] px-4 sm:px-7 lg:px-10">
        <header className="grid gap-7 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end">
          <div className="max-w-4xl">
            <p className="text-[0.64rem] font-bold uppercase tracking-[0.2em] text-white/72">
              Lane 03 · Provider-free stress witness
            </p>
            <h2
              className="mt-3 text-balance text-3xl font-semibold tracking-[-0.045em] text-white sm:text-5xl"
              id="stress-title"
            >
              The violation survives in two lines.
            </h2>
            <p className="mt-4 max-w-3xl text-sm leading-6 text-white/62 sm:text-base sm:leading-7">
              Candidate reversal changes the first-listed selection. The same reducer then replays
              every removal and keeps only the two lines without which the violation disappears.
            </p>
          </div>
          <div className="flex items-baseline gap-3 lg:text-right">
            <span className="font-mono text-[clamp(4rem,8vw,7.5rem)] font-semibold leading-[0.75] tracking-[-0.09em] text-white">
              {stress.original_line_units}
            </span>
            <ArrowRight aria-hidden="true" className="size-6 text-white/35" />
            <span className="font-mono text-[clamp(4rem,8vw,7.5rem)] font-semibold leading-[0.75] tracking-[-0.09em] text-break">
              {stress.final_line_units}
            </span>
          </div>
        </header>

        <div className="mt-10 grid gap-px overflow-hidden rounded-[1.35rem] border border-white/12 bg-white/12 xl:grid-cols-[0.78fr_1.22fr]">
          <OrderControl
            onOrderChange={onOrderChange}
            order={order}
            orderView={orderView}
            report={report}
          />
          <ReductionTrace onStepChange={onStepChange} report={report} selectedStep={selectedStep} />
        </div>

        <div className="mt-px grid gap-px overflow-hidden rounded-[1.35rem] border border-white/12 bg-white/12 lg:grid-cols-[1fr_1fr_0.8fr]">
          {stress.final_witness.map((line) => (
            <div className="bg-[#1d222a] p-5 sm:p-6" key={line.unit_id}>
              <div className="flex items-center justify-between gap-4">
                <p className="text-[0.59rem] font-bold uppercase tracking-[0.16em] text-white/48">
                  {humanize(line.trajectory_id)} · line {line.line}
                </p>
                <StatusBadge compact status="retained" />
              </div>
              <code className="mt-5 block font-mono text-5xl font-semibold tracking-[-0.06em] text-white">
                {line.content}
              </code>
              <p className="mt-4 break-all font-mono text-[0.59rem] leading-5 text-white/65">
                {line.fixture_path}
              </p>
            </div>
          ))}
          <div className="flex flex-col justify-between bg-break p-5 sm:p-6">
            <div>
              <p className="text-[0.59rem] font-bold uppercase tracking-[0.16em] text-white/90">
                Claim boundary
              </p>
              <p className="mt-4 text-sm font-semibold leading-6 text-white">
                Mechanism evidence only. No verifier, provider, population, held-out, or
                global-minimum claim.
              </p>
            </div>
            <Button
              className="mt-6 w-full"
              onClick={() =>
                onInspect(
                  inspectArtifact(
                    stress.source,
                    "Stress development case study",
                    `#artifact/${stress.source.id}`,
                  ),
                )
              }
              type="button"
              variant="inverseActive"
            >
              <FileSearch aria-hidden="true" />
              Inspect bound artifact
            </Button>
          </div>
        </div>

        <div className="mt-6 grid gap-4 border-t border-white/12 pt-5 sm:grid-cols-[1fr_auto] sm:items-center">
          <p className="max-w-4xl font-mono text-[0.62rem] leading-5 text-white/70">
            {stress.reproduction_command} · {stress.provider_calls} provider calls ·{" "}
            {stress.empirical_units} empirical units · digest {shortDigest(stress.digest)}
          </p>
          <div className="flex flex-wrap gap-2">
            <StatusBadge status={stress.status} />
            <StatusBadge status={stress.outcome} />
          </div>
        </div>
      </div>
    </section>
  );
}

interface OrderControlProps {
  report: EvidenceReport;
  orderView: StressOrderView;
  order: ReturnType<typeof stressOrderPresentation>;
  onOrderChange: (value: StressOrderView) => void;
}

function OrderControl({ report, orderView, order, onOrderChange }: OrderControlProps) {
  return (
    <div className="bg-[#1d222a] p-5 sm:p-7">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="text-[0.59rem] font-bold uppercase tracking-[0.16em] text-white/72">
            Candidate-order control
          </p>
          <p className="mt-1.5 text-sm font-semibold text-white">
            First position wins. Identity stability fails.
          </p>
        </div>
        <div aria-label="Candidate order" className="flex gap-1" role="group">
          {(["original", "transformed"] as const).map((value) => (
            <Button
              aria-pressed={orderView === value}
              key={value}
              onClick={() => onOrderChange(value)}
              size="compact"
              type="button"
              variant={orderView === value ? "inverseActive" : "inverse"}
            >
              {value === "original" ? "Original" : "Reversed"}
            </Button>
          ))}
        </div>
      </div>
      <div className="mt-7 grid items-stretch gap-3 sm:grid-cols-[1fr_auto_1fr]">
        {order.candidates.map((candidate, index) => (
          <div className="contents" key={candidate.id}>
            <div
              className={`rounded-xl border p-4 ${candidate.selected ? "border-break bg-break/12" : "border-white/12 bg-white/[0.035]"}`}
              data-selected={candidate.selected}
              data-stress-candidate={candidate.id}
            >
              <div className="flex items-center justify-between gap-3">
                <p className="font-mono text-[0.64rem] font-semibold text-white/68">
                  {candidate.id}
                </p>
                {candidate.selected ? (
                  <span className="text-[0.56rem] font-bold uppercase tracking-[0.13em] text-[#ff8b76]">
                    selected
                  </span>
                ) : null}
              </div>
              <p className="mt-5 font-mono text-4xl font-semibold tracking-[-0.05em] text-white">
                {candidate.output}
              </p>
              <p className="mt-3 font-mono text-[0.57rem] text-white/68">position {index + 1}</p>
            </div>
            {index === 0 ? (
              <ArrowRight
                aria-hidden="true"
                className="mx-auto size-5 rotate-90 self-center text-white/28 sm:rotate-0"
              />
            ) : null}
          </div>
        ))}
      </div>
      <div className="mt-6 flex items-center justify-between gap-4 border-t border-white/10 pt-4">
        <p className="font-mono text-[0.61rem] text-white/70">
          {report.stress.original_selected} → {report.stress.transformed_selected}
        </p>
        <div className="flex items-center gap-2 text-[0.6rem] font-bold uppercase tracking-[0.13em] text-[#ff8b76]">
          <RotateCcw aria-hidden="true" className="size-3.5" />
          constraint violated
        </div>
      </div>
    </div>
  );
}

interface ReductionTraceProps {
  report: EvidenceReport;
  selectedStep: StressReductionStep;
  onStepChange: (index: number) => void;
}

function ReductionTrace({ report, selectedStep, onStepChange }: ReductionTraceProps) {
  const stress = report.stress;
  return (
    <div className="min-w-0 bg-[#20252d] p-5 sm:p-7">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <p className="flex items-center gap-2 text-[0.59rem] font-bold uppercase tracking-[0.16em] text-white/72">
            <Shrink aria-hidden="true" className="size-3.5" />
            Replay-preserving reduction
          </p>
          <p className="mt-1.5 text-sm font-semibold text-white">
            {stress.reduction_attempts} attempts · {stress.accepted_reductions} removed ·{" "}
            {formatReductionPercent(stress.reduction_basis_points)} reduction
          </p>
        </div>
        <StatusBadge status={stress.minimality} />
      </div>
      <div className="mt-6 overflow-x-auto pb-2">
        <div
          aria-label="Reduction attempts"
          className="flex min-w-[106rem] items-end gap-2"
          role="group"
        >
          {stress.steps.map((step) => (
            <button
              aria-label={`Inspect reduction step ${step.index + 1}: ${step.unit_id}, ${step.decision}`}
              aria-pressed={selectedStep.index === step.index}
              className={`group flex h-14 w-6 min-w-6 items-end justify-center rounded-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white focus-visible:ring-offset-2 focus-visible:ring-offset-graphite ${selectedStep.index === step.index ? "ring-2 ring-white ring-offset-2 ring-offset-graphite" : "opacity-90"}`}
              key={step.index}
              onClick={() => onStepChange(step.index)}
              type="button"
            >
              <span
                aria-hidden="true"
                className={`w-2 rounded-sm transition-colors ${
                  step.decision === "accepted"
                    ? "h-7 bg-verified group-hover:bg-[#5b8df1]"
                    : "h-14 bg-break group-hover:bg-[#e35a43]"
                }`}
              />
            </button>
          ))}
        </div>
      </div>
      <div className="mt-2 flex flex-wrap gap-4 text-[0.57rem] font-bold uppercase tracking-[0.12em] text-white/70">
        <span className="flex items-center gap-2">
          <span className="size-2 rounded-sm bg-verified" /> accepted removal
        </span>
        <span className="flex items-center gap-2">
          <span className="size-2 rounded-sm bg-break" /> retained proof
        </span>
      </div>
      <div className="mt-6 rounded-xl border border-white/12 bg-black/12 p-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p className="text-[0.57rem] font-bold uppercase tracking-[0.13em] text-white/68">
              Step {selectedStep.index + 1} / {stress.reduction_attempts}
            </p>
            <p className="mt-1.5 break-all font-mono text-xs font-semibold text-white">
              {selectedStep.unit_id}
            </p>
          </div>
          <StatusBadge status={selectedStep.decision} />
        </div>
        <div className="mt-4 grid gap-2 sm:grid-cols-3">
          <ProofFlag label="Relation" value={selectedStep.relation_revalidated} />
          <ProofFlag label="Privacy" value={selectedStep.privacy_revalidated} />
          <ProofFlag label="Violation" value={selectedStep.violation_preserved} />
        </div>
        <div className="mt-4 grid gap-2 border-t border-white/10 pt-4 font-mono text-[0.58rem] text-white/68 sm:grid-cols-[1fr_auto_1fr] sm:items-center">
          <span className="truncate">{shortDigest(selectedStep.before_digest)}</span>
          <ArrowRight aria-hidden="true" className="size-3.5 text-white/25" />
          <span className="truncate sm:text-right">{shortDigest(selectedStep.after_digest)}</span>
        </div>
      </div>
    </div>
  );
}

function ProofFlag({ label, value }: { label: string; value: boolean }) {
  const Icon = value ? ShieldCheck : X;
  return (
    <div className="flex items-center gap-2 rounded-lg border border-white/10 bg-white/[0.035] px-3 py-2.5">
      <Icon aria-hidden="true" className={`size-4 ${value ? "text-verified" : "text-break"}`} />
      <div>
        <p className="text-[0.53rem] font-bold uppercase tracking-[0.11em] text-white/68">
          {label}
        </p>
        <p className="mt-0.5 text-[0.66rem] font-semibold text-white">
          {value ? "revalidated" : "not preserved"}
        </p>
      </div>
    </div>
  );
}
