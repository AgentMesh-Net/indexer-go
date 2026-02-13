-- Phase 5: structured tasks and accepts tables

CREATE TABLE IF NOT EXISTS tasks (
    task_id          TEXT        PRIMARY KEY,
    task_hash        TEXT        NOT NULL UNIQUE,
    chain_id         INTEGER     NOT NULL,
    escrow_address   TEXT        NOT NULL,
    employer_address TEXT        NOT NULL,
    worker_address   TEXT,
    amount_wei       TEXT        NOT NULL,
    deadline_unix    BIGINT      NOT NULL,
    title            TEXT,
    status           TEXT        NOT NULL DEFAULT 'created'
                                 CHECK (status IN ('created','accepted','released','refunded','cancelled')),
    indexer_fee_bps  INTEGER     NOT NULL DEFAULT 20,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_tasks_chain_status
    ON tasks (chain_id, status);

CREATE INDEX IF NOT EXISTS idx_tasks_employer
    ON tasks (employer_address);

CREATE TABLE IF NOT EXISTS accepts (
    accept_id      TEXT        PRIMARY KEY,
    task_id        TEXT        NOT NULL REFERENCES tasks(task_id) ON DELETE CASCADE,
    worker_address TEXT        NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_accepts_task_id
    ON accepts (task_id);
