import type { ComponentProps, ReactNode } from "react";
import { cn } from "@/lib/cn";

export function Card({ className, ...p }: ComponentProps<"div">) {
  return <div className={cn("rounded-2xl border border-border bg-surface shadow-soft", className)} {...p} />;
}

export function Badge({
  tone = "neutral",
  className,
  ...p
}: ComponentProps<"span"> & { tone?: "neutral" | "accent" | "good" | "warn" | "bad" }) {
  const tones = {
    neutral: "bg-bg-soft text-fg-2 border-border",
    accent: "bg-accent-soft text-accent-ink border-accent/20",
    good: "bg-good/10 text-good border-good/20",
    warn: "bg-warn/10 text-warn border-warn/25",
    bad: "bg-bad/10 text-bad border-bad/20",
  };
  return (
    <span
      className={cn("inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[11.5px] font-medium leading-5", tones[tone], className)}
      {...p}
    />
  );
}

export function Input({ className, ...p }: ComponentProps<"input">) {
  return (
    <input
      className={cn(
        "h-10 w-full rounded-xl border border-border bg-surface px-3 text-sm text-fg placeholder:text-subtle outline-none transition focus:border-accent focus:ring-4 focus:ring-[var(--ring)]",
        className,
      )}
      {...p}
    />
  );
}

export function Select({ className, ...p }: ComponentProps<"select">) {
  return (
    <select
      className={cn(
        "h-10 w-full rounded-xl border border-border bg-surface px-3 text-sm text-fg outline-none transition focus:border-accent focus:ring-4 focus:ring-[var(--ring)]",
        className,
      )}
      {...p}
    />
  );
}

export function Textarea({ className, ...p }: ComponentProps<"textarea">) {
  return (
    <textarea
      className={cn(
        "w-full rounded-xl border border-border bg-surface px-3 py-2 text-sm text-fg placeholder:text-subtle outline-none transition focus:border-accent focus:ring-4 focus:ring-[var(--ring)]",
        className,
      )}
      {...p}
    />
  );
}

export function Label({ className, ...p }: ComponentProps<"label">) {
  return <label className={cn("mb-1.5 block text-[13px] font-medium text-fg-2", className)} {...p} />;
}

export function Field({ label, hint, children }: { label: string; hint?: ReactNode; children: ReactNode }) {
  return (
    <div>
      <Label>{label}</Label>
      {children}
      {hint ? <p className="mt-1.5 text-xs text-muted">{hint}</p> : null}
    </div>
  );
}

export function Kbd({ children }: { children: ReactNode }) {
  return (
    <kbd className="rounded-md border border-border bg-surface-2 px-1.5 py-0.5 font-mono text-[11px] text-muted shadow-[inset_0_-1px_0_var(--border)]">
      {children}
    </kbd>
  );
}

export function Eyebrow({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <p className={cn("font-mono text-[11.5px] font-medium uppercase tracking-[0.14em] text-accent", className)}>{children}</p>
  );
}

export function EmptyState({ title, children, action }: { title: string; children?: ReactNode; action?: ReactNode }) {
  return (
    <div className="flex flex-col items-center justify-center rounded-2xl border border-dashed border-border-strong px-6 py-14 text-center">
      <div className="mb-3 size-10 rounded-full bg-accent-soft ring-8 ring-accent-soft/40" />
      <p className="font-medium">{title}</p>
      {children ? <p className="mt-1 max-w-sm text-sm text-muted">{children}</p> : null}
      {action ? <div className="mt-5">{action}</div> : null}
    </div>
  );
}
