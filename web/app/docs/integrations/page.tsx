import type { Metadata } from "next";
import { DocPage } from "@/components/docs/page";
import { CodeBlock } from "@/components/ui/code";
import { site } from "@/lib/site";

export const metadata: Metadata = {
  title: "Use with your tools",
  description: "Connect ModelRouter to Cursor, Cline, Continue, Claude Code, LangChain, LlamaIndex, the Vercel AI SDK, Open WebUI and n8n.",
  alternates: { canonical: "/docs/integrations" },
};

export default function Integrations() {
  const root = site.apiBase.replace(/\/v1$/, "");
  return (
    <DocPage path="/docs/integrations" title="Use with your tools" lead="Anything that speaks OpenAI or Anthropic works with ModelRouter.">
      <p>The pattern is always the same: set the base URL, set the API key, pick a model ID.</p>
      <table>
        <thead><tr><th>Setting</th><th>OpenAI-style tools</th><th>Anthropic-style tools</th></tr></thead>
        <tbody>
          <tr><td>Base URL</td><td><code>{site.apiBase}</code></td><td><code>{root}</code></td></tr>
          <tr><td>API key</td><td colSpan={2}><code>mr-sk-…</code></td></tr>
        </tbody>
      </table>

      <h2>Claude Code</h2>
      <CodeBlock lang="bash" code={`export ANTHROPIC_BASE_URL="${root}"
export ANTHROPIC_AUTH_TOKEN="mr-sk-..."
claude`} />

      <h2>Cursor, Cline, Continue, Roo Code</h2>
      <p>
        Choose the <em>OpenAI Compatible</em> provider, set the base URL to <code>{site.apiBase}</code>, paste your key and enter a model ID such as{" "}
        <code>anthropic/claude-sonnet-4.5</code>.
      </p>

      <h2>Vercel AI SDK</h2>
      <CodeBlock
        lang="typescript"
        code={`import { createOpenAI } from "@ai-sdk/openai";
import { generateText } from "ai";

const modelrouter = createOpenAI({ baseURL: "${site.apiBase}", apiKey: process.env.MODELROUTER_API_KEY });

const { text } = await generateText({
  model: modelrouter.chat("openai/gpt-4o-mini"),
  prompt: "Write a tagline for a tea shop in Sylhet.",
});`}
      />

      <h2>LangChain</h2>
      <CodeBlock
        lang="python"
        code={`from langchain_openai import ChatOpenAI

llm = ChatOpenAI(
    base_url="${site.apiBase}",
    api_key="mr-sk-...",
    model="meta-llama/llama-3.3-70b-instruct",
)
print(llm.invoke("Hello!").content)`}
      />

      <h2>LlamaIndex</h2>
      <CodeBlock
        lang="python"
        code={`from llama_index.llms.openai_like import OpenAILike

llm = OpenAILike(api_base="${site.apiBase}", api_key="mr-sk-...", model="openai/gpt-4o-mini", is_chat_model=True)`}
      />

      <h2>Open WebUI, LibreChat, n8n</h2>
      <p>Add an OpenAI-compatible connection with the base URL and key above. Models are discovered automatically from <code>/v1/models</code>.</p>
    </DocPage>
  );
}
