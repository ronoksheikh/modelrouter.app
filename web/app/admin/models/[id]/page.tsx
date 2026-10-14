import { ArrowLeft } from "lucide-react";
import Link from "next/link";
import { notFound } from "next/navigation";
import { PageHeader } from "@/components/app/shell";
import { Card } from "@/components/ui/primitives";
import { backend, BackendError } from "@/lib/backend";
import { requireAdmin } from "@/lib/session";
import type { AdminModel, Provider, Settings } from "@/lib/types";
import { ModelForm } from "../model-form";

export const metadata = { title: "Edit model" };

export default async function EditModel({ params }: { params: Promise<{ id: string }> }) {
  const admin = await requireAdmin();
  const { id } = await params;
  if (!/^\d+$/.test(id)) notFound();
  try {
    const [model, providers, settings] = await Promise.all([
      backend<AdminModel>(`/api/admin/models/${id}`, { userId: admin.id }),
      backend<{ data: Provider[] }>("/api/admin/providers", { userId: admin.id }),
      backend<Settings>("/api/admin/settings", { userId: admin.id }),
    ]);
    return (
      <>
        <Link href="/admin/models" className="mb-4 inline-flex items-center gap-1.5 text-sm text-muted hover:text-fg"><ArrowLeft className="size-4" /> Models</Link>
        <PageHeader title={model.name} description={<span className="font-mono">{model.slug}</span>} />
        <Card className="p-6 sm:p-8">
          <ModelForm model={model} providers={providers.data} defaultMarkupBps={settings.default_markup_bps} />
        </Card>
      </>
    );
  } catch (e) {
    if (e instanceof BackendError && e.status === 404) notFound();
    throw e;
  }
}
