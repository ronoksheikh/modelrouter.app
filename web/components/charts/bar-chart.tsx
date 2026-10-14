import { cn } from "@/lib/cn";

type Datum = { key: string; label: string; value: number; display: string };

/**
 * Single-series bar chart rendered on the server as HTML — no client JS.
 * Each bar has a hover/focus tooltip; values also appear in an accessible table.
 */
export function BarChart({ data, title, height = 180, className }: { data: Datum[]; title: string; height?: number; className?: string }) {
  const max = Math.max(...data.map((d) => d.value), 0);
  const peak = data.find((d) => d.value === max && max > 0);
  return (
    <figure className={cn("w-full", className)}>
      <figcaption className="sr-only">{title}</figcaption>
      <div className="relative" style={{ height }}>
        {/* Recessive gridlines */}
        <div aria-hidden className="absolute inset-0 flex flex-col justify-between">
          {[0, 1, 2, 3].map((i) => (
            <div key={i} className="border-t border-dashed border-border/80" />
          ))}
        </div>
        {peak ? (
          <span aria-hidden className="absolute -top-0.5 right-0 -translate-y-full pb-1 text-[11px] tabular-nums text-subtle">
            peak {peak.display}
          </span>
        ) : null}
        <div className="relative flex h-full items-end gap-[2px]" role="list">
          {data.map((d) => {
            const h = max > 0 ? Math.max((d.value / max) * 100, d.value > 0 ? 2 : 0) : 0;
            return (
              <div key={d.key} role="listitem" tabIndex={0} aria-label={`${d.label}: ${d.display}`} className="group relative flex h-full flex-1 items-end outline-none">
                <div
                  className="w-full rounded-t-[4px] bg-accent/80 transition-colors group-hover:bg-accent group-focus-visible:bg-accent"
                  style={{ height: `${h}%` }}
                />
                {d.value === 0 ? <div className="absolute bottom-0 h-px w-full bg-border-strong" /> : null}
                <div className="pointer-events-none absolute bottom-full left-1/2 z-10 mb-2 hidden -translate-x-1/2 whitespace-nowrap rounded-lg border border-border bg-surface px-2.5 py-1.5 text-xs shadow-lift group-hover:block group-focus-visible:block">
                  <span className="block text-subtle">{d.label}</span>
                  <span className="font-medium tabular-nums text-fg">{d.display}</span>
                </div>
              </div>
            );
          })}
        </div>
      </div>
      <div aria-hidden className="mt-2 flex justify-between text-[11px] text-subtle">
        <span>{data[0]?.label}</span>
        <span>{data[data.length - 1]?.label}</span>
      </div>
      <table className="sr-only">
        <caption>{title}</caption>
        <tbody>
          {data.map((d) => (
            <tr key={d.key}><th>{d.label}</th><td>{d.display}</td></tr>
          ))}
        </tbody>
      </table>
    </figure>
  );
}
