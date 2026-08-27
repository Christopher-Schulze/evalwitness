import { ShieldX } from "lucide-react";

export function FatalReport({ error }: { error: unknown }) {
  const detail = error instanceof Error ? error.message : "Unknown report error";
  return (
    <main className="grid min-h-screen place-items-center bg-background p-6 text-foreground">
      <section className="w-full max-w-xl rounded-[1.4rem] border border-break/35 bg-paper p-7 shadow-dialog">
        <span className="grid size-12 place-items-center rounded-full bg-break-soft text-break">
          <ShieldX aria-hidden="true" className="size-6" />
        </span>
        <p className="mt-6 text-[0.62rem] font-bold uppercase tracking-[0.16em] text-break-strong">
          Fail closed
        </p>
        <h1 className="mt-2 text-3xl font-semibold tracking-[-0.045em]">
          Report verification failed.
        </h1>
        <p className="mt-3 text-sm leading-6 text-muted-foreground">
          The explorer will not render scientific content whose embedded bytes or schema do not
          match the verified report contract.
        </p>
        <pre className="mt-5 overflow-x-auto rounded-xl border border-border bg-muted p-4 font-mono text-xs leading-5 text-foreground">
          {detail}
        </pre>
      </section>
    </main>
  );
}
