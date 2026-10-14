import { SignJWT } from "jose";

const BASE = (process.env.BACKEND_URL || process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8080").replace(/\/$/, "");

export class BackendError extends Error {
  constructor(public status: number, message: string) {
    super(message);
  }
}

function jwtSecret(): Uint8Array {
  const s = process.env.BACKEND_JWT_SECRET;
  if (!s) throw new Error("BACKEND_JWT_SECRET is not set");
  return new TextEncoder().encode(s);
}

/** Mints a 60-second token the Go API uses to identify the signed-in user. */
export async function userToken(userId: string): Promise<string> {
  return new SignJWT({ aud: "modelrouter-api" })
    .setProtectedHeader({ alg: "HS256", typ: "JWT" })
    .setSubject(userId)
    .setIssuedAt()
    .setExpirationTime("60s")
    .sign(jwtSecret());
}

type Options = {
  method?: string;
  body?: unknown;
  userId?: string;
  internal?: boolean;
  revalidate?: number | false;
  tags?: string[];
};

/** Server-side call to the Go backend. Never import from client components. */
export async function backend<T>(path: string, opts: Options = {}): Promise<T> {
  const headers: Record<string, string> = { Accept: "application/json" };
  if (opts.body !== undefined) headers["Content-Type"] = "application/json";
  if (opts.userId) headers.Authorization = `Bearer ${await userToken(opts.userId)}`;
  if (opts.internal) headers["X-Internal-Secret"] = process.env.BACKEND_INTERNAL_SECRET || "";

  const cacheable = !opts.userId && !opts.internal && (opts.method ?? "GET") === "GET" && opts.revalidate !== undefined;
  const res = await fetch(BASE + path, {
    method: opts.method ?? "GET",
    headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
    ...(cacheable ? { next: { revalidate: opts.revalidate, tags: opts.tags } } : { cache: "no-store" as const }),
  });
  const text = await res.text();
  let data: unknown = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = null;
  }
  if (!res.ok) {
    const msg = (data as { error?: { message?: string } } | null)?.error?.message || `Request failed (${res.status})`;
    throw new BackendError(res.status, msg);
  }
  return data as T;
}
