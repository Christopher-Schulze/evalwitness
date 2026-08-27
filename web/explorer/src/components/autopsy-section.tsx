import { EvidenceRail } from "@/components/evidence-rail";
import { MethodLane } from "@/components/method-lane";
import { SectionHeading } from "@/components/section-heading";
import { Badge } from "@/components/ui/badge";
import type { InspectionItem } from "@/lib/inspection";
import type { EvidenceReport } from "@/lib/report";

interface AutopsySectionProps {
  report: EvidenceReport;
  onInspect: (item: InspectionItem) => void;
}

export function AutopsySection({ report, onInspect }: AutopsySectionProps) {
  return (
    <section
      aria-labelledby="method-title"
      className="border-y border-border bg-paper py-10 sm:py-14"
    >
      <div className="mx-auto max-w-[94rem] space-y-12 px-4 sm:px-7 lg:px-10">
        <div>
          <SectionHeading
            aside={
              <Badge tone="rejected">
                {report.method.transitions.length} method breaks preserved
              </Badge>
            }
            description="Each generation keeps its own counterexample, denominator, guarding test, and non-claim boundary. Supersession is evidence, not cleanup."
            eyebrow="Lane 01 · Method integrity"
            id="method-title"
            title="The method had to survive its own failures first."
          />
          <MethodLane method={report.method} onInspect={onInspect} />
        </div>
        <div>
          <SectionHeading
            aside={
              <Badge tone="verified">
                {report.transport.paths[0]?.layers.length ?? 0} transport layers
              </Badge>
            }
            description="The same claim is followed through runtime witness, native export, canonical graph, retained bundle, and verifier request. The rejected path stops exactly where failability disappears."
            eyebrow="Lane 02 · Claim transport"
            id="transport-title"
            title="One path survives. One path proves where evidence stops."
          />
          <EvidenceRail onInspect={onInspect} report={report} />
        </div>
      </div>
    </section>
  );
}
