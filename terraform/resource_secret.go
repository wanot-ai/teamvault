package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceSecret() *schema.Resource {
	return &schema.Resource{
		Description: "Manages a secret in TeamVault. Secrets are stored encrypted and versioned.",

		CreateContext: resourceSecretCreate,
		ReadContext:   resourceSecretRead,
		UpdateContext: resourceSecretUpdate,
		DeleteContext: resourceSecretDelete,

		Importer: &schema.ResourceImporter{
			StateContext: resourceSecretImport,
		},

		Schema: map[string]*schema.Schema{
			"project": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The project (vault) containing this secret.",
			},
			"path": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The path of the secret within the project (e.g., 'services/database/prod/password').",
			},
			"value": {
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				Description: "The secret value. This value is stored encrypted in TeamVault.",
			},
			"type": {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          "kv",
				Description:      "The type of secret: 'kv' (key-value), 'json', or 'file'.",
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringInSlice([]string{"kv", "json", "file"}, false)),
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "A human-readable description of the secret.",
			},
			"version": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The current version number of the secret.",
			},
			"secret_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The internal ID of the secret in TeamVault.",
			},
		},
	}
}

func resourceSecretCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*TeamVaultClient)

	project := d.Get("project").(string)
	path := d.Get("path").(string)
	value := d.Get("value").(string)
	secretType := d.Get("type").(string)
	description := d.Get("description").(string)

	req := &SecretRequest{
		Value:       value,
		Description: description,
		Type:        secretType,
	}

	resp, err := client.PutSecret(project, path, req)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to create secret %s/%s: %w", project, path, err))
	}

	// Use project/path as the resource ID for idempotent operations
	d.SetId(fmt.Sprintf("%s/%s", project, path))
	d.Set("version", resp.Version)
	d.Set("secret_id", resp.ID)

	return nil
}

func resourceSecretRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*TeamVaultClient)

	project, path, err := parseSecretID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	resp, err := client.GetSecret(project, path)
	if err != nil {
		if IsNotFound(err) {
			// Secret was deleted outside Terraform
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read secret %s/%s: %w", project, path, err))
	}

	d.Set("project", resp.Project)
	d.Set("path", resp.Path)
	d.Set("value", resp.Value)
	d.Set("type", resp.SecretType)
	d.Set("description", resp.Description)
	d.Set("version", resp.Version)
	d.Set("secret_id", resp.ID)

	return nil
}

func resourceSecretUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*TeamVaultClient)

	project, path, err := parseSecretID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	if d.HasChanges("value", "description", "type") {
		req := &SecretRequest{
			Value:       d.Get("value").(string),
			Description: d.Get("description").(string),
			Type:        d.Get("type").(string),
		}

		resp, err := client.PutSecret(project, path, req)
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to update secret %s/%s: %w", project, path, err))
		}

		d.Set("version", resp.Version)
		d.Set("secret_id", resp.ID)
	}

	return nil
}

func resourceSecretDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*TeamVaultClient)

	project, path, err := parseSecretID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	if err := client.DeleteSecret(project, path); err != nil {
		if IsNotFound(err) {
			// Already deleted
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to delete secret %s/%s: %w", project, path, err))
	}

	return nil
}

// resourceSecretImport imports an existing secret into Terraform state.
// Import ID format: "project/path/to/secret"
func resourceSecretImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	id := d.Id()
	project, path, err := parseSecretID(id)
	if err != nil {
		return nil, fmt.Errorf("invalid import ID %q: expected format 'project/path/to/secret': %w", id, err)
	}

	d.SetId(id)
	d.Set("project", project)
	d.Set("path", path)

	return []*schema.ResourceData{d}, nil
}

// parseSecretID splits "project/path/to/secret" into project and path.
func parseSecretID(id string) (string, string, error) {
	parts := strings.SplitN(id, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid secret ID %q: expected format 'project/path'", id)
	}
	return parts[0], parts[1], nil
}
