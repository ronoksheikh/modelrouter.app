import { ArrowUpRight, Image as ImageIcon, Wrench } from "lucide-react";
import Link from "next/link";
import { modelHref } from "@/lib/links";
import { authorName, formatPriceString, formatTokens } from "@/lib/format";
import type { PublicModel } from "@/lib/types";
import { Badge } from "@/components/ui/primitives";

export function ModelRow({ m }: { m: PublicModel }) {
  return (
    <Link
      href={modelHref(m.id)}
      className="group grid grid-cols-[1fr_auto] items-center gap-x-6 gap-y-1 border-b border-border px-1 py-4 transition last:border-0 hover:bg-bg-soft/60 sm:grid-cols-[minmax(0,1fr)_110px_110px_90px_20px] sm:px-3"
    >
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <span className="truncate font-medium">{m.name}</span>
          {m.free ? <Badge tone="good">Free</Badge> : null}
          {m.supports_tools ? <Wrench className="size-3.5 shrink-0 text-subtle" aria-label="Supports tools" /> : null}
          {m.modalities?.includes("image") ? <ImageIcon className="size-3.5 shrink-0 text-subtle" aria-label="Accepts images" /> : null}
        </div>
        <p className="mt-0.5 truncate font-mono text-[12px] text-subtle">
          {m.id} · {authorName(m.author)}
        </p>
      </div>
      <div className="hidden text-right text-sm tabular-nums sm:block">
        <span className="text-fg">{formatPriceString(m.input_price_per_m)}</span>
        <span className="block text-[11px] text-subtle">input / 1M</span>
      </div>
      <div className="hidden text-right text-sm tabular-nums sm:block">
        <span className="text-fg">{formatPriceString(m.output_price_per_m)}</span>
        <span className="block text-[11px] text-subtle">output / 1M</span>
      </div>
      <div className="text-right text-sm tabular-nums text-muted">{formatTokens(m.context_length)}</div>
      <ArrowUpRight className="hidden size-4 text-subtle transition group-hover:translate-x-0.5 group-hover:-translate-y-0.5 group-hover:text-accent sm:block" />
    </Link>
  );
}
