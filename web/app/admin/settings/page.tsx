import { PageHeader } from "@/components/app/shell";
import { Card } from "@/components/ui/primitives";
import { backend } from "@/lib/backend";
import { requireAdmin } from "@/lib/session";
import type { Settings } from "@/lib/types";
import { SettingsForm } from "./settings-form";

export const metadata = { title: "Settings" };

export default async function AdminSettings() {
  const admin = await requireAdmin();
  const s = await backend<Settings>("/api/admin/settings", { userId: admin.id });
  return (
    <>
      <PageHeader title="Platform settings" description="Changes apply to every server within seconds." />
      <Card className="p-6 sm:p-8">
        <SettingsForm settings={s} />
      </Card>
    </>
  );
}
