package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type KeyDebit struct {
	Amount   int64
	Requests int64
}

// FlushUsage persists a batch of usage records and applies the aggregated balance
// debits in a single transaction. It returns the new balance of every debited user.
func (s *Store) FlushUsage(ctx context.Context, recs []UsageRecord, userDebits map[string]int64, keyDebits map[string]KeyDebit) (map[string]int64, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if len(recs) > 0 {
		_, err = tx.CopyFrom(ctx, pgx.Identifier{"usage_logs"},
			[]string{"request_id", "user_id", "api_key_id", "model_slug", "upstream_id", "provider_id", "endpoint", "stream",
				"status", "input_tokens", "output_tokens", "cached_tokens", "cost", "upstream_cost", "latency_ms", "ttft_ms", "error", "created_at"},
			pgx.CopyFromSlice(len(recs), func(i int) ([]any, error) {
				r := recs[i]
				var keyID any
				if r.APIKeyID != "" {
					keyID = r.APIKeyID
				}
				return []any{r.RequestID, r.UserID, keyID, r.ModelSlug, r.UpstreamID, r.ProviderID, r.Endpoint, r.Stream,
					r.Status, r.InputTokens, r.OutputTokens, r.CachedTokens, r.Cost, r.UpstreamCost, r.LatencyMS, r.TTFTMS,
					truncate(r.Error, 500), r.CreatedAt}, nil
			}))
		if err != nil {
			return nil, err
		}
	}

	balances := make(map[string]int64, len(userDebits))
	if len(userDebits) > 0 {
		ids := make([]string, 0, len(userDebits))
		amts := make([]int64, 0, len(userDebits))
		for id, a := range userDebits {
			ids = append(ids, id)
			amts = append(amts, a)
		}
		rows, err := tx.Query(ctx, `UPDATE users u SET balance = u.balance - v.amt, total_spent = u.total_spent + v.amt
			FROM (SELECT unnest($1::uuid[]) AS id, unnest($2::bigint[]) AS amt) v
			WHERE u.id = v.id RETURNING u.id::text, u.balance`, ids, amts)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			var bal int64
			if err := rows.Scan(&id, &bal); err != nil {
				rows.Close()
				return nil, err
			}
			balances[id] = bal
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	if len(keyDebits) > 0 {
		ids := make([]string, 0, len(keyDebits))
		amts := make([]int64, 0, len(keyDebits))
		reqs := make([]int64, 0, len(keyDebits))
		for id, d := range keyDebits {
			ids = append(ids, id)
			amts = append(amts, d.Amount)
			reqs = append(reqs, d.Requests)
		}
		if _, err := tx.Exec(ctx, `UPDATE api_keys k SET total_spent = k.total_spent + v.amt,
			total_requests = k.total_requests + v.n, last_used_at = now()
			FROM (SELECT unnest($1::uuid[]) AS id, unnest($2::bigint[]) AS amt, unnest($3::bigint[]) AS n) v
			WHERE k.id = v.id`, ids, amts, reqs); err != nil {
			return nil, err
		}
	}
	return balances, tx.Commit(ctx)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func (s *Store) DailyUsage(ctx context.Context, userID string, days int) ([]DailyUsage, error) {
	rows, err := s.Pool.Query(ctx, `SELECT to_char(d.day, 'YYYY-MM-DD'), COALESCE(x.requests,0), COALESCE(x.inp,0), COALESCE(x.outp,0),
			COALESCE(x.cost,0), COALESCE(x.ucost,0)
		FROM generate_series((now() AT TIME ZONE 'UTC')::date - ($2::int - 1), (now() AT TIME ZONE 'UTC')::date, interval '1 day') AS d(day)
		LEFT JOIN (
			SELECT (created_at AT TIME ZONE 'UTC')::date AS day, count(*) AS requests, sum(input_tokens) AS inp,
				sum(output_tokens) AS outp, sum(cost) AS cost, sum(upstream_cost) AS ucost
			FROM usage_logs WHERE ($1 = '' OR user_id::text = $1) AND created_at >= now() - make_interval(days => $2::int)
			GROUP BY 1
		) x ON x.day = d.day ORDER BY d.day`, userID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DailyUsage{}
	for rows.Next() {
		var d DailyUsage
		if err := rows.Scan(&d.Day, &d.Requests, &d.InputTokens, &d.OutputTokens, &d.Cost, &d.UpstreamCost); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) ModelUsage(ctx context.Context, userID string, days, limit int) ([]ModelUsage, error) {
	rows, err := s.Pool.Query(ctx, `SELECT model_slug, count(*), COALESCE(sum(input_tokens),0), COALESCE(sum(output_tokens),0),
			COALESCE(sum(cost),0), COALESCE(sum(upstream_cost),0)
		FROM usage_logs WHERE ($1 = '' OR user_id::text = $1) AND created_at >= now() - make_interval(days => $2::int)
		GROUP BY model_slug ORDER BY count(*) DESC LIMIT $3`, userID, days, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ModelUsage{}
	for rows.Next() {
		var m ModelUsage
		if err := rows.Scan(&m.Model, &m.Requests, &m.InputTokens, &m.OutputTokens, &m.Cost, &m.UpstreamCost); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

type UsageFilter struct {
	UserID     string
	KeyID      string
	Model      string
	BeforeID   int64
	Limit      int
	ErrorsOnly bool
}

func (s *Store) ListUsage(ctx context.Context, f UsageFilter) ([]UsageRecord, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	rows, err := s.Pool.Query(ctx, `SELECT id, request_id, user_id::text, COALESCE(api_key_id::text, ''), model_slug, upstream_id, provider_id,
			endpoint, stream, status, input_tokens, output_tokens, cached_tokens, cost, upstream_cost, latency_ms, ttft_ms, error, created_at
		FROM usage_logs
		WHERE ($1 = '' OR user_id::text = $1) AND ($2 = '' OR api_key_id::text = $2) AND ($3 = '' OR model_slug = $3)
			AND ($4 = 0 OR id < $4) AND (NOT $6 OR status >= 400)
		ORDER BY id DESC LIMIT $5`, f.UserID, f.KeyID, f.Model, f.BeforeID, f.Limit, f.ErrorsOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UsageRecord{}
	for rows.Next() {
		var r UsageRecord
		if err := rows.Scan(&r.ID, &r.RequestID, &r.UserID, &r.APIKeyID, &r.ModelSlug, &r.UpstreamID, &r.ProviderID, &r.Endpoint,
			&r.Stream, &r.Status, &r.InputTokens, &r.OutputTokens, &r.CachedTokens, &r.Cost, &r.UpstreamCost, &r.LatencyMS,
			&r.TTFTMS, &r.Error, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// PruneUsage deletes usage logs older than the retention window, in small batches.
func (s *Store) PruneUsage(ctx context.Context, olderThan time.Duration) (int64, error) {
	var total int64
	for {
		tag, err := s.Pool.Exec(ctx, `DELETE FROM usage_logs WHERE id IN (
			SELECT id FROM usage_logs WHERE created_at < $1 LIMIT 5000)`, time.Now().Add(-olderThan))
		if err != nil {
			return total, err
		}
		total += tag.RowsAffected()
		if tag.RowsAffected() < 5000 {
			return total, nil
		}
	}
}
