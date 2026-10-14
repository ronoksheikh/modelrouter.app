import Link from "next/link";
import { SiteFooter } from "@/components/site/footer";
import { SiteHeader } from "@/components/site/header";
import { ButtonLink } from "@/components/ui/button";

export default function NotFound() {
  return (
    <>
      <SiteHeader />
      <main id="main" className="container-page flex min-h-[60vh] flex-col items-center justify-center py-24 text-center">
        <p className="font-mono text-sm text-accent">404</p>
        <h1 className="mt-3 text-4xl font-semibold tracking-[-0.03em]">This page took a different route.</h1>
        <p className="mt-3 text-muted">The page you are looking for doesn&apos;t exist or has moved.</p>
        <div className="mt-8 flex gap-3">
          <ButtonLink href="/">Go home</ButtonLink>
          <ButtonLink href="/models" variant="secondary">Browse models</ButtonLink>
        </div>
        <Link href="/docs" className="mt-6 text-sm text-muted hover:text-fg">Or read the docs →</Link>
      </main>
      <SiteFooter />
    </>
  );
}
