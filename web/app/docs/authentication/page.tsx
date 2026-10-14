import type { Metadata } from "next";
import { Callout, DocPage } from "@/components/docs/page";
import { CodeBlock } from "@/components/ui/code";
import { site } from "@/lib/site";

export const metadata: Metadata = { title: "Authentication & keys", alternates: { canonical: "/docs/authentication" } };

export default function Auth() {
  return (
    <DocPage path="/docs/authentication" title="Authentication & keys" lead="Every request is authenticated with an API key.">
      <h2>Sending your key</h2>
      <p>Use either header — both work on every endpoint:</p>
      <CodeBlock lang="bash" code={`Authorization: Bearer mr-sk-...
# or (Anthropic style)
x-api-key: mr-sk-...`} />
      <h2>Managing keys</h2>
      <ul>
        <li><strong>Create</strong> as many keys as you need — one per app or environment is a good habit.</li>
        <li><strong>Rotate</strong> a key to replace its secret while keeping its name, limits and history. The old secret stops working immediately.</li>
        <li><strong>Disable</strong> a key temporarily, or <strong>delete</strong> it permanently.</li>
      </ul>
      <h2>Per-key limits</h2>
      <table>
        <thead><tr><th>Limit</th><th>Behaviour</th></tr></thead>
        <tbody>
          <tr><td>Spend limit</td><td>Requests return <code>402</code> once the key has spent this much.</td></tr>
          <tr><td>Rate limit</td><td>Requests per minute. Exceeding it returns <code>429</code> with <code>Retry-After</code>.</td></tr>
        </tbody>
      </table>
      <h2>Check a key</h2>
      <CodeBlock lang="bash" code={`curl ${site.apiBase}/key -H "Authorization: Bearer $MODELROUTER_API_KEY"`} />
      <CodeBlock lang="json" code={`{ "data": { "balance": "12.48", "usage": "3.07", "limit": null } }`} />
      <Callout tone="warn">If a key leaks, rotate it from the dashboard. Rotation takes effect across our whole network within a second.</Callout>
    </DocPage>
  );
}
