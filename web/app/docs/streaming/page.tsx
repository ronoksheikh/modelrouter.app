import type { Metadata } from "next";
import { DocPage } from "@/components/docs/page";
import { CodeBlock } from "@/components/ui/code";
import { site } from "@/lib/site";

export const metadata: Metadata = { title: "Streaming", alternates: { canonical: "/docs/streaming" } };

export default function Streaming() {
  return (
    <DocPage path="/docs/streaming" title="Streaming" lead="Receive tokens the moment they are generated using server-sent events.">
      <h2>OpenAI format</h2>
      <CodeBlock
        lang="python"
        code={`stream = client.chat.completions.create(
    model="openai/gpt-4o-mini",
    messages=[{"role": "user", "content": "Count to five."}],
    stream=True,
    stream_options={"include_usage": True},
)
for chunk in stream:
    if chunk.choices:
        print(chunk.choices[0].delta.content or "", end="", flush=True)`}
      />
      <p>The stream ends with <code>data: [DONE]</code>. With <code>include_usage</code>, the last chunk before it carries token usage.</p>
      <h2>Anthropic format</h2>
      <CodeBlock
        lang="python"
        code={`with client.messages.stream(
    model="anthropic/claude-sonnet-4.5",
    max_tokens=512,
    messages=[{"role": "user", "content": "Count to five."}],
) as stream:
    for text in stream.text_stream:
        print(text, end="", flush=True)`}
      />
      <h2>Raw SSE</h2>
      <CodeBlock
        lang="bash"
        code={`curl -N ${site.apiBase}/chat/completions \\
  -H "Authorization: Bearer $MODELROUTER_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"openai/gpt-4o-mini","stream":true,"messages":[{"role":"user","content":"Hi"}]}'`}
      />
      <p>
        Lines starting with <code>:</code> are keep-alive comments — ignore them. If you cancel a stream, you are only billed for the tokens
        generated before you disconnected.
      </p>
    </DocPage>
  );
}
