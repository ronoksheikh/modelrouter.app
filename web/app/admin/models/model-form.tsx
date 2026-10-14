"use client";

import { useRouter } from "next/navigation";
import { useState, useTransition } from "react";
import { Button } from "@/components/ui/button";
import { Field, Input, Select, Textarea } from "@/components/ui/primitives";
import { formatUSD, NANO_PER_USD } from "@/lib/format";
import type { AdminModel, Provider } from "@/lib/types";
import { adminDeleteModel, adminSaveModel, type ModelInput } from "../actions";

const usd = (nano: number) => String(Number((nano / NANO_PER_USD).toFixed(9)));

function Toggle({ name, label, defaultChecked }: { name: string; label: string; defaultChecked?: boolean }) {
  return (
    <label className="flex cursor-pointer items-center justify-between gap-3 rounded-xl border border-border px-3.5 py-2.5 text-sm">
      {label}
      <input type="checkbox" name={name} defaultChecked={defaultChecked} className="size-4 accent-[var(--accent)]" />
    </label>
  );
}

export function ModelForm({ model, providers, defaultMarkupBps }: { model?: AdminModel; providers: Provider[]; defaultMarkupBps: number }) {
  const router = useRouter();
  const [mode, setMode] = useState<"auto" | "manual">(model?.pricing_mode ?? "manual");
  const [error, setError] = useState<string | null>(null);
  const [pending, start] = useTransition();

  function submit(fd: FormData) {
    setError(null);
    const s = (k: string) => String(fd.get(k) ?? "").trim();
    const n = (k: string) => Number(fd.get(k) || 0);
    const input: ModelInput = {
      slug: s("slug"),
      name: s("name"),
      author: s("author"),
      description: s("description"),
      provider_id: n("provider_id"),
      upstream_id: s("upstream_id"),
      fallback_upstream: s("fallback").split(/[\n,]/).map((x) => x.trim()).filter(Boolean),
      context_length: n("context_length"),
      max_output_tokens: n("max_output_tokens"),
      pricing_mode: mode,
      markup_bps: s("markup") === "" ? null : Math.round(Number(s("markup")) * 100),
      input_price_per_m: s("input_price") || "0",
      output_price_per_m: s("output_price") || "0",
      request_fee: s("request_fee") === "" ? null : s("request_fee"),
      modalities: fd.get("vision") ? ["text", "image"] : ["text"],
      supports_tools: Boolean(fd.get("tools")),
      enabled: Boolean(fd.get("enabled")),
      featured: Boolean(fd.get("featured")),
      sort_order: n("sort_order"),
    };
    start(async () => {
      const res = await adminSaveModel(model?.id ?? null, input);
      if (res.ok) {
        router.push("/admin/models");
        router.refresh();
      } else setError(res.error);
    });
  }

  return (
    <form action={submit} className="space-y-8">
      <section className="grid gap-4 sm:grid-cols-2">
        <Field label="Public model ID" hint="What customers send as `model`. e.g. modelrouter/swift">
          <Input name="slug" required defaultValue={model?.slug} placeholder="modelrouter/swift" className="font-mono" />
        </Field>
        <Field label="Display name"><Input name="name" required defaultValue={model?.name} placeholder="Swift" /></Field>
        <Field label="Author label" hint="Shown publicly. Leave blank for ModelRouter.">
          <Input name="author" defaultValue={model?.author} placeholder="modelrouter" />
        </Field>
        <Field label="Sort order" hint="Higher shows first."><Input name="sort_order" type="number" defaultValue={model?.sort_order ?? 0} /></Field>
        <div className="sm:col-span-2">
          <Field label="Description"><Textarea name="description" rows={3} defaultValue={model?.description} /></Field>
        </div>
      </section>

      <section className="rounded-2xl border border-border bg-surface-2 p-5">
        <h3 className="text-sm font-semibold">Routing <span className="font-normal text-muted">· private, never shown to customers</span></h3>
        <div className="mt-4 grid gap-4 sm:grid-cols-2">
          <Field label="Provider">
            <Select name="provider_id" defaultValue={model?.provider_id ?? providers[0]?.id}>
              {providers.map((p) => <option key={p.id} value={p.id}>{p.name} ({p.kind})</option>)}
            </Select>
          </Field>
          <Field label="Upstream model ID" hint="The real model that serves requests.">
            <Input name="upstream_id" required defaultValue={model?.upstream_id} placeholder="openai/gpt-4o-mini" className="font-mono" />
          </Field>
          <div className="sm:col-span-2">
            <Field label="Fallback upstream models" hint="Tried in order if the primary fails. One per line.">
              <Textarea name="fallback" rows={2} className="font-mono" defaultValue={model?.fallback_upstream?.join("\n")} />
            </Field>
          </div>
          <Field label="Context length (tokens)"><Input name="context_length" type="number" min="0" defaultValue={model?.context_length ?? 0} /></Field>
          <Field label="Max output tokens (cap)" hint="Requests are clamped to this. 0 = no cap.">
            <Input name="max_output_tokens" type="number" min="0" defaultValue={model?.max_output_tokens ?? 0} />
          </Field>
        </div>
      </section>

      <section>
        <h3 className="text-sm font-semibold">Pricing</h3>
        <div className="mt-3 inline-flex rounded-xl border border-border bg-bg-soft p-1 text-[13px]">
          {(["manual", "auto"] as const).map((m) => (
            <button key={m} type="button" onClick={() => setMode(m)} className={`rounded-lg px-3 py-1.5 ${mode === m ? "bg-surface font-medium shadow-soft" : "text-muted"}`}>
              {m === "auto" ? "Auto (upstream + markup)" : "Manual price"}
            </button>
          ))}
        </div>
        <div className="mt-4 grid gap-4 sm:grid-cols-3">
          {mode === "auto" ? (
            <Field label="Markup %" hint={`Blank = platform default (${defaultMarkupBps / 100}%).`}>
              <Input name="markup" type="number" step="0.01" defaultValue={model?.markup_bps != null ? model.markup_bps / 100 : ""} />
            </Field>
          ) : (
            <>
              <Field label="Input $ / 1M tokens"><Input name="input_price" inputMode="decimal" defaultValue={model?.id ? usd(model.input_price) : "0"} /></Field>
              <Field label="Output $ / 1M tokens"><Input name="output_price" inputMode="decimal" defaultValue={model?.id ? usd(model.output_price) : "0"} /></Field>
            </>
          )}
          <Field label="Request fee $ (blank = default)">
            <Input name="request_fee" inputMode="decimal" placeholder="0.00000001" defaultValue={model?.request_fee != null ? usd(model.request_fee) : ""} />
          </Field>
        </div>
        {model?.id ? (
          <p className="mt-3 text-xs text-muted">
            Upstream cost: {formatUSD(model.upstream_input_price, { precise: true })} in / {formatUSD(model.upstream_output_price, { precise: true })} out per 1M ·
            Current sell: {formatUSD(model.input_price, { precise: true })} / {formatUSD(model.output_price, { precise: true })}
          </p>
        ) : null}
      </section>

      <section className="grid gap-2 sm:grid-cols-4">
        <Toggle name="enabled" label="Enabled" defaultChecked={model?.enabled ?? true} />
        <Toggle name="featured" label="Featured" defaultChecked={model?.featured} />
        <Toggle name="tools" label="Tool calling" defaultChecked={model?.supports_tools} />
        <Toggle name="vision" label="Image input" defaultChecked={model?.modalities?.includes("image")} />
      </section>

      {error ? <p className="rounded-xl border border-bad/20 bg-bad/5 px-4 py-3 text-sm text-bad">{error}</p> : null}
      <div className="flex items-center justify-between gap-3 border-t border-border pt-6">
        {model?.id ? (
          <Button
            type="button"
            variant="danger"
            disabled={pending}
            onClick={() =>
              confirm(`Delete ${model.slug}? Customers using it will get 404.`) &&
              start(async () => {
                const r = await adminDeleteModel(model.id);
                if (r.ok) router.push("/admin/models");
                else setError(r.error);
              })
            }
          >
            Delete model
          </Button>
        ) : <span />}
        <Button disabled={pending}>{pending ? "Saving…" : model?.id ? "Save changes" : "Create model"}</Button>
      </div>
    </form>
  );
}
