import { SectionHeading } from "@/components/section-heading";
import { StatusBadge } from "@/components/status-badge";
import { parseProfileDimensions } from "@/lib/profile-presentation";
import type { ProfileExplorerView } from "@/lib/report";
import { shortDigest } from "@/lib/utils";

interface ProfileSectionProps {
  view: ProfileExplorerView;
}

export function ProfileSection({ view }: ProfileSectionProps) {
  const dimensions = parseProfileDimensions(view.report.dimensions_json);
  return (
    <section
      aria-labelledby="profile-title"
      className="scroll-mt-4 bg-paper py-12 sm:py-16"
      id="profile"
    >
      <div className="mx-auto max-w-[94rem] px-4 sm:px-7 lg:px-10">
        <SectionHeading
          description="Multidimensional reliability profile with explicit evidence levels, capsule expressions, and no global score."
          eyebrow="Task 058"
          id="profile-title"
          title="Reliability profile"
        />
        <dl className="mt-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <ProfileFact label="Identity" value={view.report.identity} />
          <ProfileFact label="Dimensions" value={String(view.report.dimensions)} />
          <ProfileFact label="Summary" value={view.report.summary} />
          <ProfileFact label="Digest" mono value={shortDigest(view.report.digest)} />
        </dl>
        <div className="mt-8 overflow-hidden rounded-[1.35rem] border border-border-strong bg-border">
          <table className="w-full border-collapse bg-card text-left text-sm">
            <thead>
              <tr className="text-xs uppercase tracking-wide text-muted-fg">
                <th className="px-5 py-3 font-medium">Dimension</th>
                <th className="px-5 py-3 font-medium">Status</th>
                <th className="px-5 py-3 font-medium">Metric</th>
                <th className="px-5 py-3 font-medium">Scope</th>
              </tr>
            </thead>
            <tbody>
              {dimensions.map((dimension) => (
                <tr className="border-t border-border-strong" key={dimension.id}>
                  <td className="px-5 py-3 font-mono text-xs">{dimension.id}</td>
                  <td className="px-5 py-3">
                    <StatusBadge status={dimension.status} />
                  </td>
                  <td className="px-5 py-3">{dimension.metric ?? "—"}</td>
                  <td className="px-5 py-3">{dimension.scope}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
}

interface ProfileFactProps {
  label: string;
  mono?: boolean;
  value: string;
}

function ProfileFact({ label, mono = false, value }: ProfileFactProps) {
  return (
    <div className="rounded-[1.35rem] border border-border-strong bg-card px-5 py-4">
      <dt className="text-xs uppercase tracking-wide text-muted-fg">{label}</dt>
      <dd className={mono ? "mt-1 font-mono text-sm" : "mt-1 text-sm"}>{value}</dd>
    </div>
  );
}
