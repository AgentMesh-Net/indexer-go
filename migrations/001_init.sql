CREATE TABLE IF NOT EXISTS objects (
    object_id       TEXT        PRIMARY KEY,
    object_type     TEXT        NOT NULL CHECK (object_type IN ('task','bid','accept','artifact')),
    object_version  TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL,
    signer_pubkey   TEXT        NOT NULL,
    envelope_json   JSONB       NOT NULL,
    payload_json    JSONB       NOT NULL,
    inserted_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_objects_type_created_at
    ON objects (object_type, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_objects_signer_pubkey
    ON objects (signer_pubkey);

CREATE INDEX IF NOT EXISTS idx_accept_task_id
    ON objects ((envelope_json->'payload'->>'task_id'))
    WHERE object_type = 'accept';
