package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentMesh-Net/indexer-go/internal/core/envelope"
)

// PostgresRepo implements Repo using PostgreSQL.
type PostgresRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresRepo creates a new PostgresRepo.
func NewPostgresRepo(pool *pgxpool.Pool) *PostgresRepo {
	return &PostgresRepo{pool: pool}
}

func (r *PostgresRepo) InsertObject(ctx context.Context, env *envelope.Envelope) error {
	envJSON, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	createdAt, err := time.Parse(time.RFC3339Nano, env.CreatedAt)
	if err != nil {
		createdAt, err = time.Parse(time.RFC3339, env.CreatedAt)
		if err != nil {
			return fmt.Errorf("parse created_at: %w", err)
		}
	}

	const q = `INSERT INTO objects (object_id, object_type, object_version, created_at, signer_pubkey, envelope_json, payload_json)
VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err = r.pool.Exec(ctx, q,
		env.ObjectID,
		env.ObjectType,
		env.ObjectVersion,
		createdAt,
		env.Signer.PubKey,
		envJSON,
		env.Payload,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrConflict
		}
		return fmt.Errorf("insert: %w", err)
	}
	return nil
}

func (r *PostgresRepo) ListObjects(ctx context.Context, objectType string, limit int, cursor *Cursor) ([]envelope.Envelope, *Cursor, error) {
	var rows pgx.Rows
	var err error

	if cursor != nil {
		cursorTime, parseErr := time.Parse(time.RFC3339Nano, cursor.CreatedAt)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("parse cursor time: %w", parseErr)
		}
		const q = `SELECT envelope_json FROM objects
WHERE object_type = $1
  AND (created_at, object_id) < ($2, $3)
ORDER BY created_at DESC, object_id DESC
LIMIT $4`
		rows, err = r.pool.Query(ctx, q, objectType, cursorTime, cursor.ObjectID, limit+1)
	} else {
		const q = `SELECT envelope_json FROM objects
WHERE object_type = $1
ORDER BY created_at DESC, object_id DESC
LIMIT $2`
		rows, err = r.pool.Query(ctx, q, objectType, limit+1)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var items []envelope.Envelope
	for rows.Next() {
		var envJSON []byte
		if err := rows.Scan(&envJSON); err != nil {
			return nil, nil, fmt.Errorf("scan: %w", err)
		}
		var env envelope.Envelope
		if err := json.Unmarshal(envJSON, &env); err != nil {
			return nil, nil, fmt.Errorf("unmarshal: %w", err)
		}
		items = append(items, env)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("rows: %w", err)
	}

	var next *Cursor
	if len(items) > limit {
		last := items[limit-1]
		next = &Cursor{
			CreatedAt: last.CreatedAt,
			ObjectID:  last.ObjectID,
		}
		items = items[:limit]
	}

	return items, next, nil
}

func (r *PostgresRepo) GetObjectByID(ctx context.Context, id string) (*envelope.Envelope, error) {
	const q = `SELECT envelope_json FROM objects WHERE object_id = $1`
	var envJSON []byte
	err := r.pool.QueryRow(ctx, q, id).Scan(&envJSON)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query: %w", err)
	}
	var env envelope.Envelope
	if err := json.Unmarshal(envJSON, &env); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &env, nil
}
