# TeamVault

Enterprise secret management for teams and AI agents.

TeamVault lets organizations manage secrets through a web console, enforce fine-grained access policies (RBAC/ABAC/PBAC), and inject secrets into services, CI/CD pipelines, and AI agents at runtime — with envelope encryption, hash-chained audit logs, and HCL-based policy-as-code.

## Quick Start

```bash
git clone https://github.com/wanot-ai/teamvault.git
cd teamvault
docker compose up -d
```

API server runs on `http://localhost:8443`. Web UI: `cd web && npm install && npm run dev` → `http://localhost:3000`.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Organization                          │
│                                                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                  │
│  │  Team A   │  │  Team B   │  │  Team C   │                │
│  │           │  │           │  │           │                 │
│  │ Members:  │  │ Members:  │  │ Members:  │                │
│  │  alice    │  │  carol    │  │  eve      │                │
│  │  bob      │  │  dave     │  │           │                │
│  │           │  │           │  │           │                 │
│  │ Agents:   │  │ Agents:   │  │ Agents:   │                │
│  │  ci-bot   │  │  deploy   │  │  monitor  │                │
│  │  openclaw │  │  k8s-sa   │  │           │                │
│  └──────────┘  └──────────┘  └──────────┘                  │
│                                                              │
│  ┌──────────────────────────────────────────────┐           │
│  │                   Vaults                      │           │
│  │                                               │           │
│  │  services/                                    │           │
│  │  ├── payment/                                 │           │
│  │  │   ├── dev/                                 │           │
│  │  │   │   ├── STRIPE_KEY                       │           │
│  │  │   │   └── DB_URL                           │           │
│  │  │   ├── staging/                             │           │
│  │  │   │   └── STRIPE_KEY                       │           │
│  │  │   └── prod/                                │           │
│  │  │       ├── STRIPE_KEY                       │           │
│  │  │       └── DB_URL                           │           │
│  │  └── auth/                                    │           │
│  │      └── prod/                                │           │
│  │          └── JWT_SECRET                        │           │
│  └──────────────────────────────────────────────┘           │
│                                                              │
│  ┌──────────────────────────────────────────────┐           │
│  │              IAM Policies (HCL)               │           │
│  │                                               │           │
│  │  RBAC: role-based (admin, editor, viewer)     │           │
│  │  ABAC: attribute-based (env=prod, team=pay)   │           │
│  │  PBAC: policy document (Terraform-style)      │           │
│  └──────────────────────────────────────────────┘           │
└─────────────────────────────────────────────────────────────┘
```

## Features

**Secret Management**
- Hierarchical path-based secrets (any depth: `services/payment/prod/STRIPE_KEY`)
- Version history with rollback
- KV pairs, JSON blobs, file attachments
- Envelope encryption (AES-256-GCM) with KMS integration

**IAM & Access Control**
- Organizations with multiple teams
- Multiple agents per team (CI bots, AI agents, services)
- Three policy models: RBAC, ABAC, PBAC — mix and match
- HCL (Terraform-style) policy-as-code
- Service account tokens with scoped, time-limited access

**Audit & Compliance**
- Every read/write/deny recorded
- Hash-chained audit log (tamper-evident)
- Filterable audit viewer in web UI

**Developer Experience**
- CLI: `teamvault run -- node server.js` injects secrets as env vars
- REST API for programmatic access
- Web console for team management

## Policy-as-Code (HCL)

TeamVault policies use HCL syntax, managed like Terraform configurations.

### RBAC (Role-Based)

```hcl
policy "payments-team-read" {
  description = "Payments team can read payment secrets"

  role = "viewer"
  team = "payments"

  rule {
    path   = "services/payment/*"
    capabilities = ["read", "list"]
  }
}
```

### ABAC (Attribute-Based)

```hcl
policy "prod-mfa-required" {
  description = "Production secrets require MFA"

  rule {
    path   = "services/*/prod/*"
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
```

### PBAC (Policy-Based, AWS IAM-style)

```hcl
policy "ci-agent-deploy" {
  description = "CI agent can read staging/prod deploy secrets"

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
```

### Apply Policies

```bash
teamvault policy apply -f policies/
teamvault policy validate -f policies/ci-agent.hcl
teamvault policy list
teamvault policy inspect payments-team-read
```

## CLI Usage

```bash
# Login
teamvault login --server https://vault.example.com --email alice@company.com

# Manage secrets
teamvault kv put services/payment/prod/STRIPE_KEY --value sk_live_xxx
teamvault kv get services/payment/prod/STRIPE_KEY
teamvault kv list services/payment/prod

# Inject secrets and run a command
teamvault run \
  --project myproject \
  --map "STRIPE_KEY=services/payment/prod/STRIPE_KEY" \
  --map "DB_URL=services/payment/prod/DB_URL" \
  -- node server.js

# Agent token management
teamvault token create \
  --name ci-bot \
  --team platform \
  --scopes "read:services/*/staging/*" \
  --ttl 1h
```

## REST API

All endpoints require `Authorization: Bearer <token>`.

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/auth/register` | Create account |
| POST | `/api/v1/auth/login` | Get JWT |
| GET | `/api/v1/auth/me` | Current user |
| POST | `/api/v1/orgs` | Create organization |
| POST | `/api/v1/teams` | Create team |
| POST | `/api/v1/agents` | Register agent in team |
| POST | `/api/v1/projects` | Create vault/project |
| PUT | `/api/v1/secrets/{project}/{path...}` | Create/update secret |
| GET | `/api/v1/secrets/{project}/{path...}` | Read secret |
| GET | `/api/v1/secrets/{project}` | List secrets |
| DELETE | `/api/v1/secrets/{project}/{path...}` | Soft delete |
| POST | `/api/v1/policies` | Create policy |
| GET | `/api/v1/policies` | List policies |
| POST | `/api/v1/policies/validate` | Validate HCL policy |
| GET | `/api/v1/audit` | Query audit log |

## Security

- **Envelope Encryption**: Every secret version encrypted with a unique DEK (AES-256-GCM), DEK encrypted by master key
- **Audit Trail**: Hash-chained events — tamper detection built in
- **Secrets never logged**: Redaction middleware strips sensitive values from all output
- **Short-lived tokens**: JWT (1h default), service account tokens with configurable TTL
- **Policy evaluation**: Default deny — explicit allow required

## Tech Stack

- **Backend**: Go, PostgreSQL, AES-256-GCM envelope encryption
- **Frontend**: Next.js 16, TypeScript, Tailwind CSS, shadcn/ui
- **CLI**: Go (single binary, cross-platform)
- **Infrastructure**: Docker Compose (dev), Kubernetes Helm (prod)

## Roadmap

- [x] MVP: Secret CRUD, envelope encryption, JWT auth, audit log, CLI, Web UI
- [ ] Org/Team/Agent hierarchy
- [ ] HCL policy engine (RBAC + ABAC + PBAC)
- [ ] Folder-tree navigation in Web UI (HashiCorp Vault style)
- [ ] File/blob secret type
- [ ] Secret rotation framework
- [ ] Dynamic secrets (database credentials, cloud tokens)
- [ ] SSO/OIDC integration
- [ ] TEE Confidential Data Plane (Phase 2)
- [ ] ZK selective disclosure auth (Phase 3)
- [ ] Terraform provider for TeamVault

## License

Apache 2.0
