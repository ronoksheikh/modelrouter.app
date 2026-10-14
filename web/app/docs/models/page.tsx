import type { Metadata } from "next";
import { DocPage } from "@/components/docs/page";
import { CodeBlock } from "@/components/ui/code";
import { site } from "@/lib/site";

export const metadata: Metadata = { title: "Models", alternates: { canonical: "/docs/models" } };

export default function ModelsDocs() {
  return (
    <DocPage path="/docs/models" title="Models" lead="Discover models and their prices programmatically.">
      <h2>List models</h2>
      <CodeBlock lang="bash" code={`curl ${site.apiBase}/models`} />
      <CodeBlock
        lang="json"
        code={`{
  "object": "list",
  "data": [{
    "id": "openai/gpt-4o-mini",
    "object": "model",
    "owned_by": "openai",
    "name": "GPT-4o mini",
    "context_length": 128000,
    "max_output_tokens": 16384,
    "supports_tools": true,
    "pricing": { "prompt": "0.000000165", "completion": "0.00000066", "request": "0.00000001" }
  }]
}`}
      />
      <p>
        <code>pricing</code> values are USD per token (prompt and completion) and per request. Send <code>anthropic-version</code> to get the
        Anthropic list format instead.
      </p>
      <h2>Model IDs</h2>
      <p>
        IDs are stable. Use them exactly as listed, e.g. <code>openai/gpt-4o-mini</code>. Browse everything on the <a href="/models">models page</a>.
      </p>
      <h2>Free models</h2>
      <p>Free models cost nothing per token and work without credits, with a lower rate limit.</p>
    </DocPage>
  );
}
