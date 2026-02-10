// Package main contains the TeamVault Terraform provider.
//
// Resources:
//   - teamvault_secret: manage secrets
//   - teamvault_project: manage projects
//   - teamvault_policy: manage IAM policies
//
// Data Sources:
//   - teamvault_secret: read secrets
package main

import (
	"context"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Provider returns the TeamVault Terraform provider schema.
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"address": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("TEAMVAULT_ADDR", nil),
				Description: "TeamVault server address (e.g., https://vault.example.com:8443). Can also be set via TEAMVAULT_ADDR environment variable.",
			},
			"token": {
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				DefaultFunc: schema.EnvDefaultFunc("TEAMVAULT_TOKEN", nil),
				Description: "Authentication token (JWT or service account token). Can also be set via TEAMVAULT_TOKEN environment variable.",
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"teamvault_secret":  resourceSecret(),
			"teamvault_project": resourceProject(),
			"teamvault_policy":  resourcePolicy(),
		},
		DataSourcesMap: map[string]*schema.Resource{
			"teamvault_secret": dataSourceSecret(),
		},
		ConfigureContextFunc: providerConfigure,
	}
}

// providerConfigure creates and returns the TeamVault API client.
func providerConfigure(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	address := d.Get("address").(string)
	token := d.Get("token").(string)

	if address == "" {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "Missing TeamVault address",
			Detail:   "The TeamVault server address must be configured via the 'address' attribute or the TEAMVAULT_ADDR environment variable.",
		})
		return nil, diags
	}

	if token == "" {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "Missing TeamVault token",
			Detail:   "An authentication token must be configured via the 'token' attribute or the TEAMVAULT_TOKEN environment variable.",
		})
		return nil, diags
	}

	client := &TeamVaultClient{
		Address: address,
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	return client, diags
}
