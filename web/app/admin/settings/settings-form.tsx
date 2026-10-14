"use client";

import { useState, useTransition } from "react";
import { Button } from "@/components/ui/button";
import { Field, Input, Textarea } from "@/components/ui/primitives";
import { NANO_PER_USD } from "@/lib/format";
import type { Settings } from "@/lib/types";
import { adminSaveSettings } from "../actions";

const toUSD = (nano: number) => String(Number((nano / NANO_PER_USD).toFixed(9)));
const toNano = (usd: string) => Math.round(Number(usd || 0) * NANO_PER_USD);

function Section({ title, description, children }: { title: string; description: string; children: React.ReactNode }) {
  return (
    <section className="grid gap-6 border-b border-border pb-8 last:border-0 lg:grid-cols-[240px_1fr]">
      <div>
        <h3 className="text-sm font-semibold">{title}</h3>
        <p className="mt-1 text-xs leading-5 text-muted">{description}</p>
      </div>
      <div className="grid gap-4 sm:grid-cols-2">{children}</div>
    </section>
  );
}

export function SettingsForm({ settings: s }: { settings: Settings }) {
  const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null);
  const [pending, start] = useTransition();
  return (
    <form
      className="space-y-8"
      action={(fd) =>
        start(async () => {
          const g = (k: string) => String(fd.get(k) ?? "");
          const res = await adminSaveSettings({
            default_markup_bps: Math.round(Number(g("markup")) * 100),
            default_request_fee: toNano(g("fee")),
            signup_bonus: toNano(g("bonus")),
            min_topup_cents: Math.round(Number(g("min")) * 100),
            max_topup_cents: Math.round(Number(g("max")) * 100),
            bdt_per_usd: Number(g("bdt")),
            default_rpm: Number(g("rpm")),
            free_model_rpm: Number(g("free_rpm")),
            default_max_output_tokens: Number(g("max_out")),
            auto_publish_new_models: Boolean(fd.get("auto_publish")),
            maintenance_mode: Boolean(fd.get("maintenance")),
            announcement: g("announcement"),
          });
          setMsg(res.ok ? { ok: true, text: "Settings saved. Auto-priced models were repriced." } : { ok: false, text: res.error });
        })
      }
    >
      <Section title="Pricing" description="Defaults for auto-priced models. Per-model overrides win.">
        <Field label="Default markup %" hint="Added on top of upstream cost."><Input name="markup" type="number" step="0.01" defaultValue={s.default_markup_bps / 100} /></Field>
        <Field label="Default request fee (USD)" hint="0.00000001 = 0.000001 cents"><Input name="fee" inputMode="decimal" defaultValue={toUSD(s.default_request_fee)} /></Field>
        <Field label="Signup bonus (USD)" hint="Free credit for new accounts."><Input name="bonus" inputMode="decimal" defaultValue={toUSD(s.signup_bonus)} /></Field>
        <Field label="Default output cap (tokens)" hint="Applied to models without a cap. 0 = none."><Input name="max_out" type="number" min="0" defaultValue={s.default_max_output_tokens} /></Field>
      </Section>
      <Section title="Payments" description="Top-up limits and the BDT exchange rate used for Bangladeshi methods.">
        <Field label="Minimum top-up (USD)"><Input name="min" type="number" min="1" step="1" defaultValue={s.min_topup_cents / 100} /></Field>
        <Field label="Maximum top-up (USD)"><Input name="max" type="number" min="1" step="1" defaultValue={s.max_topup_cents / 100} /></Field>
        <Field label="BDT per USD"><Input name="bdt" type="number" min="1" step="0.01" defaultValue={s.bdt_per_usd} /></Field>
      </Section>
      <Section title="Limits" description="Rate limits protect upstream capacity.">
        <Field label="Default requests/min per key"><Input name="rpm" type="number" min="0" defaultValue={s.default_rpm} /></Field>
        <Field label="Free model requests/min per user"><Input name="free_rpm" type="number" min="0" defaultValue={s.free_model_rpm} /></Field>
      </Section>
      <Section title="Operations" description="Catalog automation and maintenance controls.">
        <label className="flex items-center gap-2 text-sm sm:col-span-2"><input type="checkbox" name="auto_publish" defaultChecked={s.auto_publish_new_models} className="accent-[var(--accent)]" /> Auto-publish new upstream models on sync</label>
        <label className="flex items-center gap-2 text-sm sm:col-span-2"><input type="checkbox" name="maintenance" defaultChecked={s.maintenance_mode} className="accent-[var(--accent)]" /> Maintenance mode (API returns 503)</label>
        <div className="sm:col-span-2">
          <Field label="Announcement" hint="Shown in public config for the site banner."><Textarea name="announcement" rows={2} defaultValue={s.announcement} /></Field>
        </div>
      </Section>
      <div className="flex items-center gap-3">
        <Button disabled={pending}>{pending ? "Saving…" : "Save settings"}</Button>
        {msg ? <span className={`text-sm ${msg.ok ? "text-good" : "text-bad"}`}>{msg.text}</span> : null}
      </div>
    </form>
  );
}
