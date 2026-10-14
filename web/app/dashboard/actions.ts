"use server";

import { revalidatePath } from "next/cache";
import { auth } from "@/auth";
import { backend, BackendError } from "@/lib/backend";
import type { ApiKey } from "@/lib/types";

export type ActionResult<T = undefined> = { ok: true; data?: T } | { ok: false; error: string };

async function uid(): Promise<string> {
  const s = await auth();
  if (!s?.user?.id) throw new BackendError(401, "Please sign in again.");
  return s.user.id;
}

async function run<T>(fn: (userId: string) => Promise<T>, path?: string): Promise<ActionResult<T>> {
  try {
    const data = await fn(await uid());
    if (path) revalidatePath(path);
    return { ok: true, data };
  } catch (e) {
    return { ok: false, error: e instanceof BackendError ? e.message : "Something went wrong. Please try again." };
  }
}

type KeyForm = { name?: string; spendLimitUSD?: number | null; rpmLimit?: number | null };

export async function createKey(input: KeyForm) {
  return run(
    (userId) =>
      backend<{ key: ApiKey; secret: string }>("/api/keys", {
        method: "POST",
        userId,
        body: {
          name: input.name?.trim() || "Default",
          spend_limit_usd: input.spendLimitUSD ?? undefined,
          rpm_limit: input.rpmLimit ?? undefined,
        },
      }),
    "/dashboard/keys",
  );
}

export async function updateKey(id: string, input: KeyForm & { disabled?: boolean }) {
  return run(
    (userId) =>
      backend<{ key: ApiKey }>(`/api/keys/${encodeURIComponent(id)}`, {
        method: "PATCH",
        userId,
        body: {
          name: input.name,
          disabled: input.disabled,
          spend_limit_usd: input.spendLimitUSD ?? undefined,
          clear_limit: input.spendLimitUSD === null,
          rpm_limit: input.rpmLimit ?? undefined,
        },
      }),
    "/dashboard/keys",
  );
}

export async function rotateKey(id: string) {
  return run(
    (userId) => backend<{ key: ApiKey; secret: string }>(`/api/keys/${encodeURIComponent(id)}/rotate`, { method: "POST", userId }),
    "/dashboard/keys",
  );
}

export async function deleteKey(id: string) {
  return run((userId) => backend(`/api/keys/${encodeURIComponent(id)}`, { method: "DELETE", userId }), "/dashboard/keys");
}

export async function startCheckout(method: string, amountCents: number, country: string) {
  return run((userId) =>
    backend<{ payment_id: string; checkout: { redirect_url?: string; client_data?: Record<string, string> } }>("/api/billing/checkout", {
      method: "POST",
      userId,
      body: { method, amount_cents: Math.round(amountCents), country },
    }),
  );
}

export async function updateProfile(input: { name?: string; country?: string }) {
  return run((userId) => backend("/api/me", { method: "PATCH", userId, body: input }), "/dashboard/settings");
}
