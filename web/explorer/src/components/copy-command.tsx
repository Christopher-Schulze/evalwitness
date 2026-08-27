import { Check, Copy } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";

interface CopyCommandProps {
  command: string;
  label?: string;
}

type CopyState = "idle" | "copied" | "failed";

export function CopyCommand({ command, label = "Copy command" }: CopyCommandProps) {
  const [state, setState] = useState<CopyState>("idle");

  async function copy(): Promise<void> {
    try {
      await navigator.clipboard.writeText(command);
      setState("copied");
    } catch {
      setState("failed");
    }
  }

  return (
    <div className="rounded-xl border border-graphite bg-graphite p-3 text-white shadow-control">
      <code className="block select-all overflow-x-auto whitespace-nowrap font-mono text-[0.7rem] leading-6 text-white/88">
        {command}
      </code>
      <div className="mt-2 flex items-center justify-between gap-3 border-t border-white/12 pt-2">
        <span aria-live="polite" className="text-[0.62rem] font-medium text-white/62">
          {state === "failed"
            ? "Clipboard blocked. Select the command manually."
            : state === "copied"
              ? "Copied."
              : "Offline · zero provider calls"}
        </span>
        <Button onClick={copy} size="compact" type="button" variant="inverse">
          {state === "copied" ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}
          {state === "copied" ? "Copied" : label}
        </Button>
      </div>
    </div>
  );
}
