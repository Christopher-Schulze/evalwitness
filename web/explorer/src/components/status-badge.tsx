import { AlertTriangle, Check, CircleSlash2, FlaskConical } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import type { Availability } from "@/lib/report";
import { humanize } from "@/lib/utils";

interface StatusBadgeProps {
  status: Availability | string;
  compact?: boolean;
}

export function StatusBadge({ status, compact = false }: StatusBadgeProps) {
  const presentation = statusPresentation(status);
  const Icon = presentation.icon;
  return (
    <Badge className={compact ? "min-h-5 px-2 py-0.5" : undefined} tone={presentation.tone}>
      <Icon aria-hidden="true" className="size-3" />
      {humanize(status)}
    </Badge>
  );
}

function statusPresentation(status: string) {
  if (
    [
      "available",
      "accepted",
      "current",
      "survived",
      "admitted_development",
      "supported",
      "passed",
      "measured",
    ].includes(status)
  ) {
    return { tone: "verified" as const, icon: Check };
  }
  if (
    [
      "falsified",
      "superseded",
      "first_loss",
      "not_reached",
      "unsupported",
      "rejected",
      "violated",
      "revision_required",
    ].includes(status)
  ) {
    return { tone: "rejected" as const, icon: CircleSlash2 };
  }
  if (status === "not_applicable" || status === "out_of_scope") {
    return { tone: "neutral" as const, icon: FlaskConical };
  }
  if (
    [
      "not_measured",
      "not_authorized",
      "not_run",
      "withheld_private_parent",
      "not_publicly_reproducible",
      "inconclusive",
      "analysis_inconclusive",
    ].includes(status)
  ) {
    return { tone: "unavailable" as const, icon: AlertTriangle };
  }
  return { tone: "neutral" as const, icon: FlaskConical };
}
