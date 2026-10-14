import type { Metadata } from "next";

export const revalidate = 30;
export const metadata: Metadata = { title: "Status", description: "Live status of the ModelRouter API." };

async function check() {
  const base = (process.env.BACKEND_URL || process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8080").replace(/\/$/, "");
  const started = Date.now();
  try {
    const res = await fetch(base + "/readyz", { next: { revalidate: 30 } });
    const body = (await res.json()) as { models?: number };
    return { ok: res.ok, ms: Date.now() - started, models: body.models ?? 0 };
  } catch {
    return { ok: false, ms: 0, models: 0 };
  }
}

export default async function Status() {
  const s = await check();
  return (
    <div className="container-page py-20">
      <div className="mx-auto max-w-xl rounded-2xl border border-border bg-surface p-8 shadow-soft">
        <div className="flex items-center gap-3">
          <span className={`relative flex size-3`}>
            <span className={`absolute inline-flex h-full w-full animate-ping rounded-full opacity-60 ${s.ok ? "bg-good" : "bg-bad"}`} />
            <span className={`relative inline-flex size-3 rounded-full ${s.ok ? "bg-good" : "bg-bad"}`} />
          </span>
          <h1 className="text-xl font-semibold">{s.ok ? "All systems operational" : "Service disruption"}</h1>
        </div>
        <dl className="mt-6 grid grid-cols-2 gap-4 text-sm">
          <div>
            <dt className="text-muted">API</dt>
            <dd className="mt-1 font-medium">{s.ok ? "Operational" : "Unavailable"}</dd>
          </div>
          <div>
            <dt className="text-muted">Models online</dt>
            <dd className="mt-1 font-medium tabular-nums">{s.models}</dd>
          </div>
        </dl>
        <p className="mt-6 text-xs text-subtle">Checked within the last 30 seconds.</p>
      </div>
    </div>
  );
}
