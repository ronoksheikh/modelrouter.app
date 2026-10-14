import type { Metadata } from "next";
import { DocPage } from "@/components/docs/page";
import { CodeBlock } from "@/components/ui/code";

export const metadata: Metadata = { title: "Errors & rate limits", alternates: { canonical: "/docs/errors" } };

const rows: [string, string, string][] = [
  ["400", "invalid_request_error", "The request body is malformed or missing a required field."],
  ["401", "authentication_error", "Missing, invalid, disabled or rotated API key."],
  ["402", "insufficient_quota", "Balance is empty or the key reached its spend limit."],
  ["403", "permission_error", "The account is suspended."],
  ["404", "not_found_error", "Unknown model ID."],
  ["413", "request_too_large", "Body exceeds 32 MB."],
  ["429", "rate_limit_exceeded", "Too many requests. Honour the Retry-After header."],
  ["502", "server_error", "The model provider returned an error."],
  ["503", "overloaded", "The model is temporarily unavailable. Retry with backoff."],
  ["504", "timeout", "The model took too long to respond."],
];

export default function Errors() {
  return (
    <DocPage path="/docs/errors" title="Errors & rate limits" lead="Errors use the same shape as the API format you called.">
      <h2>Error format</h2>
      <CodeBlock lang="json" title="OpenAI format" code={`{ "error": { "message": "Insufficient balance.", "type": "insufficient_quota", "code": 402 } }`} />
      <CodeBlock lang="json" title="Anthropic format" code={`{ "type": "error", "error": { "type": "billing_error", "message": "Insufficient balance." } }`} />
      <h2>Status codes</h2>
      <table>
        <thead><tr><th>Status</th><th>Type</th><th>Meaning</th></tr></thead>
        <tbody>
          {rows.map(([s, t, m]) => (
            <tr key={s}><td><code>{s}</code></td><td><code>{t}</code></td><td>{m}</td></tr>
          ))}
        </tbody>
      </table>
      <h2>Rate limits</h2>
      <p>
        Each key has a requests-per-minute limit (configurable per key). Free models have an additional per-account limit. On{" "}
        <code>429</code>, wait for the number of seconds in <code>Retry-After</code>.
      </p>
      <h2>Retries</h2>
      <p>
        We already retry transient upstream failures and fall back to backup routes before returning an error, so a <code>5xx</code> means
        the model is genuinely unavailable right now. Retry with exponential backoff. Failed requests are never charged.
      </p>
      <p>Every response includes an <code>X-Request-Id</code> header — include it when contacting support.</p>
    </DocPage>
  );
}
