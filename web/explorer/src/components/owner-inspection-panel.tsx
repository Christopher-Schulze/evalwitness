import { FileSearch, LockKeyhole, ScanEye, ShieldCheck } from "lucide-react";

import { StatusBadge } from "@/components/status-badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Table, TableCell, TableFrame, TableHead } from "@/components/ui/table";
import type { InspectionItem } from "@/lib/inspection";
import { inspectArtifact } from "@/lib/inspection";
import type { EvidenceReport } from "@/lib/report";
import { humanize, shortDigest } from "@/lib/utils";

interface OwnerInspectionPanelProps {
  inspection: EvidenceReport["owner_inspection"];
  onInspect: (item: InspectionItem) => void;
}

export function OwnerInspectionPanel({ inspection, onInspect }: OwnerInspectionPanelProps) {
  const assessments = inspection.assessments;
  return (
    <div className="space-y-4">
      <div className="grid gap-4 xl:grid-cols-[1.15fr_0.85fr]">
        <Card className="border-break/25">
          <CardHeader className="flex flex-row items-start justify-between gap-4">
            <div>
              <p className="text-[0.59rem] font-bold uppercase tracking-[0.14em] text-break-strong">
                Owner-inspection readiness lane
              </p>
              <h3 className="mt-2 text-xl font-semibold tracking-[-0.03em] text-foreground">
                Complete inspection, revision still required.
              </h3>
              <p className="mt-2 max-w-2xl text-xs leading-5 text-muted-foreground">
                This is a verified public aggregate over a withheld private chain. It is not a human
                study, independent review, verifier result, or launch authorization.
              </p>
            </div>
            <span className="grid size-11 shrink-0 place-items-center rounded-full bg-break-soft text-break">
              <ScanEye aria-hidden="true" className="size-5" />
            </span>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
              <Metric
                label="Assessments"
                value={`${assessments.completed} / ${assessments.required}`}
              />
              <Metric label="Dimensions" value={String(inspection.dimensions.length)} />
              <Metric label="Core cases" value={String(assessments.core_cases)} />
              <Metric label="Scarcity test" value={String(assessments.scarcity_test_cases)} />
            </div>
            <div className="mt-5 grid gap-3 sm:grid-cols-3">
              <Outcome label="Core" status={inspection.outcomes.core_status} />
              <Outcome label="Scarcity" status={inspection.outcomes.scarcity_status} />
              <Outcome label="Overall" status={inspection.outcomes.overall_status} />
            </div>
            <Button
              className="mt-5"
              onClick={() =>
                onInspect(
                  inspectArtifact(
                    inspection.source,
                    "Owner-inspection public attestation",
                    "#owner-inspection/source",
                  ),
                )
              }
              size="compact"
              type="button"
              variant="outline"
            >
              <FileSearch aria-hidden="true" />
              Inspect public source
            </Button>
          </CardContent>
        </Card>

        <Card className="border-unavailable/30 bg-unavailable-soft/45">
          <CardHeader className="flex flex-row items-center justify-between gap-4">
            <div>
              <p className="text-[0.59rem] font-bold uppercase tracking-[0.14em] text-unavailable-strong">
                Disclosure firewall
              </p>
              <p className="mt-1 font-mono text-[0.62rem] text-muted-foreground">
                {shortDigest(inspection.package_inventory_digest)}
              </p>
            </div>
            <LockKeyhole aria-hidden="true" className="size-5 text-unavailable" />
          </CardHeader>
          <CardContent>
            <dl className="space-y-3">
              <DisclosureRow
                label="Private chain verified"
                value={inspection.disclosure.private_chain_verified ? "yes" : "no"}
              />
              <DisclosureRow
                label="Journal identities disclosed"
                value={inspection.disclosure.private_journal_identities_disclosed ? "yes" : "no"}
              />
              <DisclosureRow
                label="Restricted evidence disclosed"
                value={inspection.disclosure.restricted_evidence_disclosed ? "yes" : "no"}
              />
              <DisclosureRow label="Human study" value={humanize(inspection.human_study_status)} />
              <DisclosureRow
                label="External action"
                value={humanize(inspection.external_action_status)}
              />
              <DisclosureRow
                label="Source reproduction"
                value={humanize(inspection.disclosure.source_reproduction)}
              />
            </dl>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between gap-4">
          <div>
            <p className="text-[0.59rem] font-bold uppercase tracking-[0.14em] text-muted-foreground">
              Sixteen-dimension aggregate
            </p>
            <h3 className="mt-1 text-lg font-semibold tracking-[-0.025em] text-foreground">
              Failures remain in the denominator.
            </h3>
          </div>
          <ShieldCheck aria-hidden="true" className="size-5 text-verified" />
        </CardHeader>
        <CardContent>
          <TableFrame>
            <Table>
              <thead>
                <tr>
                  <TableHead>Scope</TableHead>
                  <TableHead>Dimension</TableHead>
                  <TableHead className="text-right">Applicable</TableHead>
                  <TableHead className="text-right">Passed</TableHead>
                  <TableHead className="text-right">Failed</TableHead>
                  <TableHead className="text-right">Indeterminate</TableHead>
                </tr>
              </thead>
              <tbody>
                {inspection.dimensions.map((dimension) => (
                  <tr key={`${dimension.scope}:${dimension.dimension}`}>
                    <TableCell className="whitespace-nowrap font-mono text-[0.63rem]">
                      {humanize(dimension.scope)}
                    </TableCell>
                    <TableCell className="min-w-60 text-xs font-medium">
                      {humanize(dimension.dimension)}
                    </TableCell>
                    <NumberCell value={dimension.applicable} />
                    <NumberCell value={dimension.passed} />
                    <NumberCell tone="rejected" value={dimension.failed} />
                    <NumberCell tone="unavailable" value={dimension.indeterminate} />
                  </tr>
                ))}
              </tbody>
            </Table>
          </TableFrame>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <p className="text-[0.59rem] font-bold uppercase tracking-[0.14em] text-muted-foreground">
            Closed public claim boundary
          </p>
        </CardHeader>
        <CardContent className="grid gap-3 sm:grid-cols-2">
          {inspection.claims.map((claim) => (
            <article
              className="min-w-0 rounded-xl border border-border bg-muted/45 p-4"
              key={claim.id}
            >
              <div className="flex min-w-0 flex-col items-start gap-2 sm:flex-row sm:justify-between sm:gap-3">
                <p className="break-all font-mono text-[0.62rem] font-bold text-muted-foreground">
                  {claim.id}
                </p>
                <StatusBadge compact status={claim.status} />
              </div>
              <p className="mt-3 text-xs font-semibold leading-5 text-foreground">{claim.claim}</p>
              <p className="mt-2 text-[0.68rem] leading-5 text-muted-foreground">
                {claim.evidence}
              </p>
            </article>
          ))}
        </CardContent>
      </Card>
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border border-border bg-muted/55 px-4 py-3">
      <p className="font-mono text-xl font-semibold tabular-nums text-foreground">{value}</p>
      <p className="mt-1 text-[0.56rem] font-bold uppercase tracking-[0.11em] text-muted-foreground">
        {label}
      </p>
    </div>
  );
}

function Outcome({ label, status }: { label: string; status: string }) {
  return (
    <div className="flex items-center justify-between gap-3 rounded-xl border border-border px-3 py-2.5">
      <span className="text-[0.62rem] font-bold uppercase tracking-[0.1em] text-muted-foreground">
        {label}
      </span>
      <StatusBadge compact status={status} />
    </div>
  );
}

function DisclosureRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-start justify-between gap-4 border-b border-unavailable/20 pb-3 last:border-0 last:pb-0">
      <dt className="text-[0.65rem] font-medium text-muted-foreground">{label}</dt>
      <dd className="max-w-[58%] text-right font-mono text-[0.61rem] leading-5 text-foreground">
        {value}
      </dd>
    </div>
  );
}

function NumberCell({
  value,
  tone = "neutral",
}: {
  value: number;
  tone?: "neutral" | "rejected" | "unavailable";
}) {
  const color =
    value === 0
      ? "text-muted-foreground"
      : tone === "rejected"
        ? "text-break-strong"
        : tone === "unavailable"
          ? "text-unavailable-strong"
          : "text-foreground";
  return (
    <TableCell className={`text-right font-mono text-xs font-semibold tabular-nums ${color}`}>
      {value}
    </TableCell>
  );
}
