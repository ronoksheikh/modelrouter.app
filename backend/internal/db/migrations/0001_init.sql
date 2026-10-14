-- Money is stored as BIGINT nano-USD (1 USD = 1_000_000_000).
-- Token prices are stored as nano-USD per 1M tokens.

CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL DEFAULT '',
    image           TEXT NOT NULL DEFAULT '',
    email_verified  TIMESTAMPTZ,
    role            TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin')),
    banned          BOOLEAN NOT NULL DEFAULT FALSE,
    country         TEXT NOT NULL DEFAULT '',
    balance         BIGINT NOT NULL DEFAULT 0,
    total_spent     BIGINT NOT NULL DEFAULT 0,
    total_topup     BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX users_created_at_idx ON users (created_at DESC);

CREATE TABLE auth_accounts (
    provider             TEXT NOT NULL,
    provider_account_id  TEXT NOT NULL,
    user_id              UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type                 TEXT NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, provider_account_id)
);
CREATE INDEX auth_accounts_user_idx ON auth_accounts (user_id);

CREATE TABLE auth_verification_tokens (
    identifier  TEXT NOT NULL,
    token_hash  TEXT NOT NULL,
    expires     TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (identifier, token_hash)
);

CREATE TABLE api_keys (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    key_hash      TEXT NOT NULL UNIQUE,
    prefix        TEXT NOT NULL,
    disabled      BOOLEAN NOT NULL DEFAULT FALSE,
    spend_limit   BIGINT,               -- NULL = unlimited
    rpm_limit     INTEGER,              -- NULL = platform default
    total_spent   BIGINT NOT NULL DEFAULT 0,
    total_requests BIGINT NOT NULL DEFAULT 0,
    last_used_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);
CREATE INDEX api_keys_user_idx ON api_keys (user_id) WHERE deleted_at IS NULL;

CREATE TABLE providers (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    kind        TEXT NOT NULL CHECK (kind IN ('openai', 'anthropic')),
    base_url    TEXT NOT NULL,
    api_key_enc TEXT NOT NULL DEFAULT '',
    headers     JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_openrouter BOOLEAN NOT NULL DEFAULT FALSE,
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Raw catalog synced from upstream providers. Never exposed publicly.
CREATE TABLE upstream_models (
    provider_id        BIGINT NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    upstream_id        TEXT NOT NULL,
    name               TEXT NOT NULL DEFAULT '',
    description        TEXT NOT NULL DEFAULT '',
    context_length     INTEGER NOT NULL DEFAULT 0,
    max_output_tokens  INTEGER NOT NULL DEFAULT 0,
    input_price        BIGINT NOT NULL DEFAULT 0,
    output_price       BIGINT NOT NULL DEFAULT 0,
    modalities         TEXT[] NOT NULL DEFAULT '{}',
    supports_tools     BOOLEAN NOT NULL DEFAULT FALSE,
    upstream_created   TIMESTAMPTZ,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (provider_id, upstream_id)
);

-- Public models sold to users. A model maps a public slug to an upstream model.
CREATE TABLE models (
    id                 BIGSERIAL PRIMARY KEY,
    slug               TEXT NOT NULL UNIQUE,
    name               TEXT NOT NULL,
    author             TEXT NOT NULL DEFAULT '',
    description        TEXT NOT NULL DEFAULT '',
    provider_id        BIGINT NOT NULL REFERENCES providers(id),
    upstream_id        TEXT NOT NULL,
    fallback_upstream  TEXT[] NOT NULL DEFAULT '{}',
    context_length     INTEGER NOT NULL DEFAULT 0,
    max_output_tokens  INTEGER NOT NULL DEFAULT 0,   -- 0 = no cap
    pricing_mode       TEXT NOT NULL DEFAULT 'auto' CHECK (pricing_mode IN ('auto', 'manual')),
    markup_bps         INTEGER,                      -- NULL = platform default
    input_price        BIGINT NOT NULL DEFAULT 0,    -- sell price per 1M tokens
    output_price       BIGINT NOT NULL DEFAULT 0,
    request_fee        BIGINT,                       -- NULL = platform default
    modalities         TEXT[] NOT NULL DEFAULT '{text}',
    supports_tools     BOOLEAN NOT NULL DEFAULT FALSE,
    enabled            BOOLEAN NOT NULL DEFAULT TRUE,
    featured           BOOLEAN NOT NULL DEFAULT FALSE,
    is_custom          BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order         INTEGER NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX models_upstream_idx ON models (provider_id, upstream_id);

CREATE TABLE usage_logs (
    id                 BIGSERIAL PRIMARY KEY,
    request_id         TEXT NOT NULL,
    user_id            UUID NOT NULL,
    api_key_id         UUID,
    model_slug         TEXT NOT NULL,
    upstream_id        TEXT NOT NULL,
    provider_id        BIGINT NOT NULL,
    endpoint           TEXT NOT NULL,
    stream             BOOLEAN NOT NULL,
    status             INTEGER NOT NULL,
    input_tokens       INTEGER NOT NULL DEFAULT 0,
    output_tokens      INTEGER NOT NULL DEFAULT 0,
    cached_tokens      INTEGER NOT NULL DEFAULT 0,
    cost               BIGINT NOT NULL DEFAULT 0,
    upstream_cost      BIGINT NOT NULL DEFAULT 0,
    latency_ms         INTEGER NOT NULL DEFAULT 0,
    ttft_ms            INTEGER NOT NULL DEFAULT 0,
    error              TEXT NOT NULL DEFAULT '',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX usage_logs_user_idx ON usage_logs (user_id, created_at DESC);
CREATE INDEX usage_logs_created_idx ON usage_logs (created_at DESC);
CREATE INDEX usage_logs_key_idx ON usage_logs (api_key_id, created_at DESC);

CREATE TABLE payments (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID NOT NULL REFERENCES users(id),
    provider       TEXT NOT NULL,
    external_id    TEXT,
    status         TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'completed', 'failed', 'refunded', 'canceled')),
    currency       TEXT NOT NULL,
    amount_minor   BIGINT NOT NULL,          -- amount charged in the payment currency's minor unit
    credit         BIGINT NOT NULL,          -- nano-USD credited on completion
    meta           JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at   TIMESTAMPTZ
);
CREATE UNIQUE INDEX payments_external_idx ON payments (provider, external_id) WHERE external_id IS NOT NULL;
CREATE INDEX payments_user_idx ON payments (user_id, created_at DESC);
CREATE INDEX payments_created_idx ON payments (created_at DESC);

-- Append-only ledger of balance changes other than per-request usage.
CREATE TABLE credit_ledger (
    id          BIGSERIAL PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount      BIGINT NOT NULL,
    reason      TEXT NOT NULL,       -- topup | admin_adjustment | signup_bonus | refund
    ref         TEXT NOT NULL DEFAULT '',
    actor       TEXT NOT NULL DEFAULT '',
    note        TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX credit_ledger_user_idx ON credit_ledger (user_id, created_at DESC);

CREATE TABLE settings (
    key         TEXT PRIMARY KEY,
    value       JSONB NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE audit_logs (
    id          BIGSERIAL PRIMARY KEY,
    actor_id    UUID,
    actor_email TEXT NOT NULL DEFAULT '',
    action      TEXT NOT NULL,
    target      TEXT NOT NULL DEFAULT '',
    detail      JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_created_idx ON audit_logs (created_at DESC);
