import { PageHeader } from "@/components/app/shell";
import { Card } from "@/components/ui/primitives";
import { backend } from "@/lib/backend";
import { requireAdmin } from "@/lib/session";
import type { Provider, Settings } from "@/lib/types";
import { ModelForm } from "../model-form";

export const metadata = { title: "New model" };

export default async function NewModel({ searchParams }: { searchParams: Promise<{ upstream?: string; provider?: string }> }) {
  const admin = await requireAdmin();
  const sp = await searchParams;
  const [providers, settings] = await Promise.all([
    backend<{ data: Provider[] }>("/api/admin/providers", { userId: admin.id }),
    backend<Settings>("/api/admin/settings", { userId: admin.id }),
  ]);
  const prefill = sp.upstream
    ? ({ upstream_id: sp.upstream, provider_id: Number(sp.provider) || providers.data[0]?.id, pricing_mode: "manual", enabled: true } as never)
    : undefined;
  return (
    <>
      <PageHeader title="Custom model" description="Your own model ID and price, served by any upstream model. Great for free tiers and branded models." />
      <Card className="p-6 sm:p-8">
        <ModelForm providers={providers.data} defaultMarkupBps={settings.default_markup_bps} model={prefill} />
      </Card>
    </>
  );
}
