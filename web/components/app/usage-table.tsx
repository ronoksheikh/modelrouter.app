import { Badge } from "@/components/ui/primitives";
import { formatDateTime, formatNumber, formatUSD } from "@/lib/format";
import type { UsageRecord } from "@/lib/types";

export function UsageTable({ rows, admin }: { rows: UsageRecord[]; admin?: boolean }) {
  return (
    <div className="overflow-hidden rounded-2xl border border-border bg-surface shadow-soft">
      <div className="overflow-x-auto">
        <table className="w-full min-w-[820px] text-sm">
          <thead>
            <tr className="border-b border-border text-left text-[12px] text-subtle">
              <th className="px-5 py-3 font-medium">Time</th>
              <th className="px-5 py-3 font-medium">Model</th>
              {admin ? <th className="px-5 py-3 font-medium">Upstream</th> : null}
              <th className="px-5 py-3 font-medium">API</th>
              <th className="px-5 py-3 text-right font-medium">Tokens in / out</th>
              <th className="px-5 py-3 text-right font-medium">Cost</th>
              {admin ? <th className="px-5 py-3 text-right font-medium">Upstream cost</th> : null}
              <th className="px-5 py-3 text-right font-medium">Latency</th>
              <th className="px-5 py-3 font-medium">Status</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.id} className="border-b border-border last:border-0" title={r.error || r.request_id}>
                <td className="whitespace-nowrap px-5 py-3 text-muted">{formatDateTime(r.created_at)}</td>
                <td className="max-w-[220px] truncate px-5 py-3 font-mono text-[12.5px]">{r.model}</td>
                {admin ? <td className="max-w-[200px] truncate px-5 py-3 font-mono text-[12px] text-muted">{r.upstream_id}</td> : null}
                <td className="px-5 py-3 text-muted">
                  {r.endpoint === "anthropic" ? "Anthropic" : "OpenAI"}
                  {r.stream ? <span className="text-subtle"> · stream</span> : null}
                </td>
                <td className="whitespace-nowrap px-5 py-3 text-right tabular-nums">
                  {formatNumber(r.input_tokens)} <span className="text-subtle">/</span> {formatNumber(r.output_tokens)}
                </td>
                <td className="whitespace-nowrap px-5 py-3 text-right tabular-nums">{formatUSD(r.cost, { precise: true })}</td>
                {admin ? <td className="whitespace-nowrap px-5 py-3 text-right tabular-nums text-muted">{formatUSD(r.upstream_cost ?? 0, { precise: true })}</td> : null}
                <td className="whitespace-nowrap px-5 py-3 text-right tabular-nums text-muted">
                  {(r.latency_ms / 1000).toFixed(2)}s
                </td>
                <td className="px-5 py-3">
                  <Badge tone={r.status === 200 ? "good" : r.status === 499 ? "neutral" : "bad"}>{r.status === 499 ? "Cancelled" : r.status}</Badge>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
