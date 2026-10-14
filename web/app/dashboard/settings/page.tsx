import { PageHeader } from "@/components/app/shell";
import { Card } from "@/components/ui/primitives";
import { backend } from "@/lib/backend";
import { formatDate } from "@/lib/format";
import { requireUser } from "@/lib/session";
import type { User } from "@/lib/types";
import { ProfileForm } from "./profile-form";

export const metadata = { title: "Settings" };

export default async function SettingsPage() {
  const user = await requireUser("/dashboard/settings");
  const { user: me } = await backend<{ user: User }>("/api/me", { userId: user.id });
  return (
    <>
      <PageHeader title="Settings" description="Manage your profile and preferences." />
      <div className="grid gap-4 lg:grid-cols-[1.4fr_1fr]">
        <Card className="p-6">
          <h2 className="font-medium">Profile</h2>
          <ProfileForm name={me.name} country={me.country} />
        </Card>
        <Card className="p-6">
          <h2 className="font-medium">Account</h2>
          <dl className="mt-4 space-y-4 text-sm">
            <div>
              <dt className="text-muted">Email</dt>
              <dd className="mt-0.5 font-medium">{me.email}</dd>
            </div>
            <div>
              <dt className="text-muted">Member since</dt>
              <dd className="mt-0.5 font-medium">{formatDate(me.created_at)}</dd>
            </div>
            <div>
              <dt className="text-muted">Account ID</dt>
              <dd className="mt-0.5 font-mono text-xs text-fg-2">{me.id}</dd>
            </div>
          </dl>
          <p className="mt-6 border-t border-border pt-5 text-xs leading-5 text-muted">
            To delete your account, email <a className="text-accent" href="mailto:support@modelrouter.app">support@modelrouter.app</a> from this address.
          </p>
        </Card>
      </div>
    </>
  );
}
