import type {
  ArtifactRef,
  ChallengeReceipt,
  MethodGeneration,
  TransportLayer,
  TransportPath,
} from "@/lib/report";
import { humanize } from "@/lib/utils";

export type InspectionTone = "neutral" | "verified" | "rejected" | "unavailable";

export interface InspectionField {
  label: string;
  value: string;
}

export interface InspectionItem {
  id: string;
  eyebrow: string;
  title: string;
  summary: string;
  tone: InspectionTone;
  deepLink: string;
  fields: InspectionField[];
  artifact: ArtifactRef | null;
}

export function inspectArtifact(
  artifact: ArtifactRef,
  title: string,
  deepLink = `#artifact/${artifact.id}`,
): InspectionItem {
  return {
    id: `artifact:${artifact.id}`,
    eyebrow: humanize(artifact.kind),
    title,
    summary: artifact.schema_version,
    tone: "neutral",
    deepLink,
    fields: [
      { label: "Artifact ID", value: artifact.id },
      { label: "Payload SHA-256", value: artifact.payload_sha256 },
      { label: "Artifact digest", value: artifact.artifact_digest },
    ],
    artifact,
  };
}

export function inspectGeneration(generation: MethodGeneration): InspectionItem {
  return {
    id: `method:${generation.generation}`,
    eyebrow: `Method generation ${generation.generation}`,
    title: humanize(generation.state),
    summary: generation.observations.join(" "),
    tone:
      generation.state === "falsified" || generation.state === "superseded"
        ? "rejected"
        : "verified",
    deepLink: generation.deep_link,
    fields: [
      ...generation.frozen_denominators.map((count) => ({
        label: humanize(count.name),
        value: String(count.value),
      })),
      { label: "Guarding tests", value: generation.guarding_tests.join(" · ") },
      { label: "Non-claims", value: generation.non_claims.join(" · ") },
    ],
    artifact: generation.evidence[0] ?? null,
  };
}

export function inspectLayer(path: TransportPath, layer: TransportLayer): InspectionItem {
  const unavailable = layer.detail_availability !== "available";
  return {
    id: `layer:${path.path_id}:${layer.layer}`,
    eyebrow: path.label,
    title: humanize(layer.layer),
    summary: humanize(layer.status),
    tone:
      layer.status === "first_loss" || layer.status === "not_reached"
        ? "rejected"
        : unavailable
          ? "unavailable"
          : "verified",
    deepLink: layer.deep_link,
    fields: [
      { label: "Terminal path", value: humanize(path.terminal_state) },
      {
        label: "Object digest",
        value: layer.object_digest ?? humanize(layer.object_digest_availability),
      },
      {
        label: "Required fields",
        value: layer.required_fields?.join(" · ") ?? humanize(layer.detail_availability),
      },
      {
        label: "Decisive channels",
        value: layer.decisive_channels?.join(" · ") ?? humanize(layer.detail_availability),
      },
    ],
    artifact: path.evidence,
  };
}

export function inspectReceipt(receipt: ChallengeReceipt): InspectionItem {
  return {
    id: `challenge:${receipt.challenge_id}`,
    eyebrow: `Verified challenge receipt · ${receipt.claim_id}`,
    title: humanize(receipt.class),
    summary: receipt.mutation,
    tone: "rejected",
    deepLink: `#challenge/${receipt.claim_id}/${receipt.challenge_id}`,
    fields: [
      { label: "Before", value: receipt.before_state },
      { label: "After", value: receipt.after_state },
      { label: "Expected guard", value: receipt.expected_guard },
      { label: "Observed guard", value: receipt.observed_guard },
      { label: "Sealed source", value: receipt.sealed_source_digest },
      { label: "Receipt digest", value: receipt.digest },
      { label: "Replay", value: receipt.replay_command },
    ],
    artifact: null,
  };
}
