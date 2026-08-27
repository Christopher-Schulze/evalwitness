import { Database, FileSearch } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Table, TableCell, TableFrame, TableHead } from "@/components/ui/table";
import type { InspectionItem } from "@/lib/inspection";
import { inspectArtifact } from "@/lib/inspection";
import type { EvidenceReport } from "@/lib/report";
import { humanize } from "@/lib/utils";

interface DatasetPanelProps {
  dataset: EvidenceReport["dataset"];
  onInspect: (item: InspectionItem) => void;
}

export function DatasetPanel({ dataset, onInspect }: DatasetPanelProps) {
  return (
    <div className="grid gap-4 lg:grid-cols-[0.82fr_1.18fr]">
      <Card>
        <CardHeader className="flex flex-row items-start justify-between gap-4">
          <div>
            <p className="text-[0.59rem] font-bold uppercase tracking-[0.14em] text-muted-foreground">
              Dataset card
            </p>
            <h3 className="mt-2 text-lg font-semibold tracking-[-0.025em] text-foreground">
              {dataset.title}
            </h3>
            <p className="mt-1 font-mono text-[0.59rem] text-muted-foreground">
              {dataset.dataset_id}
            </p>
          </div>
          <span className="grid size-10 place-items-center rounded-full bg-verified-soft text-verified">
            <Database aria-hidden="true" className="size-5" />
          </span>
        </CardHeader>
        <CardContent>
          <TableFrame>
            <Table>
              <thead>
                <tr>
                  <TableHead>Denominator</TableHead>
                  <TableHead className="text-right">Value</TableHead>
                </tr>
              </thead>
              <tbody>
                {dataset.counts.map((count) => (
                  <tr key={count.name}>
                    <TableCell className="text-xs font-medium text-foreground">
                      {humanize(count.name)}
                    </TableCell>
                    <TableCell className="text-right font-mono text-xs font-semibold tabular-nums text-foreground">
                      {count.value}
                    </TableCell>
                  </tr>
                ))}
              </tbody>
            </Table>
          </TableFrame>
          <Button
            className="mt-4"
            onClick={() => onInspect(inspectArtifact(dataset.source, "Dataset card source"))}
            size="compact"
            type="button"
            variant="outline"
          >
            <FileSearch aria-hidden="true" />
            Inspect source
          </Button>
        </CardContent>
      </Card>
      <div className="grid gap-4 sm:grid-cols-2">
        <BoundaryList items={dataset.known_limitations} label="Known limitations" tone="rejected" />
        <BoundaryList
          items={dataset.out_of_scope_uses}
          label="Out-of-scope uses"
          tone="unavailable"
        />
      </div>
    </div>
  );
}

function BoundaryList({
  items,
  label,
  tone,
}: {
  items: string[];
  label: string;
  tone: "rejected" | "unavailable";
}) {
  return (
    <Card className={tone === "rejected" ? "border-break/25" : "border-unavailable/25"}>
      <CardHeader>
        <p
          className={`text-[0.61rem] font-bold uppercase tracking-[0.14em] ${tone === "rejected" ? "text-break-strong" : "text-unavailable-strong"}`}
        >
          {label}
        </p>
      </CardHeader>
      <CardContent>
        <ul className="space-y-3">
          {items.map((item) => (
            <li className="flex gap-3 text-xs leading-5 text-muted-foreground" key={item}>
              <span
                className={`mt-2 size-1.5 shrink-0 rounded-full ${tone === "rejected" ? "bg-break" : "bg-unavailable"}`}
              />
              {item}
            </li>
          ))}
        </ul>
      </CardContent>
    </Card>
  );
}
