# TeamVault Terraform Provider

Manage TeamVault secrets, policies, and projects as Infrastructure-as-Code.

## Status

**Stub** — This provider skeleton defines the resource and data source schemas. Full CRUD implementation requires adding `terraform-plugin-sdk/v2` as a dependency.

## Setup

### Build

```bash
cd terraform/
go mod init github.com/teamvault/terraform-provider-teamvault
go get github.com/hashicorp/terraform-plugin-sdk/v2
go build -o terraform-provider-teamvault
```

### Install (local development)

```bash
mkdir -p ~/.terraform.d/plugins/local/teamvault/teamvault/0.1.0/$(go env GOOS)_$(go env GOARCH)/
cp terraform-provider-teamvault ~/.terraform.d/plugins/local/teamvault/teamvault/0.1.0/$(go env GOOS)_$(go env GOARCH)/
```

## Configuration

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

## Resources

### teamvault_secret

Manage a secret in TeamVault.

```hcl
resource "teamvault_secret" "stripe_key" {
  project     = "my-project"
  path        = "services/payment/prod/STRIPE_KEY"
  value       = var.stripe_key
  type        = "kv"
  description = "Stripe API key for production"
}
```

### teamvault_project

Manage a project (vault).

```hcl
resource "teamvault_project" "payments" {
  name        = "payments"
  description = "Payment service secrets"
}
```

### teamvault_policy

Manage an IAM policy.

```hcl
resource "teamvault_policy" "read_only" {
  org_id      = var.org_id
  name        = "read-only-payment"
  description = "Read-only access to payment secrets"
  policy_type = "rbac"
  hcl_source  = <<-HCL
    policy "read-only-payment" {
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

## Data Sources

### teamvault_secret (read)

Read a secret value (for use in other resources).

```hcl
data "teamvault_secret" "db_password" {
  project = "my-project"
  path    = "services/database/prod/password"
}

# Use in another resource
resource "aws_db_instance" "main" {
  password = data.teamvault_secret.db_password.value
  # ...
}
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `TEAMVAULT_ADDR` | TeamVault server address |
| `TEAMVAULT_TOKEN` | Authentication token |

## Roadmap

- [ ] Full CRUD for `teamvault_secret`
- [ ] Full CRUD for `teamvault_policy`
- [ ] Full CRUD for `teamvault_project`
- [ ] Data source: `teamvault_secret`
- [ ] Import support for existing resources
- [ ] Acceptance tests
- [ ] Registry publication
