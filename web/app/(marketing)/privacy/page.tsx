import type { Metadata } from "next";
import { LegalPage } from "@/components/site/legal";

export const metadata: Metadata = { title: "Privacy Policy", alternates: { canonical: "/privacy" } };

export default function Privacy() {
  return (
    <LegalPage title="Privacy Policy" updated="October 2026">
      <p>We collect only what we need to run the service.</p>
      <h2>What we store</h2>
      <ul>
        <li>Account data: your email, name and avatar from your sign-in provider.</li>
        <li>Usage metadata: model, token counts, cost, latency and status of each request.</li>
        <li>Payment records: amounts and statuses. Card and wallet details are handled by our payment processors and never touch our servers.</li>
      </ul>
      <h2>Prompts and completions</h2>
      <p>We do not store the content of your prompts or completions. Requests are forwarded to model providers to generate responses and are subject to their data policies.</p>
      <h2>Retention</h2>
      <p>Request metadata is kept for 90 days. You can delete your account at any time by contacting support.</p>
      <h2>Contact</h2>
      <p>Email <a href="mailto:support@modelrouter.app">support@modelrouter.app</a> with any privacy request.</p>
    </LegalPage>
  );
}
