import { Fingerprint, LockKeyhole, ShieldCheck } from "lucide-react";

import type { EvidenceReport } from "@/lib/report";
import { shortDigest } from "@/lib/utils";

export function CapsuleSeal({ report }: { report: EvidenceReport }) {
  const identities: Array<readonly [string, string]> = [
    ["Capsule", report.capsule.capsule_id],
    ["Manifest", report.capsule.manifest_digest],
    ["Ledger", report.capsule.ledger_digest],
    ["Autopsy", report.capsule.autopsy_digest],
    ["Report", report.digest],
  ];
  return (
    <footer className="border-t border-border bg-graphite text-white">
      <div className="mx-auto grid max-w-[94rem] gap-8 px-4 py-10 sm:px-7 lg:grid-cols-[0.75fr_1.25fr] lg:px-10">
        <div>
          <span className="grid size-12 place-items-center rounded-full border border-verified/50 bg-verified text-white">
            <ShieldCheck aria-hidden="true" className="size-5" />
          </span>
          <h2 className="mt-5 text-2xl font-semibold tracking-[-0.04em]">
            Offline evidence, bound end to end.
          </h2>
          <p className="mt-2 max-w-lg text-sm leading-6 text-white/58">
            This static report performs no provider calls, network requests, analytics, telemetry,
            or runtime backend work.
          </p>
        </div>
        <dl className="grid grid-cols-2 gap-px overflow-hidden rounded-xl bg-white/10 sm:grid-cols-3">
          {identities.map(([label, value]) => (
            <div className="min-w-0 bg-graphite p-4" key={label}>
              <dt className="flex items-center gap-1.5 text-[0.56rem] font-bold uppercase tracking-[0.12em] text-white/65">
                {label === "Capsule" ? (
                  <LockKeyhole aria-hidden="true" className="size-3" />
                ) : (
                  <Fingerprint aria-hidden="true" className="size-3" />
                )}
                {label}
              </dt>
              <dd
                className="mt-2 truncate font-mono text-[0.65rem] font-semibold text-white/85"
                title={value}
              >
                {shortDigest(value)}
              </dd>
            </div>
          ))}
        </dl>
      </div>
    </footer>
  );
}
