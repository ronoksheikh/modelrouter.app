"use client";

import { Check, Loader2 } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useState, useTransition } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/primitives";
import { cn } from "@/lib/cn";
import { centsToBDT } from "@/lib/format";
import type { PaymentMethod } from "@/lib/types";
import { startCheckout } from "../actions";

declare global {
  interface Window {
    Paddle?: {
      Environment: { set: (env: string) => void };
      Initialize: (opts: { token: string; eventCallback?: (e: { name?: string }) => void }) => void;
      Checkout: { open: (opts: { transactionId: string }) => void };
    };
  }
}

const PRESETS = [5, 10, 25, 50, 100];
let paddleReady: Promise<NonNullable<Window["Paddle"]>> | null = null;

function loadPaddle(env: string, onComplete: () => void) {
  const token = process.env.NEXT_PUBLIC_PADDLE_CLIENT_TOKEN;
  if (!token) return null;
  paddleReady ??= new Promise((resolve, reject) => {
    const s = document.createElement("script");
    s.src = "https://cdn.paddle.com/paddle/v2/paddle.js";
    s.async = true;
    s.onload = () => {
      const P = window.Paddle!;
      if (env !== "production") P.Environment.set("sandbox");
      P.Initialize({ token, eventCallback: (e) => e.name === "checkout.completed" && onComplete() });
      resolve(P);
    };
    s.onerror = reject;
    document.head.appendChild(s);
  });
  return paddleReady;
}

export function TopUp({
  bdMethods,
  intlMethods,
  defaultRegion,
  minCents,
  maxCents,
  bdtRate,
}: {
  bdMethods: PaymentMethod[];
  intlMethods: PaymentMethod[];
  defaultRegion: "BD" | "INTL";
  minCents: number;
  maxCents: number;
  bdtRate: number;
}) {
  const router = useRouter();
  const params = useSearchParams();
  const [region, setRegion] = useState(defaultRegion);
  const methods = region === "BD" ? bdMethods : intlMethods;
  const firstAvailable = methods.find((m) => m.available)?.id ?? "";
  const [method, setMethod] = useState(firstAvailable);
  const [amount, setAmount] = useState(10);
  const [custom, setCustom] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, start] = useTransition();
  const env = process.env.NEXT_PUBLIC_PADDLE_ENV || "sandbox";

  useEffect(() => setMethod(methods.find((m) => m.available)?.id ?? ""), [region]); // eslint-disable-line react-hooks/exhaustive-deps

  // Paddle's hosted payment link sends users back with ?_ptxn=txn_...
  useEffect(() => {
    const txn = params.get("_ptxn");
    if (!txn) return;
    loadPaddle(env, () => setTimeout(() => router.refresh(), 2500))?.then((P) => P.Checkout.open({ transactionId: txn }));
  }, [params, env, router]);

  const cents = Math.round((custom ? Number(custom) : amount) * 100);
  const valid = Number.isFinite(cents) && cents >= minCents && cents <= maxCents;
  const selected = methods.find((m) => m.id === method);

  function pay() {
    setError(null);
    if (!valid || !selected) return;
    start(async () => {
      const res = await startCheckout(selected.id, cents, region === "BD" ? "BD" : "");
      if (!res.ok) {
        setError(res.error);
        return;
      }
      const co = res.data!.checkout;
      if (co.client_data?.provider === "paddle" && process.env.NEXT_PUBLIC_PADDLE_CLIENT_TOKEN) {
        const P = await loadPaddle(co.client_data.environment || env, () =>
          setTimeout(() => router.push(`/dashboard/billing?payment=${res.data!.payment_id}`), 1500),
        );
        P?.Checkout.open({ transactionId: co.client_data.transaction_id });
        return;
      }
      if (co.redirect_url) window.location.assign(co.redirect_url);
      else setError("Checkout is unavailable right now.");
    });
  }

  return (
    <div className="mt-5 space-y-5">
      <div className="inline-flex rounded-xl border border-border bg-bg-soft p-1 text-[13px]" role="tablist" aria-label="Payment region">
        {(["INTL", "BD"] as const).map((r) => (
          <button
            key={r}
            role="tab"
            aria-selected={region === r}
            onClick={() => setRegion(r)}
            className={cn("rounded-lg px-3 py-1.5 transition", region === r ? "bg-surface font-medium text-fg shadow-soft" : "text-muted hover:text-fg")}
          >
            {r === "BD" ? "🇧🇩 Bangladesh" : "International"}
          </button>
        ))}
      </div>

      <div>
        <p className="mb-2 text-[13px] font-medium text-fg-2">Amount (USD)</p>
        <div className="grid grid-cols-3 gap-2 sm:grid-cols-6">
          {PRESETS.map((p) => (
            <button
              key={p}
              onClick={() => {
                setAmount(p);
                setCustom("");
              }}
              className={cn(
                "h-10 rounded-xl border text-sm font-medium tabular-nums transition",
                !custom && amount === p ? "border-accent bg-accent-soft text-accent-ink" : "border-border hover:border-border-strong",
              )}
            >
              ${p}
            </button>
          ))}
          <Input
            aria-label="Custom amount"
            type="number"
            min={minCents / 100}
            max={maxCents / 100}
            placeholder="Other"
            value={custom}
            onChange={(e) => setCustom(e.target.value)}
            className="tabular-nums"
          />
        </div>
        {!valid ? (
          <p className="mt-2 text-xs text-bad">Enter between ${minCents / 100} and ${(maxCents / 100).toLocaleString()}.</p>
        ) : null}
      </div>

      <div className="space-y-2">
        <p className="text-[13px] font-medium text-fg-2">Payment method</p>
        {methods.map((m) => (
          <button
            key={m.id}
            disabled={!m.available}
            onClick={() => setMethod(m.id)}
            className={cn(
              "flex w-full items-center gap-3 rounded-xl border p-3.5 text-left transition disabled:cursor-not-allowed disabled:opacity-55",
              method === m.id ? "border-accent bg-accent-soft/50 ring-4 ring-[var(--ring)]/40" : "border-border hover:border-border-strong",
            )}
          >
            <span className={cn("flex size-5 shrink-0 items-center justify-center rounded-full border", method === m.id ? "border-accent bg-accent text-accent-fg" : "border-border-strong")}>
              {method === m.id ? <Check className="size-3" strokeWidth={3} /> : null}
            </span>
            <span className="min-w-0 flex-1">
              <span className="flex items-center gap-2 text-sm font-medium">
                {m.name}
                {!m.available ? <span className="rounded-full bg-bg-soft px-2 py-0.5 text-[11px] font-normal text-muted">Coming soon</span> : null}
              </span>
              <span className="mt-0.5 block text-xs text-muted">{m.description}</span>
            </span>
            <span className="shrink-0 text-xs font-medium text-subtle">{m.currency}</span>
          </button>
        ))}
      </div>

      {error ? <p className="text-sm text-bad">{error}</p> : null}
      <Button size="lg" className="w-full" onClick={pay} disabled={!valid || !selected?.available || pending}>
        {pending ? <Loader2 className="size-4 animate-spin" /> : null}
        {selected?.currency === "BDT" && valid
          ? `Pay ৳${centsToBDT(cents, bdtRate).toLocaleString()} for $${(cents / 100).toFixed(2)} credit`
          : `Pay $${valid ? (cents / 100).toFixed(2) : "—"}`}
      </Button>
      <p className="text-center text-xs text-subtle">Credits never expire. Taxes may apply at checkout.</p>
    </div>
  );
}
