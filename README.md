# TeamVault

Enterprise secret management for teams and AI agents.

TeamVault lets organizations manage secrets through a web console, enforce fine-grained access policies (RBAC/ABAC/PBAC), and inject secrets into services, CI/CD pipelines, and AI agents at runtime — with envelope encryption, hash-chained audit logs, and HCL-based policy-as-code.

---

## Table of Contents

- [Quick Start](#quick-start)
- [Architecture](#architecture)
- [Core Concepts](#core-concepts)
- [Secret Management](#secret-management)
- [IAM & Access Control](#iam--access-control)
- [Policy-as-Code (HCL)](#policy-as-code-hcl)
- [CLI Reference](#cli-reference)
- [REST API](#rest-api)
- [Web Console](#web-console)
- [OpenClaw Integration](#openclaw-integration)
- [Security Model](#security-model)
- [Deployment](#deployment)
- [Roadmap](#roadmap)

---

## Quick Start

```bash
git clone https://github.com/wanot-ai/teamvault.git
cd teamvault
docker compose up -d
```

API server: `http://localhost:8443`
Web UI: `cd web && npm install && npm run dev` → `http://localhost:3000`

```bash
# Register and login
curl -X POST http://localhost:8443/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"you@company.com","password":"secure123","name":"You"}'

# Create a project and store a secret
teamvault login --server http://localhost:8443 --email you@company.com
teamvault kv put myproject/services/payment/prod/STRIPE_KEY --value sk_live_xxx
teamvault kv get myproject/services/payment/prod/STRIPE_KEY

# Inject secrets and run your app
teamvault run \
  --project myproject \
  --map "STRIPE_KEY=services/payment/prod/STRIPE_KEY" \
  -- node server.js
```

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Organization                             │
│                                                                  │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐                │
│  │   Team A    │  │   Team B    │  │   Team C    │               │
│  │             │  │             │  │             │                │
│  │  Members:   │  │  Members:   │  │  Members:   │               │
│  │   alice     │  │   carol     │  │   eve       │               │
│  │   bob       │  │   dave      │  │             │               │
│  │             │  │             │  │             │                │
│  │  Agents:    │  │  Agents:    │  │  Agents:    │               │
│  │   ci-bot    │  │   deploy    │  │   monitor   │               │
│  │   openclaw  │  │   k8s-sa    │  │             │               │
│  └────────────┘  └────────────┘  └────────────┘                │
│                                                                  │
│  ┌──────────────────────────────────────────────────┐           │
│  │                    Vaults                         │           │
│  │                                                   │           │
│  │  services/                                        │           │
│  │  ├── payment/                                     │           │
│  │  │   ├── dev/                                     │           │
│  │  │   │   ├── STRIPE_KEY                           │           │
│  │  │   │   └── DB_URL                               │           │
│  │  │   ├── staging/                                 │           │
│  │  │   │   └── STRIPE_KEY                           │           │
│  │  │   └── prod/                                    │           │
│  │  │       ├── STRIPE_KEY                           │           │
│  │  │       └── DB_URL                               │           │
│  │  └── auth/                                        │           │
│  │      └── prod/                                    │           │
│  │          └── JWT_SECRET                            │           │
│  └──────────────────────────────────────────────────┘           │
│                                                                  │
│  ┌──────────────────────────────────────────────────┐           │
│  │             IAM Policies (HCL)                    │           │
│  │                                                   │           │
│  │  RBAC: role-based (admin, editor, viewer)         │           │
│  │  ABAC: attribute-based (env=prod, mfa=true)       │           │
│  │  PBAC: policy document (Terraform-style)          │           │
│  └──────────────────────────────────────────────────┘           │
└─────────────────────────────────────────────────────────────────┘

                    ┌─────────────┐
                    │  Web Console │ ← Team members manage secrets
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │  API Server  │ ← Envelope encryption (AES-256-GCM)
                    │    (Go)      │   Hash-chained audit log
                    └──────┬──────┘   Policy evaluation engine
                           │
              ┌────────────┼────────────┐
              │            │            │
        ┌─────▼────┐ ┌────▼─────┐ ┌───▼────────┐
        │  CLI      │ │  Agents  │ │  OpenClaw   │
        │ injection │ │  (REST)  │ │  env hook   │
        └──────────┘ └──────────┘ └────────────┘
```

---

## Core Concepts

**Organization** — A company or group. All teams, vaults, and policies belong to an org.

**Team** — A group of people and agents within an org. Teams own access to specific vaults and paths.

**Agent** — A non-human identity (CI bot, AI agent, microservice) that belongs to a team. Agents authenticate with scoped, time-limited tokens.

**Vault (Project)** — A container for secrets. Secrets within a vault use hierarchical paths.

**Secret** — A key-value pair at a specific path, with version history. Values are encrypted at rest with envelope encryption.

**Policy** — A rule that governs who can access what. TeamVault supports three policy models that can be mixed:
- **RBAC** — Role-based: assign roles (admin/editor/viewer) to users and teams
- **ABAC** — Attribute-based: conditions on environment, MFA status, IP range, time
- **PBAC** — Policy-based: full policy documents with subjects, rules, effects, and conditions (AWS IAM style)

---

## Secret Management

### Hierarchical Paths

Secrets use slash-separated paths. You define the hierarchy:

```
services/payment/prod/STRIPE_KEY
services/payment/dev/STRIPE_KEY
services/auth/prod/JWT_SECRET
infrastructure/aws/prod/ACCESS_KEY
agents/openclaw/OPENROUTER_API_KEY
```

### Version History

Every update creates a new version. Roll back anytime:

```bash
teamvault kv put myproject/db/password --value "newpass123"   # creates v2
teamvault kv versions myproject/db/password                    # list versions
teamvault kv rollback myproject/db/password --version 1        # restore v1
```

### Secret Types

- **KV (string)** — API keys, passwords, tokens
- **JSON** — structured configuration
- **File** — certificates, PEM keys, config files

### Folder Tree View

```bash
teamvault kv tree myproject

services/
├── payment/
│   ├── dev/
│   │   ├── STRIPE_KEY
│   │   └── DB_URL
│   └── prod/
│       └── STRIPE_KEY
└── auth/
    └── prod/
        └── JWT_SECRET
```

---

## IAM & Access Control

### Team & Agent Management

```bash
# Create org and team
teamvault org create --name "Acme Corp"
teamvault team create --org "Acme Corp" --name "payments"

# Add members
teamvault team add-member --team payments --email alice@acme.com --role editor
teamvault team add-member --team payments --email bob@acme.com --role viewer

# Register agents
teamvault team add-agent --team payments --name ci-bot \
  --scopes "read:services/payment/*" --ttl 24h
teamvault team add-agent --team payments --name openclaw-agent \
  --scopes "read:agents/openclaw/*" --ttl 1h
```

### How Agents Authenticate

1. An admin creates an agent in a team and gets a one-time token
2. The agent uses this token in `Authorization: Bearer sa.<token>` header
3. Every request is checked against the agent's scopes and applicable policies
4. All access (success and denied) is recorded in the audit log

---

## Policy-as-Code (HCL)

Policies are written in HCL (Terraform syntax) and applied declaratively.

### RBAC — Role-Based Access Control

Assign capabilities based on team role:

```hcl
policy "payments-team-read" {
  description = "Payments team viewers can read payment secrets"

  role = "viewer"
  team = "payments"

  rule {
    path         = "services/payment/*"
    capabilities = ["read", "list"]
  }
}

policy "payments-team-write" {
  description = "Payments team editors can read and write"

  role = "editor"
  team = "payments"

  rule {
    path         = "services/payment/*"
    capabilities = ["read", "write", "list"]
  }
}
```

### ABAC — Attribute-Based Access Control

Add conditions based on request attributes:

```hcl
policy "prod-requires-mfa" {
  description = "Production secrets require MFA and corporate IP"

  rule {
    path         = "services/*/prod/*"
    capabilities = ["read"]

    condition {
      attribute = "mfa_verified"
      operator  = "equals"
      value     = true
    }

    condition {
      attribute = "ip_range"
      operator  = "in_cidr"
      value     = "10.0.0.0/8"
    }
  }
}

policy "business-hours-only" {
  description = "Sensitive secrets only during business hours"

  rule {
    path         = "infrastructure/aws/prod/*"
    capabilities = ["read", "write"]

    condition {
      attribute = "request_hour_utc"
      operator  = "between"
      value     = [9, 18]
    }
  }
}
```

### PBAC — Policy-Based Access Control

Full policy documents with subjects, multiple rules, and mixed effects:

```hcl
policy "ci-agent-deploy" {
  description = "CI agent can read staging, limited prod access"

  subject {
    type = "agent"
    name = "ci-bot"
    team = "platform"
  }

  rule {
    effect       = "allow"
    path         = "services/*/staging/*"
    capabilities = ["read", "list"]
  }

  rule {
    effect       = "allow"
    path         = "services/*/prod/*"
    capabilities = ["read"]

    condition {
      attribute = "approval_count"
      operator  = "gte"
      value     = 2
    }
  }

  rule {
    effect       = "deny"
    path         = "services/*/prod/*"
    capabilities = ["write", "delete"]
  }
}

policy "openclaw-agent-readonly" {
  description = "OpenClaw agent reads its own secrets only"

  subject {
    type = "agent"
    name = "openclaw-agent"
    team = "ai-team"
  }

  rule {
    effect       = "allow"
    path         = "agents/openclaw/*"
    capabilities = ["read"]
  }

  rule {
    effect       = "deny"
    path         = "*"
    capabilities = ["write", "delete", "admin"]
  }
}
```

### Managing Policies

```bash
# Apply all policies in a directory
teamvault policy apply -f policies/

# Validate without applying
teamvault policy validate -f policies/ci-agent.hcl

# List active policies
teamvault policy list

# Inspect a specific policy
teamvault policy inspect ci-agent-deploy
```

### Evaluation Order

1. Collect all policies matching the subject (user role, team membership, agent identity)
2. Evaluate rules against the requested path and capability
3. Check conditions (ABAC attributes)
4. **Explicit deny wins** over allow
5. If no allow matches, **default deny**

---

## CLI Reference

### Authentication

```bash
teamvault login --server https://vault.example.com --email user@company.com
```

### Secret Operations

```bash
teamvault kv put PROJECT/PATH --value VALUE       # create or update
teamvault kv get PROJECT/PATH                      # read (prints raw value)
teamvault kv list PROJECT                           # list all secrets
teamvault kv tree PROJECT                           # folder tree view
teamvault kv delete PROJECT/PATH                    # soft delete
teamvault kv versions PROJECT/PATH                  # version history
teamvault kv rollback PROJECT/PATH --version N      # restore version
```

### Runtime Injection

```bash
# Inject secrets as environment variables and run a command
teamvault run \
  --project myproject \
  --map "STRIPE_KEY=services/payment/prod/STRIPE_KEY" \
  --map "DB_URL=services/payment/prod/DB_URL" \
  -- node server.js
```

The `run` command:
- Fetches each secret from the API
- Sets them as env vars in the child process only
- Uses `syscall.Exec` — secrets never linger in parent memory
- Never prints secret values to stdout/stderr

### Organization & Team Management

```bash
teamvault org create --name "Acme Corp"
teamvault org list

teamvault team create --org "Acme Corp" --name "payments"
teamvault team list --org "Acme Corp"
teamvault team add-member --team payments --email alice@acme.com --role editor
teamvault team add-agent --team payments --name ci-bot --scopes "read:*" --ttl 24h
```

### Tokens

```bash
teamvault token create --name ci-bot --team platform --scopes "read:services/*" --ttl 1h
```

---

## REST API

All endpoints require `Authorization: Bearer <token>`.

### Auth

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/auth/register` | Create account |
| POST | `/api/v1/auth/login` | Get JWT |
| GET | `/api/v1/auth/me` | Current user |

### Organizations & Teams

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/orgs` | Create organization |
| GET | `/api/v1/orgs` | List organizations |
| POST | `/api/v1/orgs/{id}/teams` | Create team in org |
| GET | `/api/v1/orgs/{id}/teams` | List teams |
| POST | `/api/v1/teams/{id}/members` | Add member to team |
| DELETE | `/api/v1/teams/{id}/members/{userId}` | Remove member |
| POST | `/api/v1/teams/{id}/agents` | Register agent |
| GET | `/api/v1/teams/{id}/agents` | List agents |

### Secrets

| Method | Path | Description |
|--------|------|-------------|
| PUT | `/api/v1/secrets/{project}/{path...}` | Create/update secret |
| GET | `/api/v1/secrets/{project}/{path...}` | Read secret (latest) |
| GET | `/api/v1/secrets/{project}` | List secrets in project |
| DELETE | `/api/v1/secrets/{project}/{path...}` | Soft delete |
| GET | `/api/v1/secret-versions/{project}/{path...}` | Version history |

### IAM Policies

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/iam-policies` | Create/update policy |
| GET | `/api/v1/iam-policies` | List policies |
| GET | `/api/v1/iam-policies/{id}` | Get policy detail |
| DELETE | `/api/v1/iam-policies/{id}` | Delete policy |
| POST | `/api/v1/iam-policies/validate` | Validate HCL |

### Audit

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/audit` | Query audit log (filters: action, actor, from, to) |

---

## Web Console

The web console provides a visual interface for managing everything:

- **Dashboard** — Overview of projects, recent activity
- **Projects** — Vault-style folder tree navigation for secrets
- **Secret Detail** — Masked values (click to reveal), version history, copy button
- **Organizations** — Create/manage orgs and teams
- **Team Detail** — Members table, agents table, add/remove
- **Policies** — RBAC/ABAC/PBAC tabs, HCL editor, policy visualization
- **Audit Log** — Filterable event log with outcome badges

Design: dark mode, clean and minimal (1Password-inspired).

---

## OpenClaw Integration

TeamVault integrates seamlessly with [OpenClaw](https://github.com/openclaw/openclaw) AI agents through environment variable injection. No changes to OpenClaw code required.

### Setup (3 steps)

**Step 1.** Add TeamVault credentials to `~/.openclaw/.env`:

```bash
TEAMVAULT_URL=https://vault.example.com:8443
TEAMVAULT_TOKEN=sa.your-agent-token-here
```

**Step 2.** Create `teamvault.json` in your OpenClaw workspace:

```json
{
  "project": "my-openclaw-agent",
  "mappings": {
    "OPENROUTER_API_KEY": "providers/openrouter/api-key",
    "ANTHROPIC_API_KEY": "providers/anthropic/api-key",
    "TELEGRAM_BOT_TOKEN": "channels/telegram/token",
    "DISCORD_TOKEN": "channels/discord/token",
    "BRAVE_API_KEY": "tools/brave-search/api-key"
  }
}
```

**Step 3.** Start OpenClaw with TeamVault injection:

```bash
source teamvault-env-hook.sh && openclaw gateway start
```

Or use the CLI directly:

```bash
teamvault run \
  --project my-openclaw-agent \
  --map "OPENROUTER_API_KEY=providers/openrouter/api-key" \
  --map "DISCORD_TOKEN=channels/discord/token" \
  -- openclaw gateway start
```

### How It Works

1. The hook script reads `teamvault.json` mappings
2. Fetches each secret from TeamVault via REST API (authenticated with service account token)
3. Injects values as environment variables
4. OpenClaw's `${VAR}` substitution in config picks them up automatically
5. No secrets are written to disk — injection is in-memory only

### Agent Policy Example

Lock down the OpenClaw agent to read only its own secrets:

```hcl
policy "openclaw-agent" {
  description = "OpenClaw agent: read-only access to its own secrets"

  subject {
    type = "agent"
    name = "openclaw-agent"
    team = "ai-team"
  }

  rule {
    effect       = "allow"
    path         = "providers/*"
    capabilities = ["read"]
  }

  rule {
    effect       = "allow"
    path         = "channels/*"
    capabilities = ["read"]
  }

  rule {
    effect       = "allow"
    path         = "tools/*"
    capabilities = ["read"]
  }

  rule {
    effect       = "deny"
    path         = "*"
    capabilities = ["write", "delete", "admin"]
  }
}
```

---

## Security Model

### Encryption

- **Envelope Encryption**: Every secret version is encrypted with a unique Data Encryption Key (DEK) using AES-256-GCM. The DEK is encrypted by a master key (local file for dev, KMS for production).
- **Secrets never logged**: Redaction middleware strips values from all server output, logs, and error messages.
- **Secrets never stored in plaintext**: Database contains only ciphertext + encrypted DEK + nonce.

### Authentication

- **Human users**: Email + password → JWT (1h TTL, configurable)
- **Agents**: Service account tokens (random 32-byte, bcrypt-hashed, scoped, time-limited)
- **Token format**: Humans use `Bearer <jwt>`, agents use `Bearer sa.<token>`

### Authorization

- Default deny — no access without explicit allow policy
- Admin users bypass policy checks (for bootstrapping)
- Three policy models (RBAC/ABAC/PBAC) evaluated together
- Explicit deny always wins over allow

### Audit

- Every operation recorded: reads, writes, deletes, policy changes, auth events, denied attempts
- Hash-chained log: each event includes `prev_hash` and `hash` for tamper detection
- Events include: timestamp, actor, action, resource, outcome, IP, metadata

---

## Deployment

### Development (Docker Compose)

```bash
docker compose up -d    # Postgres + API server
cd web && npm run dev   # Web UI on localhost:3000
```

### Production

- **Backend**: Single Go binary, stateless, horizontally scalable
- **Database**: PostgreSQL (managed recommended)
- **KMS**: AWS KMS / GCP KMS / Azure Key Vault for master key (replace local file)
- **Frontend**: Static export or Vercel/Netlify
- **Kubernetes**: Helm chart (coming soon)

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | required |
| `JWT_SECRET` | JWT signing key | required |
| `MASTER_KEY` | 64-char hex master key | required |
| `LISTEN_ADDR` | Server listen address | `:8443` |

---

## Roadmap

### Done

- [x] Secret CRUD with envelope encryption (AES-256-GCM)
- [x] JWT auth + service account tokens
- [x] Hash-chained audit log (tamper-evident)
- [x] CLI: login, kv get/put/list/tree, run (env injection via syscall.Exec)
- [x] Web UI: projects, secrets, audit, settings (Next.js 16, shadcn/ui, dark mode)
- [x] Docker Compose one-command startup
- [x] OpenClaw integration (env hook + teamvault.json config)
- [x] Org/Team/Agent hierarchy (enterprise IAM)
- [x] HCL policy engine (RBAC + ABAC + PBAC, Terraform-style)
- [x] Vault-style folder tree navigation (CLI + Web UI)
- [x] File/blob secret type
- [x] Secret rotation framework (cron scheduler + connector plugins)
- [x] Dynamic secrets with leases (issue/revoke/TTL/auto-cleanup)
- [x] SSO/OIDC integration (graceful degradation)
- [x] Terraform provider stub (teamvault_secret, teamvault_policy, teamvault_project)
- [x] Production hardening: rate limiting, request ID, graceful shutdown, health/ready
- [x] CLI: rotation, lease, doctor, export/import, OIDC login
- [x] Web: rotation UI, leases page, SSO button, version diff, dashboard stats
- [x] E2E demo script (9-step automated demo)

- [x] TEE Confidential Data Plane (software enclave, attestation verifier, secure channel)
- [x] ZK selective disclosure auth (BBS+ signatures, credential issuance, proof verification)
- [x] Browser extension for E2E secret viewing (Chrome/Firefox Manifest V3)
- [x] Kubernetes sidecar injector (mutating webhook) + CSI driver + Helm chart
- [x] Full Terraform provider (CRUD resources, data sources, examples)
- [x] Secret scanning (`teamvault scan`, 17 patterns, pre-commit hooks)
- [x] Webhooks (HMAC-SHA256 signed, retry logic, event types)
- [x] Multi-region replication (WAL-based, vector clocks, leader/follower)

### Next

- [ ] Production TEE backends (Intel SGX, AWS Nitro Enclaves)
- [ ] SCIM provisioning
- [ ] LDAP/Active Directory integration
- [ ] Secret expiration alerts
- [ ] Mobile app (iOS/Android)

---

## Contributing

Pull requests welcome. See `TECH_SPEC.md` for implementation details.

## License

Apache 2.0
