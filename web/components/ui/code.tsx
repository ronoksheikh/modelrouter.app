import { createHighlighter, type Highlighter } from "shiki";
import { CopyButton } from "./copy-button";
import { cn } from "@/lib/cn";

const LANGS = ["bash", "python", "typescript", "javascript", "json", "go"] as const;
export type Lang = (typeof LANGS)[number];

let highlighter: Promise<Highlighter> | null = null;
function getHighlighter() {
  highlighter ??= createHighlighter({ themes: ["github-dark-default"], langs: [...LANGS] });
  return highlighter;
}

async function highlight(code: string, lang: Lang) {
  const h = await getHighlighter();
  return h.codeToHtml(code.trim(), { lang, theme: "github-dark-default" });
}

function Frame({ title, copy, children, className }: { title?: React.ReactNode; copy: string; children: React.ReactNode; className?: string }) {
  return (
    <div className={cn("group relative overflow-hidden rounded-2xl border border-white/[0.06] bg-[var(--code-bg)] shadow-lift", className)}>
      {title}
      <div className="absolute right-2 top-2 z-10 opacity-70 transition group-hover:opacity-100">
        <CopyButton value={copy} />
      </div>
      {children}
    </div>
  );
}

/** Server-highlighted code block (no client-side highlighting cost). */
export async function CodeBlock({ code, lang = "bash", title, className }: { code: string; lang?: Lang; title?: string; className?: string }) {
  const html = await highlight(code, lang);
  return (
    <Frame
      className={cn("my-5", className)}
      copy={code.trim()}
      title={
        title ? (
          <div className="border-b border-white/[0.06] px-4 py-2.5 font-mono text-[11.5px] text-slate-400">{title}</div>
        ) : null
      }
    >
      <div
        className="overflow-x-auto px-4 py-4 pr-12 text-[13px] leading-[1.7] [&_pre]:!bg-transparent"
        dangerouslySetInnerHTML={{ __html: html }}
      />
    </Frame>
  );
}

let tabSeq = 0;

/** Tabbed code samples implemented with radio inputs — zero JavaScript. */
export async function CodeTabs({
  tabs,
  className,
  id,
}: {
  tabs: { label: string; lang: Lang; code: string }[];
  className?: string;
  id?: string;
}) {
  const name = id ?? `ct${++tabSeq}`;
  const rendered = await Promise.all(tabs.map((t) => highlight(t.code, t.lang)));
  return (
    <div className={cn("ctabs relative overflow-hidden rounded-2xl border border-white/[0.06] bg-[var(--code-bg)] shadow-lift", className)}>
      {tabs.map((t, i) => (
        <input key={t.label} type="radio" name={name} id={`${name}-${i}`} defaultChecked={i === 0} aria-label={t.label} />
      ))}
      <div className="ctabs-bar flex items-center gap-1 border-b border-white/[0.06] px-2 py-2">
        <span className="mx-2 flex gap-1.5" aria-hidden>
          <i className="size-2.5 rounded-full bg-white/10" />
          <i className="size-2.5 rounded-full bg-white/10" />
          <i className="size-2.5 rounded-full bg-white/10" />
        </span>
        {tabs.map((t, i) => (
          <label
            key={t.label}
            htmlFor={`${name}-${i}`}
            className="cursor-pointer rounded-lg px-2.5 py-1 font-mono text-[12px] text-slate-400 transition hover:text-white"
          >
            {t.label}
          </label>
        ))}
      </div>
      <div className="ctabs-panels">
        {tabs.map((t, i) => (
          <div key={t.label} className="ctabs-panel relative">
            <div className="absolute right-2 top-2 z-10">
              <CopyButton value={t.code.trim()} />
            </div>
            <div
              className="overflow-x-auto px-4 py-4 pr-12 text-[13px] leading-[1.7] [&_pre]:!bg-transparent"
              dangerouslySetInnerHTML={{ __html: rendered[i] }}
            />
          </div>
        ))}
      </div>
    </div>
  );
}
