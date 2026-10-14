import { Check } from "lucide-react";
import type { Metadata } from "next";
import { ButtonLink } from "@/components/ui/button";
import { Eyebrow } from "@/components/ui/primitives";
import { backend } from "@/lib/backend";

export const revalidate = 600;

export const metadata: Metadata = {
  title: "Pricing — pay as you go",
  description:
    "Simple pay-as-you-go pricing for AI models. Pay per token plus a tiny per-request fee. No subscriptions. Top up with cards, PayPal, or bKash and Nagad in Bangladesh.",
  alternates: { canonical: "/pricing" },
};

type PublicConfig = { min_topup_cents: number; request_fee: string; bdt_per_usd: number };

async function config(): Promise<PublicConfig> {
  try {
    return await backend<PublicConfig>("/api/public/config", { revalidate: 600 });
  } catch {
    return { min_topup_cents: 500, request_fee: "0.00000001", bdt_per_usd: 122 };
  }
}

const faqs = [
  {
    q: "How am I charged?",
    a: "Each request costs (input tokens × input price) + (output tokens × output price) + a flat request fee. Prices are listed per million tokens on every model page. Your balance updates in real time.",
  },
  {
    q: "Do credits expire?",
    a: "No. Credits stay in your account until you use them.",
  },
  {
    q: "Can I pay from Bangladesh?",
    a: "Yes. Bangladeshi users can pay in BDT with local methods such as bKash and Nagad as they come online, or use any international card.",
  },
  {
    q: "Are there free models?",
    a: "Yes — models marked Free cost nothing per token. They are rate-limited to keep them available for everyone.",
  },
  {
    q: "Can I cap my spending?",
    a: "Every API key can have its own spend limit and rate limit. Requests stop the moment a key reaches its cap.",
  },
  {
    q: "Do you offer refunds?",
    a: "Unused credits can be refunded within 14 days of purchase. See our refund policy for details.",
  },
];

export default async function PricingPage() {
  const c = await config();
  const min = (c.min_topup_cents / 100).toFixed(0);
  const faqLd = {
    "@context": "https://schema.org",
    "@type": "FAQPage",
    mainEntity: faqs.map((f) => ({ "@type": "Question", name: f.q, acceptedAnswer: { "@type": "Answer", text: f.a } })),
  };
  return (
    <div className="container-page py-14 sm:py-20">
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(faqLd) }} />
      <div className="mx-auto max-w-2xl text-center">
        <Eyebrow>Pricing</Eyebrow>
        <h1 className="mt-3 text-balance text-4xl font-semibold tracking-[-0.03em] sm:text-6xl">
          Pay only for <span className="font-serif font-normal italic text-accent">what you use</span>.
        </h1>
        <p className="mt-5 text-lg text-muted">No plans to pick. No seats. Add credits and every model is yours.</p>
      </div>

      <div className="mx-auto mt-14 grid max-w-5xl gap-4 lg:grid-cols-3">
        {[
          { k: "Tokens", v: "Per model", d: "Listed per 1M input and output tokens on each model page." },
          { k: "Request fee", v: `$${c.request_fee}`, d: "A flat fee per request — a millionth of a cent." },
          { k: "Top-ups", v: `From $${min}`, d: `Cards & PayPal worldwide. BDT for Bangladesh (≈ ৳${Math.round(c.bdt_per_usd)} per $1).` },
        ].map((x) => (
          <div key={x.k} className="rounded-2xl border border-border bg-surface p-7 shadow-soft">
            <p className="text-sm text-muted">{x.k}</p>
            <p className="mt-3 text-3xl font-semibold tracking-tight tabular-nums">{x.v}</p>
            <p className="mt-3 text-sm leading-6 text-muted">{x.d}</p>
          </div>
        ))}
      </div>

      <div className="mx-auto mt-6 max-w-5xl rounded-2xl border border-border bg-surface p-7 shadow-soft sm:p-10">
        <div className="grid gap-10 md:grid-cols-2">
          <div>
            <h2 className="text-xl font-semibold tracking-tight">Everything included</h2>
            <p className="mt-2 text-sm text-muted">Every account gets the full platform from day one.</p>
            <ButtonLink href="/login?next=/dashboard/billing" className="mt-6">
              Create account
            </ButtonLink>
          </div>
          <ul className="grid gap-3 text-[15px] sm:grid-cols-1">
            {[
              "Access to every model",
              "OpenAI & Anthropic compatible endpoints",
              "Unlimited API keys with spend caps",
              "Streaming, tools & vision",
              "Real-time usage dashboard",
              "Credits never expire",
            ].map((f) => (
              <li key={f} className="flex items-center gap-3 text-fg-2">
                <Check className="size-4 shrink-0 text-accent" /> {f}
              </li>
            ))}
          </ul>
        </div>
      </div>

      <div className="mx-auto mt-24 max-w-3xl">
        <h2 className="text-center text-2xl font-semibold tracking-tight">Questions</h2>
        <div className="mt-8 divide-y divide-border rounded-2xl border border-border bg-surface">
          {faqs.map((f) => (
            <details key={f.q} className="group px-6 py-5 [&_summary::-webkit-details-marker]:hidden">
              <summary className="flex cursor-pointer list-none items-center justify-between gap-4 font-medium">
                {f.q}
                <span className="text-xl leading-none text-subtle transition group-open:rotate-45">+</span>
              </summary>
              <p className="mt-3 text-[15px] leading-7 text-muted">{f.a}</p>
            </details>
          ))}
        </div>
      </div>
    </div>
  );
}
