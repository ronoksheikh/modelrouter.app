import { PageHeader } from "@/components/app/shell";
import { backend } from "@/lib/backend";
import { requireAdmin } from "@/lib/session";
import type { Provider } from "@/lib/types";
import { ProvidersManager } from "./providers-manager";

export const metadata = { title: "Providers" };

export default async function Providers() {
  const admin = await requireAdmin();
  const res = await backend<{ data: Provider[] }>("/api/admin/providers", { userId: admin.id });
  return (
    <>
      <PageHeader title="Providers" description="Upstream APIs that serve your models. Keys are encrypted at rest and never shown again." />
      <ProvidersManager providers={res.data} />
    </>
  );
}
