import { ArrowDown, Check, CircleSlash2, Minus } from "lucide-react";

import { StatusBadge } from "@/components/status-badge";
import type { InspectionItem } from "@/lib/inspection";
import { inspectLayer } from "@/lib/inspection";
import type { EvidenceReport, TransportLayer, TransportPath } from "@/lib/report";
import { humanize, shortDigest } from "@/lib/utils";

interface EvidenceRailProps {
  report: EvidenceReport;
  onInspect: (item: InspectionItem) => void;
}

export function EvidenceRail({ report, onInspect }: EvidenceRailProps) {
  return (
    <div className="space-y-3">
      {report.transport.paths.map((path) => (
        <TransportPathRow
          key={path.path_id}
          onInspect={onInspect}
          path={path}
          selected={path.path_id === report.transport.selected_path}
        />
      ))}
      <div className="grid gap-3 sm:grid-cols-2">
        <RailDisposition
          label="Accepted BOM"
          status={report.claim.accepted ? "accepted" : "rejected"}
          value={`${report.bom.surviving_channels.length}/${report.bom.decisive_channels.length} decisive channels survived`}
        />
        <RailDisposition
          label="Earliest loss"
          status="first_loss"
          value={`${humanize(report.loss.first_loss_state)} · ${humanize(report.loss.disposition)}`}
        />
      </div>
    </div>
  );
}

interface TransportPathRowProps {
  path: TransportPath;
  selected: boolean;
  onInspect: (item: InspectionItem) => void;
}

function TransportPathRow({ path, selected, onInspect }: TransportPathRowProps) {
  const rejected = path.layers.some((layer) => layer.status === "first_loss");
  return (
    <article
      className={`rounded-[1.2rem] border bg-paper p-4 shadow-panel sm:p-5 ${rejected ? "border-break/30" : "border-verified/30"}`}
    >
      <header className="flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
        <div className="flex items-center gap-3">
          <span
            className={`grid size-9 shrink-0 place-items-center rounded-full ${rejected ? "bg-break-soft text-break" : "bg-verified-soft text-verified"}`}
          >
            {rejected ? (
              <CircleSlash2 aria-hidden="true" className="size-4" />
            ) : (
              <Check aria-hidden="true" className="size-4" />
            )}
          </span>
          <div>
            <h3 className="text-sm font-semibold tracking-[-0.015em] text-foreground">
              {path.label}
            </h3>
            <p className="mt-0.5 font-mono text-[0.58rem] text-muted-foreground">{path.path_id}</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {selected ? (
            <span className="text-[0.57rem] font-bold uppercase tracking-[0.12em] text-break">
              Selected loss path
            </span>
          ) : null}
          <StatusBadge compact status={path.terminal_state} />
        </div>
      </header>
      <div className="relative mt-5 grid gap-2 md:grid-cols-5 md:gap-3">
        <div
          aria-hidden="true"
          className={`absolute left-[10%] right-[10%] top-5 hidden h-px md:block ${rejected ? "bg-break/30" : "bg-verified/35"}`}
        />
        {path.layers.map((layer, index) => (
          <RailNode
            index={index}
            key={layer.layer}
            layer={layer}
            onInspect={onInspect}
            path={path}
          />
        ))}
      </div>
    </article>
  );
}

interface RailNodeProps {
  path: TransportPath;
  layer: TransportLayer;
  index: number;
  onInspect: (item: InspectionItem) => void;
}

function RailNode({ path, layer, index, onInspect }: RailNodeProps) {
  const lost = layer.status === "first_loss" || layer.status === "not_reached";
  const unavailable = layer.detail_availability !== "available";
  return (
    <div className="relative">
      {index > 0 ? (
        <ArrowDown
          aria-hidden="true"
          className="absolute -top-2 left-4 size-3 text-border-strong md:hidden"
        />
      ) : null}
      <button
        aria-label={`Inspect ${path.label} ${humanize(layer.layer)}`}
        className={`relative z-10 flex w-full items-center gap-3 rounded-xl border px-3 py-3 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring md:block md:min-h-28 ${
          lost
            ? "border-break/35 bg-break-soft hover:border-break"
            : unavailable
              ? "border-unavailable/30 bg-unavailable-soft hover:border-unavailable"
              : "border-verified/25 bg-verified-soft hover:border-verified"
        }`}
        onClick={() => onInspect(inspectLayer(path, layer))}
        type="button"
      >
        <span
          className={`grid size-8 shrink-0 place-items-center rounded-full border bg-paper md:mx-auto ${lost ? "border-break text-break" : unavailable ? "border-unavailable text-unavailable" : "border-verified text-verified"}`}
        >
          {lost ? (
            <CircleSlash2 aria-hidden="true" className="size-3.5" />
          ) : unavailable ? (
            <Minus aria-hidden="true" className="size-3.5" />
          ) : (
            <Check aria-hidden="true" className="size-3.5" />
          )}
        </span>
        <span className="min-w-0 md:mt-2 md:block md:text-center">
          <span className="block text-[0.62rem] font-bold uppercase tracking-[0.1em] text-foreground">
            {humanize(layer.layer)}
          </span>
          <span className="mt-1 block truncate font-mono text-[0.54rem] text-muted-foreground">
            {layer.object_digest === null
              ? humanize(layer.object_digest_availability)
              : shortDigest(layer.object_digest)}
          </span>
          <span
            className={`mt-1 block text-[0.55rem] font-bold uppercase tracking-[0.09em] ${lost ? "text-break-strong" : unavailable ? "text-unavailable-strong" : "text-verified-strong"}`}
          >
            {humanize(layer.status)}
          </span>
        </span>
      </button>
    </div>
  );
}

interface RailDispositionProps {
  label: string;
  status: string;
  value: string;
}

function RailDisposition({ label, status, value }: RailDispositionProps) {
  return (
    <div className="flex items-center justify-between gap-4 rounded-xl border border-border bg-muted/65 px-4 py-3">
      <div>
        <p className="text-[0.58rem] font-bold uppercase tracking-[0.13em] text-muted-foreground">
          {label}
        </p>
        <p className="mt-1 text-xs font-medium text-foreground">{value}</p>
      </div>
      <StatusBadge compact status={status} />
    </div>
  );
}
