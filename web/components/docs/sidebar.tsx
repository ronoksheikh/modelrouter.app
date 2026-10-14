"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { docsNav } from "@/lib/docs";
import { cn } from "@/lib/cn";

export function DocsSidebar() {
  const path = usePathname();
  return (
    <nav aria-label="Documentation" className="space-y-7">
      {docsNav.map((g) => (
        <div key={g.title}>
          <p className="px-3 text-[12px] font-medium uppercase tracking-[0.08em] text-subtle">{g.title}</p>
          <ul className="mt-2 space-y-0.5">
            {g.items.map((i) => (
              <li key={i.href}>
                <Link
                  href={i.href}
                  className={cn(
                    "block rounded-lg px-3 py-1.5 text-[14px] transition",
                    path === i.href ? "bg-accent-soft font-medium text-accent-ink" : "text-muted hover:bg-bg-soft hover:text-fg",
                  )}
                >
                  {i.title}
                </Link>
              </li>
            ))}
          </ul>
        </div>
      ))}
    </nav>
  );
}
