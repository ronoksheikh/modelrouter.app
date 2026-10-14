import type { Metadata } from "next";
import { Callout, DocPage } from "@/components/docs/page";
import { CodeBlock, CodeTabs } from "@/components/ui/code";
import { site } from "@/lib/site";

export const metadata: Metadata = { title: "Quickstart", description: "Make your first ModelRouter API request.", alternates: { canonical: "/docs/quickstart" } };

export default function Quickstart() {
  return (
    <DocPage path="/docs/quickstart" title="Quickstart" lead="Three steps from zero to your first completion.">
      <h2>1. Create an API key</h2>
      <p>
        <a href="/login?next=/dashboard/keys">Sign in</a> and open <strong>Dashboard → API keys</strong>. Create a key and copy it — it
        starts with <code>mr-sk-</code> and is shown only once.
      </p>
      <h2>2. Add credits</h2>
      <p>Open <strong>Dashboard → Billing</strong> and top up. Free models work without credits.</p>
      <h2>3. Send a request</h2>
      <CodeBlock lang="bash" code={`export MODELROUTER_API_KEY="mr-sk-..."`} />
      <CodeTabs
        id="qs"
        tabs={[
          {
            label: "Python",
            lang: "python",
            code: `# pip install openai
import os
from openai import OpenAI

client = OpenAI(base_url="${site.apiBase}", api_key=os.environ["MODELROUTER_API_KEY"])

completion = client.chat.completions.create(
    model="openai/gpt-4o-mini",
    messages=[
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "What is the capital of Bangladesh?"},
    ],
)
print(completion.choices[0].message.content)`,
          },
          {
            label: "TypeScript",
            lang: "typescript",
            code: `// npm install openai
import OpenAI from "openai";

const client = new OpenAI({ baseURL: "${site.apiBase}", apiKey: process.env.MODELROUTER_API_KEY });

const completion = await client.chat.completions.create({
  model: "openai/gpt-4o-mini",
  messages: [{ role: "user", content: "What is the capital of Bangladesh?" }],
});
console.log(completion.choices[0].message.content);`,
          },
          {
            label: "cURL",
            lang: "bash",
            code: `curl ${site.apiBase}/chat/completions \\
  -H "Authorization: Bearer $MODELROUTER_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "openai/gpt-4o-mini",
    "messages": [{"role": "user", "content": "What is the capital of Bangladesh?"}]
  }'`,
          },
        ]}
      />
      <Callout>
        Keep keys on your server. Never ship them in a mobile app or browser bundle — anyone can extract them.
      </Callout>
      <h2>Next steps</h2>
      <ul>
        <li><a href="/docs/streaming">Stream tokens</a> as they are generated.</li>
        <li><a href="/docs/tools">Call tools</a> and send images.</li>
        <li><a href="/docs/integrations">Connect your editor or framework</a>.</li>
      </ul>
    </DocPage>
  );
}
