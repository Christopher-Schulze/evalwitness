import {
  Database,
  Files,
  FlaskConical,
  ScanEye,
  TableProperties,
  TriangleAlert,
} from "lucide-react";

import { DatasetPanel } from "@/components/dataset-panel";
import { ExtensionsPanel } from "@/components/extensions-panel";
import { LimitationsPanel } from "@/components/limitations-panel";
import { OwnerInspectionPanel } from "@/components/owner-inspection-panel";
import { ReleasePanel } from "@/components/release-panel";
import { SectionHeading } from "@/components/section-heading";
import { TableFallback } from "@/components/table-fallback";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type { InspectionItem } from "@/lib/inspection";
import type { EvidenceReport } from "@/lib/report";

interface SupportingEvidenceProps {
  report: EvidenceReport;
  onInspect: (item: InspectionItem) => void;
  activeTab: string;
  onTabChange: (value: string) => void;
}

export function SupportingEvidence({
  report,
  onInspect,
  activeTab,
  onTabChange,
}: SupportingEvidenceProps) {
  return (
    <section
      aria-labelledby="evidence-title"
      className="border-t border-border bg-paper py-12 sm:py-16"
    >
      <div className="mx-auto max-w-[94rem] px-4 sm:px-7 lg:px-10">
        <SectionHeading
          description="The explorer keeps denominators, limitations, file inventory, extension gaps, and non-visual fallbacks beside the headline result."
          eyebrow="Evidence register"
          id="evidence-title"
          title="Nothing inconvenient is filtered out."
        />
        <Tabs onValueChange={onTabChange} value={activeTab}>
          <div className="overflow-x-auto pb-1" data-print-hidden="true">
            <TabsList className="min-w-max">
              <EvidenceTab icon={Database} label="Dataset" value="dataset" />
              <EvidenceTab icon={ScanEye} label="Owner inspection" value="owner-inspection" />
              <EvidenceTab icon={TriangleAlert} label="Limitations" value="limitations" />
              <EvidenceTab icon={Files} label="Release manifest" value="release" />
              <EvidenceTab icon={FlaskConical} label="Extensions" value="extensions" />
              <EvidenceTab icon={TableProperties} label="Tables" value="tables" />
            </TabsList>
          </div>
          <TabsContent value="dataset">
            <DatasetPanel dataset={report.dataset} onInspect={onInspect} />
          </TabsContent>
          <TabsContent value="owner-inspection">
            <div id="owner-inspection">
              <OwnerInspectionPanel inspection={report.owner_inspection} onInspect={onInspect} />
            </div>
          </TabsContent>
          <TabsContent value="limitations">
            <LimitationsPanel limitations={report.limitations} onInspect={onInspect} />
          </TabsContent>
          <TabsContent value="release">
            <ReleasePanel onInspect={onInspect} release={report.release} />
          </TabsContent>
          <TabsContent value="extensions">
            <ExtensionsPanel extensions={report.extensions} />
          </TabsContent>
          <TabsContent value="tables">
            <TableFallback report={report} />
          </TabsContent>
        </Tabs>
      </div>
    </section>
  );
}

interface EvidenceTabProps {
  icon: typeof Database;
  label: string;
  value: string;
}

function EvidenceTab({ icon: Icon, label, value }: EvidenceTabProps) {
  return (
    <TabsTrigger value={value}>
      <Icon aria-hidden="true" className="size-3.5" />
      {label}
    </TabsTrigger>
  );
}
