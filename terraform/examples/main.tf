# TeamVault Terraform Provider - Complete Example
#
# This example demonstrates managing projects, secrets, and policies
# using the TeamVault Terraform provider.

terraform {
  required_version = ">= 1.0"

  required_providers {
    teamvault = {
      source  = "local/teamvault/teamvault"
      version = "0.1.0"
    }
  }
}

# Configure the TeamVault provider
provider "teamvault" {
  address = var.teamvault_address
  token   = var.teamvault_token
}

# --- Projects ---

# Create a project for the payment service
resource "teamvault_project" "payments" {
  name        = "payments"
  description = "Payment service secrets for all environments"
}

# Create a project for the API gateway
resource "teamvault_project" "api_gateway" {
  name        = "api-gateway"
  description = "API gateway configuration and secrets"
}

# --- Secrets ---

# Store a database password
resource "teamvault_secret" "db_password" {
  project     = teamvault_project.payments.name
  path        = "database/prod/password"
  value       = var.db_password
  type        = "kv"
  description = "Production database password"
}

# Store a Stripe API key
resource "teamvault_secret" "stripe_key" {
  project     = teamvault_project.payments.name
  path        = "services/stripe/api-key"
  value       = var.stripe_api_key
  type        = "kv"
  description = "Stripe API key for payment processing"
}

# Store a JSON configuration
resource "teamvault_secret" "service_config" {
  project     = teamvault_project.api_gateway.name
  path        = "config/production"
  value       = jsonencode({
    rate_limit  = 1000
    timeout_ms  = 5000
    upstream    = "https://api.internal:8080"
    cors_origins = ["https://app.example.com"]
  })
  type        = "json"
  description = "API gateway production configuration"
}

# Store a TLS certificate
resource "teamvault_secret" "tls_cert" {
  project     = teamvault_project.api_gateway.name
  path        = "tls/server.crt"
  value       = var.tls_certificate
  type        = "file"
  description = "TLS certificate for the API gateway"
}

# --- Policies ---

# Create a read-only policy for the payments project
resource "teamvault_policy" "payments_readonly" {
  org_id      = var.org_id
  name        = "payments-readonly"
  description = "Read-only access to payment secrets"
  policy_type = "rbac"

  hcl_source = <<-HCL
    policy "payments-readonly" {
      type = "rbac"

      rule {
        effect       = "allow"
        path         = "payments/*"
        capabilities = ["read", "list"]
      }

      rule {
        effect       = "deny"
        path         = "payments/database/*"
        capabilities = ["read"]
      }
    }
  HCL
}

# Create a full-access policy for the DevOps team
resource "teamvault_policy" "devops_full_access" {
  org_id      = var.org_id
  name        = "devops-full-access"
  description = "Full access for the DevOps team"
  policy_type = "rbac"

  hcl_source = <<-HCL
    policy "devops-full-access" {
      type = "rbac"

      rule {
        effect       = "allow"
        path         = "*"
        capabilities = ["read", "write", "delete", "list"]
      }
    }
  HCL
}

# --- Data Sources ---

# Read an existing secret (e.g., one created by another team)
data "teamvault_secret" "shared_api_key" {
  project = "shared-infrastructure"
  path    = "services/monitoring/api-key"
}
