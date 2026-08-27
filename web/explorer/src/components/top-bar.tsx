import { Activity, FileText, GitCompareArrows, Orbit, Printer } from "lucide-react";

import { Button } from "@/components/ui/button";
import { shortDigest } from "@/lib/utils";

interface TopBarProps {
  reportDigest: string;
  relianceAvailable: boolean;
  identicalResponseAvailable: boolean;
  onOpenTables: () => void;
}

export function TopBar({
  reportDigest,
  relianceAvailable,
  identicalResponseAvailable,
  onOpenTables,
}: TopBarProps) {
  return (
    <header className="border-b border-border bg-paper/94" data-print-hidden="true">
      <div className="mx-auto flex min-h-16 max-w-[94rem] items-center justify-between gap-4 px-4 sm:px-7 lg:px-10">
        <div className="flex min-w-0 items-center gap-3">
          <div className="grid size-9 shrink-0 place-items-center rounded-lg bg-graphite font-mono text-[0.68rem] font-bold tracking-[-0.05em] text-white">
            EW
          </div>
          <div className="min-w-0">
            <p className="truncate text-[0.72rem] font-bold uppercase tracking-[0.14em] text-foreground">
              EvalWitness
            </p>
            <p className="truncate font-mono text-[0.59rem] text-muted-foreground">
              report {shortDigest(reportDigest)}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-1.5">
          {identicalResponseAvailable ? (
            <Button asChild size="compact" variant="ghost">
              <a aria-label="Open exact-response study" href="#identical-response">
                <GitCompareArrows aria-hidden="true" />
                <span className="hidden lg:inline">Exact response</span>
              </a>
            </Button>
          ) : null}
          {relianceAvailable ? (
            <Button asChild size="compact" variant="ghost">
              <a aria-label="Open evidence reliance map" href="#reliance">
                <Orbit aria-hidden="true" />
                <span className="hidden sm:inline">Reliance map</span>
              </a>
            </Button>
          ) : null}
          <Button asChild size="compact" variant="ghost">
            <a aria-label="Open stress lab" href="#stress">
              <Activity aria-hidden="true" />
              <span className="hidden sm:inline">Stress witness</span>
            </a>
          </Button>
          <Button onClick={onOpenTables} size="compact" type="button" variant="ghost">
            <FileText aria-hidden="true" />
            <span className="hidden sm:inline">Table fallback</span>
            <span className="sm:hidden">Tables</span>
          </Button>
          <Button onClick={() => window.print()} size="compact" type="button" variant="outline">
            <Printer aria-hidden="true" />
            <span className="hidden sm:inline">Print evidence</span>
            <span className="sm:hidden">Print</span>
          </Button>
        </div>
      </div>
    </header>
  );
}
