package store

import (
	"context"

	"github.com/jackc/pgx/v5"
)

const keyCols = `id, user_id, name, prefix, disabled, spend_limit, rpm_limit, total_spent, total_requests, last_used_at, created_at`

func scanKey(row pgx.Row) (*APIKey, error) {
	var k APIKey
	if err := row.Scan(&k.ID, &k.UserID, &k.Name, &k.Prefix, &k.Disabled, &k.SpendLimit, &k.RPMLimit,
		&k.TotalSpent, &k.TotalRequests, &k.LastUsedAt, &k.CreatedAt); err != nil {
		return nil, notFound(err)
	}
	return &k, nil
}

func (s *Store) CreateKey(ctx context.Context, userID, name, hash, prefix string, spendLimit *int64, rpm *int) (*APIKey, error) {
	return scanKey(s.Pool.QueryRow(ctx, `INSERT INTO api_keys (user_id, name, key_hash, prefix, spend_limit, rpm_limit)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING `+keyCols, userID, name, hash, prefix, spendLimit, rpm))
}

func (s *Store) CountKeys(ctx context.Context, userID string) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM api_keys WHERE user_id=$1 AND deleted_at IS NULL`, userID).Scan(&n)
	return n, err
}

func (s *Store) ListKeys(ctx context.Context, userID string) ([]APIKey, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+keyCols+` FROM api_keys WHERE user_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []APIKey{}
	for rows.Next() {
		k, err := scanKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *k)
	}
	return out, rows.Err()
}

func (s *Store) GetKeyAuth(ctx context.Context, hash string) (*KeyAuth, error) {
	var a KeyAuth
	err := s.Pool.QueryRow(ctx, `SELECT k.id, k.key_hash, k.user_id, k.disabled, k.spend_limit, k.total_spent, k.rpm_limit, u.banned
		FROM api_keys k JOIN users u ON u.id = k.user_id WHERE k.key_hash=$1 AND k.deleted_at IS NULL`, hash).
		Scan(&a.KeyID, &a.KeyHash, &a.UserID, &a.Disabled, &a.SpendLimit, &a.KeySpent, &a.RPMLimit, &a.UserBanned)
	if err != nil {
		return nil, notFound(err)
	}
	return &a, nil
}

type KeyPatch struct {
	Name          *string
	Disabled      *bool
	SpendLimit    *int64
	ClearLimit    bool
	RPMLimit      *int
	ClearRPMLimit bool
}

// UpdateKey updates a key owned by userID and returns it along with its hash
// (so callers can invalidate caches).
func (s *Store) UpdateKey(ctx context.Context, userID, keyID string, p KeyPatch) (*APIKey, string, error) {
	var hash string
	k, err := scanKeyWithHash(s.Pool.QueryRow(ctx, `UPDATE api_keys SET
		name = COALESCE($3, name),
		disabled = COALESCE($4, disabled),
		spend_limit = CASE WHEN $6 THEN NULL ELSE COALESCE($5, spend_limit) END,
		rpm_limit = CASE WHEN $8 THEN NULL ELSE COALESCE($7, rpm_limit) END
		WHERE id=$1 AND ($2 = '' OR user_id::text=$2) AND deleted_at IS NULL RETURNING `+keyCols+`, key_hash`,
		keyID, userID, p.Name, p.Disabled, p.SpendLimit, p.ClearLimit, p.RPMLimit, p.ClearRPMLimit), &hash)
	return k, hash, err
}

func scanKeyWithHash(row pgx.Row, hash *string) (*APIKey, error) {
	var k APIKey
	if err := row.Scan(&k.ID, &k.UserID, &k.Name, &k.Prefix, &k.Disabled, &k.SpendLimit, &k.RPMLimit,
		&k.TotalSpent, &k.TotalRequests, &k.LastUsedAt, &k.CreatedAt, hash); err != nil {
		return nil, notFound(err)
	}
	return &k, nil
}

// RotateKey replaces the secret of an existing key, keeping its settings and stats.
// It returns the updated key and the old hash.
func (s *Store) RotateKey(ctx context.Context, userID, keyID, newHash, newPrefix string) (*APIKey, string, error) {
	var old string
	if err := s.Pool.QueryRow(ctx, `SELECT key_hash FROM api_keys WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL`,
		keyID, userID).Scan(&old); err != nil {
		return nil, "", notFound(err)
	}
	k, err := scanKey(s.Pool.QueryRow(ctx, `UPDATE api_keys SET key_hash=$3, prefix=$4
		WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL RETURNING `+keyCols, keyID, userID, newHash, newPrefix))
	return k, old, err
}

// DeleteKey soft-deletes a key; pass userID "" for admin deletes. Returns the hash.
func (s *Store) DeleteKey(ctx context.Context, userID, keyID string) (string, error) {
	var hash string
	err := s.Pool.QueryRow(ctx, `UPDATE api_keys SET deleted_at = now(), disabled = true,
		key_hash = key_hash || ':deleted:' || id::text
		WHERE id=$1 AND ($2 = '' OR user_id::text=$2) AND deleted_at IS NULL
		RETURNING split_part(key_hash, ':deleted:', 1)`, keyID, userID).Scan(&hash)
	return hash, notFound(err)
}

// KeyHashesForUser returns the active key hashes of a user (for cache invalidation).
func (s *Store) KeyHashesForUser(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `SELECT key_hash FROM api_keys WHERE user_id=$1 AND deleted_at IS NULL`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}
