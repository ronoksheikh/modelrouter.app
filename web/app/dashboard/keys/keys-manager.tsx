"use client";

import { AlertTriangle, KeyRound, MoreHorizontal, Plus, RefreshCw, Trash2 } from "lucide-react";
import { useState, useTransition } from "react";
import { Button } from "@/components/ui/button";
import { CopyButton } from "@/components/ui/copy-button";
import { Dialog } from "@/components/ui/dialog";
import { Badge, EmptyState, Field, Input } from "@/components/ui/primitives";
import { formatNumber, formatUSD, NANO_PER_USD, timeAgo } from "@/lib/format";
import type { ApiKey } from "@/lib/types";
import { createKey, deleteKey, rotateKey, updateKey } from "../actions";

function SecretReveal({ secret, onDone }: { secret: string; onDone: () => void }) {
  return (
    <div>
      <div className="flex items-start gap-2 rounded-xl border border-warn/30 bg-warn/5 p-3 text-[13px] text-fg-2">
        <AlertTriangle className="mt-0.5 size-4 shrink-0 text-warn" />
        Copy this key now. For your security it won&apos;t be shown again.
      </div>
      <div className="mt-4 flex items-center gap-2 rounded-xl bg-[var(--code-bg)] p-2 pl-3">
        <code className="min-w-0 flex-1 truncate font-mono text-[13px] text-slate-100">{secret}</code>
        <CopyButton value={secret} label="Copy key" />
      </div>
      <Button className="mt-5 w-full" onClick={onDone}>Done</Button>
    </div>
  );
}

export function KeysManager({ keys }: { keys: ApiKey[] }) {
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<ApiKey | null>(null);
  const [secret, setSecret] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pending, start] = useTransition();

  const closeAll = () => {
    setCreating(false);
    setEditing(null);
    setSecret(null);
    setError(null);
  };

  function submitCreate(fd: FormData) {
    setError(null);
    const limit = String(fd.get("limit") || "").trim();
    start(async () => {
      const res = await createKey({ name: String(fd.get("name") || ""), spendLimitUSD: limit ? Number(limit) : undefined });
      if (res.ok && res.data) setSecret(res.data.secret);
      else if (!res.ok) setError(res.error);
    });
  }

  function submitEdit(fd: FormData) {
    if (!editing) return;
    setError(null);
    const limit = String(fd.get("limit") || "").trim();
    const rpm = String(fd.get("rpm") || "").trim();
    start(async () => {
      const res = await updateKey(editing.id, {
        name: String(fd.get("name") || editing.name),
        spendLimitUSD: limit ? Number(limit) : null,
        rpmLimit: rpm ? Number(rpm) : undefined,
      });
      if (res.ok) closeAll();
      else setError(res.error);
    });
  }

  function rotate(k: ApiKey) {
    if (!confirm(`Rotate "${k.name}"? The current secret stops working immediately.`)) return;
    start(async () => {
      const res = await rotateKey(k.id);
      if (res.ok && res.data) {
        setEditing(null);
        setSecret(res.data.secret);
      } else if (!res.ok) alert(res.error);
    });
  }

  function remove(k: ApiKey) {
    if (!confirm(`Delete "${k.name}"? Apps using it will stop working.`)) return;
    start(async () => {
      const res = await deleteKey(k.id);
      if (res.ok) closeAll();
      else alert(res.error);
    });
  }

  function toggle(k: ApiKey) {
    start(async () => {
      const res = await updateKey(k.id, { disabled: !k.disabled });
      if (res.ok) closeAll();
      else alert(res.error);
    });
  }

  return (
    <>
      <div className="mb-4 flex justify-end">
        <Button onClick={() => setCreating(true)} size="sm"><Plus className="size-4" /> Create key</Button>
      </div>

      {keys.length === 0 ? (
        <EmptyState title="No API keys yet" action={<Button onClick={() => setCreating(true)}><Plus className="size-4" /> Create your first key</Button>}>
          Create a key to start making requests.
        </EmptyState>
      ) : (
        <div className="overflow-hidden rounded-2xl border border-border bg-surface shadow-soft">
          <div className="overflow-x-auto">
            <table className="w-full min-w-[720px] text-sm">
              <thead>
                <tr className="border-b border-border text-left text-[12px] text-subtle">
                  <th className="px-5 py-3 font-medium">Name</th>
                  <th className="px-5 py-3 font-medium">Key</th>
                  <th className="px-5 py-3 text-right font-medium">Spent</th>
                  <th className="px-5 py-3 text-right font-medium">Requests</th>
                  <th className="px-5 py-3 font-medium">Last used</th>
                  <th className="px-5 py-3" />
                </tr>
              </thead>
              <tbody>
                {keys.map((k) => (
                  <tr key={k.id} className="border-b border-border last:border-0">
                    <td className="px-5 py-3.5">
                      <div className="flex items-center gap-2">
                        <KeyRound className="size-4 text-subtle" />
                        <span className="font-medium">{k.name}</span>
                        {k.disabled ? <Badge tone="warn">Disabled</Badge> : null}
                      </div>
                    </td>
                    <td className="px-5 py-3.5 font-mono text-[12.5px] text-muted">{k.prefix}…</td>
                    <td className="px-5 py-3.5 text-right tabular-nums">
                      {formatUSD(k.total_spent, { precise: k.total_spent < 10_000_000 })}
                      {k.spend_limit != null ? <span className="text-subtle"> / {formatUSD(k.spend_limit)}</span> : null}
                    </td>
                    <td className="px-5 py-3.5 text-right tabular-nums">{formatNumber(k.total_requests)}</td>
                    <td className="px-5 py-3.5 text-muted">{timeAgo(k.last_used_at)}</td>
                    <td className="px-3 py-3.5 text-right">
                      <Button variant="ghost" size="sm" aria-label={`Manage ${k.name}`} onClick={() => setEditing(k)}>
                        <MoreHorizontal className="size-4" />
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      <Dialog open={creating} onClose={closeAll} title={secret ? "Your new API key" : "Create API key"} description={secret ? undefined : "Give it a name you'll recognise later."}>
        {secret ? (
          <SecretReveal secret={secret} onDone={closeAll} />
        ) : (
          <form action={submitCreate} className="space-y-4">
            <Field label="Name"><Input name="name" placeholder="e.g. Production server" maxLength={64} autoFocus /></Field>
            <Field label="Spend limit (USD, optional)" hint="Requests are rejected once this key has spent this much.">
              <Input name="limit" type="number" min="0" step="0.01" placeholder="No limit" />
            </Field>
            {error ? <p className="text-sm text-bad">{error}</p> : null}
            <Button className="w-full" disabled={pending}>{pending ? "Creating…" : "Create key"}</Button>
          </form>
        )}
      </Dialog>

      <Dialog open={!creating && (editing !== null || secret !== null)} onClose={closeAll} title={secret ? "Key rotated" : `Manage “${editing?.name ?? ""}”`}>
        {secret ? (
          <SecretReveal secret={secret} onDone={closeAll} />
        ) : editing ? (
          <div className="space-y-6">
            <form action={submitEdit} className="space-y-4">
              <Field label="Name"><Input name="name" defaultValue={editing.name} maxLength={64} /></Field>
              <div className="grid grid-cols-2 gap-3">
                <Field label="Spend limit (USD)">
                  <Input name="limit" type="number" min="0" step="0.01" placeholder="None" defaultValue={editing.spend_limit != null ? editing.spend_limit / NANO_PER_USD : ""} />
                </Field>
                <Field label="Requests / min">
                  <Input name="rpm" type="number" min="1" placeholder="Default" defaultValue={editing.rpm_limit ?? ""} />
                </Field>
              </div>
              {error ? <p className="text-sm text-bad">{error}</p> : null}
              <Button className="w-full" disabled={pending}>Save changes</Button>
            </form>
            <div className="grid grid-cols-3 gap-2 border-t border-border pt-5">
              <Button variant="secondary" size="sm" onClick={() => toggle(editing)} disabled={pending}>{editing.disabled ? "Enable" : "Disable"}</Button>
              <Button variant="secondary" size="sm" onClick={() => rotate(editing)} disabled={pending}><RefreshCw className="size-3.5" /> Rotate</Button>
              <Button variant="danger" size="sm" onClick={() => remove(editing)} disabled={pending}><Trash2 className="size-3.5" /> Delete</Button>
            </div>
          </div>
        ) : null}
      </Dialog>
    </>
  );
}
