"use client";

import { Plus, RefreshCw } from "lucide-react";
import { useState, useTransition } from "react";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Badge, Card, EmptyState, Field, Input, Select } from "@/components/ui/primitives";
import { formatDate } from "@/lib/format";
import type { Provider } from "@/lib/types";
import { adminDeleteProvider, adminSaveProvider, adminSyncProvider, type ProviderInput } from "../actions";

export function ProvidersManager({ providers }: { providers: Provider[] }) {
  const [editing, setEditing] = useState<Provider | "new" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);
  const [pending, start] = useTransition();
  const current = editing && editing !== "new" ? editing : null;

  function save(fd: FormData) {
    setError(null);
    const key = String(fd.get("api_key") || "");
    const input: ProviderInput = {
      name: String(fd.get("name") || ""),
      kind: String(fd.get("kind")) as "openai" | "anthropic",
      base_url: String(fd.get("base_url") || ""),
      is_openrouter: Boolean(fd.get("is_openrouter")),
      enabled: Boolean(fd.get("enabled")),
      ...(key ? { api_key: key } : {}),
    };
    start(async () => {
      const r = await adminSaveProvider(current?.id ?? null, input);
      if (r.ok) setEditing(null);
      else setError(r.error);
    });
  }

  function sync(p: Provider) {
    setMsg(`Syncing ${p.name}…`);
    start(async () => {
      const r = await adminSyncProvider(p.id);
      setMsg(r.ok && r.data ? `${p.name}: fetched ${r.data.fetched} models, ${r.data.added.length} new, ${r.data.repriced} repriced.` : !r.ok ? r.error : null);
    });
  }

  return (
    <>
      <div className="mb-4 flex justify-end">
        <Button size="sm" onClick={() => setEditing("new")}><Plus className="size-4" /> Add provider</Button>
      </div>
      {msg ? <p className="mb-4 rounded-xl border border-border bg-surface px-4 py-3 text-sm text-fg-2">{msg}</p> : null}
      {providers.length === 0 ? (
        <EmptyState title="No providers" action={<Button onClick={() => setEditing("new")}>Add provider</Button>}>
          Set OPENROUTER_API_KEY to create one automatically, or add any OpenAI- or Anthropic-compatible API.
        </EmptyState>
      ) : (
        <div className="grid gap-3 md:grid-cols-2">
          {providers.map((p) => (
            <Card key={p.id} className="p-5">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <p className="font-medium">{p.name}</p>
                  <p className="mt-0.5 font-mono text-xs text-subtle">{p.base_url}</p>
                </div>
                <div className="flex gap-1.5">
                  <Badge>{p.kind}</Badge>
                  {p.enabled ? <Badge tone="good">enabled</Badge> : <Badge tone="warn">disabled</Badge>}
                </div>
              </div>
              <p className="mt-3 text-xs text-muted">
                {p.has_key ? "API key set" : "No API key"} · {p.is_openrouter ? "aggregator, cost reporting on" : "direct"} · added {formatDate(p.created_at)}
              </p>
              <div className="mt-4 flex gap-2">
                <Button variant="secondary" size="sm" onClick={() => sync(p)} disabled={pending}><RefreshCw className="size-3.5" /> Sync models</Button>
                <Button variant="ghost" size="sm" onClick={() => setEditing(p)}>Edit</Button>
                <Button
                  variant="ghost"
                  size="sm"
                  className="ml-auto text-bad"
                  disabled={pending}
                  onClick={() => confirm(`Delete ${p.name}?`) && start(async () => { const r = await adminDeleteProvider(p.id); if (!r.ok) setMsg(r.error); })}
                >
                  Delete
                </Button>
              </div>
            </Card>
          ))}
        </div>
      )}

      <Dialog open={editing !== null} onClose={() => setEditing(null)} title={current ? `Edit ${current.name}` : "Add provider"}>
        <form action={save} className="space-y-4" key={current?.id ?? "new"}>
          <Field label="Name"><Input name="name" required defaultValue={current?.name} placeholder="Primary" /></Field>
          <Field label="API format">
            <Select name="kind" defaultValue={current?.kind ?? "openai"}>
              <option value="openai">OpenAI-compatible (/chat/completions)</option>
              <option value="anthropic">Anthropic native (/messages)</option>
            </Select>
          </Field>
          <Field label="Base URL" hint="Without the endpoint path, e.g. https://openrouter.ai/api/v1">
            <Input name="base_url" required type="url" defaultValue={current?.base_url} placeholder="https://api.example.com/v1" className="font-mono" />
          </Field>
          <Field label="API key" hint={current?.has_key ? "Leave blank to keep the current key." : undefined}>
            <Input name="api_key" type="password" autoComplete="off" placeholder={current?.has_key ? "••••••••" : "sk-..."} />
          </Field>
          <label className="flex items-center gap-2 text-sm"><input type="checkbox" name="is_openrouter" defaultChecked={current?.is_openrouter} className="accent-[var(--accent)]" /> Aggregator with cost reporting (OpenRouter-style)</label>
          <label className="flex items-center gap-2 text-sm"><input type="checkbox" name="enabled" defaultChecked={current?.enabled ?? true} className="accent-[var(--accent)]" /> Enabled</label>
          {error ? <p className="text-sm text-bad">{error}</p> : null}
          <Button className="w-full" disabled={pending}>{current ? "Save" : "Add provider"}</Button>
        </form>
      </Dialog>
    </>
  );
}
