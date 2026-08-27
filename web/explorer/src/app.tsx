import { useEffect, useState } from "react";

import { ArtifactInspector } from "@/components/artifact-inspector";
import { AutopsySection } from "@/components/autopsy-section";
import { CapsuleSeal } from "@/components/capsule-seal";
import { ChallengeLab } from "@/components/challenge-lab";
import { IdenticalResponseSection } from "@/components/identical-response-section";
import { RelianceSection } from "@/components/reliance-section";
import { ProfileSection } from "@/components/profile-section";
import { ScopeOverview } from "@/components/scope-overview";
import { StressLabSection } from "@/components/stress-lab-section";
import { SupportingEvidence } from "@/components/supporting-evidence";
import { TopBar } from "@/components/top-bar";
import { TooltipProvider } from "@/components/ui/tooltip";
import { defaultChallengeClass, resolveDeepLink } from "@/lib/deep-links";
import type { InspectionItem } from "@/lib/inspection";
import type { EvidenceReport } from "@/lib/report";
import { defaultStressStepIndex, type StressOrderView } from "@/lib/stress-presentation";

export function App({ report }: { report: EvidenceReport }) {
  const [inspection, setInspection] = useState<InspectionItem | null>(null);
  const [inspectorOpen, setInspectorOpen] = useState(false);
  const [evidenceTab, setEvidenceTab] = useState("dataset");
  const [challengeClass, setChallengeClass] = useState(() => defaultChallengeClass(report));
  const [stressOrder, setStressOrder] = useState<StressOrderView>("transformed");
  const [stressStepIndex, setStressStepIndex] = useState(() =>
    defaultStressStepIndex(report.stress),
  );

  useEffect(() => {
    function applyDeepLink(): void {
      const target = resolveDeepLink(report, window.location.hash);
      if (target.kind === "invalid") {
        window.history.replaceState(null, "", "#autopsy");
        setInspectorOpen(false);
        return;
      }
      if (target.kind === "inspection") {
        setInspection(target.item);
        setInspectorOpen(true);
        if (target.challengeClass !== null) {
          setChallengeClass(target.challengeClass);
          scrollTo("challenge");
        }
        return;
      }
      setInspectorOpen(false);
      if (target.kind === "evidence") {
        setEvidenceTab(target.tab);
        scrollTo(target.tab === "tables" ? "tables" : "owner-inspection");
      } else if (target.kind === "stress") {
        scrollTo("stress");
      } else if (target.kind === "reliance") {
        scrollTo("reliance");
      } else if (target.kind === "identical-response") {
        scrollTo("identical-response");
      } else if (target.kind === "challenge") {
        scrollTo("challenge");
      }
    }

    applyDeepLink();
    window.addEventListener("hashchange", applyDeepLink);
    return () => window.removeEventListener("hashchange", applyDeepLink);
  }, [report]);

  function inspect(item: InspectionItem): void {
    setInspection(item);
    setInspectorOpen(true);
    window.history.replaceState(null, "", item.deepLink);
  }

  function openTables(): void {
    setEvidenceTab("tables");
    window.requestAnimationFrame(() => document.getElementById("tables")?.scrollIntoView());
  }

  return (
    <TooltipProvider delayDuration={240}>
      <div className="min-h-screen">
        <TopBar
          onOpenTables={openTables}
          identicalResponseAvailable={report.identical_response !== undefined}
          relianceAvailable={report.reliance !== undefined}
          reportDigest={report.digest}
        />
        <main>
          <ScopeOverview report={report} />
          <AutopsySection onInspect={inspect} report={report} />
          <StressLabSection
            onInspect={inspect}
            onOrderChange={setStressOrder}
            onStepChange={setStressStepIndex}
            orderView={stressOrder}
            report={report}
            selectedStepIndex={stressStepIndex}
          />
          {report.reliance === undefined ? null : (
            <RelianceSection onInspect={inspect} view={report.reliance} />
          )}
          {report.profile === undefined ? null : <ProfileSection view={report.profile} />}
          {report.identical_response === undefined ? null : (
            <IdenticalResponseSection onInspect={inspect} view={report.identical_response} />
          )}
          <ChallengeLab
            onInspect={inspect}
            onSelectedClassChange={setChallengeClass}
            report={report}
            selectedClass={challengeClass}
          />
          <SupportingEvidence
            activeTab={evidenceTab}
            onInspect={inspect}
            onTabChange={setEvidenceTab}
            report={report}
          />
        </main>
        <CapsuleSeal report={report} />
        <ArtifactInspector item={inspection} onOpenChange={setInspectorOpen} open={inspectorOpen} />
      </div>
    </TooltipProvider>
  );
}

function scrollTo(id: string): void {
  window.requestAnimationFrame(() => document.getElementById(id)?.scrollIntoView());
}
