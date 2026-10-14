import { Plus } from "lucide-react";
import Link from "next/link";
import { PageHeader } from "@/components/app/shell";
import { SearchForm } from "@/components/app/search-form";
import { ButtonLink } from "@/components/ui/button";
import { Badge } from "@/components/ui/primitives";
import { backend } from "@/lib/backend";
import { formatBps, formatTokens, formatUSD } from "@/lib/format";
import { requireAdmin } from "@/lib/session";
import type { AdminModel, Settings } from "@/lib/types";
import { ModelsToolbar, ModelToggle } from "./models-client";

export const metadata = { title: "Models" };

export default async function AdminModels({ searchParams }: { searchParams: Promise<{ q?: string; show?: string }> }) {
  const admin = await requireAdmin();
  const sp = await searchParams;
  const res = await backend<{ data: AdminModel[]; settings: Settings }>("/api/admin/models", { userId: admin.id });
  const q = (sp.q ?? "").toLowerCase();
  const list = res.data.filter(
    (m) =>
      (!q || `${m.slug} ${m.name} ${m.upstream_id}`.toLowerCase().includes(q)) &&
      (sp.show === "disabled" ? !m.enabled : sp.show === "custom" ? m.is_custom : true),
  );
  return (
    <>
      <PageHeader
        title="Models"
        description={`${res.data.filter((m) => m.enabled).length} live of ${res.data.length} · default markup ${formatBps(res.settings.default_markup_bps)}`}
        actions={
          <>
            <ModelsToolbar ids={list.map((m) => m.id)} />
            <ButtonLink href="/admin/models/new" size="sm"><Plus className="size-4" /> Custom model</ButtonLink>
          </>
        }
      />
      <SearchForm defaultValue={sp.q} placeholder="Search slug, name or upstream">
        <select name="show" defaultValue={sp.show ?? ""} className="h-10 rounded-xl border border-border bg-surface px-3 text-sm">
          <option value="">All</option>
          <option value="disabled">Disabled</option>
          <option value="custom">Custom</option>
        </select>
      </SearchForm>
      <div className="overflow-hidden rounded-2xl border border-border bg-surface shadow-soft">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[900px] text-sm">
            <thead>
              <tr className="border-b border-border text-left text-[12px] text-subtle">
                <th className="px-5 py-3 font-medium">Model</th>
                <th className="px-5 py-3 font-medium">Upstream</th>
                <th className="px-5 py-3 text-right font-medium">Sell in / out</th>
                <th className="px-5 py-3 text-right font-medium">Cost in / out</th>
                <th className="px-5 py-3 text-right font-medium">Cap</th>
                <th className="px-5 py-3 font-medium">Pricing</th>
                <th className="px-5 py-3 font-medium">Live</th>
              </tr>
            </thead>
            <tbody>
              {list.map((m) => (
                <tr key={m.id} className="border-b border-border last:border-0 hover:bg-bg-soft/50">
                  <td className="px-5 py-3">
                    <Link href={`/admin/models/${m.id}`} className="font-medium hover:text-accent">{m.name}</Link>
                    <div className="mt-0.5 flex items-center gap-1.5">
                      <span className="font-mono text-[12px] text-subtle">{m.slug}</span>
                      {m.is_custom ? <Badge tone="accent">custom</Badge> : null}
                      {m.featured ? <Badge>featured</Badge> : null}
                    </div>
                  </td>
                  <td className="max-w-[200px] truncate px-5 py-3 font-mono text-[12px] text-muted">{m.upstream_id}</td>
                  <td className="whitespace-nowrap px-5 py-3 text-right tabular-nums">{formatUSD(m.input_price, { precise: true })} / {formatUSD(m.output_price, { precise: true })}</td>
                  <td className="whitespace-nowrap px-5 py-3 text-right tabular-nums text-muted">{formatUSD(m.upstream_input_price, { precise: true })} / {formatUSD(m.upstream_output_price, { precise: true })}</td>
                  <td className="px-5 py-3 text-right tabular-nums text-muted">{m.max_output_tokens ? formatTokens(m.max_output_tokens) : "—"}</td>
                  <td className="px-5 py-3 text-muted">{m.pricing_mode === "auto" ? `auto ${formatBps(m.markup_bps ?? res.settings.default_markup_bps)}` : "manual"}</td>
                  <td className="px-5 py-3"><ModelToggle id={m.id} enabled={m.enabled} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {list.length === 0 ? (
          <p className="px-5 py-12 text-center text-sm text-muted">
            No models. <Link href="/admin/catalog" className="text-accent">Publish from the upstream catalog</Link> or create a custom model.
          </p>
        ) : null}
      </div>
    </>
  );
}
