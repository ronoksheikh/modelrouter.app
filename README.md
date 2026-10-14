# ModelRouter

One API for every frontier model. ModelRouter is an OpenAI- and Anthropic-compatible
LLM gateway with pay-as-you-go billing, built for developers in Bangladesh and everywhere else.

```
┌──────────────┐   Auth.js session    ┌──────────────────┐  OpenAI / Anthropic  ┌──────────────┐
│  web/ (Next) │ ── signed JWT ─────▶ │  backend/ (Go)   │ ──── upstream ─────▶ │  OpenRouter  │
│  site, docs, │                      │  gateway, billing│                      │  or any API  │
│  dashboard,  │ ◀─── JSON ────────── │  admin, payments │ ◀──── stream ─────── │              │
│  admin       │                      └────────┬─────────┘                      └──────────────┘
└──────────────┘                               │ pgx
        ▲  customers' SDKs ─── /v1/* ──────────┤
                                        ┌──────▼─────┐
                                        │  Postgres  │
                                        └────────────┘
```

## What's inside

**`backend/` — Go 1.25, one dependency (pgx)**

- `POST /v1/chat/completions` (OpenAI) and `POST /v1/messages` (Anthropic), plus `/v1/models`,
  `/v1/messages/count_tokens` and `/v1/key`. Every model works with both API shapes. Requests,
  responses, tool calls, images and SSE streams are translated in both directions.
- **Public catalog ≠ upstream.** Each public model (`slug`) maps to a provider and an upstream
  model, with optional fallbacks. Responses are rewritten so customers never see the upstream
  model, vendor, request ID, cost or keep-alive comments. Upstream error messages are scrubbed.
- **Custom models:** any ID and price (including free) backed by any upstream model.
- **Pricing:** auto (upstream cost + markup, recomputed on every sync) or manual, plus a per-request
  fee (default `$0.00000001` = 0.000001¢). Money is integer nano-USD everywhere.
- **Token caps:** per-model `max_output_tokens`, enforced on every request.
- **Hot path never touches the database:** model catalog and settings live in an immutable
  in-memory snapshot; API keys are cached (hash lookup); balances are tracked by an in-memory
  ledger and flushed to Postgres in one batched transaction per second (`COPY` for usage logs).
- **Instant invalidation across replicas** via Postgres `LISTEN/NOTIFY` (key rotation, bans,
  top-ups, catalog edits).
- Retries and fallbacks before the first byte, per-key and free-tier rate limits, body size limits,
  graceful shutdown that drains streams and flushes billing.
- **Payments:** a `payments.Provider` interface with region routing. Paddle (cards/PayPal) for
  international users; an SSLCommerz implementation (bKash, Nagad, Rocket, local cards) that stays
  off until configured. Unconfigured Bangladeshi methods appear as "coming soon".
- **Admin API** for users, balances, keys, models, upstream catalog, providers, payments, request
  logs, settings and a full audit log.
- Auth.js adapter endpoints, so Postgres is the single source of truth for users.

**`web/` — Next.js 16 (App Router), Tailwind v4, Auth.js v5**

- Marketing pages, model catalog and docs are static/ISR with JSON-LD, sitemap, robots, OG image
  and canonical URLs. Code blocks are highlighted on the server (Shiki), and code tabs are CSS-only.
  Pages ship no JavaScript of their own beyond tiny islands (copy button, theme, search).
- Sign in with Google, GitHub or email magic links (Resend).
- Dashboard: balance, usage chart, top models, API keys (create/rotate/disable/limits), request
  log, top-ups (Paddle overlay or redirect; BDT pricing for Bangladesh), settings.
- Admin panel: overview with revenue/profit, users (credit, ban, promote), models (custom models,
  pricing, caps, fallbacks), upstream catalog (bulk publish), providers (sync), payments, request
  logs, audit log, platform settings.

## Quick start (local)

Requirements: Go 1.25+, Node 22+, Postgres 14+.

```bash
# 1. Database
createdb modelrouter

# 2. API
cd backend
cp .env.example .env            # set secrets, ADMIN_EMAILS and OPENROUTER_API_KEY
set -a && . ./.env && set +a
go run ./cmd/modelrouter        # migrates, creates the default provider, syncs models

# 3. Web
cd ../web
cp .env.example .env.local      # same JWT/internal secrets, AUTH_SECRET, an OAuth provider or Resend
npm install
npm run dev                     # http://localhost:3000
```

Sign in with an email listed in `ADMIN_EMAILS`, open **Admin → Upstream catalog** and publish
the models you want to sell (or turn on *Auto-publish* in **Admin → Settings**).

Or run everything with Docker: `docker compose up --build` after filling `backend/.env` and `web/.env`.

## Tests

```bash
cd backend && TEST_DATABASE_URL=postgres://…/modelrouter_test go test -race ./...
cd web && npm run typecheck && npm test && npm run build
```

The backend suite covers pricing math, the ledger (including failure and retry), all four
translation directions with streaming, the gateway against fake OpenAI and Anthropic upstreams
(leak scrubbing, caps, fallbacks, 401/402/404/429), payments (Paddle signatures, SSLCommerz
validation), and an end-to-end test against real Postgres: sign-up → key → billing → rotation →
idempotent webhook → admin.

## Adding a Bangladeshi payment method

1. Create `backend/internal/payments/<name>/` implementing `payments.Provider`:
   - `Method()` — name, `Currency: "BDT"`, `Regions: []string{payments.RegionBD}`.
   - `CreateCheckout()` — create the gateway session and return `RedirectURL`. Echo
     `req.PaymentID` as the gateway's transaction reference.
   - `ParseWebhook()` — authenticate the callback (signature, or a server-to-server validation call
     as `sslcommerz` does) and return a `WebhookEvent` with the payment ID, amount and status.
2. Register it in `buildPayments` in `backend/internal/server/server.go` behind its env config.
3. Point the gateway's IPN/webhook URL to `https://<api>/webhooks/<provider-id>`.

Credits are applied exactly once (`pending → completed` in one transaction). The amount and currency
are checked against the stored payment, and BDT amounts use the admin-configured rate.

## Production notes

- Run several API replicas behind a load balancer. They coordinate through Postgres, and there is no
  other shared state. Raise `DB_MAX_CONNS` and Postgres `max_connections` together.
- Set `PUBLIC_URL`/`WEB_URL` to your real domains and configure the Paddle webhook at
  `https://<api>/webhooks/paddle` (events: `transaction.completed`, `transaction.payment_failed`).
- Keep `ENCRYPTION_KEY` stable. It encrypts provider API keys at rest.
- Usage logs are pruned after `USAGE_RETENTION_DAYS` (default 90).
