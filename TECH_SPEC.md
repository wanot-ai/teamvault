# TeamVault — Technical Specification (MVP)

## 1. Overview

TeamVault is a secret management platform for teams. Users manage secrets via a web console, and services/agents/CI inject secrets at runtime via CLI.

**MVP Scope:** Single-org, KV secrets with envelope encryption, RBAC policies, audit log, CLI injection, and a basic web UI.

## 2. Architecture

```
┌─────────────┐     ┌──────────────┐     ┌──────────┐
│   Web UI    │────▶│  API Server  │────▶│ Postgres │
│  (Next.js)  │     │    (Go)      │     │          │
└─────────────┘     │              │     └──────────┘
                    │  - Auth      │
┌─────────────┐     │  - Secrets   │     ┌──────────┐
│    CLI      │────▶│  - Policy    │────▶│   KMS    │
│ (Go binary) │     │  - Audit     │     │ (local)  │
└─────────────┘     └──────────────┘     └──────────┘
```

## 3. Components

### 3.1 API Server (Go)

Single binary HTTP server. Endpoints:

**Auth:**
- `POST /api/v1/auth/register` — create account (email+password)
- `POST /api/v1/auth/login` — get JWT token
- `GET /api/v1/auth/me` — current user info

**Secrets:**
- `POST /api/v1/projects` — create project
- `GET /api/v1/projects` — list projects
- `PUT /api/v1/secrets/{project}/{path}` — create/update secret (new version)
- `GET /api/v1/secrets/{project}/{path}` — read secret (latest version)
- `GET /api/v1/secrets/{project}` — list secrets in project
- `DELETE /api/v1/secrets/{project}/{path}` — soft delete
- `GET /api/v1/secrets/{project}/{path}/versions` — version history

**Service Accounts:**
- `POST /api/v1/service-accounts` — create SA + token
- `GET /api/v1/service-accounts` — list SAs

**Policies:**
- `POST /api/v1/policies` — create policy
- `GET /api/v1/policies` — list policies

**Audit:**
- `GET /api/v1/audit` — query audit log (filtered)

### 3.2 Encryption

**Envelope Encryption:**
1. Each secret version gets a unique DEK (Data Encryption Key)
2. DEK encrypts the secret value using AES-256-GCM
3. DEK itself is encrypted by a Master Key
4. MVP: Master Key stored in local file (production: KMS)

```
secret_value  ──AES-256-GCM(DEK)──▶  ciphertext
DEK           ──AES-256-GCM(MK)───▶  encrypted_dek
```

Stored in DB: `ciphertext + nonce + encrypted_dek + dek_nonce + master_key_version`

### 3.3 Database Schema (PostgreSQL)

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    name TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'member',  -- 'admin' | 'member'
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT UNIQUE NOT NULL,
    description TEXT,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE secrets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID REFERENCES projects(id),
    path TEXT NOT NULL,
    description TEXT,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(project_id, path)
);

CREATE TABLE secret_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    secret_id UUID REFERENCES secrets(id),
    version INT NOT NULL,
    ciphertext BYTEA NOT NULL,
    nonce BYTEA NOT NULL,
    encrypted_dek BYTEA NOT NULL,
    dek_nonce BYTEA NOT NULL,
    master_key_version INT NOT NULL DEFAULT 1,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(secret_id, version)
);

CREATE TABLE service_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    project_id UUID REFERENCES projects(id),
    scopes TEXT[] NOT NULL DEFAULT '{"read"}',
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT now(),
    expires_at TIMESTAMPTZ
);

CREATE TABLE policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    effect TEXT NOT NULL DEFAULT 'allow',
    actions TEXT[] NOT NULL,
    resource_pattern TEXT NOT NULL,
    subject_type TEXT NOT NULL,  -- 'user' | 'service_account'
    subject_id UUID,
    conditions JSONB,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp TIMESTAMPTZ DEFAULT now(),
    actor_type TEXT NOT NULL,  -- 'user' | 'service_account'
    actor_id UUID NOT NULL,
    action TEXT NOT NULL,
    resource TEXT NOT NULL,
    outcome TEXT NOT NULL,  -- 'success' | 'denied' | 'error'
    ip TEXT,
    metadata JSONB,
    prev_hash TEXT,
    hash TEXT
);
```

### 3.4 CLI

Single Go binary: `teamvault`

```bash
# Auth
teamvault login --server https://localhost:8443 --email user@example.com

# Secrets
teamvault kv get myproject/api-keys/stripe
teamvault kv put myproject/api-keys/stripe --value sk_live_xxx
teamvault kv list myproject

# Inject & Run
teamvault run --project myproject --map "STRIPE_KEY=api-keys/stripe,DB_URL=db/postgres-url" -- node server.js

# Service Account
teamvault token create --project myproject --scopes read --ttl 1h
```

`teamvault run` flow:
1. Read config/flags for secret mappings
2. Fetch each secret from API (with auth token)
3. Set as env vars in child process
4. Exec child process
5. On exit, env vars are gone (child process only)

### 3.5 Web UI (Next.js)

Pages:
- `/login` — email + password
- `/dashboard` — project list
- `/projects/:id` — secret list, create secret
- `/projects/:id/secrets/:path` — secret detail, versions, value (masked, click to reveal)
- `/audit` — audit log table with filters
- `/settings` — service accounts, policies

### 3.6 Auth

- JWT tokens (HS256, signed with server secret)
- Token in `Authorization: Bearer <token>` header
- Service account tokens: random 32-byte hex, stored as bcrypt hash
- Service account tokens sent as `Authorization: Bearer sa.<token>`

### 3.7 Policy Evaluation

Simple path-based matching:
```
resource_pattern: "myproject/api-keys/*"
actions: ["read"]
subject_type: "service_account"
subject_id: <sa-uuid>
```

Evaluation: for each request, find matching policies. If any "allow" matches and no "deny" matches, allow. Default deny.

Admin users bypass policy checks.

## 4. Tech Stack

- **Backend:** Go 1.22+, `net/http` (stdlib), `pgx` (Postgres driver), `golang.org/x/crypto`
- **Frontend:** Next.js 14+, TypeScript, Tailwind CSS, shadcn/ui
- **Database:** PostgreSQL 16
- **CLI:** Go (same repo, `cmd/teamvault`)

## 5. Project Structure

```
teamvault/
├── cmd/
│   ├── server/         # API server main
│   └── teamvault/      # CLI main
├── internal/
│   ├── api/            # HTTP handlers
│   ├── auth/           # JWT, password hashing
│   ├── crypto/         # Envelope encryption
│   ├── db/             # Database queries
│   ├── policy/         # Policy evaluation
│   └── audit/          # Audit logging
├── migrations/         # SQL migrations
├── web/                # Next.js frontend
├── go.mod
├── go.sum
├── Makefile
├── docker-compose.yml  # Postgres + server + web
└── TECH_SPEC.md
```

## 6. MVP Milestones

1. **Backend Core** — DB schema, encryption, secret CRUD, auth
2. **CLI** — login, kv get/put/list, run (env injection)
3. **Web UI** — login, projects, secrets, audit viewer
4. **Docker Compose** — one-command startup for testing

## 7. Security Principles (MVP)

- Secrets never logged (redaction middleware)
- Audit every read/write
- Envelope encryption at rest
- JWT short-lived (1h default)
- Service account tokens scoped to project
- HTTPS only (self-signed cert for dev)
