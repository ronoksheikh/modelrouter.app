import Link from "next/link";
import { PageHeader } from "@/components/app/shell";
import { UsageTable } from "@/components/app/usage-table";
import { buttonClass } from "@/components/ui/button";
import { backend } from "@/lib/backend";
import { requireAdmin } from "@/lib/session";
import type { UsageRecord } from "@/lib/types";

export const metadata = { title: "Request logs" };

export default async function Usage({ searchParams }: { searchParams: Promise<{ user_id?: string; model?: string; errors?: string; before?: string }> }) {
  const admin = await requireAdmin();
  const sp = await searchParams;
  const q = new URLSearchParams({ limit: "100" });
  if (sp.user_id) q.set("user_id", sp.user_id);
  if (sp.model) q.set("model", sp.model);
  if (sp.errors === "1") q.set("errors", "1");
  if (sp.before && /^\d+$/.test(sp.before)) q.set("before", sp.before);
  const res = await backend<{ data: UsageRecord[]; next_before: number | null }>(`/api/admin/usage?${q}`, { userId: admin.id });
  const base = new URLSearchParams(q);
  base.delete("before");
  base.delete("limit");
  return (
    <>
      <PageHeader title="Request logs" description="Every request across the platform, newest first. Hover a row for its request ID or error." />
      <form className="mb-4 flex flex-wrap gap-2">
        <input name="user_id" defaultValue={sp.user_id} placeholder="User ID" className="h-10 w-72 rounded-xl border border-border bg-surface px-3 font-mono text-[13px]" />
        <input name="model" defaultValue={sp.model} placeholder="Model slug" className="h-10 w-56 rounded-xl border border-border bg-surface px-3 font-mono text-[13px]" />
        <label className="flex items-center gap-2 px-2 text-sm"><input type="checkbox" name="errors" value="1" defaultChecked={sp.errors === "1"} className="accent-[var(--accent)]" /> Errors only</label>
        <button className="h-10 rounded-xl border border-border bg-surface px-4 text-sm font-medium">Filter</button>
      </form>
      {res.data.length === 0 ? <p className="rounded-2xl border border-dashed border-border-strong py-14 text-center text-sm text-muted">No requests match.</p> : <UsageTable rows={res.data} admin />}
      <div className="mt-4 flex justify-end gap-2">
        {sp.before ? <Link className={buttonClass("secondary", "sm")} href={`/admin/usage?${base}`}>Newest</Link> : null}
        {res.data.length === 100 && res.next_before ? <Link className={buttonClass("secondary", "sm")} href={`/admin/usage?${base}&before=${res.next_before}`}>Older</Link> : null}
      </div>
    </>
  );
}
