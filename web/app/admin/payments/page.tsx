import Link from "next/link";
import { PageHeader } from "@/components/app/shell";
import { Pager } from "@/components/app/search-form";
import { Badge } from "@/components/ui/primitives";
import { backend } from "@/lib/backend";
import { formatDateTime, formatUSD } from "@/lib/format";
import { requireAdmin } from "@/lib/session";
import type { Payment } from "@/lib/types";
import { cn } from "@/lib/cn";

export const metadata = { title: "Payments" };
const statuses = ["", "completed", "pending", "failed", "canceled", "refunded"];

export default async function Payments({ searchParams }: { searchParams: Promise<{ status?: string; offset?: string }> }) {
  const admin = await requireAdmin();
  const sp = await searchParams;
  const status = statuses.includes(sp.status ?? "") ? sp.status ?? "" : "";
  const offset = Math.max(0, Number(sp.offset) || 0);
  const res = await backend<{ data: Payment[]; total: number }>(`/api/admin/payments?status=${status}&limit=50&offset=${offset}`, { userId: admin.id });
  return (
    <>
      <PageHeader title="Payments" description={`${res.total} payments`} />
      <div className="mb-4 flex flex-wrap gap-1.5">
        {statuses.map((s) => (
          <Link key={s} href={`/admin/payments${s ? `?status=${s}` : ""}`} className={cn("rounded-full border px-3 py-1 text-[13px] capitalize", s === status ? "border-accent/30 bg-accent-soft text-accent-ink" : "border-border text-muted hover:text-fg")}>
            {s || "all"}
          </Link>
        ))}
      </div>
      <div className="overflow-hidden rounded-2xl border border-border bg-surface shadow-soft">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[820px] text-sm">
            <thead>
              <tr className="border-b border-border text-left text-[12px] text-subtle">
                <th className="px-5 py-3 font-medium">Date</th>
                <th className="px-5 py-3 font-medium">User</th>
                <th className="px-5 py-3 font-medium">Provider</th>
                <th className="px-5 py-3 text-right font-medium">Charged</th>
                <th className="px-5 py-3 text-right font-medium">Credit</th>
                <th className="px-5 py-3 font-medium">Reference</th>
                <th className="px-5 py-3 font-medium">Status</th>
              </tr>
            </thead>
            <tbody>
              {res.data.map((p) => (
                <tr key={p.id} className="border-b border-border last:border-0">
                  <td className="whitespace-nowrap px-5 py-3 text-muted">{formatDateTime(p.created_at)}</td>
                  <td className="px-5 py-3"><Link className="hover:text-accent" href={`/admin/users/${p.user_id}`}>{p.user_email || p.user_id}</Link></td>
                  <td className="px-5 py-3">{p.provider}</td>
                  <td className="whitespace-nowrap px-5 py-3 text-right tabular-nums">{(p.amount_minor / 100).toFixed(2)} {p.currency}</td>
                  <td className="px-5 py-3 text-right tabular-nums">{formatUSD(p.credit)}</td>
                  <td className="max-w-[180px] truncate px-5 py-3 font-mono text-[11.5px] text-subtle">{p.external_id ?? p.id}</td>
                  <td className="px-5 py-3"><Badge tone={p.status === "completed" ? "good" : p.status === "pending" ? "warn" : p.status === "failed" ? "bad" : "neutral"}>{p.status}</Badge></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {res.data.length === 0 ? <p className="px-5 py-12 text-center text-sm text-muted">No payments.</p> : null}
      </div>
      <Pager base={`/admin/payments?status=${status}`} offset={offset} limit={50} total={res.total} />
    </>
  );
}
