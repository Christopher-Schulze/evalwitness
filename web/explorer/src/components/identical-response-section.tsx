import { FileSearch, GitCompareArrows, ShieldCheck } from "lucide-react";

import { CopyCommand } from "@/components/copy-command";
import { SectionHeading } from "@/components/section-heading";
import { StatusBadge } from "@/components/status-badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type { InspectionItem } from "@/lib/inspection";
import { inspectArtifact } from "@/lib/inspection";
import type { IdenticalResponseView } from "@/lib/report";
import { humanize, shortDigest } from "@/lib/utils";

interface IdenticalResponseSectionProps {
  view: IdenticalResponseView;
  onInspect: (item: InspectionItem) => void;
}

export function IdenticalResponseSection({ view, onInspect }: IdenticalResponseSectionProps) {
  return (
    <section
      aria-labelledby="identical-response-title"
      className="scroll-mt-4 border-y border-border bg-paper py-12 sm:py-16"
      id="identical-response"
    >
      <div className="mx-auto max-w-[94rem] px-4 sm:px-7 lg:px-10">
        <SectionHeading
          aside={
            <div className="flex flex-wrap gap-2">
              <StatusBadge status={view.status} />
              <StatusBadge status={view.availability} />
            </div>
          }
          description="Both decision arms consume the same sealed response record. The explorer shows where extraction semantics alone changed the decision, then exposes every claim guard and clean-clone reproduction boundary."
          eyebrow="TASK 070 · Exact-response counterfactual"
          id="identical-response-title"
          title="Same response. Different decision."
        />

        <div className="grid overflow-hidden rounded-[1.35rem] border border-border bg-border sm:grid-cols-2 xl:grid-cols-4">
          <StudyMetric label="Locked groups" value={view.comparison.task_groups} />
          <StudyMetric label="Agreements" value={view.comparison.agreements} />
          <StudyMetric label="Split decisions" value={view.comparison.disagreements} tone="break" />
          <StudyMetric label="Passing challenges" value={view.challenges.total} />
        </div>

        <Card className="mt-5 overflow-hidden">
          <Tabs defaultValue="claims">
            <div className="flex flex-col justify-between gap-4 border-b border-border px-5 py-4 sm:flex-row sm:items-center sm:px-6">
              <div className="flex items-center gap-3">
                <div className="grid size-9 place-items-center rounded-lg bg-graphite text-white">
                  <GitCompareArrows aria-hidden="true" className="size-4" />
                </div>
                <div>
                  <p className="text-sm font-semibold text-foreground">Canonical capsule path</p>
                  <p className="font-mono text-[0.62rem] text-muted-foreground">
                    {view.model} · {view.observed_calls} observed calls ·{" "}
                    {shortDigest(view.capsule_id)}
                  </p>
                </div>
              </div>
              <TabsList aria-label="Identical-response evidence view">
                <TabsTrigger value="claims">Claim chain</TabsTrigger>
                <TabsTrigger value="failures">7 disagreements</TabsTrigger>
                <TabsTrigger value="challenges">34 receipts</TabsTrigger>
              </TabsList>
            </div>

            <CardContent>
              <TabsContent className="mt-0" value="claims">
                <ClaimChain view={view} />
              </TabsContent>
              <TabsContent className="mt-0" value="failures">
                <FailureGallery view={view} />
              </TabsContent>
              <TabsContent className="mt-0" value="challenges">
                <ChallengeReceipts view={view} />
              </TabsContent>
            </CardContent>
          </Tabs>
        </Card>

        <div className="mt-5 grid gap-5 lg:grid-cols-[1fr_0.82fr]">
          <Card>
            <CardContent>
              <div className="flex items-center justify-between gap-4">
                <div>
                  <p className="text-[0.62rem] font-bold uppercase tracking-[0.16em] text-muted-foreground">
                    Evidence ceiling
                  </p>
                  <p className="mt-1 font-mono text-xs font-semibold text-foreground">
                    {humanize(view.evidence_ceiling)}
                  </p>
                </div>
                <Button
                  onClick={() =>
                    onInspect(
                      inspectArtifact(
                        view.source,
                        "TASK 070 outer capsule",
                        `#artifact/${view.source.id}`,
                      ),
                    )
                  }
                  size="compact"
                  type="button"
                  variant="outline"
                >
                  <FileSearch aria-hidden="true" />
                  Inspect capsule
                </Button>
              </div>
              <ul className="mt-5 grid gap-2 text-xs leading-5 text-muted-foreground sm:grid-cols-2">
                {view.limitations.map((limitation) => (
                  <li className="border-l-2 border-break/55 pl-3" key={limitation}>
                    {limitation}
                  </li>
                ))}
              </ul>
            </CardContent>
          </Card>

          <div>
            <div className="mb-3 flex items-center justify-between gap-3">
              <div>
                <p className="text-[0.62rem] font-bold uppercase tracking-[0.16em] text-muted-foreground">
                  Clean-clone reproduction
                </p>
                <p className="mt-1 text-xs text-muted-foreground">
                  {view.reproduction.artifacts_verified} artifacts · network{" "}
                  {view.reproduction.network} · 0 provider calls
                </p>
              </div>
              <ShieldCheck aria-hidden="true" className="size-5 text-verified" />
            </div>
            <CopyCommand command={view.reproduction.command} label="Copy reproduction" />
          </div>
        </div>
      </div>
    </section>
  );
}

function StudyMetric({
  label,
  value,
  tone = "default",
}: {
  label: string;
  value: number;
  tone?: "default" | "break";
}) {
  return (
    <div className="bg-paper px-5 py-5 sm:px-6">
      <p className="text-[0.59rem] font-bold uppercase tracking-[0.16em] text-muted-foreground">
        {label}
      </p>
      <p
        className={`mt-2 font-mono text-4xl font-semibold tracking-[-0.06em] ${tone === "break" ? "text-break" : "text-foreground"}`}
      >
        {value}
      </p>
    </div>
  );
}

function ClaimChain({ view }: { view: IdenticalResponseView }) {
  return (
    <ol className="grid gap-px overflow-hidden rounded-xl border border-border bg-border">
      {view.claims.map((claim, index) => (
        <li
          className="grid gap-3 bg-paper px-4 py-3.5 md:grid-cols-[4.8rem_1fr_auto] md:items-center"
          key={claim.claim_id}
        >
          <div>
            <p className="font-mono text-xs font-semibold text-foreground">{claim.claim_id}</p>
            <p className="mt-0.5 text-[0.58rem] uppercase tracking-[0.12em] text-muted-foreground">
              step {String(index + 1).padStart(2, "0")}
            </p>
          </div>
          <div>
            <p className="text-sm font-medium leading-5 text-foreground">{claim.statement}</p>
            <p className="mt-1 font-mono text-[0.6rem] text-muted-foreground">
              {claim.evidence_components.length} bound components · {claim.challenge_receipts}{" "}
              guards
            </p>
          </div>
          <div className="flex items-center gap-2 md:justify-end">
            <span className="font-mono text-[0.62rem] font-semibold text-muted-foreground">
              {claim.evidence_level}
            </span>
            <StatusBadge compact status={claim.status} />
          </div>
        </li>
      ))}
    </ol>
  );
}

function FailureGallery({ view }: { view: IdenticalResponseView }) {
  return (
    <div className="grid gap-3 lg:grid-cols-2">
      {view.failures.map((failure) => (
        <article className="rounded-xl border border-border bg-muted/45 p-4" key={failure.group_id}>
          <div className="flex items-start justify-between gap-3">
            <div>
              <p className="text-sm font-semibold text-foreground">{humanize(failure.task_id)}</p>
              <p className="mt-1 font-mono text-[0.58rem] text-muted-foreground">
                record {shortDigest(failure.record_digest)}
              </p>
            </div>
            <StatusBadge compact status="violated" />
          </div>
          <div className="mt-4 grid grid-cols-2 gap-2">
            <Decision label="Distribution aware" view={failure.distribution_aware} />
            <Decision label="Chosen token" view={failure.chosen_token} />
          </div>
        </article>
      ))}
    </div>
  );
}

function Decision({
  label,
  view,
}: {
  label: string;
  view: IdenticalResponseView["failures"][number]["chosen_token"];
}) {
  return (
    <div className="rounded-lg border border-border bg-paper p-3">
      <p className="text-[0.57rem] font-bold uppercase tracking-[0.12em] text-muted-foreground">
        {label}
      </p>
      <p className="mt-2 font-mono text-xs font-semibold text-foreground">
        {humanize(view.state)}
        {view.selected_index === undefined ? "" : ` · #${view.selected_index + 1}`}
      </p>
      <p className="mt-1 font-mono text-[0.58rem] text-muted-foreground">margin {view.margin}</p>
    </div>
  );
}

function ChallengeReceipts({ view }: { view: IdenticalResponseView }) {
  return (
    <div>
      <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
        {view.challenges.class_counts.map((item) => (
          <div className="rounded-lg border border-border bg-muted/45 px-3 py-2.5" key={item.name}>
            <p className="text-[0.58rem] font-bold uppercase tracking-[0.11em] text-muted-foreground">
              {humanize(item.name)}
            </p>
            <p className="mt-1 font-mono text-lg font-semibold text-foreground">{item.value}</p>
          </div>
        ))}
      </div>
      <div
        aria-label="Challenge receipts"
        className="mt-4 max-h-72 overflow-y-auto rounded-xl border border-border focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        role="region"
        tabIndex={0}
      >
        {view.challenges.receipts.map((receipt) => (
          <div
            className="grid gap-2 border-b border-border px-4 py-3 last:border-b-0 md:grid-cols-[8rem_1fr_auto] md:items-center"
            key={receipt.digest}
          >
            <div>
              <p className="font-mono text-xs font-semibold text-foreground">{receipt.claim_id}</p>
              <p className="mt-0.5 text-[0.58rem] text-muted-foreground">
                {humanize(receipt.class)}
              </p>
            </div>
            <div>
              <p className="text-xs leading-5 text-foreground">{receipt.mutation}</p>
              <p className="mt-0.5 font-mono text-[0.58rem] text-muted-foreground">
                {receipt.before_state} → {receipt.after_state}
              </p>
            </div>
            <StatusBadge compact status="passed" />
          </div>
        ))}
      </div>
    </div>
  );
}
