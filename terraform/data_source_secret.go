package main

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceSecret() *schema.Resource {
	return &schema.Resource{
		Description: "Reads a secret from TeamVault. Use this data source to reference existing secrets in other Terraform resources.",

		ReadContext: dataSourceSecretRead,

		Schema: map[string]*schema.Schema{
			"project": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The project (vault) containing the secret.",
			},
			"path": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The path of the secret within the project.",
			},
			"value": {
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
				Description: "The decrypted secret value.",
			},
			"version": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The current version number of the secret.",
			},
			"type": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The type of the secret (kv, json, file).",
			},
			"description": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The description of the secret.",
			},
			"secret_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The internal ID of the secret.",
			},
		},
	}
}

func dataSourceSecretRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*TeamVaultClient)

	project := d.Get("project").(string)
	path := d.Get("path").(string)

	resp, err := client.GetSecret(project, path)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to read secret %s/%s: %w", project, path, err))
	}

	d.SetId(fmt.Sprintf("%s/%s", project, path))
	d.Set("value", resp.Value)
	d.Set("version", resp.Version)
	d.Set("type", resp.SecretType)
	d.Set("description", resp.Description)
	d.Set("secret_id", resp.ID)

	return nil
}
