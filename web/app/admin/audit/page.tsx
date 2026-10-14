import { PageHeader } from "@/components/app/shell";
import { backend } from "@/lib/backend";
import { formatDateTime } from "@/lib/format";
import { requireAdmin } from "@/lib/session";

export const metadata = { title: "Audit log" };

type Log = { id: number; actor_email: string; action: string; target: string; detail: Record<string, unknown>; created_at: string };

export default async function Audit() {
  const admin = await requireAdmin();
  const res = await backend<{ data: Log[] }>("/api/admin/audit?limit=200", { userId: admin.id });
  return (
    <>
      <PageHeader title="Audit log" description="Every administrative change, who made it and when." />
      <div className="overflow-hidden rounded-2xl border border-border bg-surface shadow-soft">
        <ul className="divide-y divide-border">
          {res.data.map((l) => (
            <li key={l.id} className="grid gap-1 px-5 py-3 text-sm sm:grid-cols-[160px_200px_1fr]">
              <span className="text-muted">{formatDateTime(l.created_at)}</span>
              <span className="truncate">{l.actor_email || "system"}</span>
              <span className="min-w-0">
                <span className="font-mono text-[12.5px] text-accent">{l.action}</span>
                {l.target ? <span className="text-fg-2"> · {l.target}</span> : null}
                {Object.keys(l.detail ?? {}).length ? (
                  <span className="mt-0.5 block truncate font-mono text-[11.5px] text-subtle">{JSON.stringify(l.detail)}</span>
                ) : null}
              </span>
            </li>
          ))}
        </ul>
        {res.data.length === 0 ? <p className="px-5 py-12 text-center text-sm text-muted">No admin activity yet.</p> : null}
      </div>
    </>
  );
}
