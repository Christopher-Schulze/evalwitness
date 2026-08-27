import type { ReactNode } from "react";

import { StatusBadge } from "@/components/status-badge";
import { Table, TableCell, TableFrame, TableHead } from "@/components/ui/table";
import { parseProfileDimensions } from "@/lib/profile-presentation";
import type { EvidenceReport, RelianceOutcome } from "@/lib/report";
import { humanize, shortDigest } from "@/lib/utils";

export function TableFallback({ report }: { report: EvidenceReport }) {
  return (
    <div className="space-y-7" id="tables">
      <FallbackSection
        description="Every method state, denominator, parent, guard, and non-claim remains available without the visual lane."
        title="Method integrity table"
      >
        <MethodTable report={report} />
      </FallbackSection>
      <FallbackSection
        description="Both the successful path and earliest-loss path remain visible by default."
        title="Claim transport table"
      >
        <TransportTable report={report} />
      </FallbackSection>
      <FallbackSection
        description="Registered classes without an applicable receipt remain explicit rather than disappearing."
        title="Challenge receipt table"
      >
        <ChallengeTable report={report} />
      </FallbackSection>
      <FallbackSection
        description="All 53 replayed removal attempts remain visible, including the rejected attempts that protect the final witness."
        title="Stress reduction table"
      >
        <StressTable report={report} />
      </FallbackSection>
      {report.reliance === undefined ? null : (
        <FallbackSection
          description="Every registered factor-outcome dimension remains visible with its exact denominator, interval policy, route, data role, capsule expression, and unsupported state."
          title="Evidence reliance table"
        >
          <RelianceTable outcomes={report.reliance.outcomes} />
        </FallbackSection>
      )}
      {report.profile === undefined ? null : (
        <FallbackSection
          description="Every reliability dimension keeps its status, metric, scope, evidence level, capsule expression, and caveats without a global score."
          title="Reliability profile table"
        >
          <ProfileTable view={report.profile} />
        </FallbackSection>
      )}
    </div>
  );
}

function ProfileTable({ view }: { view: NonNullable<EvidenceReport["profile"]> }) {
  const dimensions = parseProfileDimensions(view.report.dimensions_json);
  return (
    <TableFrame>
      <Table>
        <thead>
          <tr>
            <TableHead>Dimension</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Metric</TableHead>
            <TableHead>Scope</TableHead>
          </tr>
        </thead>
        <tbody>
          {dimensions.map((dimension) => (
            <tr key={dimension.id}>
              <TableCell>
                <span className="font-mono text-xs">{dimension.id}</span>
              </TableCell>
              <TableCell>
                <StatusBadge status={dimension.status} />
              </TableCell>
              <TableCell>{dimension.metric ?? "—"}</TableCell>
              <TableCell>{dimension.scope}</TableCell>
            </tr>
          ))}
        </tbody>
      </Table>
    </TableFrame>
  );
}

function RelianceTable({ outcomes }: { outcomes: RelianceOutcome[] }) {
  return (
    <TableFrame>
      <Table>
        <thead>
          <tr>
            <TableHead>Term</TableHead>
            <TableHead>Outcome</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Estimate / interval</TableHead>
            <TableHead>Numerator / denominator / invalid</TableHead>
            <TableHead>Unit / policy</TableHead>
            <TableHead>Scope</TableHead>
            <TableHead>Capsule expression</TableHead>
          </tr>
        </thead>
        <tbody>
          {outcomes.map((outcome) => (
            <tr key={outcome.dimension_id}>
              <TableCell className="min-w-56 text-xs font-semibold">
                {outcome.factors.map(humanize).join(" × ")}
                <span className="mt-1 block font-mono text-[0.57rem] text-muted-foreground">
                  {humanize(outcome.term_kind)}
                </span>
              </TableCell>
              <TableCell className="min-w-48 font-mono text-[0.63rem]">
                {outcome.outcome_id}
              </TableCell>
              <TableCell>
                <StatusBadge compact status={outcome.status} />
              </TableCell>
              <TableCell className="min-w-64 font-mono text-[0.63rem] leading-5">
                {outcome.estimate === undefined || outcome.estimate === null
                  ? outcome.reason
                  : `${outcome.estimate.estimate} [${outcome.estimate.lower}, ${outcome.estimate.upper}] · adjusted p=${outcome.estimate.adjusted_p_value}`}
              </TableCell>
              <TableCell className="whitespace-nowrap font-mono text-[0.63rem]">
                {outcome.numerator} / {outcome.denominator} / {outcome.invalid_cells}
              </TableCell>
              <TableCell className="min-w-80 font-mono text-[0.6rem] leading-5">
                {outcome.unit} · {outcome.interval} · {outcome.policy}
              </TableCell>
              <TableCell className="min-w-72 font-mono text-[0.6rem] leading-5">
                {outcome.route} · {outcome.domain} · {outcome.data_role} · {outcome.evidence_level}{" "}
                · {shortDigest(outcome.capsule_id)}
              </TableCell>
              <TableCell className="min-w-48 font-mono text-[0.6rem] leading-5">
                {outcome.capsule_expression}
              </TableCell>
            </tr>
          ))}
        </tbody>
      </Table>
    </TableFrame>
  );
}

function StressTable({ report }: { report: EvidenceReport }) {
  return (
    <TableFrame>
      <Table>
        <thead>
          <tr>
            <TableHead>Step</TableHead>
            <TableHead>Unit</TableHead>
            <TableHead>Decision</TableHead>
            <TableHead>Relation</TableHead>
            <TableHead>Privacy</TableHead>
            <TableHead>Violation</TableHead>
            <TableHead>After digest</TableHead>
          </tr>
        </thead>
        <tbody>
          {report.stress.steps.map((step) => (
            <tr key={step.index}>
              <TableCell className="font-mono text-xs font-semibold">{step.index + 1}</TableCell>
              <TableCell className="min-w-52 font-mono text-[0.63rem]">{step.unit_id}</TableCell>
              <TableCell>
                <StatusBadge compact status={step.decision} />
              </TableCell>
              <TableCell className="font-mono text-[0.63rem]">
                {step.relation_revalidated ? "revalidated" : "failed"}
              </TableCell>
              <TableCell className="font-mono text-[0.63rem]">
                {step.privacy_revalidated ? "revalidated" : "failed"}
              </TableCell>
              <TableCell className="font-mono text-[0.63rem]">
                {step.violation_preserved ? "preserved" : "lost"}
              </TableCell>
              <TableCell className="whitespace-nowrap font-mono text-[0.63rem]">
                {shortDigest(step.after_digest)}
              </TableCell>
            </tr>
          ))}
        </tbody>
      </Table>
    </TableFrame>
  );
}

function MethodTable({ report }: { report: EvidenceReport }) {
  return (
    <TableFrame>
      <Table>
        <thead>
          <tr>
            <TableHead>Generation</TableHead>
            <TableHead>State</TableHead>
            <TableHead>Frozen denominators</TableHead>
            <TableHead>Evidence parents</TableHead>
            <TableHead>Guarding tests</TableHead>
            <TableHead>Non-claims</TableHead>
          </tr>
        </thead>
        <tbody>
          {report.method.generations.map((generation) => (
            <tr key={generation.generation}>
              <TableCell className="font-mono text-xs font-semibold">
                {generation.generation}
              </TableCell>
              <TableCell>
                <StatusBadge compact status={generation.state} />
              </TableCell>
              <TableCell className="min-w-64 font-mono text-[0.63rem] leading-5">
                {generation.frozen_denominators
                  .map((count) => `${count.name}=${count.value}`)
                  .join(" · ")}
              </TableCell>
              <TableCell className="min-w-52 font-mono text-[0.63rem] leading-5">
                {generation.evidence
                  .map((artifact) => shortDigest(artifact.artifact_digest))
                  .join(" · ")}
              </TableCell>
              <TableCell className="min-w-72 text-[0.68rem] leading-5 text-muted-foreground">
                {generation.guarding_tests.join(" · ")}
              </TableCell>
              <TableCell className="min-w-80 text-[0.68rem] leading-5 text-muted-foreground">
                {generation.non_claims.join(" · ")}
              </TableCell>
            </tr>
          ))}
        </tbody>
      </Table>
    </TableFrame>
  );
}

function TransportTable({ report }: { report: EvidenceReport }) {
  return (
    <TableFrame>
      <Table>
        <thead>
          <tr>
            <TableHead>Path</TableHead>
            <TableHead>Layer</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Object digest</TableHead>
            <TableHead>Decisive channels</TableHead>
            <TableHead>Terminal disposition</TableHead>
          </tr>
        </thead>
        <tbody>
          {report.transport.paths.flatMap((path) =>
            path.layers.map((layer) => (
              <tr key={`${path.path_id}:${layer.layer}`}>
                <TableCell className="min-w-48 font-mono text-[0.64rem]">{path.path_id}</TableCell>
                <TableCell className="whitespace-nowrap text-xs font-semibold">
                  {humanize(layer.layer)}
                </TableCell>
                <TableCell>
                  <StatusBadge compact status={layer.status} />
                </TableCell>
                <TableCell className="whitespace-nowrap font-mono text-[0.63rem]">
                  {layer.object_digest === null
                    ? humanize(layer.object_digest_availability)
                    : shortDigest(layer.object_digest)}
                </TableCell>
                <TableCell className="min-w-48 text-[0.68rem] text-muted-foreground">
                  {layer.decisive_channels?.join(" · ") ?? humanize(layer.detail_availability)}
                </TableCell>
                <TableCell className="min-w-52 text-[0.68rem] text-muted-foreground">
                  {humanize(path.terminal_state)}
                </TableCell>
              </tr>
            )),
          )}
        </tbody>
      </Table>
    </TableFrame>
  );
}

function ChallengeTable({ report }: { report: EvidenceReport }) {
  return (
    <TableFrame>
      <Table>
        <thead>
          <tr>
            <TableHead>Class</TableHead>
            <TableHead>Availability</TableHead>
            <TableHead>Before → after</TableHead>
            <TableHead>Expected / observed guard</TableHead>
            <TableHead>Source identity</TableHead>
            <TableHead>Replay</TableHead>
          </tr>
        </thead>
        <tbody>
          {report.challenge.classes.map((challenge) => (
            <tr key={challenge.class}>
              <TableCell className="whitespace-nowrap text-xs font-semibold">
                {humanize(challenge.class)}
              </TableCell>
              <TableCell>
                <StatusBadge compact status={challenge.availability} />
              </TableCell>
              <TableCell className="min-w-56 font-mono text-[0.63rem] leading-5">
                {challenge.receipt === null
                  ? humanize(challenge.inapplicable_reason)
                  : `${challenge.receipt.before_state} → ${challenge.receipt.after_state}`}
              </TableCell>
              <TableCell className="min-w-64 font-mono text-[0.63rem] leading-5">
                {challenge.receipt === null
                  ? "not applicable"
                  : `${challenge.receipt.expected_guard} / ${challenge.receipt.observed_guard}`}
              </TableCell>
              <TableCell className="whitespace-nowrap font-mono text-[0.63rem]">
                {challenge.receipt === null
                  ? "no receipt"
                  : shortDigest(challenge.receipt.sealed_source_digest)}
              </TableCell>
              <TableCell className="min-w-[34rem] font-mono text-[0.63rem] leading-5">
                {challenge.receipt?.replay_command ?? "not applicable"}
              </TableCell>
            </tr>
          ))}
        </tbody>
      </Table>
    </TableFrame>
  );
}

interface FallbackSectionProps {
  title: string;
  description: string;
  children: ReactNode;
}

function FallbackSection({ title, description, children }: FallbackSectionProps) {
  return (
    <section>
      <h3 className="text-lg font-semibold tracking-[-0.025em] text-foreground">{title}</h3>
      <p className="mb-3 mt-1 text-xs leading-5 text-muted-foreground">{description}</p>
      {children}
    </section>
  );
}
