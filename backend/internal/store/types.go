package store

import "time"

type User struct {
	ID            string     `json:"id"`
	Email         string     `json:"email"`
	Name          string     `json:"name"`
	Image         string     `json:"image"`
	EmailVerified *time.Time `json:"email_verified"`
	Role          string     `json:"role"`
	Banned        bool       `json:"banned"`
	Country       string     `json:"country"`
	Balance       int64      `json:"balance"`
	TotalSpent    int64      `json:"total_spent"`
	TotalTopup    int64      `json:"total_topup"`
	CreatedAt     time.Time  `json:"created_at"`
}

type APIKey struct {
	ID            string     `json:"id"`
	UserID        string     `json:"user_id"`
	Name          string     `json:"name"`
	Prefix        string     `json:"prefix"`
	Disabled      bool       `json:"disabled"`
	SpendLimit    *int64     `json:"spend_limit"`
	RPMLimit      *int       `json:"rpm_limit"`
	TotalSpent    int64      `json:"total_spent"`
	TotalRequests int64      `json:"total_requests"`
	LastUsedAt    *time.Time `json:"last_used_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

// KeyAuth is everything the gateway needs to authorize a request, loaded with a
// single indexed query and cached in memory.
type KeyAuth struct {
	KeyID      string
	KeyHash    string
	UserID     string
	Disabled   bool
	SpendLimit *int64
	KeySpent   int64
	RPMLimit   *int
	UserBanned bool
}

type Provider struct {
	ID           int64             `json:"id"`
	Name         string            `json:"name"`
	Kind         string            `json:"kind"` // openai | anthropic
	BaseURL      string            `json:"base_url"`
	APIKeyEnc    string            `json:"-"`
	HasKey       bool              `json:"has_key"`
	Headers      map[string]string `json:"headers"`
	IsOpenRouter bool              `json:"is_openrouter"`
	Enabled      bool              `json:"enabled"`
	CreatedAt    time.Time         `json:"created_at"`
}

type UpstreamModel struct {
	ProviderID      int64      `json:"provider_id"`
	UpstreamID      string     `json:"upstream_id"`
	Name            string     `json:"name"`
	Description     string     `json:"description"`
	ContextLength   int        `json:"context_length"`
	MaxOutputTokens int        `json:"max_output_tokens"`
	InputPrice      int64      `json:"input_price"`
	OutputPrice     int64      `json:"output_price"`
	Modalities      []string   `json:"modalities"`
	SupportsTools   bool       `json:"supports_tools"`
	UpstreamCreated *time.Time `json:"upstream_created"`
	UpdatedAt       time.Time  `json:"updated_at"`
	PublishedAs     []string   `json:"published_as,omitempty"`
}

type Model struct {
	ID               int64     `json:"id"`
	Slug             string    `json:"slug"`
	Name             string    `json:"name"`
	Author           string    `json:"author"`
	Description      string    `json:"description"`
	ProviderID       int64     `json:"provider_id"`
	UpstreamID       string    `json:"upstream_id"`
	FallbackUpstream []string  `json:"fallback_upstream"`
	ContextLength    int       `json:"context_length"`
	MaxOutputTokens  int       `json:"max_output_tokens"`
	PricingMode      string    `json:"pricing_mode"`
	MarkupBps        *int      `json:"markup_bps"`
	InputPrice       int64     `json:"input_price"`
	OutputPrice      int64     `json:"output_price"`
	RequestFee       *int64    `json:"request_fee"`
	Modalities       []string  `json:"modalities"`
	SupportsTools    bool      `json:"supports_tools"`
	Enabled          bool      `json:"enabled"`
	Featured         bool      `json:"featured"`
	IsCustom         bool      `json:"is_custom"`
	SortOrder        int       `json:"sort_order"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	// Joined, admin only.
	UpstreamInputPrice  int64 `json:"upstream_input_price"`
	UpstreamOutputPrice int64 `json:"upstream_output_price"`
}

type UsageRecord struct {
	ID           int64     `json:"id"`
	RequestID    string    `json:"request_id"`
	UserID       string    `json:"user_id"`
	APIKeyID     string    `json:"api_key_id"`
	ModelSlug    string    `json:"model"`
	UpstreamID   string    `json:"upstream_id,omitempty"`
	ProviderID   int64     `json:"provider_id,omitempty"`
	Endpoint     string    `json:"endpoint"`
	Stream       bool      `json:"stream"`
	Status       int       `json:"status"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	CachedTokens int       `json:"cached_tokens"`
	Cost         int64     `json:"cost"`
	UpstreamCost int64     `json:"upstream_cost,omitempty"`
	LatencyMS    int       `json:"latency_ms"`
	TTFTMS       int       `json:"ttft_ms"`
	Error        string    `json:"error,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type Payment struct {
	ID          string         `json:"id"`
	UserID      string         `json:"user_id"`
	UserEmail   string         `json:"user_email,omitempty"`
	Provider    string         `json:"provider"`
	ExternalID  *string        `json:"external_id"`
	Status      string         `json:"status"`
	Currency    string         `json:"currency"`
	AmountMinor int64          `json:"amount_minor"`
	Credit      int64          `json:"credit"`
	Meta        map[string]any `json:"meta"`
	CreatedAt   time.Time      `json:"created_at"`
	CompletedAt *time.Time     `json:"completed_at"`
}

type LedgerEntry struct {
	ID        int64     `json:"id"`
	UserID    string    `json:"user_id"`
	Amount    int64     `json:"amount"`
	Reason    string    `json:"reason"`
	Ref       string    `json:"ref"`
	Actor     string    `json:"actor"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

type AuditLog struct {
	ID         int64          `json:"id"`
	ActorID    *string        `json:"actor_id"`
	ActorEmail string         `json:"actor_email"`
	Action     string         `json:"action"`
	Target     string         `json:"target"`
	Detail     map[string]any `json:"detail"`
	CreatedAt  time.Time      `json:"created_at"`
}

// Settings are platform-wide knobs editable from the admin panel.
type Settings struct {
	DefaultMarkupBps       int     `json:"default_markup_bps"`
	DefaultRequestFee      int64   `json:"default_request_fee"`
	SignupBonus            int64   `json:"signup_bonus"`
	MinTopupCents          int64   `json:"min_topup_cents"`
	MaxTopupCents          int64   `json:"max_topup_cents"`
	BDTPerUSD              float64 `json:"bdt_per_usd"`
	DefaultRPM             int     `json:"default_rpm"`
	FreeModelRPM           int     `json:"free_model_rpm"`
	DefaultMaxOutputTokens int     `json:"default_max_output_tokens"`
	AutoPublishNewModels   bool    `json:"auto_publish_new_models"`
	MaintenanceMode        bool    `json:"maintenance_mode"`
	Announcement           string  `json:"announcement"`
}

func DefaultSettings() Settings {
	return Settings{
		DefaultMarkupBps:  1000, // 10%
		DefaultRequestFee: 10,   // 0.000001 cents
		MinTopupCents:     500,
		MaxTopupCents:     500_000,
		BDTPerUSD:         122,
		DefaultRPM:        600,
		FreeModelRPM:      20,
	}
}

type DailyUsage struct {
	Day          string `json:"day"`
	Requests     int64  `json:"requests"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	Cost         int64  `json:"cost"`
	UpstreamCost int64  `json:"upstream_cost,omitempty"`
}

type ModelUsage struct {
	Model        string `json:"model"`
	Requests     int64  `json:"requests"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	Cost         int64  `json:"cost"`
	UpstreamCost int64  `json:"upstream_cost,omitempty"`
}
