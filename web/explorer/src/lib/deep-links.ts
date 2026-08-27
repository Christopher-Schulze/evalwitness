import { inspectArtifact, inspectGeneration, inspectLayer, inspectReceipt } from "@/lib/inspection";
import type { InspectionItem } from "@/lib/inspection";
import type { ArtifactRef, EvidenceReport } from "@/lib/report";
import { humanize } from "@/lib/utils";

export type DeepLinkTarget =
  | { kind: "autopsy" }
  | { kind: "challenge" }
  | { kind: "identical-response" }
  | { kind: "reliance" }
  | { kind: "stress" }
  | { kind: "evidence"; tab: "owner-inspection" | "tables" }
  | { kind: "inspection"; challengeClass: string | null; item: InspectionItem }
  | { kind: "invalid" };

export function defaultChallengeClass(report: EvidenceReport): string {
  return (
    report.challenge.classes.find(
      (challenge) => challenge.class === "denominator-deletion" && challenge.receipt !== null,
    ) ??
    report.challenge.classes.find((challenge) => challenge.receipt !== null) ??
    report.challenge.classes[0]!
  ).class;
}

export function resolveDeepLink(report: EvidenceReport, hash: string): DeepLinkTarget {
  if (hash === "" || hash === "#autopsy") {
    return { kind: "autopsy" };
  }
  if (hash === "#challenge") {
    return { kind: "challenge" };
  }
  if (hash === "#stress") {
    return { kind: "stress" };
  }
  if (hash === "#identical-response" && report.identical_response !== undefined) {
    return { kind: "identical-response" };
  }
  if (hash === "#reliance" && report.reliance !== undefined) {
    return { kind: "reliance" };
  }
  if (hash === "#owner-inspection" || hash === "#tables") {
    return { kind: "evidence", tab: hash === "#tables" ? "tables" : "owner-inspection" };
  }

  const generation = report.method.generations.find((candidate) => candidate.deep_link === hash);
  if (generation !== undefined) {
    return { kind: "inspection", challengeClass: null, item: inspectGeneration(generation) };
  }
  for (const path of report.transport.paths) {
    const layer = path.layers.find((candidate) => candidate.deep_link === hash);
    if (layer !== undefined) {
      return { kind: "inspection", challengeClass: null, item: inspectLayer(path, layer) };
    }
  }
  for (const challenge of report.challenge.classes) {
    if (
      challenge.receipt !== null &&
      `#challenge/${challenge.receipt.claim_id}/${challenge.receipt.challenge_id}` === hash
    ) {
      return {
        kind: "inspection",
        challengeClass: challenge.class,
        item: inspectReceipt(challenge.receipt),
      };
    }
  }

  const artifact = reportArtifacts(report).find(
    (candidate) => `#artifact/${candidate.id}` === hash,
  );
  if (artifact !== undefined) {
    return {
      kind: "inspection",
      challengeClass: null,
      item: inspectArtifact(artifact, humanize(artifact.schema_version)),
    };
  }
  return { kind: "invalid" };
}

function reportArtifacts(report: EvidenceReport): ArtifactRef[] {
  const artifacts: ArtifactRef[] = [
    report.capsule.ledger_source,
    report.capsule.autopsy_source,
    report.claim.source,
    report.bom.source,
    report.owner_inspection.source,
    report.loss.source,
    report.dataset.source,
    report.release.source,
    report.challenge.source,
    report.reproduction.source,
    report.stress.source,
  ];
  if (report.reliance !== undefined) {
    artifacts.push(report.reliance.source);
  }
  if (report.identical_response !== undefined) {
    artifacts.push(
      report.identical_response.source,
      report.identical_response.analysis_source,
      report.identical_response.reproduction.source,
    );
  }
  for (const generation of report.method.generations) {
    artifacts.push(...generation.evidence);
  }
  for (const transition of report.method.transitions) {
    artifacts.push(...transition.evidence);
  }
  for (const path of report.transport.paths) {
    artifacts.push(path.evidence);
  }
  for (const limitation of report.limitations) {
    artifacts.push(limitation.source);
    if (limitation.resolved_by !== null) {
      artifacts.push(limitation.resolved_by);
    }
  }
  return artifacts;
}
