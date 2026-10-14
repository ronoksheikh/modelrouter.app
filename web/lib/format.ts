export const NANO_PER_USD = 1_000_000_000;

/** Formats nano-USD as a dollar string. Tiny amounts keep enough precision to be meaningful. */
export function formatUSD(nano: number, opts: { precise?: boolean } = {}): string {
  const usd = nano / NANO_PER_USD;
  const abs = Math.abs(usd);
  const sign = usd < 0 ? "-" : "";
  if (abs === 0) return "$0.00";
  if (abs >= 0.01 && !opts.precise) {
    return sign + "$" + abs.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  }
  if (abs >= 0.01) {
    return sign + "$" + abs.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 6 });
  }
  return sign + "$" + trimZeros(abs.toFixed(9));
}

/** Formats a decimal USD string coming from the API (e.g. "0.165"). */
export function formatPriceString(usd: string): string {
  const n = Number(usd);
  if (!Number.isFinite(n) || n === 0) return "Free";
  if (n >= 1) return "$" + trimZeros(n.toFixed(2));
  if (n >= 0.01) return "$" + trimZeros(n.toFixed(4));
  return "$" + trimZeros(n.toFixed(9));
}

function trimZeros(s: string): string {
  if (!s.includes(".")) return s;
  s = s.replace(/0+$/, "");
  return s.endsWith(".") ? s.slice(0, -1) : s;
}

/** 1_048_576 → "1.05M", 128000 → "128K". */
export function formatTokens(n: number): string {
  if (!n) return "—";
  if (n >= 1_000_000) return trimZeros((n / 1_000_000).toFixed(2)) + "M";
  if (n >= 1_000) return trimZeros((n / 1_000).toFixed(n >= 100_000 ? 0 : 1)) + "K";
  return String(n);
}

export function formatNumber(n: number): string {
  return new Intl.NumberFormat("en-US", { notation: n >= 100_000 ? "compact" : "standard", maximumFractionDigits: 1 }).format(n);
}

export function formatDate(iso: string | Date): string {
  const d = typeof iso === "string" ? new Date(iso) : iso;
  return d.toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" });
}

export function formatDateTime(iso: string | Date): string {
  const d = typeof iso === "string" ? new Date(iso) : iso;
  return d.toLocaleString("en-US", { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" });
}

export function timeAgo(iso: string | null | undefined, now = Date.now()): string {
  if (!iso) return "Never";
  const s = Math.max(0, Math.round((now - new Date(iso).getTime()) / 1000));
  if (s < 60) return "Just now";
  const m = Math.round(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.round(m / 60);
  if (h < 24) return `${h}h ago`;
  const d = Math.round(h / 24);
  if (d < 30) return `${d}d ago`;
  return formatDate(iso);
}

/** Converts USD cents to BDT using the platform rate, rounded up to whole taka. */
export function centsToBDT(cents: number, rate: number): number {
  return Math.ceil((cents / 100) * rate);
}

/** Basis points → "10%". */
export function formatBps(bps: number): string {
  return trimZeros((bps / 100).toFixed(2)) + "%";
}

/** Model author slug → display name. */
export function authorName(author: string): string {
  const map: Record<string, string> = {
    openai: "OpenAI", anthropic: "Anthropic", google: "Google", "meta-llama": "Meta", meta: "Meta",
    mistralai: "Mistral", deepseek: "DeepSeek", qwen: "Qwen", "x-ai": "xAI", cohere: "Cohere",
    microsoft: "Microsoft", nvidia: "NVIDIA", amazon: "Amazon", perplexity: "Perplexity", moonshotai: "Moonshot",
    "z-ai": "Z.ai", minimax: "MiniMax", modelrouter: "ModelRouter",
  };
  if (!author) return "ModelRouter";
  return map[author.toLowerCase()] ?? author.charAt(0).toUpperCase() + author.slice(1);
}
