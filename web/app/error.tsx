"use client";

import { Button } from "@/components/ui/button";

export default function Error({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <main className="flex min-h-[60vh] flex-col items-center justify-center px-4 text-center">
      <h1 className="text-2xl font-semibold tracking-tight">Something went wrong</h1>
      <p className="mt-2 max-w-sm text-sm text-muted">We couldn&apos;t load this page. It&apos;s usually temporary — please try again.</p>
      <Button className="mt-6" onClick={reset}>Try again</Button>
    </main>
  );
}
