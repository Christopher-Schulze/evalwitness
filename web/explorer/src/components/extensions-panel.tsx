import { CircleDashed, PlugZap } from "lucide-react";

import { StatusBadge } from "@/components/status-badge";
import type { EvidenceReport } from "@/lib/report";
import { humanize } from "@/lib/utils";

export function ExtensionsPanel({ extensions }: { extensions: EvidenceReport["extensions"] }) {
  return (
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
      {extensions.map((extension) => {
        const available = extension.availability === "available";
        return (
          <article
            className={`rounded-[1.1rem] border p-5 ${available ? "border-verified/25 bg-verified-soft" : "border-unavailable/25 bg-unavailable-soft"}`}
            key={extension.extension_id}
          >
            <div className="flex items-start justify-between gap-3">
              <span
                className={`grid size-10 place-items-center rounded-full border bg-paper ${available ? "border-verified/25 text-verified" : "border-unavailable/25 text-unavailable"}`}
              >
                {extension.components.length === 0 ? (
                  <CircleDashed aria-hidden="true" className="size-5" />
                ) : (
                  <PlugZap aria-hidden="true" className="size-5" />
                )}
              </span>
              <StatusBadge compact status={extension.availability} />
            </div>
            <p className="mt-5 text-[0.58rem] font-bold uppercase tracking-[0.13em] text-muted-foreground">
              {extension.owner_task}
            </p>
            <h3 className="mt-1 text-base font-semibold tracking-[-0.02em] text-foreground">
              {humanize(extension.extension_id)}
            </h3>
            <dl
              className={`mt-5 space-y-3 border-t pt-4 ${available ? "border-verified/20" : "border-unavailable/20"}`}
            >
              <div>
                <dt
                  className={`text-[0.55rem] font-bold uppercase tracking-[0.1em] ${available ? "text-verified-strong" : "text-unavailable-strong"}`}
                >
                  Required component
                </dt>
                <dd className="mt-1 break-all font-mono text-[0.62rem] leading-5 text-foreground">
                  {extension.required_types.join(" · ")}
                </dd>
              </div>
              <div>
                <dt
                  className={`text-[0.55rem] font-bold uppercase tracking-[0.1em] ${available ? "text-verified-strong" : "text-unavailable-strong"}`}
                >
                  Missing
                </dt>
                <dd className="mt-1 break-all font-mono text-[0.62rem] leading-5 text-foreground">
                  {extension.missing_types.join(" · ") || "none"}
                </dd>
              </div>
            </dl>
          </article>
        );
      })}
    </div>
  );
}
