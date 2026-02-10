-- Advanced features: TEE sessions, webhooks, replication

-- TEE Sessions: tracks attested secure channel sessions
CREATE TABLE IF NOT EXISTS tee_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_key_hash TEXT NOT NULL UNIQUE,
    client_pubkey TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_tee_sessions_hash ON tee_sessions(session_key_hash);
CREATE INDEX IF NOT EXISTS idx_tee_sessions_expires ON tee_sessions(expires_at);

-- Webhooks: registered webhook endpoints per organization
CREATE TABLE IF NOT EXISTS webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id TEXT NOT NULL,
    url TEXT NOT NULL,
    secret TEXT NOT NULL,
    events JSONB NOT NULL DEFAULT '[]',
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_webhooks_org ON webhooks(org_id);
CREATE INDEX IF NOT EXISTS idx_webhooks_active ON webhooks(org_id, active) WHERE active = true;

-- Replication Log: WAL-based replication entries
CREATE TABLE IF NOT EXISTS replication_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    operation TEXT NOT NULL,
    table_name TEXT NOT NULL,
    row_id TEXT NOT NULL,
    data_json JSONB,
    timestamp TIMESTAMPTZ DEFAULT now(),
    node_id TEXT NOT NULL,
    vector_clock JSONB
);

CREATE INDEX IF NOT EXISTS idx_replication_log_timestamp ON replication_log(timestamp);
CREATE INDEX IF NOT EXISTS idx_replication_log_table_row ON replication_log(table_name, row_id);
CREATE INDEX IF NOT EXISTS idx_replication_log_node ON replication_log(node_id);

-- Cleanup: auto-expire old TEE sessions
-- (Application-level job should delete expired sessions periodically)
