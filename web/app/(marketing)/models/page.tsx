import type { Metadata } from "next";
import { ModelExplorer } from "@/components/site/model-explorer";
import { Eyebrow } from "@/components/ui/primitives";
import { getModels } from "@/lib/catalog";

export const revalidate = 300;

export const metadata: Metadata = {
  title: "Models — pricing, context length and capabilities",
  description:
    "Browse every model on ModelRouter: GPT, Claude, Gemini, Llama, DeepSeek, Qwen and more. Compare per-token pricing and context windows, then call any of them with one API key.",
  alternates: { canonical: "/models" },
};

export default async function ModelsPage() {
  const models = await getModels();
  const itemList = {
    "@context": "https://schema.org",
    "@type": "ItemList",
    itemListElement: models.slice(0, 100).map((m, i) => ({ "@type": "ListItem", position: i + 1, name: m.name, url: `/models/${m.id}` })),
  };
  return (
    <div className="container-page py-14 sm:py-20">
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(itemList) }} />
      <div className="max-w-2xl">
        <Eyebrow>Models</Eyebrow>
        <h1 className="mt-3 text-4xl font-semibold tracking-[-0.03em] sm:text-5xl">Every model, one key.</h1>
        <p className="mt-4 text-lg text-muted">
          Compare prices and context windows, then use any model ID with the OpenAI or Anthropic API.
        </p>
      </div>
      <div className="mt-10">
        {models.length > 0 ? (
          <ModelExplorer models={models.map((m) => ({ ...m, description: "" }))} />
        ) : (
          <p className="rounded-2xl border border-dashed border-border-strong py-16 text-center text-muted">
            The model catalog is refreshing. Please check back in a moment.
          </p>
        )}
      </div>
    </div>
  );
}
