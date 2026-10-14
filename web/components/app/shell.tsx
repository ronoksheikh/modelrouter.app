import { LogOut } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";
import { signOut } from "@/auth";
import { LogoMark } from "@/components/ui/logo";
import { ThemeToggle } from "@/components/ui/theme-toggle";
import { NavLink } from "./nav-link";

export type ShellNavItem = { href: string; label: string; icon: ReactNode; exact?: boolean };

export function AppShell({
  nav,
  user,
  badge,
  children,
  footer,
}: {
  nav: ShellNavItem[];
  user: { name?: string | null; email?: string | null; image?: string | null };
  badge?: ReactNode;
  footer?: ReactNode;
  children: ReactNode;
}) {
  const initial = (user.name || user.email || "?").charAt(0).toUpperCase();
  return (
    <div className="min-h-dvh lg:grid lg:grid-cols-[248px_1fr]">
      <aside className="sticky top-0 z-30 border-b border-border bg-bg/80 backdrop-blur-xl lg:h-dvh lg:border-b-0 lg:border-r">
        <div className="flex h-14 items-center justify-between px-4 lg:h-16 lg:px-5">
          <Link href="/" className="flex items-center gap-2.5" aria-label="Home">
            <LogoMark className="size-6" />
            <span className="text-[14px] font-semibold tracking-tight">ModelRouter</span>
            {badge}
          </Link>
          <div className="flex items-center gap-1 lg:hidden">
            <ThemeToggle />
          </div>
        </div>
        <nav aria-label="App" className="flex gap-1 overflow-x-auto px-3 pb-3 [scrollbar-width:none] lg:flex-col lg:overflow-visible lg:pb-0">
          {nav.map((n) => (
            <NavLink key={n.href} href={n.href} exact={n.exact}>
              <span className="text-subtle [&_svg]:size-4">{n.icon}</span>
              {n.label}
            </NavLink>
          ))}
        </nav>
        <div className="absolute inset-x-0 bottom-0 hidden p-3 lg:block">
          {footer}
          <div className="flex items-center gap-3 rounded-xl border border-border bg-surface p-2.5 shadow-soft">
            {user.image ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img src={user.image} alt="" className="size-8 rounded-full" referrerPolicy="no-referrer" />
            ) : (
              <span className="flex size-8 items-center justify-center rounded-full bg-accent-soft text-sm font-medium text-accent-ink">{initial}</span>
            )}
            <div className="min-w-0 flex-1">
              <p className="truncate text-[13px] font-medium">{user.name || "Account"}</p>
              <p className="truncate text-[11.5px] text-subtle">{user.email}</p>
            </div>
            <ThemeToggle />
            <form action={async () => { "use server"; await signOut({ redirectTo: "/" }); }}>
              <button aria-label="Sign out" title="Sign out" className="inline-flex size-9 items-center justify-center rounded-xl text-muted transition hover:bg-bg-soft hover:text-fg">
                <LogOut className="size-4" />
              </button>
            </form>
          </div>
        </div>
      </aside>
      <main id="main" className="min-w-0">
        <div className="mx-auto w-full max-w-6xl px-4 py-8 sm:px-6 lg:px-10 lg:py-10">{children}</div>
      </main>
    </div>
  );
}

export function PageHeader({ title, description, actions }: { title: string; description?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="mb-8 flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
      <div>
        <h1 className="text-2xl font-semibold tracking-[-0.02em]">{title}</h1>
        {description ? <p className="mt-1.5 text-sm text-muted">{description}</p> : null}
      </div>
      {actions ? <div className="flex shrink-0 gap-2">{actions}</div> : null}
    </div>
  );
}

export function Stat({ label, value, hint }: { label: string; value: ReactNode; hint?: ReactNode }) {
  return (
    <div className="rounded-2xl border border-border bg-surface p-5 shadow-soft">
      <p className="text-[13px] text-muted">{label}</p>
      <p className="mt-2 text-[26px] font-semibold leading-none tracking-tight tabular-nums">{value}</p>
      {hint ? <p className="mt-2 text-xs text-subtle">{hint}</p> : null}
    </div>
  );
}
