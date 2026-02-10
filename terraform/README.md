# TeamVault Terraform Provider

Manage TeamVault secrets, projects, and IAM policies as Infrastructure-as-Code.

## Features

- **Full CRUD** for secrets (create, read, update, delete)
- **Project management** to organize secrets into namespaces
- **IAM policy management** with HCL-based policy definitions
- **Data source** to read existing secrets for use in other resources
- **Import support** for adopting existing resources into Terraform state
- **Sensitive value handling** — secret values are marked sensitive in Terraform state

## Requirements

- Terraform >= 1.0
- Go >= 1.23 (for building from source)
- A running TeamVault server

## Installation

### From Source

```bash
cd terraform/
go build -o terraform-provider-teamvault

# Install for local development
ARCH=$(go env GOOS)_$(go env GOARCH)
mkdir -p ~/.terraform.d/plugins/local/teamvault/teamvault/0.1.0/${ARCH}/
cp terraform-provider-teamvault ~/.terraform.d/plugins/local/teamvault/teamvault/0.1.0/${ARCH}/
```

### Provider Configuration

```hcl
terraform {
  required_providers {
    teamvault = {
      source  = "local/teamvault/teamvault"
      version = "0.1.0"
    }
  }
}

provider "teamvault" {
  address = "https://vault.example.com:8443"  # or TEAMVAULT_ADDR env var
  token   = var.teamvault_token                # or TEAMVAULT_TOKEN env var
}
```

## Configuration

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `address` | string | Yes | TeamVault server URL. Also: `TEAMVAULT_ADDR` env var. |
| `token` | string | Yes | Auth token (JWT or service account). Also: `TEAMVAULT_TOKEN` env var. |

## Resources

### `teamvault_project`

Manages a project (vault namespace) in TeamVault.

```hcl
resource "teamvault_project" "payments" {
  name        = "payments"
  description = "Payment service secrets"
}
```

#### Arguments

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | Unique project name |
| `description` | string | No | Project description |

#### Attributes

| Name | Description |
|------|-------------|
| `id` | Project ID |
| `created_by` | Creator user/SA ID |
| `created_at` | Creation timestamp |

#### Import

```bash
terraform import teamvault_project.payments payments
```

---

### `teamvault_secret`

Manages an encrypted, versioned secret in TeamVault.

```hcl
resource "teamvault_secret" "db_password" {
  project     = "payments"
  path        = "database/prod/password"
  value       = var.db_password
  type        = "kv"
  description = "Production database password"
}
```

#### Arguments

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `project` | string | Yes | Project name (ForceNew) |
| `path` | string | Yes | Secret path (ForceNew) |
| `value` | string | Yes | Secret value (sensitive) |
| `type` | string | No | Secret type: `kv`, `json`, `file` (default: `kv`) |
| `description` | string | No | Secret description |

#### Attributes

| Name | Description |
|------|-------------|
| `id` | Resource ID (`project/path`) |
| `version` | Current version number |
| `secret_id` | Internal secret UUID |

#### Import

```bash
# Import format: project/path
terraform import teamvault_secret.db_password payments/database/prod/password
```

---

### `teamvault_policy`

Manages an IAM policy with HCL-based policy definitions.

```hcl
resource "teamvault_policy" "read_only" {
  org_id      = var.org_id
  name        = "read-only-payments"
  description = "Read-only access to payment secrets"
  policy_type = "rbac"

  hcl_source = <<-HCL
    policy "read-only-payments" {
      type = "rbac"
      rule {
        effect       = "allow"
        path         = "payments/*"
        capabilities = ["read", "list"]
      }
    }
  HCL
}
```

#### Arguments

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `org_id` | string | Yes | Organization ID (ForceNew) |
| `name` | string | Yes | Policy name |
| `description` | string | No | Policy description |
| `policy_type` | string | Yes | Type: `rbac`, `abac`, `pbac` |
| `hcl_source` | string | No | Policy definition in HCL format |

#### Attributes

| Name | Description |
|------|-------------|
| `id` | Policy ID |
| `policy_doc` | Compiled JSON policy document |
| `created_by` | Creator user ID |
| `created_at` | Creation timestamp |
| `updated_at` | Last update timestamp |

#### Import

```bash
# Import by policy ID
terraform import teamvault_policy.read_only <policy-uuid>
```

---

## Data Sources

### `teamvault_secret` (data)

Reads an existing secret value. Useful for referencing secrets managed outside Terraform.

```hcl
data "teamvault_secret" "db_password" {
  project = "payments"
  path    = "database/prod/password"
}

# Use the value in another resource
resource "aws_db_instance" "main" {
  password = data.teamvault_secret.db_password.value
}
```

#### Arguments

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `project` | string | Yes | Project name |
| `path` | string | Yes | Secret path |

#### Attributes

| Name | Description |
|------|-------------|
| `value` | Decrypted secret value (sensitive) |
| `version` | Current version number |
| `type` | Secret type |
| `description` | Secret description |
| `secret_id` | Internal secret UUID |

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `TEAMVAULT_ADDR` | TeamVault server address |
| `TEAMVAULT_TOKEN` | Authentication token |

## Examples

See the [`examples/`](./examples/) directory for a complete working example including:

- Provider configuration
- Project creation
- Secret management (KV, JSON, file types)
- IAM policy definitions
- Data source usage
- Output values

### Quick Start

```bash
cd examples/

# Set your TeamVault credentials
export TEAMVAULT_ADDR="https://vault.example.com:8443"
export TEAMVAULT_TOKEN="your-token-here"

# Create a variables file
cat > terraform.tfvars <<EOF
org_id         = "your-org-id"
db_password    = "super-secret-password"
stripe_api_key = "sk_live_..."
EOF

# Initialize and apply
terraform init
terraform plan
terraform apply
```

## API Endpoints Used

| Operation | Method | Endpoint |
|-----------|--------|----------|
| Create/Update Secret | `PUT` | `/api/v1/secrets/{project}/{path}` |
| Read Secret | `GET` | `/api/v1/secrets/{project}/{path}` |
| Delete Secret | `DELETE` | `/api/v1/secrets/{project}/{path}` |
| Create Project | `POST` | `/api/v1/projects` |
| List Projects | `GET` | `/api/v1/projects` |
| Create IAM Policy | `POST` | `/api/v1/iam-policies` |
| Read IAM Policy | `GET` | `/api/v1/iam-policies/{id}` |
| Update IAM Policy | `PUT` | `/api/v1/iam-policies/{id}` |
| Delete IAM Policy | `DELETE` | `/api/v1/iam-policies/{id}` |

## Development

### Building

```bash
go build -o terraform-provider-teamvault
```

### Testing

```bash
# Unit tests
go test ./...

# Acceptance tests (requires a running TeamVault server)
TEAMVAULT_ADDR=https://localhost:8443 \
TEAMVAULT_TOKEN=test-token \
TF_ACC=1 go test ./... -v
```

### Debug Mode

```bash
# Run the provider in debug mode for step-through debugging
go build -gcflags="all=-N -l" -o terraform-provider-teamvault
TF_LOG=DEBUG terraform plan
```

## Security Notes

- Secret values are stored in Terraform state. **Always encrypt your state file** (use remote backends with encryption).
- The provider communicates with TeamVault over HTTPS. Ensure your server has valid TLS certificates.
- Use service account tokens with minimal scopes for CI/CD environments.
- Consider using the `teamvault_secret` data source instead of hardcoding secrets in Terraform configurations.
