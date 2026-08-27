import { cva, type VariantProps } from "class-variance-authority";
import type { HTMLAttributes } from "react";

import { cn } from "@/lib/utils";

const badgeVariants = cva(
  "inline-flex min-h-6 items-center gap-1.5 rounded-full border px-2.5 py-1 text-[0.61rem] font-bold uppercase leading-none tracking-[0.12em]",
  {
    variants: {
      tone: {
        neutral: "border-border-strong bg-paper text-muted-foreground",
        verified: "border-verified/30 bg-verified-soft text-verified-strong",
        rejected: "border-break/35 bg-break-soft text-break-strong",
        unavailable: "border-unavailable/35 bg-unavailable-soft text-unavailable-strong",
        graphite: "border-graphite bg-graphite text-white",
      },
    },
    defaultVariants: { tone: "neutral" },
  },
);

interface BadgeProps extends HTMLAttributes<HTMLSpanElement>, VariantProps<typeof badgeVariants> {}

export function Badge({ className, tone, ...props }: BadgeProps) {
  return <span className={cn(badgeVariants({ tone }), className)} {...props} />;
}
