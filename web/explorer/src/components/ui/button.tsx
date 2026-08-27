import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import type { ButtonHTMLAttributes } from "react";

import { cn } from "@/lib/utils";

export const buttonVariants = cva(
  "inline-flex min-h-10 items-center justify-center gap-2 rounded-full border text-[0.69rem] font-bold uppercase tracking-[0.14em] transition-colors duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:pointer-events-none disabled:opacity-45 [&_svg]:size-4 [&_svg]:shrink-0",
  {
    variants: {
      variant: {
        primary: "border-graphite bg-graphite px-5 text-white hover:bg-foreground",
        verified: "border-verified bg-verified px-5 text-white hover:bg-verified-strong",
        destructive: "border-break bg-break px-5 text-white hover:bg-break-strong",
        outline:
          "border-border-strong bg-paper px-5 text-foreground hover:border-graphite hover:bg-muted",
        ghost:
          "border-transparent bg-transparent px-3 text-muted-foreground hover:bg-muted hover:text-foreground",
        evidenceRow:
          "h-auto min-h-16 w-full justify-start rounded-none border-x-0 border-t-0 border-b border-border bg-paper px-4 py-3 text-left normal-case tracking-normal hover:bg-muted focus-visible:z-10 data-[selected=true]:bg-verified-soft data-[selected=true]:shadow-[inset_3px_0_0_var(--verified)]",
        inverse:
          "border-white/15 bg-white/5 px-3 text-white hover:border-white/30 hover:bg-white/10 hover:text-white",
        inverseActive: "border-white bg-white px-3 text-graphite hover:bg-white/90",
      },
      size: {
        default: "h-10",
        compact: "h-8 min-h-8 px-3 text-[0.62rem]",
        icon: "size-10 min-h-10 p-0",
      },
    },
    defaultVariants: { variant: "outline", size: "default" },
  },
);

interface ButtonProps
  extends ButtonHTMLAttributes<HTMLButtonElement>, VariantProps<typeof buttonVariants> {
  asChild?: boolean;
}

export function Button({ asChild = false, className, variant, size, ...props }: ButtonProps) {
  const Component = asChild ? Slot : "button";
  return <Component className={cn(buttonVariants({ variant, size }), className)} {...props} />;
}
