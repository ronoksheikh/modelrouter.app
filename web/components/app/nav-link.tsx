"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

export function NavLink({ href, children, exact }: { href: string; children: ReactNode; exact?: boolean }) {
  const path = usePathname();
  const active = exact ? path === href : path === href || path.startsWith(href + "/");
  return (
    <Link
      href={href}
      aria-current={active ? "page" : undefined}
      className={cn(
        "flex shrink-0 items-center gap-2.5 rounded-lg px-3 py-2 text-[14px] transition",
        active ? "bg-surface font-medium text-fg shadow-soft ring-1 ring-border" : "text-muted hover:bg-bg-soft hover:text-fg",
      )}
    >
      {children}
    </Link>
  );
}
