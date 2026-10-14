import type { Metadata } from "next";
import Link from "next/link";
import { DocPage } from "@/components/docs/page";
import { CodeBlock } from "@/components/ui/code";
import { site } from "@/lib/site";

export const metadata: Metadata = {
  title: "Introduction",
  description: "ModelRouter gives you one OpenAI- and Anthropic-compatible API for hundreds of AI models.",
  alternates: { canonical: "/docs" },
};

const cards = [
  { href: "/docs/quickstart", title: "Quickstart", body: "Your first request in under a minute." },
  { href: "/docs/openai", title: "OpenAI API", body: "Chat Completions, drop-in compatible." },
  { href: "/docs/anthropic", title: "Anthropic API", body: "Messages API — works with Claude Code." },
  { href: "/docs/integrations", title: "Integrations", body: "Cursor, Cline, LangChain, Vercel AI SDK…" },
];

export default function DocsHome() {
  return (
    <DocPage path="/docs" title="Introduction" lead="ModelRouter is a single API for hundreds of AI models. Use the SDKs you already know — just change the base URL.">
      <div className="not-prose my-8 grid gap-3 sm:grid-cols-2">
        {cards.map((c) => (
          <Link key={c.href} href={c.href} className="rounded-2xl border border-border bg-surface p-5 no-underline shadow-soft transition hover:border-border-strong">
            <span className="font-medium text-fg">{c.title}</span>
            <span className="mt-1 block text-sm text-muted">{c.body}</span>
          </Link>
        ))}
      </div>
      <h2>Base URLs</h2>
      <table>
        <thead>
          <tr><th>API</th><th>Base URL</th></tr>
        </thead>
        <tbody>
          <tr><td>OpenAI-compatible</td><td><code>{site.apiBase}</code></td></tr>
          <tr><td>Anthropic-compatible</td><td><code>{site.apiBase.replace(/\/v1$/, "")}</code></td></tr>
        </tbody>
      </table>
      <h2>How it works</h2>
      <p>
        Every model has a stable ID such as <code>openai/gpt-4o-mini</code> or <code>anthropic/claude-sonnet-4.5</code>. Send it in the{" "}
        <code>model</code> field of either API. Any model works with both API shapes — we translate requests, responses, tool calls and
        streams for you.
      </p>
      <CodeBlock
        lang="bash"
        code={`curl ${site.apiBase}/chat/completions \\
  -H "Authorization: Bearer $MODELROUTER_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model": "openai/gpt-4o-mini", "messages": [{"role": "user", "content": "Hello"}]}'`}
      />
      <h2>Why ModelRouter</h2>
      <ul>
        <li>One key and one bill for every model.</li>
        <li>OpenAI and Anthropic wire formats, interchangeable.</li>
        <li>Pay as you go, in USD or BDT.</li>
        <li>Per-key spend caps and rate limits, instant rotation.</li>
      </ul>
    </DocPage>
  );
}
