package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

const paymentCols = `p.id, p.user_id, COALESCE(u.email, ''), p.provider, p.external_id, p.status, p.currency, p.amount_minor,
	p.credit, p.meta, p.created_at, p.completed_at`

const paymentFrom = ` FROM payments p LEFT JOIN users u ON u.id = p.user_id `

func scanPayment(row pgx.Row) (*Payment, error) {
	var p Payment
	if err := row.Scan(&p.ID, &p.UserID, &p.UserEmail, &p.Provider, &p.ExternalID, &p.Status, &p.Currency, &p.AmountMinor,
		&p.Credit, &p.Meta, &p.CreatedAt, &p.CompletedAt); err != nil {
		return nil, notFound(err)
	}
	if p.Meta == nil {
		p.Meta = map[string]any{}
	}
	return &p, nil
}

func (s *Store) CreatePayment(ctx context.Context, userID, provider, currency string, amountMinor, credit int64, meta map[string]any) (*Payment, error) {
	if meta == nil {
		meta = map[string]any{}
	}
	var id string
	if err := s.Pool.QueryRow(ctx, `INSERT INTO payments (user_id, provider, currency, amount_minor, credit, meta)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`, userID, provider, currency, amountMinor, credit, meta).Scan(&id); err != nil {
		return nil, notFound(err)
	}
	return s.GetPayment(ctx, id)
}

func (s *Store) GetPayment(ctx context.Context, id string) (*Payment, error) {
	return scanPayment(s.Pool.QueryRow(ctx, `SELECT `+paymentCols+paymentFrom+` WHERE p.id=$1`, id))
}

func (s *Store) SetPaymentExternal(ctx context.Context, id, externalID string, meta map[string]any) error {
	b, _ := json.Marshal(meta)
	if meta == nil {
		b = []byte("{}")
	}
	_, err := s.Pool.Exec(ctx, `UPDATE payments SET external_id=$2, meta = meta || $3::jsonb WHERE id=$1`, id, externalID, string(b))
	return notFound(err)
}

var ErrAlreadyProcessed = errors.New("payment already processed")

// CompletePayment marks a pending payment completed and credits the user, exactly once.
func (s *Store) CompletePayment(ctx context.Context, id, externalID string, meta map[string]any) (*Payment, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	b, _ := json.Marshal(meta)
	if meta == nil {
		b = []byte("{}")
	}
	var userID string
	var credit int64
	err = tx.QueryRow(ctx, `UPDATE payments SET status='completed', completed_at=now(),
		external_id = COALESCE(NULLIF($2, ''), external_id), meta = meta || $3::jsonb
		WHERE id=$1 AND status='pending' RETURNING user_id, credit`, id, externalID, string(b)).Scan(&userID, &credit)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAlreadyProcessed
	}
	if err != nil {
		return nil, notFound(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE users SET balance = balance + $2, total_topup = total_topup + $2, updated_at = now() WHERE id=$1`,
		userID, credit); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO credit_ledger (user_id, amount, reason, ref) VALUES ($1, $2, 'topup', $3)`,
		userID, credit, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetPayment(ctx, id)
}

// SetPaymentStatus moves a pending payment to a terminal non-completed status.
func (s *Store) SetPaymentStatus(ctx context.Context, id, status string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE payments SET status=$2 WHERE id=$1 AND status='pending'`, id, status)
	return err
}

func (s *Store) ListPayments(ctx context.Context, userID, status string, limit, offset int) ([]Payment, int64, error) {
	var total int64
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM payments p WHERE ($1 = '' OR p.user_id::text=$1) AND ($2 = '' OR p.status=$2)`,
		userID, status).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT `+paymentCols+paymentFrom+`
		WHERE ($1 = '' OR p.user_id::text=$1) AND ($2 = '' OR p.status=$2) ORDER BY p.created_at DESC LIMIT $3 OFFSET $4`,
		userID, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Payment{}
	for rows.Next() {
		p, err := scanPayment(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *p)
	}
	return out, total, rows.Err()
}

// ---------- Settings ----------

func (s *Store) GetSettings(ctx context.Context) (Settings, error) {
	st := DefaultSettings()
	var raw []byte
	err := s.Pool.QueryRow(ctx, `SELECT value FROM settings WHERE key='platform'`).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	// Unmarshal over defaults so newly added fields keep sane values.
	_ = json.Unmarshal(raw, &st)
	return st, nil
}

func (s *Store) PutSettings(ctx context.Context, st Settings) error {
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `INSERT INTO settings (key, value) VALUES ('platform', $1::jsonb)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`, string(b))
	return err
}

// ---------- Audit ----------

func (s *Store) Audit(ctx context.Context, actorID, actorEmail, action, target string, detail map[string]any) {
	if detail == nil {
		detail = map[string]any{}
	}
	var actor any
	if actorID != "" {
		actor = actorID
	}
	_, _ = s.Pool.Exec(ctx, `INSERT INTO audit_logs (actor_id, actor_email, action, target, detail) VALUES ($1,$2,$3,$4,$5)`,
		actor, actorEmail, action, target, detail)
}

func (s *Store) ListAudit(ctx context.Context, limit, offset int) ([]AuditLog, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, actor_id::text, actor_email, action, target, detail, created_at
		FROM audit_logs ORDER BY id DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditLog{}
	for rows.Next() {
		var a AuditLog
		if err := rows.Scan(&a.ID, &a.ActorID, &a.ActorEmail, &a.Action, &a.Target, &a.Detail, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ---------- Admin overview ----------

type Overview struct {
	Users          int64 `json:"users"`
	NewUsers       int64 `json:"new_users"`
	ActiveUsers    int64 `json:"active_users"`
	Requests       int64 `json:"requests"`
	Errors         int64 `json:"errors"`
	InputTokens    int64 `json:"input_tokens"`
	OutputTokens   int64 `json:"output_tokens"`
	Revenue        int64 `json:"revenue"`       // usage charged to users
	UpstreamCost   int64 `json:"upstream_cost"` // what upstream providers cost us
	Topups         int64 `json:"topups"`        // money received
	OutstandingBal int64 `json:"outstanding_balance"`
	AvgLatencyMS   int64 `json:"avg_latency_ms"`
}

func (s *Store) Overview(ctx context.Context, days int) (*Overview, error) {
	var o Overview
	err := s.Pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM users),
		(SELECT count(*) FROM users WHERE created_at >= now() - make_interval(days => $1::int)),
		(SELECT COALESCE(sum(balance),0) FROM users),
		(SELECT COALESCE(sum(credit),0) FROM payments WHERE status='completed' AND completed_at >= now() - make_interval(days => $1::int))`,
		days).Scan(&o.Users, &o.NewUsers, &o.OutstandingBal, &o.Topups)
	if err != nil {
		return nil, err
	}
	err = s.Pool.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE status >= 400), count(DISTINCT user_id),
		COALESCE(sum(input_tokens),0), COALESCE(sum(output_tokens),0), COALESCE(sum(cost),0), COALESCE(sum(upstream_cost),0),
		COALESCE(avg(latency_ms)::bigint, 0)
		FROM usage_logs WHERE created_at >= now() - make_interval(days => $1::int)`, days).
		Scan(&o.Requests, &o.Errors, &o.ActiveUsers, &o.InputTokens, &o.OutputTokens, &o.Revenue, &o.UpstreamCost, &o.AvgLatencyMS)
	return &o, err
}

type TopUser struct {
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	Requests int64  `json:"requests"`
	Cost     int64  `json:"cost"`
}

func (s *Store) TopUsers(ctx context.Context, days, limit int) ([]TopUser, error) {
	rows, err := s.Pool.Query(ctx, `SELECT l.user_id::text, COALESCE(u.email, ''), count(*), COALESCE(sum(l.cost),0)
		FROM usage_logs l LEFT JOIN users u ON u.id = l.user_id
		WHERE l.created_at >= now() - make_interval(days => $1::int)
		GROUP BY l.user_id, u.email ORDER BY sum(l.cost) DESC LIMIT $2`, days, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TopUser{}
	for rows.Next() {
		var t TopUser
		if err := rows.Scan(&t.UserID, &t.Email, &t.Requests, &t.Cost); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
