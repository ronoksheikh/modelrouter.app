import { ArrowLeft, ArrowRight } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";
import { docsFlat } from "@/lib/docs";

export function DocPage({ path, title, lead, children }: { path: string; title: string; lead?: string; children: ReactNode }) {
  const i = docsFlat.findIndex((d) => d.href === path);
  const prev = i > 0 ? docsFlat[i - 1] : null;
  const next = i >= 0 && i < docsFlat.length - 1 ? docsFlat[i + 1] : null;
  return (
    <article className="min-w-0">
      <h1 className="text-[34px] font-semibold leading-tight tracking-[-0.03em]">{title}</h1>
      {lead ? <p className="mt-3 text-lg leading-8 text-muted">{lead}</p> : null}
      <div className="prose-docs mt-6">{children}</div>
      <nav className="mt-16 grid gap-3 border-t border-border pt-8 sm:grid-cols-2" aria-label="Pagination">
        {prev ? (
          <Link href={prev.href} className="group rounded-xl border border-border p-4 transition hover:border-border-strong">
            <span className="flex items-center gap-1 text-xs text-subtle"><ArrowLeft className="size-3" /> Previous</span>
            <span className="mt-1 block font-medium group-hover:text-accent">{prev.title}</span>
          </Link>
        ) : <span />}
        {next ? (
          <Link href={next.href} className="group rounded-xl border border-border p-4 text-right transition hover:border-border-strong">
            <span className="flex items-center justify-end gap-1 text-xs text-subtle">Next <ArrowRight className="size-3" /></span>
            <span className="mt-1 block font-medium group-hover:text-accent">{next.title}</span>
          </Link>
        ) : null}
      </nav>
    </article>
  );
}

export function Callout({ children, tone = "info" }: { children: ReactNode; tone?: "info" | "warn" }) {
  return (
    <div
      className={
        "my-5 rounded-xl border px-4 py-3 text-[14.5px] leading-7 " +
        (tone === "warn" ? "border-warn/30 bg-warn/5 text-fg-2" : "border-accent/20 bg-accent-soft/60 text-fg-2")
      }
    >
      {children}
    </div>
  );
}
