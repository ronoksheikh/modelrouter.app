export const docsNav = [
  {
    title: "Getting started",
    items: [
      { href: "/docs", title: "Introduction" },
      { href: "/docs/quickstart", title: "Quickstart" },
      { href: "/docs/authentication", title: "Authentication & keys" },
    ],
  },
  {
    title: "API",
    items: [
      { href: "/docs/openai", title: "OpenAI-compatible API" },
      { href: "/docs/anthropic", title: "Anthropic-compatible API" },
      { href: "/docs/streaming", title: "Streaming" },
      { href: "/docs/tools", title: "Tools & vision" },
      { href: "/docs/models", title: "Models" },
      { href: "/docs/errors", title: "Errors & rate limits" },
    ],
  },
  {
    title: "Guides",
    items: [
      { href: "/docs/integrations", title: "Use with your tools" },
      { href: "/docs/billing", title: "Billing & payments" },
    ],
  },
];

export const docsFlat = docsNav.flatMap((g) => g.items);
