package store

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ---------- Providers ----------

const providerCols = `id, name, kind, base_url, api_key_enc, headers, is_openrouter, enabled, created_at`

func scanProvider(row pgx.Row) (*Provider, error) {
	var p Provider
	if err := row.Scan(&p.ID, &p.Name, &p.Kind, &p.BaseURL, &p.APIKeyEnc, &p.Headers, &p.IsOpenRouter, &p.Enabled, &p.CreatedAt); err != nil {
		return nil, notFound(err)
	}
	p.HasKey = p.APIKeyEnc != ""
	if p.Headers == nil {
		p.Headers = map[string]string{}
	}
	return &p, nil
}

func (s *Store) ListProviders(ctx context.Context) ([]Provider, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+providerCols+` FROM providers ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Provider{}
	for rows.Next() {
		p, err := scanProvider(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (s *Store) GetProvider(ctx context.Context, id int64) (*Provider, error) {
	return scanProvider(s.Pool.QueryRow(ctx, `SELECT `+providerCols+` FROM providers WHERE id=$1`, id))
}

func (s *Store) CreateProvider(ctx context.Context, p Provider) (*Provider, error) {
	if p.Headers == nil {
		p.Headers = map[string]string{}
	}
	return scanProvider(s.Pool.QueryRow(ctx, `INSERT INTO providers (name, kind, base_url, api_key_enc, headers, is_openrouter, enabled)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING `+providerCols,
		p.Name, p.Kind, strings.TrimRight(p.BaseURL, "/"), p.APIKeyEnc, p.Headers, p.IsOpenRouter, p.Enabled))
}

type ProviderPatch struct {
	Name         *string
	Kind         *string
	BaseURL      *string
	APIKeyEnc    *string
	Headers      map[string]string
	IsOpenRouter *bool
	Enabled      *bool
}

func (s *Store) UpdateProvider(ctx context.Context, id int64, p ProviderPatch) (*Provider, error) {
	var base *string
	if p.BaseURL != nil {
		b := strings.TrimRight(*p.BaseURL, "/")
		base = &b
	}
	var headers any
	if p.Headers != nil {
		headers = p.Headers
	}
	return scanProvider(s.Pool.QueryRow(ctx, `UPDATE providers SET
		name = COALESCE($2, name), kind = COALESCE($3, kind), base_url = COALESCE($4, base_url),
		api_key_enc = COALESCE($5, api_key_enc), headers = COALESCE($6, headers),
		is_openrouter = COALESCE($7, is_openrouter), enabled = COALESCE($8, enabled), updated_at = now()
		WHERE id=$1 RETURNING `+providerCols, id, p.Name, p.Kind, base, p.APIKeyEnc, headers, p.IsOpenRouter, p.Enabled))
}

func (s *Store) DeleteProvider(ctx context.Context, id int64) error {
	var n int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM models WHERE provider_id=$1`, id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return ErrConflict
	}
	tag, err := s.Pool.Exec(ctx, `DELETE FROM providers WHERE id=$1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// ---------- Upstream catalog ----------

// UpsertUpstreamModels stores the synced upstream catalog and returns the IDs of
// models that did not exist before.
func (s *Store) UpsertUpstreamModels(ctx context.Context, providerID int64, ms []UpstreamModel) ([]string, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	existing := map[string]bool{}
	rows, err := tx.Query(ctx, `SELECT upstream_id FROM upstream_models WHERE provider_id=$1`, providerID)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		existing[id] = true
	}
	batch := &pgx.Batch{}
	var added []string
	for _, m := range ms {
		if !existing[m.UpstreamID] {
			added = append(added, m.UpstreamID)
		}
		if m.Modalities == nil {
			m.Modalities = []string{"text"}
		}
		batch.Queue(`INSERT INTO upstream_models (provider_id, upstream_id, name, description, context_length, max_output_tokens,
			input_price, output_price, modalities, supports_tools, upstream_created, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11, now())
			ON CONFLICT (provider_id, upstream_id) DO UPDATE SET name=EXCLUDED.name, description=EXCLUDED.description,
			context_length=EXCLUDED.context_length, max_output_tokens=EXCLUDED.max_output_tokens,
			input_price=EXCLUDED.input_price, output_price=EXCLUDED.output_price, modalities=EXCLUDED.modalities,
			supports_tools=EXCLUDED.supports_tools, upstream_created=EXCLUDED.upstream_created, updated_at=now()`,
			providerID, m.UpstreamID, m.Name, m.Description, m.ContextLength, m.MaxOutputTokens,
			m.InputPrice, m.OutputPrice, m.Modalities, m.SupportsTools, m.UpstreamCreated)
	}
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return nil, err
	}
	return added, tx.Commit(ctx)
}

func (s *Store) ListUpstreamModels(ctx context.Context, providerID int64, q string, limit, offset int) ([]UpstreamModel, int64, error) {
	like := "%" + strings.ToLower(strings.TrimSpace(q)) + "%"
	var total int64
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM upstream_models WHERE ($1 = 0 OR provider_id=$1)
		AND (lower(upstream_id) LIKE $2 OR lower(name) LIKE $2)`, providerID, like).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT u.provider_id, u.upstream_id, u.name, u.description, u.context_length, u.max_output_tokens,
		u.input_price, u.output_price, u.modalities, u.supports_tools, u.upstream_created, u.updated_at,
		COALESCE((SELECT array_agg(m.slug) FROM models m WHERE m.provider_id=u.provider_id AND m.upstream_id=u.upstream_id), '{}')
		FROM upstream_models u WHERE ($1 = 0 OR u.provider_id=$1) AND (lower(u.upstream_id) LIKE $2 OR lower(u.name) LIKE $2)
		ORDER BY u.upstream_created DESC NULLS LAST, u.upstream_id LIMIT $3 OFFSET $4`, providerID, like, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []UpstreamModel{}
	for rows.Next() {
		var m UpstreamModel
		if err := rows.Scan(&m.ProviderID, &m.UpstreamID, &m.Name, &m.Description, &m.ContextLength, &m.MaxOutputTokens,
			&m.InputPrice, &m.OutputPrice, &m.Modalities, &m.SupportsTools, &m.UpstreamCreated, &m.UpdatedAt, &m.PublishedAs); err != nil {
			return nil, 0, err
		}
		out = append(out, m)
	}
	return out, total, rows.Err()
}

// ---------- Public models ----------

const modelCols = `m.id, m.slug, m.name, m.author, m.description, m.provider_id, m.upstream_id, m.fallback_upstream,
	m.context_length, m.max_output_tokens, m.pricing_mode, m.markup_bps, m.input_price, m.output_price, m.request_fee,
	m.modalities, m.supports_tools, m.enabled, m.featured, m.is_custom, m.sort_order, m.created_at, m.updated_at,
	COALESCE(u.input_price, 0), COALESCE(u.output_price, 0)`

const modelFrom = ` FROM models m LEFT JOIN upstream_models u ON u.provider_id = m.provider_id AND u.upstream_id = m.upstream_id `

func scanModel(row pgx.Row) (*Model, error) {
	var m Model
	if err := row.Scan(&m.ID, &m.Slug, &m.Name, &m.Author, &m.Description, &m.ProviderID, &m.UpstreamID, &m.FallbackUpstream,
		&m.ContextLength, &m.MaxOutputTokens, &m.PricingMode, &m.MarkupBps, &m.InputPrice, &m.OutputPrice, &m.RequestFee,
		&m.Modalities, &m.SupportsTools, &m.Enabled, &m.Featured, &m.IsCustom, &m.SortOrder, &m.CreatedAt, &m.UpdatedAt,
		&m.UpstreamInputPrice, &m.UpstreamOutputPrice); err != nil {
		return nil, notFound(err)
	}
	return &m, nil
}

func (s *Store) ListModels(ctx context.Context, includeDisabled bool) ([]Model, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+modelCols+modelFrom+` WHERE ($1 OR m.enabled)
		ORDER BY m.featured DESC, m.sort_order DESC, m.created_at DESC, m.slug`, includeDisabled)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Model{}
	for rows.Next() {
		m, err := scanModel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

func (s *Store) GetModel(ctx context.Context, id int64) (*Model, error) {
	return scanModel(s.Pool.QueryRow(ctx, `SELECT `+modelCols+modelFrom+` WHERE m.id=$1`, id))
}

func (s *Store) CreateModel(ctx context.Context, m Model) (*Model, error) {
	if m.Modalities == nil {
		m.Modalities = []string{"text"}
	}
	if m.FallbackUpstream == nil {
		m.FallbackUpstream = []string{}
	}
	if m.PricingMode == "" {
		m.PricingMode = "manual"
	}
	var id int64
	err := s.Pool.QueryRow(ctx, `INSERT INTO models (slug, name, author, description, provider_id, upstream_id, fallback_upstream,
		context_length, max_output_tokens, pricing_mode, markup_bps, input_price, output_price, request_fee, modalities,
		supports_tools, enabled, featured, is_custom, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20) RETURNING id`,
		m.Slug, m.Name, m.Author, m.Description, m.ProviderID, m.UpstreamID, m.FallbackUpstream, m.ContextLength,
		m.MaxOutputTokens, m.PricingMode, m.MarkupBps, m.InputPrice, m.OutputPrice, m.RequestFee, m.Modalities,
		m.SupportsTools, m.Enabled, m.Featured, m.IsCustom, m.SortOrder).Scan(&id)
	if err != nil {
		return nil, notFound(err)
	}
	return s.GetModel(ctx, id)
}

// UpdateModel overwrites all editable fields of a model.
func (s *Store) UpdateModel(ctx context.Context, m Model) (*Model, error) {
	if m.FallbackUpstream == nil {
		m.FallbackUpstream = []string{}
	}
	if m.Modalities == nil {
		m.Modalities = []string{"text"}
	}
	tag, err := s.Pool.Exec(ctx, `UPDATE models SET slug=$2, name=$3, author=$4, description=$5, provider_id=$6, upstream_id=$7,
		fallback_upstream=$8, context_length=$9, max_output_tokens=$10, pricing_mode=$11, markup_bps=$12, input_price=$13,
		output_price=$14, request_fee=$15, modalities=$16, supports_tools=$17, enabled=$18, featured=$19, is_custom=$20,
		sort_order=$21, updated_at=now() WHERE id=$1`,
		m.ID, m.Slug, m.Name, m.Author, m.Description, m.ProviderID, m.UpstreamID, m.FallbackUpstream, m.ContextLength,
		m.MaxOutputTokens, m.PricingMode, m.MarkupBps, m.InputPrice, m.OutputPrice, m.RequestFee, m.Modalities,
		m.SupportsTools, m.Enabled, m.Featured, m.IsCustom, m.SortOrder)
	if err != nil {
		return nil, notFound(err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return s.GetModel(ctx, m.ID)
}

func (s *Store) DeleteModel(ctx context.Context, id int64) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM models WHERE id=$1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// SetModelsEnabled bulk-enables/disables models.
func (s *Store) SetModelsEnabled(ctx context.Context, ids []int64, enabled bool) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `UPDATE models SET enabled=$2, updated_at=now() WHERE id = ANY($1)`, ids, enabled)
	return tag.RowsAffected(), err
}

// RepriceAuto recomputes sell prices of auto-priced models from the upstream
// catalog: sell = ceil(upstream * (10000 + markup) / 10000). It also refreshes
// context length and output caps that were never customized (0).
func (s *Store) RepriceAuto(ctx context.Context, defaultMarkupBps int) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `UPDATE models m SET
		input_price  = (u.input_price  * (10000 + COALESCE(m.markup_bps, $1)) + 9999) / 10000,
		output_price = (u.output_price * (10000 + COALESCE(m.markup_bps, $1)) + 9999) / 10000,
		context_length = CASE WHEN m.context_length = 0 THEN u.context_length ELSE m.context_length END,
		updated_at = now()
		FROM upstream_models u
		WHERE m.pricing_mode = 'auto' AND u.provider_id = m.provider_id AND u.upstream_id = m.upstream_id
		AND (m.input_price  IS DISTINCT FROM (u.input_price  * (10000 + COALESCE(m.markup_bps, $1)) + 9999) / 10000
		  OR m.output_price IS DISTINCT FROM (u.output_price * (10000 + COALESCE(m.markup_bps, $1)) + 9999) / 10000
		  OR (m.context_length = 0 AND u.context_length <> 0))`, defaultMarkupBps)
	return tag.RowsAffected(), err
}

// PublishUpstream creates public models for the given upstream IDs (slug = upstream id)
// with automatic pricing. Existing slugs are skipped. Returns the number created.
func (s *Store) PublishUpstream(ctx context.Context, providerID int64, upstreamIDs []string, markupBps *int, enabled bool) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `INSERT INTO models (slug, name, author, description, provider_id, upstream_id, context_length,
		max_output_tokens, pricing_mode, markup_bps, modalities, supports_tools, enabled)
		SELECT u.upstream_id,
			CASE WHEN position(': ' in u.name) > 0 THEN substr(u.name, position(': ' in u.name) + 2) ELSE u.name END,
			CASE WHEN position('/' in u.upstream_id) > 0 THEN split_part(u.upstream_id, '/', 1) ELSE '' END,
			u.description, u.provider_id, u.upstream_id, u.context_length, 0, 'auto', $3, u.modalities, u.supports_tools, $4
		FROM upstream_models u WHERE u.provider_id=$1 AND u.upstream_id = ANY($2)
		ON CONFLICT (slug) DO NOTHING`, providerID, upstreamIDs, markupBps, enabled)
	return tag.RowsAffected(), err
}
