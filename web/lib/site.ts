export const site = {
  name: "ModelRouter",
  url: (process.env.NEXT_PUBLIC_SITE_URL || "http://localhost:3000").replace(/\/$/, ""),
  apiBase: (process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8080").replace(/\/$/, "") + "/v1",
  description:
    "One API for every frontier model. OpenAI- and Anthropic-compatible, pay-as-you-go pricing, local payments for Bangladesh and cards worldwide.",
  tagline: "One API for every frontier model",
  twitter: "@modelrouter",
  email: "support@modelrouter.app",
};

export const nav = [
  { href: "/models", label: "Models" },
  { href: "/pricing", label: "Pricing" },
  { href: "/docs", label: "Docs" },
];
