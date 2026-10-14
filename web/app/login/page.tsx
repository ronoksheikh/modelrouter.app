import type { Metadata } from "next";
import Link from "next/link";
import { redirect } from "next/navigation";
import { auth, enabledProviders, signIn } from "@/auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/primitives";
import { Logo } from "@/components/ui/logo";

export const metadata: Metadata = { title: "Sign in", robots: { index: false } };

function safeNext(n: string | undefined) {
  return n && n.startsWith("/") && !n.startsWith("//") ? n : "/dashboard";
}

const errors: Record<string, string> = {
  OAuthAccountNotLinked: "This email is linked to another sign-in method. Use the one you signed up with.",
  Verification: "That sign-in link has expired or was already used. Request a new one.",
  AccessDenied: "Access denied.",
  Configuration: "Sign-in is temporarily unavailable. Please try again shortly.",
};

function GoogleIcon() {
  return (
    <svg viewBox="0 0 24 24" className="size-[18px]" aria-hidden>
      <path fill="#4285F4" d="M23.5 12.3c0-.8-.1-1.6-.2-2.3H12v4.4h6.5a5.6 5.6 0 0 1-2.4 3.6v3h3.9c2.3-2.1 3.5-5.2 3.5-8.7Z" />
      <path fill="#34A853" d="M12 24c3.2 0 6-1.1 8-2.9l-3.9-3c-1.1.7-2.5 1.2-4.1 1.2-3.1 0-5.8-2.1-6.7-5H1.3v3.1A12 12 0 0 0 12 24Z" />
      <path fill="#FBBC05" d="M5.3 14.3a7.2 7.2 0 0 1 0-4.6V6.6h-4a12 12 0 0 0 0 10.8l4-3.1Z" />
      <path fill="#EA4335" d="M12 4.8c1.8 0 3.3.6 4.6 1.8l3.4-3.4A12 12 0 0 0 1.3 6.6l4 3.1c.9-2.9 3.6-4.9 6.7-4.9Z" />
    </svg>
  );
}

function GitHubIcon() {
  return (
    <svg viewBox="0 0 24 24" className="size-[18px]" fill="currentColor" aria-hidden>
      <path d="M12 .5a11.5 11.5 0 0 0-3.6 22.4c.6.1.8-.3.8-.6v-2c-3.2.7-3.9-1.5-3.9-1.5-.5-1.3-1.3-1.7-1.3-1.7-1-.7.1-.7.1-.7 1.2.1 1.8 1.2 1.8 1.2 1 1.8 2.8 1.3 3.5 1 .1-.8.4-1.3.7-1.6-2.6-.3-5.3-1.3-5.3-5.7 0-1.3.5-2.3 1.2-3.1-.1-.3-.5-1.5.1-3.1 0 0 1-.3 3.2 1.2a11 11 0 0 1 5.8 0c2.2-1.5 3.2-1.2 3.2-1.2.6 1.6.2 2.8.1 3.1.8.8 1.2 1.9 1.2 3.1 0 4.4-2.7 5.4-5.3 5.7.4.4.8 1.1.8 2.2v3.2c0 .3.2.7.8.6A11.5 11.5 0 0 0 12 .5Z" />
    </svg>
  );
}

export default async function LoginPage({ searchParams }: { searchParams: Promise<{ next?: string; error?: string }> }) {
  const sp = await searchParams;
  const next = safeNext(sp.next);
  const session = await auth();
  if (session?.user) redirect(next);
  const none = !enabledProviders.google && !enabledProviders.github && !enabledProviders.email;

  return (
    <div className="relative flex min-h-dvh flex-col">
      <div aria-hidden className="pointer-events-none absolute inset-0 -z-10">
        <div className="bg-grid absolute inset-0" />
        <div className="absolute left-1/2 top-0 h-96 w-[700px] -translate-x-1/2 rounded-full bg-[radial-gradient(closest-side,var(--glacier),transparent)] opacity-60 blur-2xl" />
      </div>
      <header className="container-page flex h-16 items-center">
        <Link href="/" aria-label="Home"><Logo /></Link>
      </header>
      <main id="main" className="flex flex-1 items-center justify-center px-4 pb-24">
        <div className="w-full max-w-[380px] animate-rise">
          <h1 className="text-center text-[28px] font-semibold tracking-[-0.03em]">Welcome to ModelRouter</h1>
          <p className="mt-2 text-center text-sm text-muted">Sign in or create an account in seconds.</p>

          {sp.error ? (
            <p role="alert" className="mt-6 rounded-xl border border-bad/20 bg-bad/5 px-4 py-3 text-sm text-bad">
              {errors[sp.error] ?? "Something went wrong. Please try again."}
            </p>
          ) : null}

          <div className="mt-8 space-y-2.5">
            {enabledProviders.google ? (
              <form action={async () => { "use server"; await signIn("google", { redirectTo: next }); }}>
                <Button variant="secondary" size="lg" className="w-full"><GoogleIcon /> Continue with Google</Button>
              </form>
            ) : null}
            {enabledProviders.github ? (
              <form action={async () => { "use server"; await signIn("github", { redirectTo: next }); }}>
                <Button variant="secondary" size="lg" className="w-full"><GitHubIcon /> Continue with GitHub</Button>
              </form>
            ) : null}
          </div>

          {enabledProviders.email ? (
            <>
              {enabledProviders.google || enabledProviders.github ? (
                <div className="my-6 flex items-center gap-3 text-xs text-subtle">
                  <span className="h-px flex-1 bg-border" /> or <span className="h-px flex-1 bg-border" />
                </div>
              ) : null}
              <form
                className="space-y-2.5"
                action={async (fd: FormData) => {
                  "use server";
                  await signIn("resend", { email: String(fd.get("email") ?? ""), redirectTo: next });
                }}
              >
                <label htmlFor="email" className="sr-only">Email</label>
                <Input id="email" name="email" type="email" required autoComplete="email" placeholder="you@company.com" className="h-12" />
                <Button size="lg" className="w-full">Continue with email</Button>
              </form>
            </>
          ) : null}

          {none ? (
            <p className="mt-6 rounded-xl border border-warn/30 bg-warn/5 px-4 py-3 text-sm text-fg-2">
              No sign-in methods are configured. Set <code>AUTH_GOOGLE_ID</code>, <code>AUTH_GITHUB_ID</code> or <code>AUTH_RESEND_KEY</code>.
            </p>
          ) : null}

          <p className="mt-8 text-center text-xs leading-5 text-subtle">
            By continuing you agree to our <Link href="/terms" className="underline underline-offset-2 hover:text-fg">Terms</Link> and{" "}
            <Link href="/privacy" className="underline underline-offset-2 hover:text-fg">Privacy Policy</Link>.
          </p>
        </div>
      </main>
    </div>
  );
}
