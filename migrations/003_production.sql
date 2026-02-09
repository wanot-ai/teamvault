-- Production features: file secrets, rotation schedules, leases, OIDC users

-- Add secret_type to secrets table ('kv', 'json', 'file')
ALTER TABLE secrets ADD COLUMN IF NOT EXISTS secret_type TEXT NOT NULL DEFAULT 'kv';
ALTER TABLE secrets ADD COLUMN IF NOT EXISTS metadata JSONB;

-- Rotation schedules
CREATE TABLE IF NOT EXISTS rotation_schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    secret_id UUID REFERENCES secrets(id) NOT NULL,
    cron_expression TEXT NOT NULL,
    connector_type TEXT NOT NULL,
    connector_config JSONB,
    last_rotated_at TIMESTAMPTZ,
    next_rotation_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(secret_id)
);

CREATE INDEX IF NOT EXISTS idx_rotation_schedules_next ON rotation_schedules(next_rotation_at) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_rotation_schedules_secret ON rotation_schedules(secret_id);

-- Leases (dynamic secrets)
CREATE TABLE IF NOT EXISTS leases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID REFERENCES orgs(id),
    secret_path TEXT NOT NULL,
    lease_type TEXT NOT NULL,
    value_ciphertext BYTEA NOT NULL,
    encrypted_dek BYTEA NOT NULL,
    nonce BYTEA NOT NULL,
    dek_nonce BYTEA NOT NULL,
    issued_to TEXT NOT NULL,
    issued_at TIMESTAMPTZ DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_leases_expires ON leases(expires_at) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_leases_issued_to ON leases(issued_to);
CREATE INDEX IF NOT EXISTS idx_leases_org ON leases(org_id);

-- OIDC user linking (allow users created via OIDC to have an oidc_subject)
ALTER TABLE users ADD COLUMN IF NOT EXISTS oidc_issuer TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS oidc_subject TEXT;

CREATE INDEX IF NOT EXISTS idx_users_oidc ON users(oidc_issuer, oidc_subject) WHERE oidc_subject IS NOT NULL;
