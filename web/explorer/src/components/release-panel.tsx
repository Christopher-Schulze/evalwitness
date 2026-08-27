import { FileCheck2, FileSearch } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Table, TableCell, TableFrame, TableHead } from "@/components/ui/table";
import type { InspectionItem } from "@/lib/inspection";
import { inspectArtifact } from "@/lib/inspection";
import type { EvidenceReport } from "@/lib/report";
import { humanize, shortDigest } from "@/lib/utils";

interface ReleasePanelProps {
  release: EvidenceReport["release"];
  onInspect: (item: InspectionItem) => void;
}

export function ReleasePanel({ release, onInspect }: ReleasePanelProps) {
  return (
    <Card>
      <CardHeader className="flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
        <div>
          <p className="text-[0.6rem] font-bold uppercase tracking-[0.14em] text-muted-foreground">
            Public development release
          </p>
          <h3 className="mt-2 text-xl font-semibold tracking-[-0.03em] text-foreground">
            {release.release_id}
          </h3>
          <p className="mt-1 font-mono text-[0.62rem] text-muted-foreground">
            version {release.version}
          </p>
        </div>
        <div className="flex items-center gap-3 rounded-xl border border-verified/25 bg-verified-soft px-4 py-3 text-verified-strong">
          <FileCheck2 aria-hidden="true" className="size-5" />
          <div>
            <p className="font-mono text-lg font-semibold tabular-nums">
              {release.files_verified}/{release.files}
            </p>
            <p className="text-[0.55rem] font-bold uppercase tracking-[0.11em]">files verified</p>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <TableFrame>
          <Table>
            <thead>
              <tr>
                <TableHead>Path</TableHead>
                <TableHead>Role</TableHead>
                <TableHead className="text-right">Bytes</TableHead>
                <TableHead>SHA-256</TableHead>
              </tr>
            </thead>
            <tbody>
              {release.manifest.map((file) => (
                <tr key={file.path}>
                  <TableCell className="min-w-80 font-mono text-[0.65rem] text-foreground">
                    {file.path}
                  </TableCell>
                  <TableCell className="whitespace-nowrap text-xs text-muted-foreground">
                    {humanize(file.role)}
                  </TableCell>
                  <TableCell className="text-right font-mono text-xs tabular-nums text-foreground">
                    {file.bytes.toLocaleString("en-US")}
                  </TableCell>
                  <TableCell className="whitespace-nowrap font-mono text-[0.64rem] text-muted-foreground">
                    {shortDigest(file.payload_sha256)}
                  </TableCell>
                </tr>
              ))}
            </tbody>
          </Table>
        </TableFrame>
        <Button
          className="mt-4"
          onClick={() => onInspect(inspectArtifact(release.source, "Release manifest source"))}
          size="compact"
          type="button"
          variant="outline"
        >
          <FileSearch aria-hidden="true" />
          Inspect release source
        </Button>
      </CardContent>
    </Card>
  );
}
