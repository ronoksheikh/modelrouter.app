import type { ReactNode } from "react";

export function LegalPage({ title, updated, children }: { title: string; updated: string; children: ReactNode }) {
  return (
    <div className="container-page py-14 sm:py-20">
      <article className="prose-docs mx-auto max-w-2xl">
        <h1 className="text-4xl font-semibold tracking-[-0.03em]">{title}</h1>
        <p className="!mt-3 text-sm !text-subtle">Last updated {updated}</p>
        {children}
      </article>
    </div>
  );
}
