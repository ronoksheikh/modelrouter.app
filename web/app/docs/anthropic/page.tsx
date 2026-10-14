import type { Metadata } from "next";
import { Callout, DocPage } from "@/components/docs/page";
import { CodeBlock } from "@/components/ui/code";
import { site } from "@/lib/site";

export const metadata: Metadata = {
  title: "Anthropic-compatible API",
  description: "Use the Anthropic SDK, Messages API and Claude Code with any model on ModelRouter.",
  alternates: { canonical: "/docs/anthropic" },
};

export default function AnthropicDocs() {
  const root = site.apiBase.replace(/\/v1$/, "");
  return (
    <DocPage path="/docs/anthropic" title="Anthropic-compatible API" lead="Speak the Anthropic Messages API to any model — Claude or otherwise.">
      <h2>Endpoint</h2>
      <CodeBlock lang="bash" code={`POST ${site.apiBase}/messages
POST ${site.apiBase}/messages/count_tokens`} />
      <h2>Using the Anthropic SDK</h2>
      <p>Point the SDK at our root URL (the SDK adds <code>/v1</code> itself):</p>
      <CodeBlock
        lang="python"
        title="Python"
        code={`import anthropic

client = anthropic.Anthropic(base_url="${root}", api_key="mr-sk-...")

message = client.messages.create(
    model="anthropic/claude-sonnet-4.5",
    max_tokens=1024,
    system="You are concise.",
    messages=[{"role": "user", "content": "Explain recursion in one sentence."}],
)
print(message.content[0].text)`}
      />
      <CodeBlock
        lang="typescript"
        title="TypeScript"
        code={`import Anthropic from "@anthropic-ai/sdk";

const client = new Anthropic({ baseURL: "${root}", apiKey: process.env.MODELROUTER_API_KEY });

const msg = await client.messages.create({
  model: "google/gemini-2.5-flash", // any model works
  max_tokens: 1024,
  messages: [{ role: "user", content: "Hello!" }],
});`}
      />
      <h2>Claude Code</h2>
      <p>Run Claude Code against ModelRouter by setting two environment variables:</p>
      <CodeBlock
        lang="bash"
        code={`export ANTHROPIC_BASE_URL="${root}"
export ANTHROPIC_AUTH_TOKEN="mr-sk-..."
export ANTHROPIC_MODEL="anthropic/claude-sonnet-4.5"
claude`}
      />
      <h2>Supported features</h2>
      <ul>
        <li>System prompts (string or text blocks)</li>
        <li>Streaming with the standard event sequence (<code>message_start</code> … <code>message_stop</code>)</li>
        <li>Tool use: <code>tools</code>, <code>tool_choice</code>, <code>tool_use</code> and <code>tool_result</code> blocks</li>
        <li>Images (base64 and URL sources)</li>
        <li>Extended thinking budgets on models that support reasoning</li>
      </ul>
      <Callout>
        <code>max_tokens</code> is required, as in the Anthropic API. Values above a model&apos;s output limit are capped automatically.
      </Callout>
    </DocPage>
  );
}
