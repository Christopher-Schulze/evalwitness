import { Fingerprint, Link2, ShieldCheck } from "lucide-react";

import { StatusBadge } from "@/components/status-badge";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { useMediaQuery } from "@/hooks/use-media-query";
import type { InspectionItem } from "@/lib/inspection";
import { shortDigest } from "@/lib/utils";

interface ArtifactInspectorProps {
  item: InspectionItem | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function ArtifactInspector({ item, open, onOpenChange }: ArtifactInspectorProps) {
  const compactViewport = useMediaQuery("(max-width: 639px)");
  if (item === null) {
    return null;
  }
  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent
        aria-describedby="artifact-inspector-description"
        data-viewport={compactViewport ? "compact" : "desktop"}
      >
        <div className="pr-10">
          <div className="flex flex-wrap items-center gap-2">
            <StatusBadge status={item.tone} />
            <span className="text-[0.61rem] font-bold uppercase tracking-[0.15em] text-muted-foreground">
              {item.eyebrow}
            </span>
          </div>
          <DialogTitle className="mt-4 text-3xl font-semibold tracking-[-0.045em] text-foreground">
            {item.title}
          </DialogTitle>
          <DialogDescription
            id="artifact-inspector-description"
            className="mt-3 text-sm leading-6 text-muted-foreground"
          >
            {item.summary}
          </DialogDescription>
        </div>
        <dl className="mt-7 divide-y divide-border overflow-hidden rounded-xl border border-border bg-muted/35">
          {item.fields.map((field) => (
            <div
              className="grid gap-1 px-4 py-3 sm:grid-cols-[9rem_1fr] sm:gap-4"
              key={field.label}
            >
              <dt className="text-[0.58rem] font-bold uppercase tracking-[0.12em] text-muted-foreground">
                {field.label}
              </dt>
              <dd className="break-all font-mono text-[0.68rem] leading-5 text-foreground">
                {field.value}
              </dd>
            </div>
          ))}
        </dl>
        {item.artifact === null ? null : (
          <div className="mt-5 rounded-xl border border-verified/25 bg-verified-soft p-4">
            <div className="flex items-center gap-2 text-[0.6rem] font-bold uppercase tracking-[0.14em] text-verified-strong">
              <ShieldCheck aria-hidden="true" className="size-4" />
              Bound artifact
            </div>
            <dl className="mt-4 grid gap-3 sm:grid-cols-2">
              <ArtifactFact
                icon={Fingerprint}
                label="Artifact digest"
                value={shortDigest(item.artifact.artifact_digest)}
              />
              <ArtifactFact
                icon={Link2}
                label="Payload SHA-256"
                value={shortDigest(item.artifact.payload_sha256)}
              />
            </dl>
          </div>
        )}
        <a
          className="mt-5 inline-flex items-center gap-2 font-mono text-[0.64rem] font-semibold text-verified underline decoration-verified/35 underline-offset-4 hover:decoration-verified focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          href={item.deepLink}
        >
          <Link2 aria-hidden="true" className="size-3.5" />
          {item.deepLink}
        </a>
      </DialogContent>
    </Dialog>
  );
}

interface ArtifactFactProps {
  icon: typeof Fingerprint;
  label: string;
  value: string;
}

function ArtifactFact({ icon: Icon, label, value }: ArtifactFactProps) {
  return (
    <div>
      <dt className="flex items-center gap-1.5 text-[0.56rem] font-bold uppercase tracking-[0.11em] text-muted-foreground">
        <Icon aria-hidden="true" className="size-3" />
        {label}
      </dt>
      <dd className="mt-1.5 font-mono text-xs font-semibold text-foreground">{value}</dd>
    </div>
  );
}
