import type { Metadata } from "next";
import { Callout, DocPage } from "@/components/docs/page";
import { CodeBlock } from "@/components/ui/code";
import { site } from "@/lib/site";

export const metadata: Metadata = {
  title: "OpenAI-compatible API",
  description: "Use the OpenAI SDK or Chat Completions API with any model on ModelRouter.",
  alternates: { canonical: "/docs/openai" },
};

const params: [string, string, string][] = [
  ["model", "string", "Required. A model ID from /v1/models."],
  ["messages", "array", "Required. system, developer, user, assistant and tool messages."],
  ["max_tokens", "integer", "Maximum tokens to generate. Capped at the model's limit."],
  ["temperature", "number", "0–2. Higher is more random."],
  ["top_p", "number", "Nucleus sampling."],
  ["stop", "string | array", "Up to 4 stop sequences."],
  ["stream", "boolean", "Stream tokens as server-sent events."],
  ["stream_options", "object", "{ include_usage: true } adds a final usage chunk."],
  ["tools", "array", "Function definitions the model may call."],
  ["tool_choice", "string | object", "auto, none, required, or a specific function."],
  ["response_format", "object", "JSON mode / structured outputs where supported."],
  ["seed, presence_penalty, frequency_penalty, logit_bias, user", "", "Passed through to models that support them."],
];

export default function OpenAIDocs() {
  return (
    <DocPage path="/docs/openai" title="OpenAI-compatible API" lead="Drop-in compatible with the OpenAI Chat Completions API and every SDK built on it.">
      <h2>Endpoint</h2>
      <CodeBlock lang="bash" code={`POST ${site.apiBase}/chat/completions`} />
      <h2>Using the official SDKs</h2>
      <CodeBlock
        lang="python"
        title="Python"
        code={`from openai import OpenAI

client = OpenAI(base_url="${site.apiBase}", api_key="mr-sk-...")`}
      />
      <CodeBlock
        lang="typescript"
        title="TypeScript"
        code={`import OpenAI from "openai";

const client = new OpenAI({ baseURL: "${site.apiBase}", apiKey: "mr-sk-..." });`}
      />
      <h2>Request parameters</h2>
      <table>
        <thead><tr><th>Parameter</th><th>Type</th><th>Description</th></tr></thead>
        <tbody>
          {params.map(([p, t, d]) => (
            <tr key={p}><td><code>{p}</code></td><td>{t}</td><td>{d}</td></tr>
          ))}
        </tbody>
      </table>
      <h2>Response</h2>
      <CodeBlock
        lang="json"
        code={`{
  "id": "chatcmpl-5f1c2a...",
  "object": "chat.completion",
  "created": 1760000000,
  "model": "openai/gpt-4o-mini",
  "choices": [{
    "index": 0,
    "message": { "role": "assistant", "content": "Dhaka." },
    "finish_reason": "stop"
  }],
  "usage": { "prompt_tokens": 14, "completion_tokens": 3, "total_tokens": 17 }
}`}
      />
      <Callout>
        Every model accepts this format — including Claude models. Requests to models served natively in the Anthropic format are
        translated automatically, including tool calls and streaming.
      </Callout>
      <h2>Other endpoints</h2>
      <table>
        <thead><tr><th>Endpoint</th><th>Description</th></tr></thead>
        <tbody>
          <tr><td><code>GET /v1/models</code></td><td>List available models with pricing.</td></tr>
          <tr><td><code>GET /v1/models/{"{id}"}</code></td><td>Retrieve one model.</td></tr>
          <tr><td><code>GET /v1/key</code></td><td>Balance and usage of the calling key.</td></tr>
        </tbody>
      </table>
    </DocPage>
  );
}
