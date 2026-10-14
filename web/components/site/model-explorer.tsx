"use client";

import { Search } from "lucide-react";
import { useDeferredValue, useMemo, useState } from "react";
import type { PublicModel } from "@/lib/types";
import { authorName } from "@/lib/format";
import { cn } from "@/lib/cn";
import { ModelRow } from "./model-row";

type Sort = "featured" | "newest" | "price" | "context";

export function ModelExplorer({ models }: { models: PublicModel[] }) {
  const [q, setQ] = useState("");
  const [author, setAuthor] = useState("all");
  const [onlyFree, setOnlyFree] = useState(false);
  const [onlyTools, setOnlyTools] = useState(false);
  const [sort, setSort] = useState<Sort>("featured");
  const query = useDeferredValue(q.trim().toLowerCase());

  const authors = useMemo(() => {
    const counts = new Map<string, number>();
    models.forEach((m) => counts.set(m.author || "modelrouter", (counts.get(m.author || "modelrouter") ?? 0) + 1));
    return [...counts.entries()].sort((a, b) => b[1] - a[1]).slice(0, 12);
  }, [models]);

  const list = useMemo(() => {
    let l = models.filter((m) => {
      if (onlyFree && !m.free) return false;
      if (onlyTools && !m.supports_tools) return false;
      if (author !== "all" && (m.author || "modelrouter") !== author) return false;
      if (query && !`${m.id} ${m.name} ${m.author}`.toLowerCase().includes(query)) return false;
      return true;
    });
    if (sort === "newest") l = [...l].sort((a, b) => b.created_at.localeCompare(a.created_at));
    if (sort === "price") l = [...l].sort((a, b) => Number(a.input_price_per_m) - Number(b.input_price_per_m));
    if (sort === "context") l = [...l].sort((a, b) => b.context_length - a.context_length);
    return l;
  }, [models, query, author, onlyFree, onlyTools, sort]);

  const chip = (active: boolean) =>
    cn(
      "h-8 shrink-0 rounded-full border px-3 text-[13px] transition",
      active ? "border-accent/30 bg-accent-soft text-accent-ink" : "border-border bg-surface text-muted hover:text-fg",
    );

  return (
    <div>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute left-3.5 top-1/2 size-4 -translate-y-1/2 text-subtle" />
          <input
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder={`Search ${models.length} models…`}
            aria-label="Search models"
            className="h-11 w-full rounded-xl border border-border bg-surface pl-10 pr-3 text-sm outline-none transition placeholder:text-subtle focus:border-accent focus:ring-4 focus:ring-[var(--ring)]"
          />
        </div>
        <select
          value={sort}
          onChange={(e) => setSort(e.target.value as Sort)}
          aria-label="Sort models"
          className="h-11 rounded-xl border border-border bg-surface px-3 text-sm outline-none focus:border-accent"
        >
          <option value="featured">Featured</option>
          <option value="newest">Newest</option>
          <option value="price">Lowest price</option>
          <option value="context">Largest context</option>
        </select>
      </div>
      <div className="mt-3 flex gap-2 overflow-x-auto pb-1 [scrollbar-width:none]">
        <button className={chip(author === "all")} onClick={() => setAuthor("all")}>All</button>
        <button className={chip(onlyFree)} onClick={() => setOnlyFree(!onlyFree)}>Free</button>
        <button className={chip(onlyTools)} onClick={() => setOnlyTools(!onlyTools)}>Tool use</button>
        <span className="mx-1 w-px shrink-0 bg-border" />
        {authors.map(([a, n]) => (
          <button key={a} className={chip(author === a)} onClick={() => setAuthor(author === a ? "all" : a)}>
            {authorName(a)} <span className="text-subtle">{n}</span>
          </button>
        ))}
      </div>

      <div className="mt-6 rounded-2xl border border-border bg-surface p-2 shadow-soft sm:p-3">
        <div className="hidden grid-cols-[minmax(0,1fr)_110px_110px_90px_20px] gap-x-6 border-b border-border px-3 pb-3 pt-1 text-[12px] font-medium text-subtle sm:grid">
          <span>Model</span>
          <span className="text-right">Input</span>
          <span className="text-right">Output</span>
          <span className="text-right">Context</span>
          <span />
        </div>
        {list.length === 0 ? (
          <p className="px-3 py-16 text-center text-sm text-muted">No models match your filters.</p>
        ) : (
          list.map((m) => <ModelRow key={m.id} m={m} />)
        )}
      </div>
      <p className="mt-3 text-xs text-subtle">Prices in USD per 1M tokens. A small per-request fee applies to paid models.</p>
    </div>
  );
}
