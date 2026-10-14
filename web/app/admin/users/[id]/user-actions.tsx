"use client";

import { useState, useTransition } from "react";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/primitives";
import type { ApiKey, User } from "@/lib/types";
import { adminCreditUser, adminDeleteKey, adminUpdateKey, adminUpdateUser } from "../../actions";

export function UserActions({ user, self }: { user: User; self: boolean }) {
  const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null);
  const [pending, start] = useTransition();
  const done = (r: { ok: boolean; error?: string }, text: string) => setMsg(r.ok ? { ok: true, text } : { ok: false, text: r.error ?? "Failed" });

  return (
    <div className="mt-4 space-y-5">
      <form
        className="space-y-3"
        action={(fd) =>
          start(async () => {
            const amount = Number(fd.get("amount"));
            if (!amount) return setMsg({ ok: false, text: "Enter a non-zero amount." });
            if (!confirm(`${amount > 0 ? "Add" : "Remove"} $${Math.abs(amount)} ${amount > 0 ? "to" : "from"} ${user.email}?`)) return;
            done(await adminCreditUser(user.id, amount, String(fd.get("note") || "")), "Balance updated.");
          })
        }
      >
        <div className="grid grid-cols-[110px_1fr] gap-2">
          <Field label="Amount (USD)"><Input name="amount" type="number" step="0.01" placeholder="10 or -5" /></Field>
          <Field label="Note"><Input name="note" placeholder="Reason (visible in ledger)" /></Field>
        </div>
        <Button size="sm" className="w-full" disabled={pending}>Adjust balance</Button>
      </form>
      <div className="grid grid-cols-2 gap-2 border-t border-border pt-5">
        <Button
          variant="secondary"
          size="sm"
          disabled={pending || self}
          onClick={() => start(async () => done(await adminUpdateUser(user.id, { role: user.role === "admin" ? "user" : "admin" }), "Role updated."))}
        >
          {user.role === "admin" ? "Remove admin" : "Make admin"}
        </Button>
        <Button
          variant={user.banned ? "secondary" : "danger"}
          size="sm"
          disabled={pending || self}
          onClick={() => {
            if (!user.banned && !confirm(`Suspend ${user.email}? All their keys stop working.`)) return;
            start(async () => done(await adminUpdateUser(user.id, { banned: !user.banned }), user.banned ? "User restored." : "User suspended."));
          }}
        >
          {user.banned ? "Unsuspend" : "Suspend"}
        </Button>
      </div>
      {msg ? <p className={`text-sm ${msg.ok ? "text-good" : "text-bad"}`}>{msg.text}</p> : null}
    </div>
  );
}

export function KeyActions({ k }: { k: ApiKey }) {
  const [pending, start] = useTransition();
  return (
    <span className="inline-flex gap-1">
      <Button variant="ghost" size="sm" disabled={pending} onClick={() => start(async () => void (await adminUpdateKey(k.id, { disabled: !k.disabled })))}>
        {k.disabled ? "Enable" : "Disable"}
      </Button>
      <Button
        variant="ghost"
        size="sm"
        className="text-bad"
        disabled={pending}
        onClick={() => confirm("Delete this key?") && start(async () => void (await adminDeleteKey(k.id)))}
      >
        Delete
      </Button>
    </span>
  );
}
