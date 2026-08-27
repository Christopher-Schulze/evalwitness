import { ArrowRight, ShieldAlert, ShieldCheck } from "lucide-react";
import { motion, useReducedMotion } from "motion/react";

import { StatusBadge } from "@/components/status-badge";
import type { InspectionItem } from "@/lib/inspection";
import { inspectGeneration } from "@/lib/inspection";
import type { EvidenceReport, MethodGeneration } from "@/lib/report";
import { humanize } from "@/lib/utils";

interface MethodLaneProps {
  method: EvidenceReport["method"];
  onInspect: (item: InspectionItem) => void;
}

export function MethodLane({ method, onInspect }: MethodLaneProps) {
  const reducedMotion = useReducedMotion();
  return (
    <div>
      <div className="grid gap-3 lg:grid-cols-3">
        {method.generations.map((generation, index) => (
          <GenerationCard
            current={generation.generation === method.current}
            generation={generation}
            index={index}
            key={generation.generation}
            onInspect={onInspect}
            reducedMotion={reducedMotion ?? false}
          />
        ))}
      </div>
      <div className="mt-3 grid gap-2 lg:grid-cols-2">
        {method.transitions.map((transition) => (
          <div
            className="flex min-w-0 items-start gap-3 rounded-xl border border-border bg-muted/70 px-4 py-3"
            key={`${transition.from}-${transition.to}`}
          >
            <span className="mt-0.5 flex shrink-0 items-center gap-1 font-mono text-[0.64rem] font-bold text-break-strong">
              {transition.from}
              <ArrowRight aria-hidden="true" className="size-3" />
              {transition.to}
            </span>
            <div className="min-w-0">
              <p className="text-xs leading-5 text-muted-foreground">{transition.reason}</p>
              <p className="mt-1 break-all font-mono text-[0.6rem] leading-5 text-break-strong">
                guard: {transition.guarding_test}
              </p>
            </div>
          </div>
        ))}
      </div>
      <p className="mt-3 rounded-xl border-l-2 border-unavailable bg-unavailable-soft px-4 py-3 text-xs leading-5 text-unavailable-strong">
        {method.boundary}
      </p>
    </div>
  );
}

interface GenerationCardProps {
  generation: MethodGeneration;
  current: boolean;
  index: number;
  reducedMotion: boolean;
  onInspect: (item: InspectionItem) => void;
}

function GenerationCard({
  generation,
  current,
  index,
  reducedMotion,
  onInspect,
}: GenerationCardProps) {
  const rejected = generation.state === "falsified" || generation.state === "superseded";
  const Icon = rejected ? ShieldAlert : ShieldCheck;
  return (
    <motion.button
      animate={{ opacity: 1, y: 0 }}
      aria-label={`Inspect method generation ${generation.generation}`}
      className={`group relative min-h-52 overflow-hidden rounded-[1.15rem] border bg-paper p-5 text-left shadow-panel transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
        rejected ? "border-break/35 hover:border-break" : "border-verified/35 hover:border-verified"
      }`}
      initial={reducedMotion ? false : { opacity: 0, y: 10 }}
      onClick={() => onInspect(inspectGeneration(generation))}
      transition={{
        delay: reducedMotion ? 0 : index * 0.06,
        duration: 0.28,
        ease: [0.2, 0.8, 0.2, 1],
      }}
      type="button"
    >
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="font-mono text-[0.66rem] font-bold uppercase tracking-[0.16em] text-muted-foreground">
            Generation {generation.generation}
          </p>
          <p className="mt-2 text-xl font-semibold tracking-[-0.035em] text-foreground">
            {humanize(generation.state)}
          </p>
        </div>
        <div
          className={`grid size-10 place-items-center rounded-full ${rejected ? "bg-break-soft text-break" : "bg-verified-soft text-verified"}`}
        >
          <Icon aria-hidden="true" className="size-5" />
        </div>
      </div>
      <div className="mt-6 grid grid-cols-2 gap-2">
        {primaryCounts(generation).map((count) => (
          <div className="rounded-lg border border-border bg-muted/55 px-3 py-2" key={count.name}>
            <p className="font-mono text-lg font-semibold tabular-nums text-foreground">
              {count.value}
            </p>
            <p className="mt-0.5 text-[0.57rem] font-bold uppercase leading-4 tracking-[0.1em] text-muted-foreground">
              {humanize(count.name)}
            </p>
          </div>
        ))}
      </div>
      <div className="mt-4 flex items-center justify-between">
        <StatusBadge compact status={generation.state} />
        {current ? (
          <span className="text-[0.58rem] font-bold uppercase tracking-[0.13em] text-verified">
            Current method
          </span>
        ) : null}
      </div>
    </motion.button>
  );
}

function primaryCounts(generation: MethodGeneration) {
  const namesByGeneration: Record<string, string[]> = {
    v1: ["fixtures", "legacy_false_acceptances"],
    v2: ["v2_false_acceptances", "v3_repaired_negatives"],
    v3: ["natural_total_attempts", "scarcity_admitted", "scarcity_target", "scarcity_test_role"],
  };
  const selected = new Set(namesByGeneration[generation.generation] ?? []);
  return generation.frozen_denominators.filter((count) => selected.has(count.name));
}
