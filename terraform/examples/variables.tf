# TeamVault Provider Configuration
variable "teamvault_address" {
  type        = string
  description = "TeamVault server address"
  default     = "https://vault.example.com:8443"
}

variable "teamvault_token" {
  type        = string
  sensitive   = true
  description = "TeamVault authentication token"
}

# Organization
variable "org_id" {
  type        = string
  description = "TeamVault organization ID for policies"
}

# Secret values (pass via terraform.tfvars or environment)
variable "db_password" {
  type        = string
  sensitive   = true
  description = "Production database password"
}

variable "stripe_api_key" {
  type        = string
  sensitive   = true
  description = "Stripe API key"
}

variable "tls_certificate" {
  type        = string
  sensitive   = true
  description = "TLS certificate PEM content"
  default     = ""
}
