import { Activity, CreditCard, KeyRound, LayoutGrid, Settings, Shield } from "lucide-react";
import type { Metadata } from "next";
import { AppShell, type ShellNavItem } from "@/components/app/shell";
import { requireUser } from "@/lib/session";

export const metadata: Metadata = { title: "Dashboard", robots: { index: false, follow: false } };

export default async function DashboardLayout({ children }: { children: React.ReactNode }) {
  const user = await requireUser();
  const nav: ShellNavItem[] = [
    { href: "/dashboard", label: "Overview", icon: <LayoutGrid />, exact: true },
    { href: "/dashboard/keys", label: "API keys", icon: <KeyRound /> },
    { href: "/dashboard/activity", label: "Activity", icon: <Activity /> },
    { href: "/dashboard/billing", label: "Billing", icon: <CreditCard /> },
    { href: "/dashboard/settings", label: "Settings", icon: <Settings /> },
  ];
  if (user.role === "admin") nav.push({ href: "/admin", label: "Admin", icon: <Shield /> });
  return (
    <AppShell nav={nav} user={user}>
      {children}
    </AppShell>
  );
}
