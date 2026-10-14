import { PageHeader } from "@/components/app/shell";
import { backend } from "@/lib/backend";
import { requireUser } from "@/lib/session";
import type { ApiKey } from "@/lib/types";
import { KeysManager } from "./keys-manager";

export const metadata = { title: "API keys" };

export default async function KeysPage() {
  const user = await requireUser("/dashboard/keys");
  const { data } = await backend<{ data: ApiKey[] }>("/api/keys", { userId: user.id });
  return (
    <>
      <PageHeader title="API keys" description="Keys authenticate your requests. Keep them secret — treat them like passwords." />
      <KeysManager keys={data} />
    </>
  );
}
