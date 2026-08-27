import { ArrowRight, FileSearch } from "lucide-react";

import { StatusBadge } from "@/components/status-badge";
import { Button } from "@/components/ui/button";
import type { InspectionItem } from "@/lib/inspection";
import { inspectArtifact } from "@/lib/inspection";
import type { EvidenceReport } from "@/lib/report";
import { humanize } from "@/lib/utils";

interface LimitationsPanelProps {
  limitations: EvidenceReport["limitations"];
  onInspect: (item: InspectionItem) => void;
}

export function LimitationsPanel({ limitations, onInspect }: LimitationsPanelProps) {
  return (
    <div className="overflow-hidden rounded-[1.2rem] border border-border bg-paper shadow-panel">
      {limitations.map((limitation, index) => (
        <article
          className="grid gap-4 border-b border-border p-5 last:border-b-0 lg:grid-cols-[12rem_1fr_1fr_auto] lg:items-center"
          key={limitation.id}
        >
          <div>
            <span className="font-mono text-[0.56rem] text-muted-foreground">
              L{String(index + 1).padStart(2, "0")}
            </span>
            <h3 className="mt-1 text-sm font-semibold text-foreground">
              {humanize(limitation.id)}
            </h3>
            <div className="mt-2">
              <StatusBadge compact status={limitation.current_availability} />
            </div>
          </div>
          <div>
            <p className="text-[0.56rem] font-bold uppercase tracking-[0.11em] text-muted-foreground">
              Current boundary
            </p>
            <p className="mt-1.5 text-xs leading-5 text-foreground">{limitation.boundary}</p>
          </div>
          <div>
            <p className="flex items-center gap-1.5 text-[0.56rem] font-bold uppercase tracking-[0.11em] text-muted-foreground">
              <ArrowRight aria-hidden="true" className="size-3" /> Required resolution
            </p>
            <p className="mt-1.5 text-xs leading-5 text-muted-foreground">
              {limitation.required_resolution}
            </p>
          </div>
          <Button
            aria-label={`Inspect source for ${humanize(limitation.id)}`}
            onClick={() =>
              onInspect(
                inspectArtifact(limitation.source, `${humanize(limitation.id)} limitation source`),
              )
            }
            size="icon"
            type="button"
            variant="ghost"
          >
            <FileSearch aria-hidden="true" />
          </Button>
        </article>
      ))}
    </div>
  );
}
