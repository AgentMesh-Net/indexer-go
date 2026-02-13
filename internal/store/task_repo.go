package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TaskStatus enumerates task lifecycle states.
const (
	TaskStatusCreated   = "created"
	TaskStatusAccepted  = "accepted"
	TaskStatusReleased  = "released"
	TaskStatusRefunded  = "refunded"
	TaskStatusCancelled = "cancelled"
)

// Task represents a structured task row.
type Task struct {
	TaskID          string
	TaskHash        string
	ChainID         int
	EscrowAddress   string
	EmployerAddress string
	WorkerAddress   string
	AmountWei       string
	DeadlineUnix    int64
	Title           string
	Status          string
	IndexerFeeBPS   int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Accept represents a worker accept row.
type Accept struct {
	AcceptID      string
	TaskID        string
	WorkerAddress string
	CreatedAt     time.Time
}

// TaskRepo defines structured task/accept storage operations.
type TaskRepo interface {
	InsertTask(ctx context.Context, t *Task) error
	GetTask(ctx context.Context, taskID string) (*Task, error)
	ListTasks(ctx context.Context, chainID int, status string, limit, offset int) ([]*Task, error)
	InsertAccept(ctx context.Context, a *Accept) error
	UpdateTaskWorker(ctx context.Context, taskID, workerAddress, status string) error
}

// PostgresTaskRepo implements TaskRepo using PostgreSQL.
type PostgresTaskRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresTaskRepo creates a PostgresTaskRepo.
func NewPostgresTaskRepo(pool *pgxpool.Pool) *PostgresTaskRepo {
	return &PostgresTaskRepo{pool: pool}
}

func (r *PostgresTaskRepo) InsertTask(ctx context.Context, t *Task) error {
	const q = `
INSERT INTO tasks (task_id, task_hash, chain_id, escrow_address, employer_address,
                   amount_wei, deadline_unix, title, status, indexer_fee_bps, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,now(),now())`
	_, err := r.pool.Exec(ctx, q,
		t.TaskID, t.TaskHash, t.ChainID, t.EscrowAddress, t.EmployerAddress,
		t.AmountWei, t.DeadlineUnix, t.Title, t.Status, t.IndexerFeeBPS,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrConflict
		}
		return fmt.Errorf("insert task: %w", err)
	}
	return nil
}

func (r *PostgresTaskRepo) GetTask(ctx context.Context, taskID string) (*Task, error) {
	const q = `
SELECT task_id, task_hash, chain_id, escrow_address, employer_address,
       COALESCE(worker_address,''), amount_wei, deadline_unix,
       COALESCE(title,''), status, indexer_fee_bps, created_at, updated_at
FROM tasks WHERE task_id = $1`
	row := r.pool.QueryRow(ctx, q, taskID)
	t := &Task{}
	err := row.Scan(
		&t.TaskID, &t.TaskHash, &t.ChainID, &t.EscrowAddress, &t.EmployerAddress,
		&t.WorkerAddress, &t.AmountWei, &t.DeadlineUnix,
		&t.Title, &t.Status, &t.IndexerFeeBPS, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get task: %w", err)
	}
	return t, nil
}

func (r *PostgresTaskRepo) ListTasks(ctx context.Context, chainID int, status string, limit, offset int) ([]*Task, error) {
	q := `
SELECT task_id, task_hash, chain_id, escrow_address, employer_address,
       COALESCE(worker_address,''), amount_wei, deadline_unix,
       COALESCE(title,''), status, indexer_fee_bps, created_at, updated_at
FROM tasks WHERE 1=1`
	args := []any{}
	idx := 1
	if chainID > 0 {
		q += fmt.Sprintf(" AND chain_id = $%d", idx)
		args = append(args, chainID)
		idx++
	}
	if status != "" {
		q += fmt.Sprintf(" AND status = $%d", idx)
		args = append(args, status)
		idx++
	}
	q += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		t := &Task{}
		if err := rows.Scan(
			&t.TaskID, &t.TaskHash, &t.ChainID, &t.EscrowAddress, &t.EmployerAddress,
			&t.WorkerAddress, &t.AmountWei, &t.DeadlineUnix,
			&t.Title, &t.Status, &t.IndexerFeeBPS, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (r *PostgresTaskRepo) InsertAccept(ctx context.Context, a *Accept) error {
	const q = `INSERT INTO accepts (accept_id, task_id, worker_address, created_at) VALUES ($1,$2,$3,now())`
	_, err := r.pool.Exec(ctx, q, a.AcceptID, a.TaskID, a.WorkerAddress)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrConflict
		}
		return fmt.Errorf("insert accept: %w", err)
	}
	return nil
}

func (r *PostgresTaskRepo) UpdateTaskWorker(ctx context.Context, taskID, workerAddress, status string) error {
	const q = `UPDATE tasks SET worker_address=$1, status=$2, updated_at=now() WHERE task_id=$3`
	_, err := r.pool.Exec(ctx, q, workerAddress, status, taskID)
	if err != nil {
		return fmt.Errorf("update task worker: %w", err)
	}
	return nil
}
