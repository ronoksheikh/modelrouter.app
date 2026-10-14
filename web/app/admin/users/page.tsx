import Link from "next/link";
import { PageHeader } from "@/components/app/shell";
import { Pager, SearchForm } from "@/components/app/search-form";
import { Badge } from "@/components/ui/primitives";
import { backend } from "@/lib/backend";
import { formatDate, formatUSD } from "@/lib/format";
import { requireAdmin } from "@/lib/session";
import type { User } from "@/lib/types";

export const metadata = { title: "Users" };

export default async function UsersPage({ searchParams }: { searchParams: Promise<{ q?: string; offset?: string }> }) {
  const admin = await requireAdmin();
  const sp = await searchParams;
  const offset = Math.max(0, Number(sp.offset) || 0);
  const q = new URLSearchParams({ q: sp.q ?? "", limit: "50", offset: String(offset) });
  const res = await backend<{ data: User[]; total: number }>(`/api/admin/users?${q}`, { userId: admin.id });
  return (
    <>
      <PageHeader title="Users" description={`${res.total.toLocaleString()} accounts`} />
      <SearchForm defaultValue={sp.q} placeholder="Search by email, name or ID" />
      <div className="overflow-hidden rounded-2xl border border-border bg-surface shadow-soft">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[760px] text-sm">
            <thead>
              <tr className="border-b border-border text-left text-[12px] text-subtle">
                <th className="px-5 py-3 font-medium">User</th>
                <th className="px-5 py-3 font-medium">Country</th>
                <th className="px-5 py-3 text-right font-medium">Balance</th>
                <th className="px-5 py-3 text-right font-medium">Spent</th>
                <th className="px-5 py-3 text-right font-medium">Topped up</th>
                <th className="px-5 py-3 font-medium">Joined</th>
              </tr>
            </thead>
            <tbody>
              {res.data.map((u) => (
                <tr key={u.id} className="border-b border-border last:border-0 hover:bg-bg-soft/50">
                  <td className="px-5 py-3">
                    <Link href={`/admin/users/${u.id}`} className="font-medium hover:text-accent">{u.email}</Link>
                    <div className="mt-0.5 flex gap-1.5">
                      {u.name ? <span className="text-xs text-subtle">{u.name}</span> : null}
                      {u.role === "admin" ? <Badge tone="accent">admin</Badge> : null}
                      {u.banned ? <Badge tone="bad">banned</Badge> : null}
                    </div>
                  </td>
                  <td className="px-5 py-3 text-muted">{u.country || "—"}</td>
                  <td className="px-5 py-3 text-right tabular-nums">{formatUSD(u.balance)}</td>
                  <td className="px-5 py-3 text-right tabular-nums">{formatUSD(u.total_spent)}</td>
                  <td className="px-5 py-3 text-right tabular-nums">{formatUSD(u.total_topup)}</td>
                  <td className="px-5 py-3 text-muted">{formatDate(u.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
      <Pager base={`/admin/users?q=${encodeURIComponent(sp.q ?? "")}`} offset={offset} limit={50} total={res.total} />
    </>
  );
}
