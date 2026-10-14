import Link from "next/link";
import { auth } from "@/auth";
import { ButtonLink } from "@/components/ui/button";
import { Logo } from "@/components/ui/logo";
import { ThemeToggle } from "@/components/ui/theme-toggle";
import { nav } from "@/lib/site";
import { MobileNav } from "./mobile-nav";

/** Static header used on fully static pages (no session lookup). */
export function SiteHeader({ signedIn }: { signedIn?: boolean }) {
  return (
    <header className="sticky top-0 z-40 border-b border-transparent bg-bg/70 backdrop-blur-xl [@supports(backdrop-filter:blur(0))]:bg-bg/60">
      <div className="container-page flex h-16 items-center justify-between gap-4">
        <div className="flex items-center gap-8">
          <Link href="/" aria-label="ModelRouter home" className="rounded-lg">
            <Logo />
          </Link>
          <nav aria-label="Main" className="hidden items-center gap-1 md:flex">
            {nav.map((n) => (
              <Link key={n.href} href={n.href} className="rounded-lg px-3 py-1.5 text-sm text-muted transition hover:bg-bg-soft hover:text-fg">
                {n.label}
              </Link>
            ))}
          </nav>
        </div>
        <div className="flex items-center gap-1.5">
          <ThemeToggle />
          {signedIn ? (
            <ButtonLink href="/dashboard" size="sm" className="hidden sm:inline-flex">
              Dashboard
            </ButtonLink>
          ) : (
            <>
              <Link href="/login" className="hidden rounded-lg px-3 py-1.5 text-sm text-muted transition hover:text-fg sm:block">
                Sign in
              </Link>
              <ButtonLink href="/login?next=/dashboard/keys" size="sm" className="hidden sm:inline-flex">
                Get API key
              </ButtonLink>
            </>
          )}
          <MobileNav signedIn={signedIn} />
        </div>
      </div>
    </header>
  );
}

/** Header that reflects the session (for dynamic pages). */
export async function SessionHeader() {
  const session = await auth();
  return <SiteHeader signedIn={Boolean(session?.user)} />;
}
