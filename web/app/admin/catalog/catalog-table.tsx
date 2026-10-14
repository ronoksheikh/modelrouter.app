"use client";

import Link from "next/link";
import { useState, useTransition } from "react";
import { Button } from "@/components/ui/button";
import { Badge, Input } from "@/components/ui/primitives";
import { formatTokens, formatUSD } from "@/lib/format";
import type { UpstreamModel } from "@/lib/types";
import { adminPublish } from "../actions";

export function CatalogTable({ models, providerId, defaultMarkupBps }: { models: UpstreamModel[]; providerId: number; defaultMarkupBps: number }) {
  const [sel, setSel] = useState<Set<string>>(new Set());
  const [markup, setMarkup] = useState("");
  const [msg, setMsg] = useState<string | null>(null);
  const [pending, start] = useTransition();
  const unpublished = models.filter((m) => !m.published_as?.length);
  const toggle = (id: string) => setSel((s) => { const n = new Set(s); if (n.has(id)) n.delete(id); else n.add(id); return n; });

  return (
    <>
      <div className="mb-3 flex flex-wrap items-center gap-2 rounded-2xl border border-border bg-surface p-3 shadow-soft">
        <span className="px-1 text-sm text-muted">{sel.size} selected</span>
        <Button variant="ghost" size="sm" onClick={() => setSel(new Set(unpublished.map((m) => m.upstream_id)))}>Select unpublished</Button>
        <Button variant="ghost" size="sm" onClick={() => setSel(new Set())}>Clear</Button>
        <div className="ml-auto flex items-center gap-2">
          <Input value={markup} onChange={(e) => setMarkup(e.target.value)} type="number" step="0.1" placeholder={`Markup % (default ${defaultMarkupBps / 100})`} className="h-8 w-48 text-[13px]" />
          <Button
            size="sm"
            disabled={pending || sel.size === 0}
            onClick={() =>
              start(async () => {
                const r = await adminPublish(providerId, [...sel], markup === "" ? null : Math.round(Number(markup) * 100));
                setMsg(r.ok ? `Published ${(r.data as { published: number }).published} models.` : r.error);
                if (r.ok) setSel(new Set());
              })
            }
          >
            Publish {sel.size || ""}
          </Button>
        </div>
      </div>
      {msg ? <p className="mb-3 text-sm text-good">{msg}</p> : null}
      <div className="overflow-hidden rounded-2xl border border-border bg-surface shadow-soft">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[860px] text-sm">
            <thead>
              <tr className="border-b border-border text-left text-[12px] text-subtle">
                <th className="w-10 px-4 py-3" />
                <th className="px-3 py-3 font-medium">Upstream model</th>
                <th className="px-3 py-3 text-right font-medium">Cost in / out (1M)</th>
                <th className="px-3 py-3 text-right font-medium">Context</th>
                <th className="px-3 py-3 font-medium">Published as</th>
                <th className="px-3 py-3" />
              </tr>
            </thead>
            <tbody>
              {models.map((m) => (
                <tr key={m.upstream_id} className="border-b border-border last:border-0">
                  <td className="px-4 py-2.5">
                    <input type="checkbox" aria-label={`Select ${m.upstream_id}`} checked={sel.has(m.upstream_id)} onChange={() => toggle(m.upstream_id)} className="size-4 accent-[var(--accent)]" />
                  </td>
                  <td className="px-3 py-2.5">
                    <span className="font-medium">{m.name}</span>
                    <span className="block font-mono text-[12px] text-subtle">{m.upstream_id}</span>
                  </td>
                  <td className="whitespace-nowrap px-3 py-2.5 text-right tabular-nums">
                    {m.input_price === 0 && m.output_price === 0 ? <Badge tone="good">free</Badge> : `${formatUSD(m.input_price, { precise: true })} / ${formatUSD(m.output_price, { precise: true })}`}
                  </td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-muted">{formatTokens(m.context_length)}</td>
                  <td className="px-3 py-2.5">
                    <div className="flex flex-wrap gap-1">
                      {m.published_as?.map((s) => <Badge key={s} tone="accent">{s}</Badge>)}
                    </div>
                  </td>
                  <td className="px-3 py-2.5 text-right">
                    <Link href={`/admin/models/new?upstream=${encodeURIComponent(m.upstream_id)}&provider=${providerId}`} className="whitespace-nowrap text-[13px] text-accent hover:underline">
                      Custom model →
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </>
  );
}
