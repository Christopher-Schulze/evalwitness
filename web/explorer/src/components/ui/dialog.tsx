import * as DialogPrimitive from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import type { ComponentProps } from "react";

import { cn } from "@/lib/utils";

export const Dialog = DialogPrimitive.Root;
export const DialogTrigger = DialogPrimitive.Trigger;
export const DialogTitle = DialogPrimitive.Title;
export const DialogDescription = DialogPrimitive.Description;

export function DialogContent({
  className,
  children,
  ...props
}: ComponentProps<typeof DialogPrimitive.Content>) {
  return (
    <DialogPrimitive.Portal>
      <div
        aria-hidden="true"
        className="pointer-events-auto fixed inset-0 z-50 bg-graphite/55 backdrop-blur-[2px]"
      />
      <DialogPrimitive.Content
        className={cn(
          "fixed inset-x-3 bottom-3 z-50 max-h-[calc(100svh-1.5rem)] overflow-y-auto rounded-[1.5rem] border border-border-strong bg-paper p-5 shadow-dialog focus:outline-none sm:left-1/2 sm:top-1/2 sm:bottom-auto sm:w-[min(46rem,calc(100vw-2rem))] sm:-translate-x-1/2 sm:-translate-y-1/2 sm:p-7",
          className,
        )}
        data-evidence-dialog=""
        {...props}
      >
        {children}
        <DialogPrimitive.Close className="absolute right-4 top-4 grid size-10 place-items-center rounded-full border border-border bg-paper text-muted-foreground transition-colors hover:border-graphite hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
          <X aria-hidden="true" className="size-4" />
          <span className="sr-only">Close inspector</span>
        </DialogPrimitive.Close>
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  );
}
