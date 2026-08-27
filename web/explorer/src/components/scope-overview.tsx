import { Check, Fingerprint, Network, ShieldAlert, ShieldCheck } from "lucide-react";

import { CopyCommand } from "@/components/copy-command";
import { StatusBadge } from "@/components/status-badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import type { EvidenceReport } from "@/lib/report";
import { formatTimestamp, humanize, shortDigest } from "@/lib/utils";

interface ScopeOverviewProps {
  report: EvidenceReport;
}

export function ScopeOverview({ report }: ScopeOverviewProps) {
  return (
    <section
      aria-labelledby="autopsy-title"
      className="mx-auto max-w-[94rem] px-4 pb-7 pt-7 sm:px-7 sm:pt-10 lg:px-10"
    >
      <div className="grid gap-7 lg:grid-cols-[minmax(0,1.35fr)_minmax(20rem,0.65fr)] lg:items-end">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <StatusBadge status={report.claim.accepted ? "accepted" : "rejected"} />
            <StatusBadge status={report.scope.evidence_role} />
            <StatusBadge status={report.scope.route_availability} />
            <Button
              asChild
              className="w-full sm:ml-auto sm:w-auto"
              size="compact"
              variant="destructive"
            >
              <a href="#challenge">
                <ShieldAlert aria-hidden="true" />
                Break this claim
              </a>
            </Button>
          </div>
          <p className="mt-7 text-[0.67rem] font-bold uppercase tracking-[0.2em] text-muted-foreground">
            Claim autopsy · {report.claim.claim_id}
          </p>
          <h1
            id="autopsy-title"
            className="mt-3 max-w-5xl text-balance text-[clamp(2.7rem,6vw,6.2rem)] font-semibold leading-[0.9] tracking-[-0.07em] text-foreground"
          >
            {humanize(report.claim.failable_property)}
          </h1>
          <p className="mt-5 max-w-3xl text-pretty text-base leading-7 text-muted-foreground sm:text-lg">
            {report.claim.observable_failure_condition}
          </p>
        </div>
        <Card className="overflow-hidden border-graphite bg-graphite text-white">
          <div className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-5 p-5 sm:p-6">
            <div className="grid size-14 place-items-center rounded-full border border-verified/70 bg-verified text-white shadow-[0_0_0_7px_rgb(29_94_232_/_0.15)]">
              <Check aria-hidden="true" className="size-6" strokeWidth={2.6} />
            </div>
            <div>
              <p className="text-[0.62rem] font-bold uppercase tracking-[0.18em] text-white/55">
                Current disposition
              </p>
              <p className="mt-1 text-xl font-semibold tracking-[-0.03em]">
                {report.claim.accepted
                  ? "Accepted, narrowly scoped"
                  : "Rejected by the evidence contract"}
              </p>
            </div>
            <dl className="col-span-2 grid grid-cols-2 gap-px overflow-hidden rounded-xl bg-white/10 sm:grid-cols-4">
              <ScopeFact
                icon={ShieldCheck}
                label="Evidence"
                value={humanize(report.scope.evidence_role)}
              />
              <ScopeFact
                icon={Network}
                label="Route"
                value={report.scope.route ?? humanize(report.scope.route_availability)}
              />
              <ScopeFact
                icon={Fingerprint}
                label="Capsule"
                value={shortDigest(report.capsule.capsule_id)}
                mono
              />
              <ScopeFact
                icon={Check}
                label="Verified"
                value={`${report.release.files_verified}/${report.release.files} files`}
              />
            </dl>
            <p className="col-span-2 font-mono text-[0.61rem] leading-5 text-white/52">
              Freshness {report.bom.freshness.state} · evaluated{" "}
              {formatTimestamp(report.bom.freshness.evaluated_at)} UTC
            </p>
          </div>
        </Card>
      </div>
      <div className="mt-7 grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(24rem,0.62fr)]">
        <ScopeStrip report={report} />
        <CopyCommand command={report.reproduction.command} />
      </div>
    </section>
  );
}

interface ScopeFactProps {
  icon: typeof Check;
  label: string;
  value: string;
  mono?: boolean;
}

function ScopeFact({ icon: Icon, label, value, mono = false }: ScopeFactProps) {
  return (
    <div className="min-w-0 bg-graphite p-3">
      <dt className="flex items-center gap-1.5 text-[0.56rem] font-bold uppercase tracking-[0.13em] text-white/48">
        <Icon aria-hidden="true" className="size-3" />
        {label}
      </dt>
      <dd
        className={`mt-1.5 truncate text-[0.7rem] font-semibold text-white ${mono ? "font-mono" : ""}`}
      >
        {value}
      </dd>
    </div>
  );
}

function ScopeStrip({ report }: ScopeOverviewProps) {
  const facts = [
    ["Evaluator", report.scope.evaluator],
    ["Release", report.scope.release_version],
    ["Development fixtures", String(report.scope.development_fixtures)],
    ["Empirical task groups", String(report.scope.empirical_task_groups)],
    ["Research sources", String(report.scope.research_admitted_sources)],
    ["Provider calls", String(report.scope.provider_calls)],
  ];
  return (
    <dl className="grid grid-cols-2 gap-px overflow-hidden rounded-xl border border-border bg-border sm:grid-cols-3">
      {facts.map(([label, value]) => (
        <div className="min-w-0 bg-paper px-4 py-3" key={label}>
          <dt className="text-[0.57rem] font-bold uppercase tracking-[0.13em] text-muted-foreground">
            {label}
          </dt>
          <dd className="mt-1 truncate font-mono text-xs font-semibold text-foreground">{value}</dd>
        </div>
      ))}
    </dl>
  );
}
