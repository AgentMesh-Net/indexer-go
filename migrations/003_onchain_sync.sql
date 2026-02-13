-- Phase 6A: add signature fields to tasks and accepts
ALTER TABLE tasks
    ADD COLUMN IF NOT EXISTS employer_signature TEXT,
    ADD COLUMN IF NOT EXISTS onchain_created_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS released_at        TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS refunded_at        TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS onchain_tx_hash    TEXT;

ALTER TABLE accepts
    ADD COLUMN IF NOT EXISTS worker_signature TEXT;

-- Phase 6A3: prevent same worker from accepting same task twice
CREATE UNIQUE INDEX IF NOT EXISTS idx_accepts_task_worker
    ON accepts (task_id, worker_address);

-- Phase 6A: extend status CHECK to include onchain states
-- PostgreSQL: drop old constraint, add new one
DO $$
BEGIN
    ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_status_check;
    ALTER TABLE tasks ADD CONSTRAINT tasks_status_check
        CHECK (status IN ('created','accepted','accepted_onchain','released','refunded','cancelled'));
EXCEPTION WHEN others THEN
    NULL; -- ignore if constraint name differs
END $$;
