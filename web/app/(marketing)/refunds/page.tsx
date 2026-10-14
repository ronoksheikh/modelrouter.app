import type { Metadata } from "next";
import { LegalPage } from "@/components/site/legal";

export const metadata: Metadata = { title: "Refund Policy", alternates: { canonical: "/refunds" } };

export default function Refunds() {
  return (
    <LegalPage title="Refund Policy" updated="October 2026">
      <p>We want you to only pay for what you use.</p>
      <h2>Unused credits</h2>
      <p>You can request a refund of unused credits within 14 days of purchase. Refunds go back to the original payment method, minus any credits already consumed.</p>
      <h2>Failed requests</h2>
      <p>Requests that fail on our side are never charged. If you believe you were charged for a failed request, contact us with the request ID shown in your activity log.</p>
      <h2>How to request</h2>
      <p>Email <a href="mailto:support@modelrouter.app">support@modelrouter.app</a> from your account email with the payment date and amount.</p>
    </LegalPage>
  );
}
