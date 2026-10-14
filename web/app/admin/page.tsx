import Link from "next/link";
import { PageHeader, Stat } from "@/components/app/shell";
import { BarChart } from "@/components/charts/bar-chart";
import { Card } from "@/components/ui/primitives";
import { backend } from "@/lib/backend";
import { formatNumber, formatTokens, formatUSD } from "@/lib/format";
import { requireAdmin } from "@/lib/session";
import type { DailyUsage, ModelUsage, Overview } from "@/lib/types";
import { cn } from "@/lib/cn";

type Resp = {
  overview: Overview;
  daily: DailyUsage[];
  top_models: ModelUsage[];
  top_users: { user_id: string; email: string; requests: number; cost: number }[];
  live: { models_enabled: number; billing_queue: number };
};

const ranges = [7, 30, 90];

export default async function AdminOverview({ searchParams }: { searchParams: Promise<{ days?: string }> }) {
  const user = await requireAdmin();
  const days = ranges.includes(Number((await searchParams).days)) ? Number((await searchParams).days) : 30;
  const d = await backend<Resp>(`/api/admin/overview?days=${days}`, { userId: user.id });
  const o = d.overview;
  const profit = o.revenue - o.upstream_cost;
  const margin = o.revenue > 0 ? (profit / o.revenue) * 100 : 0;
  const errRate = o.requests > 0 ? (o.errors / o.requests) * 100 : 0;
  const day = (s: string) => new Date(s + "T00:00:00Z").toLocaleDateString("en-US", { month: "short", day: "numeric", timeZone: "UTC" });

  return (
    <>
      <PageHeader
        title="Overview"
        description={`Platform performance over the last ${days} days.`}
        actions={
          <div className="inline-flex rounded-xl border border-border bg-bg-soft p-1 text-[13px]">
            {ranges.map((r) => (
              <Link key={r} href={`/admin?days=${r}`} className={cn("rounded-lg px-3 py-1", r === days ? "bg-surface font-medium shadow-soft" : "text-muted hover:text-fg")}>
                {r}d
              </Link>
            ))}
          </div>
        }
      />
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <Stat label="Revenue (usage)" value={formatUSD(o.revenue)} hint={`Top-ups ${formatUSD(o.topups)}`} />
        <Stat label="Gross profit" value={formatUSD(profit)} hint={`${margin.toFixed(1)}% margin · upstream ${formatUSD(o.upstream_cost)}`} />
        <Stat label="Requests" value={formatNumber(o.requests)} hint={`${errRate.toFixed(2)}% errors · ${o.avg_latency_ms} ms avg`} />
        <Stat label="Users" value={formatNumber(o.users)} hint={`+${o.new_users} new · ${o.active_users} active`} />
      </div>

      <div className="mt-4 grid gap-4 lg:grid-cols-2">
        <Card className="p-6">
          <h2 className="font-medium">Daily revenue</h2>
          <BarChart
            className="mt-8"
            title="Daily revenue"
            data={d.daily.map((x) => ({ key: x.day, label: day(x.day), value: x.cost, display: `${formatUSD(x.cost, { precise: true })} · profit ${formatUSD(x.cost - (x.upstream_cost ?? 0), { precise: true })}` }))}
          />
        </Card>
        <Card className="p-6">
          <h2 className="font-medium">Daily requests</h2>
          <BarChart
            className="mt-8"
            title="Daily requests"
            data={d.daily.map((x) => ({ key: x.day, label: day(x.day), value: x.requests, display: `${formatNumber(x.requests)} requests · ${formatTokens(x.input_tokens + x.output_tokens)} tokens` }))}
          />
        </Card>
      </div>

      <div className="mt-4 grid gap-4 lg:grid-cols-2">
        <Card className="p-6">
          <h2 className="font-medium">Top models</h2>
          <table className="mt-4 w-full text-sm">
            <thead><tr className="text-left text-[12px] text-subtle"><th className="pb-2 font-medium">Model</th><th className="pb-2 text-right font-medium">Requests</th><th className="pb-2 text-right font-medium">Revenue</th><th className="pb-2 text-right font-medium">Profit</th></tr></thead>
            <tbody>
              {d.top_models.map((m) => (
                <tr key={m.model} className="border-t border-border">
                  <td className="max-w-[200px] truncate py-2.5 font-mono text-[12.5px]">{m.model}</td>
                  <td className="py-2.5 text-right tabular-nums">{formatNumber(m.requests)}</td>
                  <td className="py-2.5 text-right tabular-nums">{formatUSD(m.cost)}</td>
                  <td className="py-2.5 text-right tabular-nums text-muted">{formatUSD(m.cost - (m.upstream_cost ?? 0))}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {d.top_models.length === 0 ? <p className="py-6 text-sm text-muted">No traffic yet.</p> : null}
        </Card>
        <Card className="p-6">
          <h2 className="font-medium">Top customers</h2>
          <table className="mt-4 w-full text-sm">
            <thead><tr className="text-left text-[12px] text-subtle"><th className="pb-2 font-medium">User</th><th className="pb-2 text-right font-medium">Requests</th><th className="pb-2 text-right font-medium">Spend</th></tr></thead>
            <tbody>
              {d.top_users.map((u) => (
                <tr key={u.user_id} className="border-t border-border">
                  <td className="max-w-[220px] truncate py-2.5"><Link className="hover:text-accent" href={`/admin/users/${u.user_id}`}>{u.email || u.user_id}</Link></td>
                  <td className="py-2.5 text-right tabular-nums">{formatNumber(u.requests)}</td>
                  <td className="py-2.5 text-right tabular-nums">{formatUSD(u.cost)}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {d.top_users.length === 0 ? <p className="py-6 text-sm text-muted">No traffic yet.</p> : null}
        </Card>
      </div>

      <div className="mt-4 grid grid-cols-2 gap-3 lg:grid-cols-4">
        <Stat label="Outstanding balances" value={formatUSD(o.outstanding_balance)} hint="Credits owed to users" />
        <Stat label="Tokens" value={formatTokens(o.input_tokens + o.output_tokens)} hint={`${formatTokens(o.input_tokens)} in · ${formatTokens(o.output_tokens)} out`} />
        <Stat label="Models live" value={d.live.models_enabled} />
        <Stat label="Billing queue" value={d.live.billing_queue} hint="Unflushed usage records" />
      </div>
    </>
  );
}
