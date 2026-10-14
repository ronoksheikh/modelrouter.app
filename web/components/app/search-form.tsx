import { Search } from "lucide-react";

export function SearchForm({ defaultValue, placeholder, children }: { defaultValue?: string; placeholder: string; children?: React.ReactNode }) {
  return (
    <form className="mb-4 flex flex-wrap gap-2">
      <div className="relative min-w-[220px] flex-1">
        <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-subtle" />
        <input
          name="q"
          defaultValue={defaultValue}
          placeholder={placeholder}
          className="h-10 w-full rounded-xl border border-border bg-surface pl-9 pr-3 text-sm outline-none focus:border-accent focus:ring-4 focus:ring-[var(--ring)]"
        />
      </div>
      {children}
      <button className="h-10 rounded-xl border border-border bg-surface px-4 text-sm font-medium hover:border-border-strong">Search</button>
    </form>
  );
}

export function Pager({ base, offset, limit, total }: { base: string; offset: number; limit: number; total: number }) {
  const sep = base.includes("?") ? "&" : "?";
  return (
    <div className="mt-4 flex items-center justify-between text-sm text-muted">
      <span>
        {total === 0 ? 0 : offset + 1}–{Math.min(offset + limit, total)} of {total}
      </span>
      <div className="flex gap-2">
        {offset > 0 ? <a className="rounded-lg border border-border px-3 py-1.5 hover:border-border-strong" href={`${base}${sep}offset=${Math.max(0, offset - limit)}`}>Previous</a> : null}
        {offset + limit < total ? <a className="rounded-lg border border-border px-3 py-1.5 hover:border-border-strong" href={`${base}${sep}offset=${offset + limit}`}>Next</a> : null}
      </div>
    </div>
  );
}
