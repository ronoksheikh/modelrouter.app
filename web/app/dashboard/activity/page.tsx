import Link from "next/link";
import { PageHeader } from "@/components/app/shell";
import { UsageTable } from "@/components/app/usage-table";
import { buttonClass } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/primitives";
import { backend } from "@/lib/backend";
import { requireUser } from "@/lib/session";
import type { UsageRecord } from "@/lib/types";

export const metadata = { title: "Activity" };

export default async function ActivityPage({ searchParams }: { searchParams: Promise<{ before?: string }> }) {
  const user = await requireUser("/dashboard/activity");
  const { before } = await searchParams;
  const q = new URLSearchParams({ limit: "50" });
  if (before && /^\d+$/.test(before)) q.set("before", before);
  const res = await backend<{ data: UsageRecord[]; next_before: number | null }>(`/api/activity?${q}`, { userId: user.id });
  return (
    <>
      <PageHeader title="Activity" description="Every request made with your keys. Hover a row to see its request ID." />
      {res.data.length === 0 ? (
        <EmptyState title={before ? "No older requests" : "No requests yet"}>Requests appear here a second after they finish.</EmptyState>
      ) : (
        <UsageTable rows={res.data} />
      )}
      <div className="mt-4 flex justify-end gap-2">
        {before ? <Link href="/dashboard/activity" className={buttonClass("secondary", "sm")}>Newest</Link> : null}
        {res.data.length === 50 && res.next_before ? (
          <Link href={`/dashboard/activity?before=${res.next_before}`} className={buttonClass("secondary", "sm")}>Older</Link>
        ) : null}
      </div>
    </>
  );
}
