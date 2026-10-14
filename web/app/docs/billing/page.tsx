import type { Metadata } from "next";
import { DocPage } from "@/components/docs/page";

export const metadata: Metadata = { title: "Billing & payments", alternates: { canonical: "/docs/billing" } };

export default function BillingDocs() {
  return (
    <DocPage path="/docs/billing" title="Billing & payments" lead="Prepaid credits, charged per request.">
      <h2>How a request is priced</h2>
      <p>
        <code>cost = input_tokens × input_price + output_tokens × output_price + request_fee</code>
      </p>
      <p>
        Token prices are per million tokens and listed on every model page. The request fee is a flat amount per request, a tiny fraction of a
        cent. Requests that fail are not charged.
      </p>
      <h2>Topping up</h2>
      <ul>
        <li><strong>International:</strong> Visa, Mastercard, Amex, Apple Pay, Google Pay and PayPal, processed by Paddle in USD.</li>
        <li><strong>Bangladesh:</strong> pay in BDT with local methods such as bKash and Nagad. The BDT amount is shown before you pay.</li>
      </ul>
      <p>Credits are added as soon as the payment is confirmed — usually within seconds — and never expire.</p>
      <h2>Controlling spend</h2>
      <ul>
        <li>Give each key a spend limit.</li>
        <li>Watch daily cost and per-model usage in the dashboard.</li>
        <li>Every request appears in your activity log with its exact cost.</li>
      </ul>
      <h2>Invoices & refunds</h2>
      <p>
        Paddle emails a receipt for every card payment. See the <a href="/refunds">refund policy</a> for unused credits.
      </p>
    </DocPage>
  );
}
