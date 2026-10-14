import { headers } from "next/headers";
import { Suspense } from "react";
import { PageHeader } from "@/components/app/shell";
import { Badge, Card } from "@/components/ui/primitives";
import { backend } from "@/lib/backend";
import { formatDateTime, formatUSD } from "@/lib/format";
import { requireUser } from "@/lib/session";
import type { LedgerEntry, Payment, PaymentMethod, User } from "@/lib/types";
import { TopUp } from "./top-up";

export const metadata = { title: "Billing" };

type Methods = { methods: PaymentMethod[]; min_topup_cents: number; max_topup_cents: number; bdt_per_usd: number };

const statusTone = { completed: "good", pending: "warn", failed: "bad", canceled: "neutral", refunded: "neutral" } as const;

function amount(p: Payment) {
  const v = p.amount_minor / 100;
  return p.currency === "BDT" ? `৳${v.toLocaleString("en-US", { maximumFractionDigits: 2 })}` : `$${v.toFixed(2)}`;
}

export default async function BillingPage({ searchParams }: { searchParams: Promise<{ payment?: string; canceled?: string }> }) {
  const user = await requireUser("/dashboard/billing");
  const sp = await searchParams;
  const h = await headers();
  const geo = (h.get("x-vercel-ip-country") || h.get("cf-ipcountry") || "").toUpperCase();

  const [me, bd, intl, payments, ledger, returned] = await Promise.all([
    backend<{ user: User; available_balance: number }>("/api/me", { userId: user.id }),
    backend<Methods>("/api/billing/methods?country=BD", { userId: user.id }),
    backend<Methods>("/api/billing/methods?country=US", { userId: user.id }),
    backend<{ data: Payment[] }>("/api/billing/payments?limit=20", { userId: user.id }),
    backend<{ data: LedgerEntry[] }>("/api/billing/ledger?limit=20", { userId: user.id }),
    sp.payment && /^[0-9a-f-]{36}$/.test(sp.payment)
      ? backend<Payment>(`/api/billing/payments/${sp.payment}`, { userId: user.id }).catch(() => null)
      : Promise.resolve(null),
  ]);
  const region = (me.user.country || geo) === "BD" ? "BD" : "INTL";

  return (
    <>
      <PageHeader title="Billing" description="Prepaid credits. Pay once, use any model, no expiry." />

      {returned ? (
        <div
          role="status"
          className={`mb-6 rounded-xl border px-4 py-3 text-sm ${
            returned.status === "completed" ? "border-good/25 bg-good/5 text-good" : sp.canceled ? "border-border bg-bg-soft text-muted" : "border-warn/30 bg-warn/5 text-fg-2"
          }`}
        >
          {returned.status === "completed"
            ? `Payment received — ${formatUSD(returned.credit)} added to your balance.`
            : sp.canceled
              ? "Payment cancelled. You have not been charged."
              : "We're confirming your payment. Credits appear automatically within a minute — refresh to update."}
        </div>
      ) : null}

      <div className="grid gap-4 lg:grid-cols-[1fr_1.4fr]">
        <Card className="relative overflow-hidden p-6">
          <div aria-hidden className="absolute -right-16 -top-16 size-56 rounded-full bg-[radial-gradient(closest-side,var(--glacier),transparent)] opacity-70" />
          <p className="relative text-sm text-muted">Available balance</p>
          <p className="relative mt-3 text-5xl font-semibold tracking-[-0.03em] tabular-nums">{formatUSD(me.available_balance)}</p>
          <dl className="relative mt-8 grid grid-cols-2 gap-4 border-t border-border pt-5 text-sm">
            <div>
              <dt className="text-muted">Total added</dt>
              <dd className="mt-1 font-medium tabular-nums">{formatUSD(me.user.total_topup)}</dd>
            </div>
            <div>
              <dt className="text-muted">Total spent</dt>
              <dd className="mt-1 font-medium tabular-nums">{formatUSD(me.user.total_spent)}</dd>
            </div>
          </dl>
        </Card>
        <Card className="p-6">
          <h2 className="font-medium">Add credits</h2>
          <Suspense>
          <TopUp
            bdMethods={bd.methods}
            intlMethods={intl.methods}
            defaultRegion={region}
            minCents={intl.min_topup_cents}
            maxCents={intl.max_topup_cents}
            bdtRate={intl.bdt_per_usd}
          />
          </Suspense>
        </Card>
      </div>

      <div className="mt-4 grid gap-4 lg:grid-cols-2">
        <Card className="p-6">
          <h2 className="font-medium">Payments</h2>
          {payments.data.length === 0 ? (
            <p className="mt-4 text-sm text-muted">No payments yet.</p>
          ) : (
            <ul className="mt-3 divide-y divide-border">
              {payments.data.map((p) => (
                <li key={p.id} className="flex items-center justify-between gap-3 py-3 text-sm">
                  <div>
                    <p className="font-medium tabular-nums">{amount(p)} <span className="font-normal text-muted">→ {formatUSD(p.credit)}</span></p>
                    <p className="text-xs text-subtle">{formatDateTime(p.created_at)} · {p.provider}</p>
                  </div>
                  <Badge tone={statusTone[p.status]}>{p.status}</Badge>
                </li>
              ))}
            </ul>
          )}
        </Card>
        <Card className="p-6">
          <h2 className="font-medium">Credit history</h2>
          {ledger.data.length === 0 ? (
            <p className="mt-4 text-sm text-muted">No credit changes yet.</p>
          ) : (
            <ul className="mt-3 divide-y divide-border">
              {ledger.data.map((e) => (
                <li key={e.id} className="flex items-center justify-between gap-3 py-3 text-sm">
                  <div>
                    <p className="font-medium capitalize">{e.reason.replace(/_/g, " ")}</p>
                    <p className="text-xs text-subtle">{formatDateTime(e.created_at)}{e.note ? ` · ${e.note}` : ""}</p>
                  </div>
                  <span className={`tabular-nums ${e.amount >= 0 ? "text-good" : "text-bad"}`}>
                    {e.amount >= 0 ? "+" : ""}{formatUSD(e.amount)}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </Card>
      </div>
    </>
  );
}
