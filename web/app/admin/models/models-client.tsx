"use client";

import { useTransition } from "react";
import { Button } from "@/components/ui/button";
import { adminBulkModels, adminReprice } from "../actions";

export function ModelToggle({ id, enabled }: { id: number; enabled: boolean }) {
  const [pending, start] = useTransition();
  return (
    <button
      role="switch"
      aria-checked={enabled}
      aria-label={enabled ? "Disable model" : "Enable model"}
      disabled={pending}
      onClick={() => start(async () => void (await adminBulkModels([id], !enabled)))}
      className={`relative h-5 w-9 rounded-full transition ${enabled ? "bg-accent" : "bg-border-strong"} ${pending ? "opacity-50" : ""}`}
    >
      <span className={`absolute top-0.5 size-4 rounded-full bg-white shadow transition-all ${enabled ? "left-[18px]" : "left-0.5"}`} />
    </button>
  );
}

export function ModelsToolbar({ ids }: { ids: number[] }) {
  const [pending, start] = useTransition();
  return (
    <>
      <Button
        variant="secondary"
        size="sm"
        disabled={pending}
        onClick={() =>
          start(async () => {
            const r = await adminReprice();
            alert(r.ok ? `Repriced ${(r.data as { repriced: number }).repriced} models from upstream.` : r.error);
          })
        }
      >
        Reprice auto models
      </Button>
      <Button
        variant="secondary"
        size="sm"
        disabled={pending || ids.length === 0}
        onClick={() => confirm(`Disable all ${ids.length} listed models?`) && start(async () => void (await adminBulkModels(ids, false)))}
      >
        Disable listed
      </Button>
    </>
  );
}
