import { ArrowRight } from "lucide-react";
import Link from "next/link";
import { PageHeader, Stat } from "@/components/app/shell";
import { BarChart } from "@/components/charts/bar-chart";
import { ButtonLink } from "@/components/ui/button";
import { CodeBlock } from "@/components/ui/code";
import { Card, EmptyState } from "@/components/ui/primitives";
import { backend } from "@/lib/backend";
import { formatNumber, formatTokens, formatUSD } from "@/lib/format";
import { requireUser } from "@/lib/session";
import { site } from "@/lib/site";
import type { ApiKey, DailyUsage, ModelUsage, User } from "@/lib/types";

export default async function Overview() {
  const user = await requireUser();
  const [me, daily, models, keys] = await Promise.all([
    backend<{ user: User; available_balance: number }>("/api/me", { userId: user.id }),
    backend<{ data: DailyUsage[] }>("/api/usage/daily?days=30", { userId: user.id }),
    backend<{ data: ModelUsage[] }>("/api/usage/models?days=30", { userId: user.id }),
    backend<{ data: ApiKey[] }>("/api/keys", { userId: user.id }),
  ]);
  const spend = daily.data.reduce((a, d) => a + d.cost, 0);
  const requests = daily.data.reduce((a, d) => a + d.requests, 0);
  const tokens = daily.data.reduce((a, d) => a + d.input_tokens + d.output_tokens, 0);
  const firstName = (me.user.name || "").split(" ")[0];

  return (
    <>
      <PageHeader
        title={firstName ? `Welcome back, ${firstName}` : "Overview"}
        description="Your usage over the last 30 days."
        actions={<ButtonLink href="/dashboard/billing" size="sm">Add credits</ButtonLink>}
      />
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <Stat label="Balance" value={formatUSD(me.available_balance)} hint={me.available_balance <= 0 ? "Top up to use paid models" : "Available now"} />
        <Stat label="Spend · 30d" value={formatUSD(spend, { precise: spend < 10_000_000 })} />
        <Stat label="Requests · 30d" value={formatNumber(requests)} />
        <Stat label="Tokens · 30d" value={formatTokens(tokens)} />
      </div>

      <div className="mt-4 grid gap-4 lg:grid-cols-[1.6fr_1fr]">
        <Card className="p-6">
          <div className="flex items-baseline justify-between">
            <h2 className="font-medium">Daily spend</h2>
            <span className="text-xs text-subtle">USD</span>
          </div>
          <BarChart
            className="mt-8"
            title="Daily spend over the last 30 days"
            data={daily.data.map((d) => ({
              key: d.day,
              label: new Date(d.day + "T00:00:00Z").toLocaleDateString("en-US", { month: "short", day: "numeric", timeZone: "UTC" }),
              value: d.cost,
              display: `${formatUSD(d.cost, { precise: true })} · ${formatNumber(d.requests)} req`,
            }))}
          />
        </Card>
        <Card className="p-6">
          <h2 className="font-medium">Top models</h2>
          {models.data.length === 0 ? (
            <p className="mt-6 text-sm text-muted">No requests yet.</p>
          ) : (
            <ul className="mt-4 space-y-3">
              {models.data.slice(0, 6).map((m) => {
                const share = spend > 0 ? (m.cost / spend) * 100 : 0;
                return (
                  <li key={m.model}>
                    <div className="flex items-baseline justify-between gap-3 text-sm">
                      <span className="truncate font-mono text-[12.5px]">{m.model}</span>
                      <span className="shrink-0 tabular-nums text-muted">{formatUSD(m.cost, { precise: m.cost < 10_000_000 })}</span>
                    </div>
                    <div className="mt-1.5 h-1.5 rounded-full bg-bg-soft">
                      <div className="h-full rounded-full bg-accent/80" style={{ width: `${Math.max(share, 1)}%` }} />
                    </div>
                  </li>
                );
              })}
            </ul>
          )}
        </Card>
      </div>

      {keys.data.length === 0 ? (
        <div className="mt-4">
          <EmptyState
            title="Create your first API key"
            action={<ButtonLink href="/dashboard/keys">Create key <ArrowRight className="size-4" /></ButtonLink>}
          >
            Then point any OpenAI or Anthropic SDK at ModelRouter.
          </EmptyState>
        </div>
      ) : (
        <Card className="mt-4 p-6">
          <div className="flex items-center justify-between">
            <h2 className="font-medium">Quick start</h2>
            <Link href="/docs/quickstart" className="text-sm text-accent hover:underline">Docs</Link>
          </div>
          <CodeBlock
            className="!mb-0"
            lang="bash"
            code={`curl ${site.apiBase}/chat/completions \\
  -H "Authorization: Bearer $MODELROUTER_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model": "openai/gpt-4o-mini", "messages": [{"role": "user", "content": "Hello"}]}'`}
          />
        </Card>
      )}
    </>
  );
}
