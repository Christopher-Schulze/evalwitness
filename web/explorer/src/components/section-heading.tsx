import type { ReactNode } from "react";

interface SectionHeadingProps {
  id: string;
  eyebrow: string;
  title: string;
  description: string;
  aside?: ReactNode;
}

export function SectionHeading({ id, eyebrow, title, description, aside }: SectionHeadingProps) {
  return (
    <header className="mb-5 flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
      <div className="max-w-3xl">
        <p className="text-[0.64rem] font-bold uppercase tracking-[0.18em] text-muted-foreground">
          {eyebrow}
        </p>
        <h2
          className="mt-2 text-balance text-2xl font-semibold tracking-[-0.035em] text-foreground sm:text-3xl"
          id={id}
        >
          {title}
        </h2>
        <p className="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground">{description}</p>
      </div>
      {aside}
    </header>
  );
}
