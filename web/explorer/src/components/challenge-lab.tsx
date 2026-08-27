import { ArrowRight, CircleSlash2, FileCheck2, LockKeyhole, ShieldAlert } from "lucide-react";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";

import { CopyCommand } from "@/components/copy-command";
import { SectionHeading } from "@/components/section-heading";
import { StatusBadge } from "@/components/status-badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type { InspectionItem } from "@/lib/inspection";
import { inspectReceipt } from "@/lib/inspection";
import type { ChallengeClass, EvidenceReport } from "@/lib/report";
import { humanize, shortDigest } from "@/lib/utils";

interface ChallengeLabProps {
  report: EvidenceReport;
  onInspect: (item: InspectionItem) => void;
  selectedClass: string;
  onSelectedClassChange: (value: string) => void;
}

export function ChallengeLab({
  report,
  onInspect,
  selectedClass,
  onSelectedClassChange,
}: ChallengeLabProps) {
  const reducedMotion = useReducedMotion();

  return (
    <section
      aria-labelledby="challenge-title"
      className="mx-auto max-w-[94rem] scroll-mt-6 px-4 py-12 sm:px-7 sm:py-16 lg:px-10"
      id="challenge"
    >
      <SectionHeading
        aside={<StatusBadge status={report.challenge.claim_status} />}
        description="Select a ledger-registered mutation. Every visible withdrawal comes from an exact offline verifier receipt over an ephemeral copy; the sealed source digest remains unchanged."
        eyebrow={`Break this claim · ${report.challenge.total_receipts} verified receipts`}
        id="challenge-title"
        title={report.challenge.claim_text}
      />
      <Tabs onValueChange={onSelectedClassChange} value={selectedClass}>
        <div className="overflow-x-auto pb-1" data-print-hidden="true">
          <TabsList aria-label="Registered challenge classes" className="min-w-max">
            {report.challenge.classes.map((challenge) => (
              <TabsTrigger key={challenge.class} value={challenge.class}>
                <span
                  className={`size-1.5 rounded-full ${challenge.receipt === null ? "bg-unavailable" : "bg-break"}`}
                />
                {humanize(challenge.class)}
              </TabsTrigger>
            ))}
          </TabsList>
        </div>
        {report.challenge.classes.map((challenge) => (
          <TabsContent key={challenge.class} value={challenge.class}>
            <AnimatePresence mode="wait">
              <motion.div
                animate={{ opacity: 1, y: 0 }}
                initial={reducedMotion ? false : { opacity: 0, y: 8 }}
                key={challenge.class}
                transition={{ duration: 0.22, ease: [0.2, 0.8, 0.2, 1] }}
              >
                {challenge.receipt === null ? (
                  <UnavailableChallenge challenge={challenge} />
                ) : (
                  <VerifiedChallenge challenge={challenge} onInspect={onInspect} />
                )}
              </motion.div>
            </AnimatePresence>
          </TabsContent>
        ))}
      </Tabs>
    </section>
  );
}

interface VerifiedChallengeProps {
  challenge: ChallengeClass;
  onInspect: (item: InspectionItem) => void;
}

function VerifiedChallenge({ challenge, onInspect }: VerifiedChallengeProps) {
  const receipt = challenge.receipt;
  if (receipt === null) {
    return null;
  }
  return (
    <Card className="overflow-hidden border-break/35">
      <div className="grid lg:grid-cols-[0.8fr_1.35fr_0.85fr]">
        <ChallengeMutation challenge={challenge} />
        <ChallengeTransition challenge={challenge} />
        <ChallengeReceiptSummary challenge={challenge} />
      </div>
      <div className="grid gap-3 border-t border-border bg-muted/45 p-4 sm:grid-cols-[1fr_auto] sm:items-center sm:px-6">
        <CopyCommand command={receipt.replay_command} label="Copy replay" />
        <Button
          onClick={() => onInspect(inspectReceipt(receipt))}
          type="button"
          variant="destructive"
        >
          <FileCheck2 aria-hidden="true" />
          Inspect receipt
        </Button>
      </div>
    </Card>
  );
}

function ChallengeMutation({ challenge }: { challenge: ChallengeClass }) {
  const receipt = challenge.receipt!;
  return (
    <div className="border-b border-border bg-break-soft p-5 lg:border-b-0 lg:border-r sm:p-6">
      <p className="flex items-center gap-2 text-[0.61rem] font-bold uppercase tracking-[0.15em] text-break-strong">
        <ShieldAlert aria-hidden="true" className="size-4" />
        Ephemeral mutation
      </p>
      <p className="mt-5 text-lg font-semibold leading-7 tracking-[-0.025em] text-foreground">
        {receipt.mutation}
      </p>
      <p className="mt-5 font-mono text-[0.61rem] leading-5 text-break-strong">
        {receipt.challenge_id}
      </p>
      <div className="mt-5 flex items-center justify-between gap-3">
        <StatusBadge compact status={challenge.availability} />
        <span className="text-[0.58rem] font-bold uppercase tracking-[0.11em] text-muted-foreground">
          {challenge.global_receipt_count} class receipts
        </span>
      </div>
    </div>
  );
}

function ChallengeTransition({ challenge }: { challenge: ChallengeClass }) {
  const receipt = challenge.receipt!;
  return (
    <div className="p-5 sm:p-7">
      <p className="text-[0.61rem] font-bold uppercase tracking-[0.15em] text-muted-foreground">
        Observed state transition
      </p>
      <div className="mt-5 grid items-center gap-3 sm:grid-cols-[1fr_auto_1fr]">
        <StateCard label="Before" state={receipt.before_state} tone="verified" />
        <ArrowRight
          aria-hidden="true"
          className="mx-auto size-5 rotate-90 text-break sm:rotate-0"
        />
        <StateCard label="After" state={receipt.after_state} tone="rejected" />
      </div>
      <dl className="mt-5 grid gap-2 sm:grid-cols-2">
        <GuardFact label="Expected guard" value={receipt.expected_guard} />
        <GuardFact label="Observed guard" value={receipt.observed_guard} />
      </dl>
    </div>
  );
}

function ChallengeReceiptSummary({ challenge }: { challenge: ChallengeClass }) {
  const receipt = challenge.receipt!;
  const unchanged = receipt.sealed_source_digest === receipt.after_sealed_source_digest;
  return (
    <div className="border-t border-border bg-graphite p-5 text-white lg:border-l lg:border-t-0 sm:p-6">
      <p className="flex items-center gap-2 text-[0.61rem] font-bold uppercase tracking-[0.15em] text-white/55">
        <LockKeyhole aria-hidden="true" className="size-4" />
        Source integrity
      </p>
      <div className="mt-5 flex items-center gap-3">
        <span className="grid size-10 place-items-center rounded-full bg-verified text-white">
          {unchanged ? (
            <FileCheck2 aria-hidden="true" className="size-5" />
          ) : (
            <CircleSlash2 aria-hidden="true" className="size-5" />
          )}
        </span>
        <div>
          <p className="text-sm font-semibold">
            {unchanged ? "Sealed source unchanged" : "Source mismatch"}
          </p>
          <p className="mt-1 font-mono text-[0.58rem] text-white/52">
            {shortDigest(receipt.sealed_source_digest)}
          </p>
        </div>
      </div>
      <dl className="mt-6 space-y-3 border-t border-white/12 pt-5">
        <ReceiptFact label="Verifier" value={receipt.verifier} />
        <ReceiptFact label="Receipt" value={shortDigest(receipt.digest)} />
        <ReceiptFact label="Result" value={receipt.passed ? "guard matched" : "guard mismatch"} />
      </dl>
    </div>
  );
}

function UnavailableChallenge({ challenge }: { challenge: ChallengeClass }) {
  return (
    <Card className="border-unavailable/35 bg-unavailable-soft p-6 sm:p-8">
      <div className="flex max-w-3xl items-start gap-4">
        <span className="grid size-11 shrink-0 place-items-center rounded-full border border-unavailable/30 bg-paper text-unavailable">
          <CircleSlash2 aria-hidden="true" className="size-5" />
        </span>
        <div>
          <StatusBadge status={challenge.availability} />
          <h3 className="mt-4 text-xl font-semibold tracking-[-0.025em] text-foreground">
            {humanize(challenge.class)}
          </h3>
          <p className="mt-2 text-sm leading-6 text-unavailable-strong">
            No receipt exists for this selected claim: {humanize(challenge.inapplicable_reason)}.
          </p>
          <p className="mt-3 text-xs text-muted-foreground">
            Global receipts in this class: {challenge.global_receipt_count}
          </p>
        </div>
      </div>
    </Card>
  );
}

function StateCard({
  label,
  state,
  tone,
}: {
  label: string;
  state: string;
  tone: "verified" | "rejected";
}) {
  return (
    <div
      className={`rounded-xl border p-4 ${tone === "verified" ? "border-verified/30 bg-verified-soft" : "border-break/35 bg-break-soft"}`}
    >
      <p className="text-[0.57rem] font-bold uppercase tracking-[0.12em] text-muted-foreground">
        {label}
      </p>
      <p
        className={`mt-2 font-mono text-xs font-semibold ${tone === "verified" ? "text-verified-strong" : "text-break-strong"}`}
      >
        {state}
      </p>
    </div>
  );
}

function GuardFact({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-border bg-muted/60 px-3 py-2.5">
      <dt className="text-[0.56rem] font-bold uppercase tracking-[0.11em] text-muted-foreground">
        {label}
      </dt>
      <dd className="mt-1 font-mono text-[0.65rem] font-semibold text-foreground">{value}</dd>
    </div>
  );
}

function ReceiptFact({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-[0.55rem] font-bold uppercase tracking-[0.11em] text-white/65">
        {label}
      </dt>
      <dd className="mt-1 break-all font-mono text-[0.62rem] leading-5 text-white/82">{value}</dd>
    </div>
  );
}
