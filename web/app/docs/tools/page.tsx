import type { Metadata } from "next";
import { DocPage } from "@/components/docs/page";
import { CodeBlock } from "@/components/ui/code";

export const metadata: Metadata = { title: "Tools & vision", alternates: { canonical: "/docs/tools" } };

export default function Tools() {
  return (
    <DocPage path="/docs/tools" title="Tools & vision" lead="Let models call your functions and look at images.">
      <h2>Tool calling</h2>
      <p>Models marked with tool support accept function definitions in either API format.</p>
      <CodeBlock
        lang="python"
        code={`tools = [{
    "type": "function",
    "function": {
        "name": "get_weather",
        "description": "Current weather for a city",
        "parameters": {
            "type": "object",
            "properties": {"city": {"type": "string"}},
            "required": ["city"],
        },
    },
}]

resp = client.chat.completions.create(
    model="openai/gpt-4o-mini",
    messages=[{"role": "user", "content": "Weather in Chattogram?"}],
    tools=tools,
)
call = resp.choices[0].message.tool_calls[0]
print(call.function.name, call.function.arguments)`}
      />
      <p>
        Send the result back as a <code>tool</code> message with the matching <code>tool_call_id</code>, then call the API again. In the
        Anthropic format use <code>tool_use</code> / <code>tool_result</code> blocks — we translate between the two.
      </p>
      <h2>Images</h2>
      <CodeBlock
        lang="python"
        code={`resp = client.chat.completions.create(
    model="openai/gpt-4o-mini",
    messages=[{
        "role": "user",
        "content": [
            {"type": "text", "text": "What is in this picture?"},
            {"type": "image_url", "image_url": {"url": "https://example.com/photo.jpg"}},
        ],
    }],
)`}
      />
      <p>Base64 data URLs work too. Requests are limited to 32 MB.</p>
    </DocPage>
  );
}
