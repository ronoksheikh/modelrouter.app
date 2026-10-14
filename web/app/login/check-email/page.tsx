import { MailCheck } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { Logo } from "@/components/ui/logo";

export const metadata: Metadata = { title: "Check your email", robots: { index: false } };

export default function CheckEmail() {
  return (
    <div className="flex min-h-dvh flex-col">
      <header className="container-page flex h-16 items-center">
        <Link href="/" aria-label="Home"><Logo /></Link>
      </header>
      <main id="main" className="flex flex-1 items-center justify-center px-4 pb-24">
        <div className="max-w-sm animate-rise text-center">
          <div className="mx-auto flex size-14 items-center justify-center rounded-2xl bg-accent-soft text-accent">
            <MailCheck className="size-6" />
          </div>
          <h1 className="mt-6 text-2xl font-semibold tracking-tight">Check your inbox</h1>
          <p className="mt-2 text-sm leading-6 text-muted">We sent you a sign-in link. It expires in 24 hours. You can close this tab.</p>
          <Link href="/login" className="mt-8 inline-block text-sm text-accent hover:underline">Use a different email</Link>
        </div>
      </main>
    </div>
  );
}
