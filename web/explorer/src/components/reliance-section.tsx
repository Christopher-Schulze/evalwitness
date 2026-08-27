import { Activity, FileSearch, Fingerprint, Network, ShieldAlert } from "lucide-react";
import { useState } from "react";

import { StatusBadge } from "@/components/status-badge";
import { Button } from "@/components/ui/button";
import type { InspectionItem } from "@/lib/inspection";
import { inspectArtifact } from "@/lib/inspection";
import {
  ratioPercent,
  relianceEffectScale,
  relianceIntervalGeometry,
  relianceOutcomeOptions,
  selectRelianceOutcomes,
  selectorForOutcome,
  signedDecimal,
  type RelianceOutcomeID,
  type RelianceTermKind,
} from "@/lib/reliance-presentation";
import type {
  RelianceArmContrast,
  RelianceOutcome,
  RelianceSelector,
  RelianceView,
} from "@/lib/report";
import { humanize, shortDigest } from "@/lib/utils";

interface RelianceSectionProps {
  view: RelianceView;
  onInspect: (item: InspectionItem) => void;
}

export function RelianceSection({ view, onInspect }: RelianceSectionProps) {
  const [outcomeID, setOutcomeID] = useState<RelianceOutcomeID>("visible_probability_mass");
  const [termKind, setTermKind] = useState<RelianceTermKind>("main_effect");
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const outcomes = selectRelianceOutcomes(view, outcomeID, termKind);
  const selected = outcomes.find((outcome) => outcome.dimension_id === selectedID) ?? outcomes[0];
  return (
    <section
      aria-labelledby="reliance-title"
      className="scroll-mt-4 bg-paper py-12 sm:py-16"
      id="reliance"
    >
      <div className="mx-auto max-w-[94rem] px-4 sm:px-7 lg:px-10">
        <RelianceHeader onInspect={onInspect} view={view} />
        <RelianceControls
          onOutcomeChange={setOutcomeID}
          onTermKindChange={setTermKind}
          outcomeID={outcomeID}
          termKind={termKind}
        />
        <div className="mt-5 grid overflow-hidden rounded-[1.35rem] border border-border-strong bg-border lg:grid-cols-[minmax(0,1.35fr)_minmax(20rem,0.65fr)]">
          <RelianceMap
            onSelect={setSelectedID}
            outcomes={outcomes}
            selectedID={selected?.dimension_id ?? ""}
          />
          {selected === undefined ? null : <RelianceInspector outcome={selected} view={view} />}
        </div>
        <div className="mt-8 grid gap-6 xl:grid-cols-[1.15fr_0.85fr]">
          <ArmTransferLane contrasts={view.arm_contrasts} />
          <RelianceWitnessLane view={view} />
        </div>
        <RelianceBoundary view={view} />
      </div>
    </section>
  );
}

function RelianceHeader({
  view,
  onInspect,
}: {
  view: RelianceView;
  onInspect: RelianceSectionProps["onInspect"];
}) {
  const excluded = ratioPercent(view.excluded_cells, view.registered_cells);
  return (
    <header className="grid gap-7 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end">
      <div className="max-w-4xl">
        <p className="text-[0.64rem] font-bold uppercase tracking-[0.2em] text-verified">
          Evidence reliance map
        </p>
        <h2
          className="mt-3 text-balance text-3xl font-semibold tracking-[-0.045em] text-foreground sm:text-5xl"
          id="reliance-title"
        >
          What actually moves the verifier?
        </h2>
        <p className="mt-4 max-w-3xl text-sm leading-6 text-muted-foreground sm:text-base sm:leading-7">
          Typed evidence interventions, full score-distribution outcomes, selector retention, and
          exact uncertainty. This is local mechanism evidence, not agent causality.
        </p>
      </div>
      <Button
        onClick={() =>
          onInspect(
            inspectArtifact(view.source, "Evidence reliance map", `#artifact/${view.source.id}`),
          )
        }
        type="button"
        variant="outline"
      >
        <FileSearch aria-hidden="true" />
        Inspect map
      </Button>
      <dl className="grid gap-px overflow-hidden rounded-xl border border-border bg-border sm:col-span-2 sm:grid-cols-4">
        <RelianceFact label="Source tasks" value={String(view.source_tasks)} />
        <RelianceFact label="Registered cells" value={String(view.registered_cells)} />
        <RelianceFact label="Outcome bearing" value={String(view.outcome_bearing_cells)} />
        <RelianceFact label="Excluded" value={`${view.excluded_cells} · ${excluded}`} />
      </dl>
    </header>
  );
}

function RelianceFact({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-paper px-4 py-3">
      <dt className="text-[0.57rem] font-bold uppercase tracking-[0.13em] text-muted-foreground">
        {label}
      </dt>
      <dd className="mt-1 font-mono text-sm font-semibold text-foreground">{value}</dd>
    </div>
  );
}

interface RelianceControlsProps {
  outcomeID: RelianceOutcomeID;
  termKind: RelianceTermKind;
  onOutcomeChange: (value: RelianceOutcomeID) => void;
  onTermKindChange: (value: RelianceTermKind) => void;
}

function RelianceControls(props: RelianceControlsProps) {
  return (
    <div className="mt-8 flex flex-col gap-3 border-y border-border py-3 lg:flex-row lg:items-center lg:justify-between">
      <div aria-label="Reliance outcome" className="flex flex-wrap gap-1.5" role="group">
        {relianceOutcomeOptions.map((option) => (
          <Button
            aria-pressed={props.outcomeID === option.id}
            key={option.id}
            onClick={() => props.onOutcomeChange(option.id)}
            size="compact"
            type="button"
            variant={props.outcomeID === option.id ? "primary" : "ghost"}
          >
            {option.label}
          </Button>
        ))}
      </div>
      <div aria-label="Reliance term type" className="flex gap-1.5" role="group">
        {(["main_effect", "interaction"] as const).map((kind) => (
          <Button
            aria-pressed={props.termKind === kind}
            key={kind}
            onClick={() => props.onTermKindChange(kind)}
            size="compact"
            type="button"
            variant={props.termKind === kind ? "verified" : "outline"}
          >
            {humanize(kind)}
          </Button>
        ))}
      </div>
    </div>
  );
}

function RelianceMap({
  outcomes,
  selectedID,
  onSelect,
}: {
  outcomes: RelianceOutcome[];
  selectedID: string;
  onSelect: (id: string) => void;
}) {
  const scale = relianceEffectScale(outcomes);
  return (
    <div className="min-w-0 bg-paper" role="list">
      <div className="grid grid-cols-[minmax(8rem,0.8fr)_minmax(10rem,1.2fr)_auto] gap-4 border-b border-border bg-muted px-4 py-3 text-[0.57rem] font-bold uppercase tracking-[0.13em] text-muted-foreground">
        <span>Evidence factor</span>
        <span>Effect and family-adjusted interval</span>
        <span>Estimate</span>
      </div>
      {outcomes.map((outcome) => (
        <div key={outcome.dimension_id} role="listitem">
          <Button
            aria-label={`Inspect ${outcome.factors.join(" by ")} ${outcome.outcome_id}`}
            data-selected={selectedID === outcome.dimension_id}
            onClick={() => onSelect(outcome.dimension_id)}
            type="button"
            variant="evidenceRow"
          >
            <span className="grid w-full grid-cols-[minmax(8rem,0.8fr)_minmax(10rem,1.2fr)_auto] items-center gap-4">
              <span className="min-w-0">
                <span className="block truncate text-xs font-semibold text-foreground">
                  {outcome.factors.map(humanize).join(" × ")}
                </span>
                <span className="mt-1 block font-mono text-[0.56rem] text-muted-foreground">
                  {outcome.numerator}/{outcome.denominator} eligible
                </span>
              </span>
              <EffectGlyph outcome={outcome} scale={scale} />
              <span className="font-mono text-xs font-semibold text-foreground">
                {outcome.estimate === undefined || outcome.estimate === null
                  ? "n/a"
                  : signedDecimal(outcome.estimate.estimate)}
              </span>
            </span>
          </Button>
        </div>
      ))}
    </div>
  );
}

function EffectGlyph({ outcome, scale }: { outcome: RelianceOutcome; scale: number }) {
  const geometry = relianceIntervalGeometry(outcome, scale);
  if (geometry === null) {
    return (
      <span className="text-[0.62rem] font-semibold text-unavailable-strong">
        {humanize(outcome.status)}
      </span>
    );
  }
  const positive = Number(outcome.estimate?.estimate ?? "0") >= 0;
  return (
    <svg aria-hidden="true" className="h-8 w-full" preserveAspectRatio="none" viewBox="0 0 200 32">
      <line stroke="currentColor" className="text-border-strong" x1="100" x2="100" y1="2" y2="30" />
      <line
        stroke="currentColor"
        className="text-muted-foreground"
        strokeWidth="1.5"
        x1={geometry.lower}
        x2={geometry.upper}
        y1="16"
        y2="16"
      />
      <line
        stroke="currentColor"
        className="text-muted-foreground"
        x1={geometry.lower}
        x2={geometry.lower}
        y1="11"
        y2="21"
      />
      <line
        stroke="currentColor"
        className="text-muted-foreground"
        x1={geometry.upper}
        x2={geometry.upper}
        y1="11"
        y2="21"
      />
      <circle
        className={positive ? "fill-verified" : "fill-break"}
        cx={geometry.point}
        cy="16"
        r="4.5"
      />
    </svg>
  );
}

function RelianceInspector({ outcome, view }: { outcome: RelianceOutcome; view: RelianceView }) {
  const selector = selectorForOutcome(view, outcome);
  return (
    <aside className="min-w-0 bg-graphite p-5 text-white sm:p-7" aria-live="polite">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-[0.58rem] font-bold uppercase tracking-[0.16em] text-white/52">
            Selected contrast
          </p>
          <h3 className="mt-2 text-xl font-semibold tracking-[-0.03em]">
            {outcome.factors.map(humanize).join(" × ")}
          </h3>
        </div>
        <StatusBadge status={outcome.status} />
      </div>
      <p className="mt-5 font-mono text-4xl font-semibold tracking-[-0.06em] text-white">
        {outcome.estimate === undefined || outcome.estimate === null
          ? "not measured"
          : signedDecimal(outcome.estimate.estimate)}
      </p>
      <p className="mt-2 font-mono text-[0.65rem] leading-5 text-white/62">
        {outcome.estimate === undefined || outcome.estimate === null
          ? outcome.reason
          : `[${outcome.estimate.lower}, ${outcome.estimate.upper}] · adjusted p=${outcome.estimate.adjusted_p_value}`}
      </p>
      <dl className="mt-6 grid grid-cols-2 gap-px overflow-hidden rounded-xl bg-white/10">
        <InspectorFact label="Numerator" value={String(outcome.numerator)} />
        <InspectorFact label="Denominator" value={String(outcome.denominator)} />
        <InspectorFact label="Invalid cells" value={String(outcome.invalid_cells)} />
        <InspectorFact label="Unit" value={humanize(outcome.unit)} />
        <InspectorFact label="Route" value={outcome.route} />
        <InspectorFact label="Domain" value={humanize(outcome.domain)} />
        <InspectorFact label="Data role" value={humanize(outcome.data_role)} />
        <InspectorFact label="Evidence" value={outcome.evidence_level} />
      </dl>
      <p className="mt-5 break-all font-mono text-[0.57rem] leading-5 text-white/52">
        {outcome.policy} · {outcome.interval} · capsule {shortDigest(outcome.capsule_id)}
      </p>
      {selector === null ? (
        <p className="mt-6 border-t border-white/10 pt-5 text-xs leading-5 text-white/62">
          Selector alignment is not defined for this interaction.
        </p>
      ) : (
        <SelectorInspector selector={selector} />
      )}
    </aside>
  );
}

function InspectorFact({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0 bg-graphite px-3 py-2.5">
      <dt className="text-[0.53rem] font-bold uppercase tracking-[0.12em] text-white/70">
        {label}
      </dt>
      <dd className="mt-1 break-words font-mono text-[0.62rem] leading-5 text-white">{value}</dd>
    </div>
  );
}

function SelectorInspector({ selector }: { selector: RelianceSelector }) {
  return (
    <div className="mt-6 border-t border-white/10 pt-5">
      <div className="flex items-center justify-between gap-3">
        <p className="text-[0.58rem] font-bold uppercase tracking-[0.15em] text-white/52">
          Selector retention
        </p>
        <StatusBadge compact status={selector.effect_status} />
      </div>
      <div className="mt-4 space-y-4">
        {selector.budgets.map((budget) => (
          <SelectorBudget budget={budget} key={budget.budget_tokens} />
        ))}
      </div>
    </div>
  );
}

function SelectorBudget({ budget }: { budget: RelianceSelector["budgets"][number] }) {
  const width = 240;
  const exact = (budget.exact_targets / budget.assignment_targets) * width;
  const changed = (budget.changed_targets / budget.assignment_targets) * width;
  const unrendered = (budget.unrendered_targets / budget.assignment_targets) * width;
  const dropped = width - exact - changed - unrendered;
  return (
    <div>
      <div className="flex items-center justify-between gap-3 font-mono text-[0.61rem] text-white/72">
        <span>{budget.budget_tokens} tokens</span>
        <span>
          {budget.exact_targets + budget.changed_targets}/{budget.assignment_targets} retained
        </span>
      </div>
      <svg
        aria-label={`${budget.budget_tokens} token selector retention: ${budget.exact_targets} exact, ${budget.changed_targets} changed, ${budget.unrendered_targets} unrendered, ${budget.dropped_targets} dropped`}
        className="mt-2 h-3 w-full"
        preserveAspectRatio="none"
        role="img"
        viewBox={`0 0 ${width} 12`}
      >
        <rect className="fill-verified" height="12" width={exact} x="0" />
        <rect className="fill-unavailable" height="12" width={changed} x={exact} />
        <rect className="fill-break" height="12" width={unrendered} x={exact + changed} />
        <rect
          className="fill-white/20"
          height="12"
          width={dropped}
          x={exact + changed + unrendered}
        />
      </svg>
      <p className="mt-2 text-[0.57rem] leading-5 text-white/52">
        exact {budget.exact_targets} · changed {budget.changed_targets} · unrendered{" "}
        {budget.unrendered_targets} · dropped {budget.dropped_targets}
      </p>
      {budget.risk_flags.map((risk) => (
        <p className="mt-1 text-[0.57rem] font-semibold leading-5 text-[#ffb764]" key={risk}>
          {humanize(risk)}
        </p>
      ))}
    </div>
  );
}

function ArmTransferLane({ contrasts }: { contrasts: RelianceArmContrast[] }) {
  return (
    <article className="min-w-0 rounded-[1.2rem] border border-border bg-paper p-5 sm:p-6">
      <div className="flex items-center gap-3">
        <Network aria-hidden="true" className="size-5 text-verified" />
        <div>
          <p className="text-[0.58rem] font-bold uppercase tracking-[0.15em] text-muted-foreground">
            Transfer boundary
          </p>
          <h3 className="mt-1 text-lg font-semibold tracking-[-0.025em]">
            Every prespecified arm stays visible.
          </h3>
        </div>
      </div>
      <div className="mt-5 divide-y divide-border">
        {contrasts.map((contrast) => (
          <ArmContrastRow
            contrast={contrast}
            key={`${contrast.comparison_digest}:${contrast.contrast_id}`}
          />
        ))}
      </div>
    </article>
  );
}

function ArmContrastRow({ contrast }: { contrast: RelianceArmContrast }) {
  return (
    <div className="grid min-w-0 gap-3 py-3 sm:grid-cols-[minmax(0,9rem)_minmax(0,1fr)_auto] sm:items-center">
      <p className="text-xs font-semibold text-foreground">{humanize(contrast.kind)}</p>
      <div className="min-w-0">
        <p className="break-all text-[0.66rem] leading-5 text-muted-foreground">
          {contrast.reason || contrast.changed_dimensions.map(humanize).join(" · ")}
        </p>
        <p className="mt-1 font-mono text-[0.57rem] text-muted-foreground">
          {contrast.numerator}/{contrast.denominator} paired · {contrast.invalid_pairs} invalid
        </p>
      </div>
      <StatusBadge compact status={contrast.support} />
    </div>
  );
}

function RelianceWitnessLane({ view }: { view: RelianceView }) {
  return (
    <article className="rounded-[1.2rem] border border-border bg-muted p-5 sm:p-6">
      <div className="flex items-center gap-3">
        <Fingerprint aria-hidden="true" className="size-5 text-break" />
        <div>
          <p className="text-[0.58rem] font-bold uppercase tracking-[0.15em] text-muted-foreground">
            Minimal reliance witness
          </p>
          <h3 className="mt-1 text-lg font-semibold tracking-[-0.025em]">
            Proof identity, never private content.
          </h3>
        </div>
      </div>
      <div className="mt-5 space-y-4">
        {view.witnesses.map((witness) => (
          <div
            className="rounded-xl border border-border bg-paper p-4"
            key={witness.witness_digest}
          >
            <div className="flex items-center justify-between gap-3">
              <p className="text-xs font-semibold text-foreground">{humanize(witness.case_id)}</p>
              <StatusBadge compact status="one_minimal" />
            </div>
            <dl className="mt-4 grid grid-cols-2 gap-3">
              <WitnessFact label="Final units" value={String(witness.final_units)} />
              <WitnessFact label="Evaluations" value={String(witness.evaluations)} />
              <WitnessFact label="Witness" value={shortDigest(witness.witness_digest)} />
              <WitnessFact label="Relation" value={shortDigest(witness.relation_digest)} />
            </dl>
            <p className="mt-4 font-mono text-[0.56rem] leading-5 text-muted-foreground">
              {humanize(witness.binding_status)} · raw trajectory content shown=false
            </p>
          </div>
        ))}
      </div>
    </article>
  );
}

function WitnessFact({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-[0.53rem] font-bold uppercase tracking-[0.11em] text-muted-foreground">
        {label}
      </dt>
      <dd className="mt-1 font-mono text-[0.65rem] font-semibold text-foreground">{value}</dd>
    </div>
  );
}

function RelianceBoundary({ view }: { view: RelianceView }) {
  return (
    <div className="mt-8 grid gap-5 border-l-2 border-break bg-break-soft p-5 sm:grid-cols-[auto_1fr] sm:p-6">
      <span className="grid size-11 place-items-center rounded-full bg-break text-white">
        <ShieldAlert aria-hidden="true" className="size-5" />
      </span>
      <div>
        <p className="text-[0.59rem] font-bold uppercase tracking-[0.15em] text-break-strong">
          Causal boundary
        </p>
        <p className="mt-2 max-w-4xl text-sm font-semibold leading-6 text-foreground">
          The randomized evidence intervention can support a local verifier-output reliance claim.
          It cannot assign agent-step responsibility, expose model internals, or establish provider,
          route, model-family, entrypoint, or population transfer.
        </p>
        <p className="mt-3 flex items-center gap-2 font-mono text-[0.59rem] text-break-strong">
          <Activity aria-hidden="true" className="size-3.5" />
          {view.forbidden_claims.length} forbidden claim classes remain capsule-bound
        </p>
      </div>
    </div>
  );
}
