"use server";

import { revalidatePath } from "next/cache";
import { auth } from "@/auth";
import { backend, BackendError } from "@/lib/backend";

export type AdminResult<T = unknown> = { ok: true; data?: T } | { ok: false; error: string };

async function adminCall<T>(path: string, method: string, body?: unknown, revalidate?: string[]): Promise<AdminResult<T>> {
  const s = await auth();
  if (!s?.user?.id || s.user.role !== "admin") return { ok: false, error: "Admin access required." };
  try {
    const data = await backend<T>(path, { method, body, userId: s.user.id });
    revalidate?.forEach((p) => revalidatePath(p, "layout"));
    return { ok: true, data };
  } catch (e) {
    return { ok: false, error: e instanceof BackendError ? e.message : "Request failed." };
  }
}

export async function adminUpdateUser(id: string, patch: { role?: string; banned?: boolean }) {
  return adminCall(`/api/admin/users/${id}`, "PATCH", patch, ["/admin/users"]);
}
export async function adminCreditUser(id: string, amountUSD: number, note: string) {
  return adminCall(`/api/admin/users/${id}/credit`, "POST", { amount_usd: amountUSD, note }, ["/admin/users"]);
}
export async function adminUpdateKey(id: string, patch: { disabled?: boolean }) {
  return adminCall(`/api/admin/keys/${id}`, "PATCH", patch, ["/admin/users"]);
}
export async function adminDeleteKey(id: string) {
  return adminCall(`/api/admin/keys/${id}`, "DELETE", undefined, ["/admin/users"]);
}

export type ModelInput = {
  slug: string;
  name: string;
  author: string;
  description: string;
  provider_id: number;
  upstream_id: string;
  fallback_upstream: string[];
  context_length: number;
  max_output_tokens: number;
  pricing_mode: "auto" | "manual";
  markup_bps: number | null;
  input_price_per_m: string;
  output_price_per_m: string;
  request_fee: string | null;
  modalities: string[];
  supports_tools: boolean;
  enabled: boolean;
  featured: boolean;
  sort_order: number;
};

export async function adminSaveModel(id: number | null, input: ModelInput) {
  return id
    ? adminCall<{ id: number }>(`/api/admin/models/${id}`, "PUT", input, ["/admin/models", "/models", "/"])
    : adminCall<{ id: number }>("/api/admin/models", "POST", input, ["/admin/models", "/models", "/"]);
}
export async function adminDeleteModel(id: number) {
  return adminCall(`/api/admin/models/${id}`, "DELETE", undefined, ["/admin/models", "/models", "/"]);
}
export async function adminBulkModels(ids: number[], enabled: boolean) {
  return adminCall("/api/admin/models/bulk", "POST", { ids, enabled }, ["/admin/models", "/models", "/"]);
}
export async function adminReprice() {
  return adminCall<{ repriced: number }>("/api/admin/models/reprice", "POST", undefined, ["/admin/models", "/models"]);
}
export async function adminPublish(providerId: number, upstreamIds: string[], markupBps: number | null) {
  return adminCall<{ published: number }>(
    "/api/admin/upstream-models/publish",
    "POST",
    { provider_id: providerId, upstream_ids: upstreamIds, markup_bps: markupBps },
    ["/admin/catalog", "/admin/models", "/models", "/"],
  );
}

export type ProviderInput = {
  name?: string;
  kind?: "openai" | "anthropic";
  base_url?: string;
  api_key?: string;
  is_openrouter?: boolean;
  enabled?: boolean;
};
export async function adminSaveProvider(id: number | null, input: ProviderInput) {
  return id
    ? adminCall(`/api/admin/providers/${id}`, "PATCH", input, ["/admin/providers"])
    : adminCall("/api/admin/providers", "POST", input, ["/admin/providers"]);
}
export async function adminDeleteProvider(id: number) {
  return adminCall(`/api/admin/providers/${id}`, "DELETE", undefined, ["/admin/providers"]);
}
export async function adminSyncProvider(id: number) {
  return adminCall<{ fetched: number; added: string[]; published: number; repriced: number }>(
    `/api/admin/providers/${id}/sync`, "POST", undefined, ["/admin/providers", "/admin/catalog", "/admin/models", "/models"],
  );
}
export async function adminSaveSettings(settings: Record<string, unknown>) {
  return adminCall("/api/admin/settings", "PUT", settings, ["/admin/settings", "/admin/models", "/pricing", "/models"]);
}
