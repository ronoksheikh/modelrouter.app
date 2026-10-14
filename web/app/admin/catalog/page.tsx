import { PageHeader } from "@/components/app/shell";
import { Pager, SearchForm } from "@/components/app/search-form";
import { EmptyState } from "@/components/ui/primitives";
import { ButtonLink } from "@/components/ui/button";
import { backend } from "@/lib/backend";
import { requireAdmin } from "@/lib/session";
import type { Provider, Settings, UpstreamModel } from "@/lib/types";
import { CatalogTable } from "./catalog-table";

export const metadata = { title: "Upstream catalog" };

export default async function Catalog({ searchParams }: { searchParams: Promise<{ q?: string; offset?: string; provider?: string }> }) {
  const admin = await requireAdmin();
  const sp = await searchParams;
  const offset = Math.max(0, Number(sp.offset) || 0);
  const [providers, settings] = await Promise.all([
    backend<{ data: Provider[] }>("/api/admin/providers", { userId: admin.id }),
    backend<Settings>("/api/admin/settings", { userId: admin.id }),
  ]);
  const providerId = Number(sp.provider) || providers.data[0]?.id || 0;
  const q = new URLSearchParams({ q: sp.q ?? "", limit: "100", offset: String(offset), provider_id: String(providerId) });
  const res = providerId
    ? await backend<{ data: UpstreamModel[]; total: number }>(`/api/admin/upstream-models?${q}`, { userId: admin.id })
    : { data: [], total: 0 };

  return (
    <>
      <PageHeader title="Upstream catalog" description="Models available from your providers. Publish them to sell under the same ID, or map a custom model to them." />
      {providers.data.length === 0 ? (
        <EmptyState title="No providers yet" action={<ButtonLink href="/admin/providers">Add a provider</ButtonLink>}>
          Add an upstream provider and sync its catalog.
        </EmptyState>
      ) : (
        <>
          <SearchForm defaultValue={sp.q} placeholder="Search upstream models">
            <select name="provider" defaultValue={String(providerId)} className="h-10 rounded-xl border border-border bg-surface px-3 text-sm">
              {providers.data.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
            </select>
          </SearchForm>
          {res.total === 0 ? (
            <EmptyState title="Catalog is empty" action={<ButtonLink href="/admin/providers" variant="secondary">Sync from Providers</ButtonLink>}>
              Run a sync to pull the provider&apos;s model list.
            </EmptyState>
          ) : (
            <>
              <CatalogTable models={res.data} providerId={providerId} defaultMarkupBps={settings.default_markup_bps} />
              <Pager base={`/admin/catalog?q=${encodeURIComponent(sp.q ?? "")}&provider=${providerId}`} offset={offset} limit={100} total={res.total} />
            </>
          )}
        </>
      )}
    </>
  );
}
