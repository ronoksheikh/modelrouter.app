export type PublicModel = {
  id: string;
  name: string;
  author: string;
  description: string;
  context_length: number;
  max_output_tokens: number;
  input_price_per_m: string;
  output_price_per_m: string;
  request_fee: string;
  modalities: string[];
  supports_tools: boolean;
  featured: boolean;
  free: boolean;
  created_at: string;
};

export type User = {
  id: string;
  email: string;
  name: string;
  image: string;
  role: "user" | "admin";
  banned: boolean;
  country: string;
  balance: number;
  total_spent: number;
  total_topup: number;
  created_at: string;
};

export type ApiKey = {
  id: string;
  user_id: string;
  name: string;
  prefix: string;
  disabled: boolean;
  spend_limit: number | null;
  rpm_limit: number | null;
  total_spent: number;
  total_requests: number;
  last_used_at: string | null;
  created_at: string;
};

export type DailyUsage = { day: string; requests: number; input_tokens: number; output_tokens: number; cost: number; upstream_cost?: number };
export type ModelUsage = { model: string; requests: number; input_tokens: number; output_tokens: number; cost: number; upstream_cost?: number };

export type UsageRecord = {
  id: number;
  request_id: string;
  user_id: string;
  api_key_id: string;
  model: string;
  upstream_id?: string;
  provider_id?: number;
  endpoint: string;
  stream: boolean;
  status: number;
  input_tokens: number;
  output_tokens: number;
  cached_tokens: number;
  cost: number;
  upstream_cost?: number;
  latency_ms: number;
  ttft_ms: number;
  error?: string;
  created_at: string;
};

export type PaymentMethod = {
  id: string;
  name: string;
  description: string;
  currency: string;
  regions: string[];
  logos: string[];
  available: boolean;
};

export type Payment = {
  id: string;
  user_id: string;
  user_email?: string;
  provider: string;
  external_id: string | null;
  status: "pending" | "completed" | "failed" | "refunded" | "canceled";
  currency: string;
  amount_minor: number;
  credit: number;
  created_at: string;
  completed_at: string | null;
};

export type LedgerEntry = { id: number; amount: number; reason: string; ref: string; actor: string; note: string; created_at: string };

export type Provider = {
  id: number;
  name: string;
  kind: "openai" | "anthropic";
  base_url: string;
  has_key: boolean;
  headers: Record<string, string>;
  is_openrouter: boolean;
  enabled: boolean;
  created_at: string;
};

export type AdminModel = {
  id: number;
  slug: string;
  name: string;
  author: string;
  description: string;
  provider_id: number;
  upstream_id: string;
  fallback_upstream: string[];
  context_length: number;
  max_output_tokens: number;
  pricing_mode: "auto" | "manual";
  markup_bps: number | null;
  input_price: number;
  output_price: number;
  request_fee: number | null;
  modalities: string[];
  supports_tools: boolean;
  enabled: boolean;
  featured: boolean;
  is_custom: boolean;
  sort_order: number;
  upstream_input_price: number;
  upstream_output_price: number;
};

export type UpstreamModel = {
  provider_id: number;
  upstream_id: string;
  name: string;
  description: string;
  context_length: number;
  max_output_tokens: number;
  input_price: number;
  output_price: number;
  modalities: string[];
  supports_tools: boolean;
  published_as?: string[];
};

export type Settings = {
  default_markup_bps: number;
  default_request_fee: number;
  signup_bonus: number;
  min_topup_cents: number;
  max_topup_cents: number;
  bdt_per_usd: number;
  default_rpm: number;
  free_model_rpm: number;
  default_max_output_tokens: number;
  auto_publish_new_models: boolean;
  maintenance_mode: boolean;
  announcement: string;
};

export type Overview = {
  users: number;
  new_users: number;
  active_users: number;
  requests: number;
  errors: number;
  input_tokens: number;
  output_tokens: number;
  revenue: number;
  upstream_cost: number;
  topups: number;
  outstanding_balance: number;
  avg_latency_ms: number;
};
