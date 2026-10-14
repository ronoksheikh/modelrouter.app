import { ArrowRight, Gauge, KeyRound, Layers, LineChart, Receipt, Wallet } from "lucide-react";
import Link from "next/link";
import { ButtonLink } from "@/components/ui/button";
import { CodeTabs } from "@/components/ui/code";
import { Eyebrow } from "@/components/ui/primitives";
import { SpotlightGroup } from "@/components/ui/spotlight";
import { ModelRow } from "@/components/site/model-row";
import { getModels } from "@/lib/catalog";
import { site } from "@/lib/site";

export const revalidate = 300;

const families = ["OpenAI", "Anthropic", "Google", "Meta", "Mistral", "DeepSeek", "Qwen", "xAI", "Cohere", "Moonshot", "NVIDIA", "MiniMax"];

const features = [
  { icon: Layers, title: "Two APIs, one key", body: "Speak OpenAI Chat Completions or Anthropic Messages. We translate between them, tools and streaming included." },
  { icon: Gauge, title: "Built for speed", body: "A lean Go gateway streams every token the moment it arrives, with automatic retries and fallbacks." },
  { icon: Receipt, title: "Pay per request", body: "No subscriptions or minimums. Pay for tokens plus a tiny per-request fee — fractions of a cent." },
  { icon: Wallet, title: "Pay your way", body: "Cards and PayPal worldwide. bKash, Nagad and local methods for Bangladesh." },
  { icon: KeyRound, title: "Keys you control", body: "Create, rotate and revoke keys instantly. Set spend caps and rate limits per key." },
  { icon: LineChart, title: "Every request, accounted", body: "Live usage, cost per model and a full request log — down to the last token." },
];

const steps = [
  { n: "01", title: "Sign in", body: "Google, GitHub or email. Takes ten seconds." },
  { n: "02", title: "Add credits", body: "Top up from $5 with a card, PayPal or — in Bangladesh — mobile banking." },
  { n: "03", title: "Ship", body: "Point any OpenAI or Anthropic SDK at our base URL. That's the whole migration." },
];

function code(base: string) {
  return [
    {
      label: "Python",
      lang: "python" as const,
      code: `from openai import OpenAI

client = OpenAI(
    base_url="${base}",
    api_key="mr-sk-...",
)

reply = client.chat.completions.create(
    model="openai/gpt-4o-mini",
    messages=[{"role": "user", "content": "Hello from Dhaka!"}],
)
print(reply.choices[0].message.content)`,
    },
    {
      label: "TypeScript",
      lang: "typescript" as const,
      code: `import OpenAI from "openai";

const client = new OpenAI({
  baseURL: "${base}",
  apiKey: process.env.MODELROUTER_API_KEY,
});

const stream = await client.chat.completions.create({
  model: "anthropic/claude-sonnet-4.5",
  messages: [{ role: "user", content: "Write a haiku about rivers." }],
  stream: true,
});
for await (const chunk of stream) process.stdout.write(chunk.choices[0]?.delta?.content ?? "");`,
    },
    {
      label: "Anthropic",
      lang: "python" as const,
      code: `import anthropic

client = anthropic.Anthropic(
    base_url="${base.replace(/\/v1$/, "")}",
    api_key="mr-sk-...",
)

message = client.messages.create(
    model="anthropic/claude-sonnet-4.5",
    max_tokens=1024,
    messages=[{"role": "user", "content": "Summarise this in one line."}],
)`,
    },
    {
      label: "cURL",
      lang: "bash" as const,
      code: `curl ${base}/chat/completions \\
  -H "Authorization: Bearer $MODELROUTER_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "google/gemini-2.5-flash",
    "messages": [{"role": "user", "content": "Hi!"}]
  }'`,
    },
  ];
}

export default async function Home() {
  const models = await getModels();
  const showcase = [...models].sort((a, b) => Number(b.featured) - Number(a.featured)).slice(0, 6);
  const jsonLd = {
    "@context": "https://schema.org",
    "@graph": [
      { "@type": "Organization", name: site.name, url: site.url, logo: `${site.url}/icon.svg` },
      { "@type": "WebSite", name: site.name, url: site.url },
      {
        "@type": "SoftwareApplication",
        name: site.name,
        applicationCategory: "DeveloperApplication",
        operatingSystem: "Any",
        description: site.description,
        offers: { "@type": "Offer", price: "0", priceCurrency: "USD", description: "Pay as you go" },
      },
    ],
  };

  return (
    <>
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }} />

      {/* Hero */}
      <section className="relative overflow-hidden">
        <div aria-hidden className="pointer-events-none absolute inset-0 -z-10">
          <div className="bg-grid absolute inset-0" />
          <div className="absolute -top-40 left-1/2 h-[520px] w-[900px] -translate-x-1/2 animate-drift rounded-full bg-[radial-gradient(closest-side,var(--glacier),transparent)] opacity-70 blur-2xl" />
          <div className="absolute -top-10 right-[8%] h-72 w-72 animate-drift rounded-full bg-[radial-gradient(closest-side,color-mix(in_oklab,var(--teal)_35%,transparent),transparent)] blur-2xl [animation-delay:-6s]" />
        </div>
        <div className="container-page pb-20 pt-16 sm:pt-24 lg:pb-28">
          <div className="mx-auto max-w-3xl text-center">
            <Link
              href="/docs/anthropic"
              className="inline-flex animate-rise items-center gap-2 rounded-full border border-border bg-surface/70 py-1 pl-1 pr-3 text-[13px] text-fg-2 shadow-soft backdrop-blur transition hover:border-border-strong"
            >
              <span className="rounded-full bg-accent-soft px-2 py-0.5 text-[11.5px] font-medium text-accent-ink">New</span>
              <span className="text-shine">Anthropic Messages API, on every model</span>
              <ArrowRight className="size-3.5 text-subtle" />
            </Link>
            <h1 className="mt-7 animate-rise text-balance text-[42px] font-semibold leading-[1.04] tracking-[-0.035em] [animation-delay:80ms] sm:text-6xl lg:text-[72px]">
              One API for every{" "}
              <span className="font-serif text-[1.08em] font-normal italic tracking-[-0.01em] text-accent">frontier</span> model.
            </h1>
            <p className="mx-auto mt-6 max-w-xl animate-rise text-pretty text-lg leading-8 text-muted [animation-delay:160ms]">
              Hundreds of models behind a single key. OpenAI- and Anthropic-compatible, streaming-fast, and priced per request — pay in
              dollars or taka.
            </p>
            <div className="mt-9 flex animate-rise flex-col items-center justify-center gap-3 [animation-delay:240ms] sm:flex-row">
              <ButtonLink href="/login?next=/dashboard/keys" size="lg" className="w-full sm:w-auto">
                Get your API key <ArrowRight className="size-4" />
              </ButtonLink>
              <ButtonLink href="/docs/quickstart" variant="secondary" size="lg" className="w-full sm:w-auto">
                Read the quickstart
              </ButtonLink>
            </div>
            <p className="mt-5 animate-rise text-[13px] text-subtle [animation-delay:300ms]">
              Drop-in base URL <code className="rounded-md bg-bg-soft px-1.5 py-0.5 font-mono text-fg-2">{site.apiBase}</code>
            </p>
          </div>

          <div className="mx-auto mt-16 max-w-3xl animate-rise [animation-delay:380ms]">
            <CodeTabs id="hero" tabs={code(site.apiBase)} />
          </div>
        </div>
      </section>

      {/* Model families marquee */}
      <section aria-label="Model families" className="border-y border-border bg-surface/50 py-7">
        <div className="relative overflow-hidden [mask-image:linear-gradient(to_right,transparent,#000_12%,#000_88%,transparent)]">
          <div className="flex w-max animate-marquee gap-14 pr-14 hover:[animation-play-state:paused]">
            {[...families, ...families].map((f, i) => (
              <span key={i} aria-hidden={i >= families.length} className="text-lg font-medium tracking-tight text-subtle">
                {f}
              </span>
            ))}
          </div>
        </div>
      </section>

      {/* Features */}
      <section className="container-page py-24 sm:py-32">
        <div className="max-w-2xl">
          <Eyebrow>Why ModelRouter</Eyebrow>
          <h2 className="mt-3 text-balance text-3xl font-semibold tracking-[-0.03em] sm:text-[42px] sm:leading-[1.1]">
            Everything you need. <span className="text-muted">Nothing you don&apos;t.</span>
          </h2>
        </div>
        <SpotlightGroup className="mt-14 grid gap-px overflow-hidden rounded-3xl border border-border bg-border sm:grid-cols-2 lg:grid-cols-3">
          {features.map((f) => (
            <div key={f.title} className="spotlight bg-surface p-7 sm:p-8">
              <div className="flex size-10 items-center justify-center rounded-xl border border-border bg-bg-soft text-accent">
                <f.icon className="size-[18px]" strokeWidth={1.75} />
              </div>
              <h3 className="mt-6 font-medium tracking-tight">{f.title}</h3>
              <p className="mt-2 text-[15px] leading-7 text-muted">{f.body}</p>
            </div>
          ))}
        </SpotlightGroup>
      </section>

      {/* Models */}
      <section className="border-t border-border bg-surface/40">
        <div className="container-page py-24 sm:py-28">
          <div className="flex flex-col justify-between gap-6 sm:flex-row sm:items-end">
            <div className="max-w-xl">
              <Eyebrow>Models</Eyebrow>
              <h2 className="mt-3 text-balance text-3xl font-semibold tracking-[-0.03em] sm:text-[42px] sm:leading-[1.1]">
                {models.length > 0 ? `${models.length} models.` : "Every major model."} One endpoint.
              </h2>
              <p className="mt-4 text-muted">Switch models by changing a single string. Transparent per-token pricing, shown up front.</p>
            </div>
            <ButtonLink href="/models" variant="secondary">
              Browse all models <ArrowRight className="size-4" />
            </ButtonLink>
          </div>
          {showcase.length > 0 ? (
            <div className="mt-10 rounded-2xl border border-border bg-surface p-2 shadow-soft sm:p-3">
              {showcase.map((m) => (
                <ModelRow key={m.id} m={m} />
              ))}
            </div>
          ) : null}
        </div>
      </section>

      {/* How it works */}
      <section className="container-page py-24 sm:py-32">
        <div className="grid gap-14 lg:grid-cols-[1fr_1.2fr] lg:gap-20">
          <div>
            <Eyebrow>Get started</Eyebrow>
            <h2 className="mt-3 text-balance text-3xl font-semibold tracking-[-0.03em] sm:text-[42px] sm:leading-[1.1]">
              From sign-in to first token in under a minute.
            </h2>
            <p className="mt-4 max-w-md text-muted">
              Already using the OpenAI or Anthropic SDK? Change the base URL and key. Your code, tools and streaming keep working.
            </p>
          </div>
          <ol className="relative space-y-3">
            {steps.map((s) => (
              <li key={s.n} className="flex gap-5 rounded-2xl border border-border bg-surface p-6 shadow-soft">
                <span className="font-mono text-sm text-accent">{s.n}</span>
                <div>
                  <p className="font-medium">{s.title}</p>
                  <p className="mt-1 text-[15px] leading-7 text-muted">{s.body}</p>
                </div>
              </li>
            ))}
          </ol>
        </div>
      </section>

      {/* Pricing teaser */}
      <section className="container-page pb-24 sm:pb-32">
        <div className="relative overflow-hidden rounded-[28px] border border-border bg-sea px-6 py-14 text-center sm:px-14 sm:py-20 dark:bg-surface">
          <div aria-hidden className="absolute inset-0 -z-0 opacity-60 [background:radial-gradient(600px_circle_at_20%_0%,color-mix(in_oklab,var(--accent)_45%,transparent),transparent_60%),radial-gradient(500px_circle_at_90%_100%,color-mix(in_oklab,var(--teal)_35%,transparent),transparent_60%)]" />
          <div className="relative">
            <p className="font-mono text-[11.5px] uppercase tracking-[0.14em] text-glacier dark:text-accent">Simple pricing</p>
            <h2 className="mx-auto mt-4 max-w-2xl text-balance text-3xl font-semibold tracking-[-0.03em] text-white sm:text-5xl dark:text-fg">
              Pay for tokens. Plus <span className="font-serif font-normal italic">a millionth</span> of a cent.
            </h2>
            <p className="mx-auto mt-5 max-w-lg text-[15px] leading-7 text-white/70 dark:text-muted">
              No subscription, no seat fees, no expiry on credits. Top up from $5 and watch every request in your dashboard.
            </p>
            <div className="mt-9 flex flex-col justify-center gap-3 sm:flex-row">
              <ButtonLink href="/login?next=/dashboard/billing" size="lg" className="bg-white text-[#0b1324] hover:bg-white/90 dark:bg-accent dark:text-accent-fg">
                Start building
              </ButtonLink>
              <ButtonLink href="/pricing" size="lg" variant="ghost" className="text-white hover:bg-white/10 hover:text-white dark:text-fg">
                See pricing
              </ButtonLink>
            </div>
          </div>
        </div>
      </section>
    </>
  );
}
