import { Boxes, CreditCard, Gauge, History, Library, ScrollText, Server, Settings, Users, ArrowLeft } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { AppShell, type ShellNavItem } from "@/components/app/shell";
import { requireAdmin } from "@/lib/session";

export const metadata: Metadata = { title: { default: "Admin", template: "%s · Admin" }, robots: { index: false, follow: false } };

export default async function AdminLayout({ children }: { children: React.ReactNode }) {
  const user = await requireAdmin();
  const nav: ShellNavItem[] = [
    { href: "/admin", label: "Overview", icon: <Gauge />, exact: true },
    { href: "/admin/users", label: "Users", icon: <Users /> },
    { href: "/admin/models", label: "Models", icon: <Boxes /> },
    { href: "/admin/catalog", label: "Upstream catalog", icon: <Library /> },
    { href: "/admin/providers", label: "Providers", icon: <Server /> },
    { href: "/admin/payments", label: "Payments", icon: <CreditCard /> },
    { href: "/admin/usage", label: "Request logs", icon: <ScrollText /> },
    { href: "/admin/audit", label: "Audit log", icon: <History /> },
    { href: "/admin/settings", label: "Settings", icon: <Settings /> },
  ];
  return (
    <AppShell
      nav={nav}
      user={user}
      badge={<span className="rounded-md bg-accent-soft px-1.5 py-0.5 text-[10.5px] font-semibold uppercase tracking-wider text-accent-ink">Admin</span>}
      footer={
        <Link href="/dashboard" className="mb-2 flex items-center gap-2 rounded-lg px-3 py-2 text-[13px] text-muted hover:bg-bg-soft hover:text-fg">
          <ArrowLeft className="size-3.5" /> Back to dashboard
        </Link>
      }
    >
      {children}
    </AppShell>
  );
}
