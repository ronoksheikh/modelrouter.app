"use client";

import { Menu, X } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";
import { nav } from "@/lib/site";

export function MobileNav({ signedIn }: { signedIn?: boolean }) {
  const [open, setOpen] = useState(false);
  useEffect(() => {
    document.body.style.overflow = open ? "hidden" : "";
    return () => {
      document.body.style.overflow = "";
    };
  }, [open]);
  return (
    <div className="md:hidden">
      <button
        type="button"
        aria-label={open ? "Close menu" : "Open menu"}
        aria-expanded={open}
        onClick={() => setOpen(!open)}
        className="inline-flex size-9 items-center justify-center rounded-xl text-fg-2 hover:bg-bg-soft"
      >
        {open ? <X className="size-5" /> : <Menu className="size-5" />}
      </button>
      {open ? (
        <div className="fixed inset-x-0 top-16 bottom-0 z-50 animate-rise bg-bg px-4 pt-4">
          <nav className="flex flex-col" aria-label="Mobile">
            {[...nav, signedIn ? { href: "/dashboard", label: "Dashboard" } : { href: "/login", label: "Sign in" }].map((n) => (
              <Link key={n.href} href={n.href} onClick={() => setOpen(false)} className="border-b border-border py-4 text-lg font-medium">
                {n.label}
              </Link>
            ))}
          </nav>
          {!signedIn ? (
            <Link
              href="/login?next=/dashboard/keys"
              onClick={() => setOpen(false)}
              className="mt-6 flex h-12 items-center justify-center rounded-xl bg-accent font-medium text-accent-fg"
            >
              Get your API key
            </Link>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
