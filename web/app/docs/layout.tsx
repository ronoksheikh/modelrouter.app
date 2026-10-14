import type { Metadata } from "next";
import { DocsSidebar } from "@/components/docs/sidebar";
import { SiteFooter } from "@/components/site/footer";
import { SiteHeader } from "@/components/site/header";

export const metadata: Metadata = {
  title: { default: "Documentation", template: "%s · ModelRouter Docs" },
};

export default function DocsLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      <SiteHeader />
      <div className="container-page grid gap-10 py-10 lg:grid-cols-[220px_minmax(0,1fr)] lg:gap-14 lg:py-14">
        <aside className="hidden lg:block">
          <div className="sticky top-24">
            <DocsSidebar />
          </div>
        </aside>
        <details className="rounded-xl border border-border bg-surface px-4 py-3 lg:hidden">
          <summary className="cursor-pointer text-sm font-medium">Documentation menu</summary>
          <div className="mt-4">
            <DocsSidebar />
          </div>
        </details>
        <main id="main" className="min-w-0 max-w-3xl">{children}</main>
      </div>
      <SiteFooter />
    </>
  );
}
