import { ArrowLeft } from "lucide-react";
import Link from "next/link";
import { notFound } from "next/navigation";
import { PageHeader, Stat } from "@/components/app/shell";
import { BarChart } from "@/components/charts/bar-chart";
import { Badge, Card } from "@/components/ui/primitives";
import { backend, BackendError } from "@/lib/backend";
import { formatDate, formatDateTime, formatNumber, formatUSD, timeAgo } from "@/lib/format";
import { requireAdmin } from "@/lib/session";
import type { ApiKey, DailyUsage, LedgerEntry, Payment, User } from "@/lib/types";
import { UserActions, KeyActions } from "./user-actions";

type Resp = { user: User; available_balance: number; keys: ApiKey[]; ledger: LedgerEntry[]; payments: Payment[]; daily: DailyUsage[] };

export default async function AdminUser({ params }: { params: Promise<{ id: string }> }) {
  const admin = await requireAdmin();
  const { id } = await params;
  let d: Resp;
  try {
    d = await backend<Resp>(`/api/admin/users/${encodeURIComponent(id)}`, { userId: admin.id });
  } catch (e) {
    if (e instanceof BackendError && (e.status === 404 || e.status === 400 || e.status === 500)) notFound();
    throw e;
  }
  const u = d.user;
  return (
    <>
      <Link href="/admin/users" className="mb-4 inline-flex items-center gap-1.5 text-sm text-muted hover:text-fg"><ArrowLeft className="size-4" /> Users</Link>
      <PageHeader
        title={u.email}
        description={<span className="flex flex-wrap items-center gap-2">{u.name || "No name"} · joined {formatDate(u.created_at)} {u.role === "admin" ? <Badge tone="accent">admin</Badge> : null}{u.banned ? <Badge tone="bad">banned</Badge> : null}</span>}
      />
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <Stat label="Available" value={formatUSD(d.available_balance)} />
        <Stat label="Total spent" value={formatUSD(u.total_spent)} />
        <Stat label="Total topped up" value={formatUSD(u.total_topup)} />
        <Stat label="Keys" value={d.keys.length} />
      </div>
      <div className="mt-4 grid gap-4 lg:grid-cols-[1.5fr_1fr]">
        <Card className="p-6">
          <h2 className="font-medium">Spend · 30 days</h2>
          <BarChart className="mt-8" title="Daily spend" data={d.daily.map((x) => ({ key: x.day, label: x.day.slice(5), value: x.cost, display: `${formatUSD(x.cost, { precise: true })} · ${x.requests} req` }))} />
        </Card>
        <Card className="p-6">
          <h2 className="font-medium">Manage</h2>
          <UserActions user={u} self={u.id === admin.id} />
        </Card>
      </div>
      <Card className="mt-4 p-6">
        <h2 className="font-medium">API keys</h2>
        <table className="mt-4 w-full text-sm">
          <thead><tr className="text-left text-[12px] text-subtle"><th className="pb-2 font-medium">Name</th><th className="pb-2 font-medium">Prefix</th><th className="pb-2 text-right font-medium">Spent</th><th className="pb-2 text-right font-medium">Requests</th><th className="pb-2 font-medium">Last used</th><th /></tr></thead>
          <tbody>
            {d.keys.map((k) => (
              <tr key={k.id} className="border-t border-border">
                <td className="py-2.5">{k.name} {k.disabled ? <Badge tone="warn">disabled</Badge> : null}</td>
                <td className="py-2.5 font-mono text-xs text-muted">{k.prefix}…</td>
                <td className="py-2.5 text-right tabular-nums">{formatUSD(k.total_spent, { precise: true })}</td>
                <td className="py-2.5 text-right tabular-nums">{formatNumber(k.total_requests)}</td>
                <td className="py-2.5 text-muted">{timeAgo(k.last_used_at)}</td>
                <td className="py-2.5 text-right"><KeyActions k={k} /></td>
              </tr>
            ))}
          </tbody>
        </table>
        {d.keys.length === 0 ? <p className="py-4 text-sm text-muted">No keys.</p> : null}
      </Card>
      <div className="mt-4 grid gap-4 lg:grid-cols-2">
        <Card className="p-6">
          <h2 className="font-medium">Payments</h2>
          <ul className="mt-3 divide-y divide-border text-sm">
            {d.payments.map((p) => (
              <li key={p.id} className="flex justify-between py-2.5"><span>{formatUSD(p.credit)} <span className="text-subtle">· {p.provider} · {formatDateTime(p.created_at)}</span></span><Badge tone={p.status === "completed" ? "good" : p.status === "pending" ? "warn" : "neutral"}>{p.status}</Badge></li>
            ))}
          </ul>
          {d.payments.length === 0 ? <p className="py-4 text-sm text-muted">No payments.</p> : null}
        </Card>
        <Card className="p-6">
          <h2 className="font-medium">Ledger</h2>
          <ul className="mt-3 divide-y divide-border text-sm">
            {d.ledger.map((e) => (
              <li key={e.id} className="flex justify-between gap-3 py-2.5">
                <span className="min-w-0 truncate">{e.reason.replace(/_/g, " ")} <span className="text-subtle">· {e.actor || "system"}{e.note ? ` · ${e.note}` : ""}</span></span>
                <span className={`shrink-0 tabular-nums ${e.amount >= 0 ? "text-good" : "text-bad"}`}>{formatUSD(e.amount)}</span>
              </li>
            ))}
          </ul>
          {d.ledger.length === 0 ? <p className="py-4 text-sm text-muted">No entries.</p> : null}
        </Card>
      </div>
      <p className="mt-6 text-sm"><Link className="text-accent hover:underline" href={`/admin/usage?user_id=${u.id}`}>View this user&apos;s request logs →</Link></p>
    </>
  );
}
