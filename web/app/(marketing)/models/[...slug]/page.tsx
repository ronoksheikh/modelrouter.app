import { ArrowLeft, Check, Minus } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ButtonLink } from "@/components/ui/button";
import { CodeTabs } from "@/components/ui/code";
import { Badge } from "@/components/ui/primitives";
import { getModel, getModels } from "@/lib/catalog";
import { authorName, formatDate, formatPriceString, formatTokens } from "@/lib/format";
import { site } from "@/lib/site";

export const revalidate = 300;
export const dynamicParams = true;

type Props = { params: Promise<{ slug: string[] }> };

export async function generateStaticParams() {
  // Pre-render featured models at build; the rest render on first visit (ISR).
  const models = await getModels();
  return models.filter((m) => m.featured).slice(0, 50).map((m) => ({ slug: m.id.split("/") }));
}

async function load(params: Props["params"]) {
  const { slug } = await params;
  return getModel(slug.map(decodeURIComponent).join("/"));
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const m = await load(params);
  if (!m) return { title: "Model not found" };
  const desc =
    (m.description ? m.description.slice(0, 150) + (m.description.length > 150 ? "…" : "") + " " : "") +
    `${formatPriceString(m.input_price_per_m)}/M input, ${formatPriceString(m.output_price_per_m)}/M output, ${formatTokens(m.context_length)} context.`;
  return {
    title: `${m.name} API — pricing & context`,
    description: desc,
    alternates: { canonical: `/models/${m.id}` },
    openGraph: { title: `${m.name} on ModelRouter`, description: desc },
  };
}

export default async function ModelPage({ params }: Props) {
  const m = await load(params);
  if (!m) notFound();

  const stats = [
    { label: "Input", value: formatPriceString(m.input_price_per_m), sub: "per 1M tokens" },
    { label: "Output", value: formatPriceString(m.output_price_per_m), sub: "per 1M tokens" },
    { label: "Context", value: formatTokens(m.context_length), sub: "tokens" },
    { label: "Max output", value: m.max_output_tokens ? formatTokens(m.max_output_tokens) : "Model max", sub: "tokens per request" },
  ];
  const caps = [
    { label: "Streaming", ok: true },
    { label: "Tool / function calling", ok: m.supports_tools },
    { label: "Image input", ok: m.modalities?.includes("image") },
    { label: "OpenAI-compatible API", ok: true },
    { label: "Anthropic-compatible API", ok: true },
  ];
  const jsonLd = {
    "@context": "https://schema.org",
    "@type": "Product",
    name: `${m.name} API`,
    brand: { "@type": "Brand", name: authorName(m.author) },
    description: m.description || `${m.name} via ModelRouter`,
    url: `${site.url}/models/${m.id}`,
    offers: { "@type": "Offer", priceCurrency: "USD", price: m.input_price_per_m, availability: "https://schema.org/InStock" },
  };

  return (
    <div className="container-page py-12 sm:py-16">
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }} />
      <Link href="/models" className="inline-flex items-center gap-1.5 text-sm text-muted hover:text-fg">
        <ArrowLeft className="size-4" /> All models
      </Link>
      <div className="mt-6 flex flex-col justify-between gap-6 lg:flex-row lg:items-end">
        <div className="max-w-2xl">
          <div className="flex flex-wrap items-center gap-2">
            <Badge>{authorName(m.author)}</Badge>
            {m.free ? <Badge tone="good">Free</Badge> : null}
            {m.featured ? <Badge tone="accent">Featured</Badge> : null}
          </div>
          <h1 className="mt-4 text-4xl font-semibold tracking-[-0.03em] sm:text-5xl">{m.name}</h1>
          <p className="mt-3 font-mono text-sm text-muted">{m.id}</p>
        </div>
        <ButtonLink href="/login?next=/dashboard/keys" size="lg">Use this model</ButtonLink>
      </div>

      <dl className="mt-10 grid grid-cols-2 gap-px overflow-hidden rounded-2xl border border-border bg-border lg:grid-cols-4">
        {stats.map((s) => (
          <div key={s.label} className="bg-surface p-5 sm:p-6">
            <dt className="text-[13px] text-muted">{s.label}</dt>
            <dd className="mt-2 text-2xl font-semibold tracking-tight tabular-nums">{s.value}</dd>
            <dd className="mt-0.5 text-xs text-subtle">{s.sub}</dd>
          </div>
        ))}
      </dl>

      <div className="mt-12 grid gap-12 lg:grid-cols-[1fr_340px]">
        <div className="min-w-0">
          {m.description ? (
            <>
              <h2 className="text-lg font-semibold tracking-tight">About</h2>
              <p className="mt-3 whitespace-pre-line text-[15px] leading-7 text-fg-2">{m.description}</p>
            </>
          ) : null}
          <h2 className="mt-10 text-lg font-semibold tracking-tight">Quick start</h2>
          <CodeTabs
            id="model"
            className="mt-4"
            tabs={[
              {
                label: "Python",
                lang: "python",
                code: `from openai import OpenAI

client = OpenAI(base_url="${site.apiBase}", api_key="mr-sk-...")
resp = client.chat.completions.create(
    model="${m.id}",
    messages=[{"role": "user", "content": "Hello!"}],
)
print(resp.choices[0].message.content)`,
              },
              {
                label: "TypeScript",
                lang: "typescript",
                code: `import OpenAI from "openai";

const client = new OpenAI({ baseURL: "${site.apiBase}", apiKey: process.env.MODELROUTER_API_KEY });
const resp = await client.chat.completions.create({
  model: "${m.id}",
  messages: [{ role: "user", content: "Hello!" }],
});`,
              },
              {
                label: "cURL",
                lang: "bash",
                code: `curl ${site.apiBase}/chat/completions \\
  -H "Authorization: Bearer $MODELROUTER_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model": "${m.id}", "messages": [{"role": "user", "content": "Hello!"}]}'`,
              },
            ]}
          />
        </div>
        <aside className="h-fit rounded-2xl border border-border bg-surface p-6 shadow-soft">
          <h2 className="text-sm font-semibold">Capabilities</h2>
          <ul className="mt-4 space-y-3">
            {caps.map((c) => (
              <li key={c.label} className="flex items-center justify-between text-sm">
                <span className={c.ok ? "text-fg-2" : "text-subtle"}>{c.label}</span>
                {c.ok ? <Check className="size-4 text-good" aria-label="Yes" /> : <Minus className="size-4 text-subtle" aria-label="No" />}
              </li>
            ))}
          </ul>
          <div className="mt-6 border-t border-border pt-5 text-sm text-muted">
            <p>
              Request fee: <span className="font-mono text-fg-2">{formatPriceString(m.request_fee)}</span>
            </p>
            <p className="mt-1">Added {formatDate(m.created_at)}</p>
          </div>
        </aside>
      </div>
    </div>
  );
}
