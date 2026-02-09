// Package main contains the TeamVault Terraform provider.
//
// This is a provider skeleton. When terraform-plugin-sdk/v2 is added as a
// dependency, uncomment the implementation below.
//
// Resources:
//   - teamvault_secret: manage secrets
//   - teamvault_policy: manage IAM policies
//   - teamvault_project: manage projects
//
// Data Sources:
//   - teamvault_secret: read secrets
package main

// Provider schema and configuration.
//
// When terraform-plugin-sdk is available, this becomes:
//
//   import (
//       "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
//   )
//
//   func Provider() *schema.Provider {
//       return &schema.Provider{
//           Schema: map[string]*schema.Schema{
//               "address": {
//                   Type:        schema.TypeString,
//                   Required:    true,
//                   DefaultFunc: schema.EnvDefaultFunc("TEAMVAULT_ADDR", nil),
//                   Description: "TeamVault server address (e.g., https://vault.example.com:8443)",
//               },
//               "token": {
//                   Type:        schema.TypeString,
//                   Required:    true,
//                   Sensitive:   true,
//                   DefaultFunc: schema.EnvDefaultFunc("TEAMVAULT_TOKEN", nil),
//                   Description: "Authentication token (JWT or service account token)",
//               },
//           },
//           ResourcesMap: map[string]*schema.Resource{
//               "teamvault_secret":  resourceSecret(),
//               "teamvault_policy":  resourcePolicy(),
//               "teamvault_project": resourceProject(),
//           },
//           DataSourcesMap: map[string]*schema.Resource{
//               "teamvault_secret": dataSourceSecret(),
//           },
//           ConfigureContextFunc: providerConfigure,
//       }
//   }
//
//   func providerConfigure(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
//       address := d.Get("address").(string)
//       token := d.Get("token").(string)
//       client := &TeamVaultClient{
//           Address: address,
//           Token:   token,
//           HTTP:    &http.Client{Timeout: 30 * time.Second},
//       }
//       return client, nil
//   }

// TeamVaultClient is the API client used by Terraform resources.
type TeamVaultClient struct {
	Address string
	Token   string
}

// --- Resource: teamvault_secret ---
//
//   func resourceSecret() *schema.Resource {
//       return &schema.Resource{
//           CreateContext: resourceSecretCreate,
//           ReadContext:   resourceSecretRead,
//           UpdateContext: resourceSecretUpdate,
//           DeleteContext: resourceSecretDelete,
//           Schema: map[string]*schema.Schema{
//               "project": {Type: schema.TypeString, Required: true, ForceNew: true},
//               "path":    {Type: schema.TypeString, Required: true, ForceNew: true},
//               "value":   {Type: schema.TypeString, Required: true, Sensitive: true},
//               "type":    {Type: schema.TypeString, Optional: true, Default: "kv"},
//               "description": {Type: schema.TypeString, Optional: true},
//               "version": {Type: schema.TypeInt, Computed: true},
//           },
//       }
//   }

// --- Resource: teamvault_policy ---
//
//   func resourcePolicy() *schema.Resource {
//       return &schema.Resource{
//           CreateContext: resourcePolicyCreate,
//           ReadContext:   resourcePolicyRead,
//           UpdateContext: resourcePolicyUpdate,
//           DeleteContext: resourcePolicyDelete,
//           Schema: map[string]*schema.Schema{
//               "org_id":      {Type: schema.TypeString, Required: true},
//               "name":        {Type: schema.TypeString, Required: true},
//               "description": {Type: schema.TypeString, Optional: true},
//               "policy_type": {Type: schema.TypeString, Required: true},
//               "hcl_source":  {Type: schema.TypeString, Optional: true},
//               "policy_doc":  {Type: schema.TypeString, Optional: true},
//           },
//       }
//   }

// --- Resource: teamvault_project ---
//
//   func resourceProject() *schema.Resource {
//       return &schema.Resource{
//           CreateContext: resourceProjectCreate,
//           ReadContext:   resourceProjectRead,
//           UpdateContext: resourceProjectUpdate,
//           DeleteContext: resourceProjectDelete,
//           Schema: map[string]*schema.Schema{
//               "name":        {Type: schema.TypeString, Required: true},
//               "description": {Type: schema.TypeString, Optional: true},
//           },
//       }
//   }

// --- Data Source: teamvault_secret ---
//
//   func dataSourceSecret() *schema.Resource {
//       return &schema.Resource{
//           ReadContext: dataSourceSecretRead,
//           Schema: map[string]*schema.Schema{
//               "project": {Type: schema.TypeString, Required: true},
//               "path":    {Type: schema.TypeString, Required: true},
//               "value":   {Type: schema.TypeString, Computed: true, Sensitive: true},
//               "version": {Type: schema.TypeInt, Computed: true},
//               "type":    {Type: schema.TypeString, Computed: true},
//           },
//       }
//   }
