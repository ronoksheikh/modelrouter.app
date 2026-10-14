"use client";

import { Check, Copy } from "lucide-react";
import { useState } from "react";
import { cn } from "@/lib/cn";

export function CopyButton({ value, className, label }: { value: string; className?: string; label?: string }) {
  const [done, setDone] = useState(false);
  return (
    <button
      type="button"
      aria-label={label ?? "Copy to clipboard"}
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(value);
          setDone(true);
          setTimeout(() => setDone(false), 1500);
        } catch {
          /* clipboard unavailable */
        }
      }}
      className={cn(
        "inline-flex size-8 items-center justify-center rounded-lg text-subtle transition hover:bg-white/10 hover:text-white",
        className,
      )}
    >
      {done ? <Check className="size-4" /> : <Copy className="size-4" />}
      {label ? <span className="sr-only">{label}</span> : null}
    </button>
  );
}
