# Project IDs
output "payments_project_id" {
  value       = teamvault_project.payments.id
  description = "The ID of the payments project"
}

output "api_gateway_project_id" {
  value       = teamvault_project.api_gateway.id
  description = "The ID of the API gateway project"
}

# Secret versions (useful for tracking changes)
output "db_password_version" {
  value       = teamvault_secret.db_password.version
  description = "Current version of the database password secret"
}

output "stripe_key_version" {
  value       = teamvault_secret.stripe_key.version
  description = "Current version of the Stripe API key secret"
}

# Policy IDs
output "payments_readonly_policy_id" {
  value       = teamvault_policy.payments_readonly.id
  description = "The ID of the payments read-only policy"
}

output "devops_policy_id" {
  value       = teamvault_policy.devops_full_access.id
  description = "The ID of the DevOps full-access policy"
}

# Data source values (demonstrate reading existing secrets)
output "shared_api_key_version" {
  value       = data.teamvault_secret.shared_api_key.version
  description = "Version of the shared monitoring API key"
}
