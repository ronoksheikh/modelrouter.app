// Package store is the Postgres data access layer.
package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

type Store struct {
	Pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store { return &Store{Pool: pool} }

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return ErrConflict
	}
	return err
}

const userCols = `id, email, name, image, email_verified, role, banned, country, balance, total_spent, total_topup, created_at`

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.Image, &u.EmailVerified, &u.Role, &u.Banned,
		&u.Country, &u.Balance, &u.TotalSpent, &u.TotalTopup, &u.CreatedAt)
	if err != nil {
		return nil, notFound(err)
	}
	return &u, nil
}

// CreateUser inserts a user and grants the signup bonus atomically.
func (s *Store) CreateUser(ctx context.Context, email, name, image string, verified *time.Time, role string, bonus int64) (*User, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	u, err := scanUser(tx.QueryRow(ctx, `INSERT INTO users (email, name, image, email_verified, role, balance)
		VALUES (lower($1), $2, $3, $4, $5, $6) RETURNING `+userCols,
		strings.TrimSpace(email), name, image, verified, role, bonus))
	if err != nil {
		return nil, err
	}
	if bonus > 0 {
		if _, err := tx.Exec(ctx, `INSERT INTO credit_ledger (user_id, amount, reason) VALUES ($1, $2, 'signup_bonus')`, u.ID, bonus); err != nil {
			return nil, err
		}
	}
	return u, tx.Commit(ctx)
}

func (s *Store) GetUser(ctx context.Context, id string) (*User, error) {
	return scanUser(s.Pool.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE id=$1`, id))
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	return scanUser(s.Pool.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE email=lower($1)`, strings.TrimSpace(email)))
}

func (s *Store) GetUserByAccount(ctx context.Context, provider, accountID string) (*User, error) {
	return scanUser(s.Pool.QueryRow(ctx, `SELECT `+prefixed("u", userCols)+` FROM users u
		JOIN auth_accounts a ON a.user_id = u.id WHERE a.provider=$1 AND a.provider_account_id=$2`, provider, accountID))
}

type UserPatch struct {
	Name          *string
	Image         *string
	EmailVerified *time.Time
	Email         *string
	Role          *string
	Banned        *bool
	Country       *string
}

func (s *Store) UpdateUser(ctx context.Context, id string, p UserPatch) (*User, error) {
	return scanUser(s.Pool.QueryRow(ctx, `UPDATE users SET
		name = COALESCE($2, name),
		image = COALESCE($3, image),
		email_verified = COALESCE($4, email_verified),
		email = COALESCE(lower($5), email),
		role = COALESCE($6, role),
		banned = COALESCE($7, banned),
		country = COALESCE($8, country),
		updated_at = now()
		WHERE id=$1 RETURNING `+userCols, id, p.Name, p.Image, p.EmailVerified, p.Email, p.Role, p.Banned, p.Country))
}

func (s *Store) LinkAccount(ctx context.Context, userID, provider, accountID, typ string) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO auth_accounts (provider, provider_account_id, user_id, type)
		VALUES ($1, $2, $3, $4) ON CONFLICT (provider, provider_account_id) DO UPDATE SET user_id = EXCLUDED.user_id`,
		provider, accountID, userID, typ)
	return err
}

func (s *Store) CreateVerificationToken(ctx context.Context, identifier, tokenHash string, expires time.Time) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO auth_verification_tokens (identifier, token_hash, expires) VALUES ($1, $2, $3)
		ON CONFLICT (identifier, token_hash) DO UPDATE SET expires = EXCLUDED.expires`, identifier, tokenHash, expires)
	if err == nil {
		// Opportunistic cleanup of expired tokens.
		_, _ = s.Pool.Exec(ctx, `DELETE FROM auth_verification_tokens WHERE expires < now() - interval '1 day'`)
	}
	return err
}

// UseVerificationToken deletes and returns a token (single use).
func (s *Store) UseVerificationToken(ctx context.Context, identifier, tokenHash string) (time.Time, error) {
	var exp time.Time
	err := s.Pool.QueryRow(ctx, `DELETE FROM auth_verification_tokens WHERE identifier=$1 AND token_hash=$2 RETURNING expires`,
		identifier, tokenHash).Scan(&exp)
	return exp, notFound(err)
}

func (s *Store) ListUsers(ctx context.Context, q string, limit, offset int) ([]User, int64, error) {
	like := "%" + strings.ToLower(strings.TrimSpace(q)) + "%"
	var total int64
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE ($1 = '%%' OR email LIKE $1 OR lower(name) LIKE $1 OR id::text = $2)`,
		like, strings.TrimSpace(q)).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT `+userCols+` FROM users
		WHERE ($1 = '%%' OR email LIKE $1 OR lower(name) LIKE $1 OR id::text = $2)
		ORDER BY created_at DESC LIMIT $3 OFFSET $4`, like, strings.TrimSpace(q), limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *u)
	}
	return out, total, rows.Err()
}

// AdjustBalance changes a user's balance and records a ledger entry.
func (s *Store) AdjustBalance(ctx context.Context, userID string, amount int64, reason, ref, actor, note string) (int64, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var bal int64
	if err := tx.QueryRow(ctx, `UPDATE users SET balance = balance + $2, updated_at = now() WHERE id=$1 RETURNING balance`,
		userID, amount).Scan(&bal); err != nil {
		return 0, notFound(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO credit_ledger (user_id, amount, reason, ref, actor, note) VALUES ($1,$2,$3,$4,$5,$6)`,
		userID, amount, reason, ref, actor, note); err != nil {
		return 0, err
	}
	return bal, tx.Commit(ctx)
}

func (s *Store) GetBalance(ctx context.Context, userID string) (int64, error) {
	var b int64
	err := s.Pool.QueryRow(ctx, `SELECT balance FROM users WHERE id=$1`, userID).Scan(&b)
	return b, notFound(err)
}

func (s *Store) ListLedger(ctx context.Context, userID string, limit int) ([]LedgerEntry, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, user_id, amount, reason, ref, actor, note, created_at FROM credit_ledger
		WHERE user_id=$1 ORDER BY created_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LedgerEntry
	for rows.Next() {
		var e LedgerEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.Amount, &e.Reason, &e.Ref, &e.Actor, &e.Note, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func prefixed(alias, cols string) string {
	parts := strings.Split(cols, ",")
	for i, p := range parts {
		parts[i] = alias + "." + strings.TrimSpace(p)
	}
	return strings.Join(parts, ", ")
}
