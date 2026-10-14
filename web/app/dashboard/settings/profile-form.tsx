"use client";

import { useState, useTransition } from "react";
import { Button } from "@/components/ui/button";
import { Field, Input, Select } from "@/components/ui/primitives";
import { updateProfile } from "../actions";

const COUNTRIES: [string, string][] = [
  ["", "Not set"], ["BD", "Bangladesh"], ["IN", "India"], ["PK", "Pakistan"], ["NP", "Nepal"], ["LK", "Sri Lanka"],
  ["US", "United States"], ["GB", "United Kingdom"], ["CA", "Canada"], ["AU", "Australia"], ["DE", "Germany"],
  ["FR", "France"], ["NL", "Netherlands"], ["AE", "United Arab Emirates"], ["SA", "Saudi Arabia"], ["SG", "Singapore"],
  ["MY", "Malaysia"], ["ID", "Indonesia"], ["JP", "Japan"], ["BR", "Brazil"], ["NG", "Nigeria"], ["ZZ", "Other"],
];

export function ProfileForm({ name, country }: { name: string; country: string }) {
  const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null);
  const [pending, start] = useTransition();
  return (
    <form
      className="mt-5 space-y-4"
      action={(fd) =>
        start(async () => {
          const c = String(fd.get("country") || "");
          const res = await updateProfile({ name: String(fd.get("name") || ""), country: c === "ZZ" ? "" : c });
          setMsg(res.ok ? { ok: true, text: "Saved." } : { ok: false, text: res.error });
        })
      }
    >
      <Field label="Name"><Input name="name" defaultValue={name} maxLength={80} /></Field>
      <Field label="Country" hint="Used to show local payment methods, such as bKash and Nagad in Bangladesh.">
        <Select name="country" defaultValue={country}>
          {COUNTRIES.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
        </Select>
      </Field>
      <div className="flex items-center gap-3">
        <Button disabled={pending}>{pending ? "Saving…" : "Save"}</Button>
        {msg ? <span className={`text-sm ${msg.ok ? "text-good" : "text-bad"}`}>{msg.text}</span> : null}
      </div>
    </form>
  );
}
