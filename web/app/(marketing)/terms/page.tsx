import type { Metadata } from "next";
import { LegalPage } from "@/components/site/legal";

export const metadata: Metadata = { title: "Terms of Service", alternates: { canonical: "/terms" } };

export default function Terms() {
  return (
    <LegalPage title="Terms of Service" updated="October 2026">
      <p>These terms govern your use of ModelRouter&apos;s website and API. By creating an account you agree to them.</p>
      <h2>The service</h2>
      <p>ModelRouter provides an API that gives access to AI language models from multiple providers through a single endpoint. Availability of individual models may change.</p>
      <h2>Accounts and keys</h2>
      <p>You are responsible for keeping your API keys secret and for all usage made with them. Rotate or revoke a key immediately if you believe it was exposed.</p>
      <h2>Credits and billing</h2>
      <ul>
        <li>The service is prepaid. Usage is deducted from your credit balance at the prices shown at the time of the request.</li>
        <li>Credits do not expire and are not transferable.</li>
        <li>Prices may change; changes never apply retroactively to completed requests.</li>
      </ul>
      <h2>Acceptable use</h2>
      <p>You may not use the service for unlawful purposes, to generate content that exploits minors, to attack systems, or to violate the usage policies of the underlying model creators. We may suspend accounts that do.</p>
      <h2>Disclaimer</h2>
      <p>Model outputs can be inaccurate. The service is provided &quot;as is&quot; without warranties, and our liability is limited to the amount you paid in the 12 months before a claim.</p>
      <h2>Contact</h2>
      <p>Questions? Email <a href="mailto:support@modelrouter.app">support@modelrouter.app</a>.</p>
    </LegalPage>
  );
}
