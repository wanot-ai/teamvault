package main

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceProject() *schema.Resource {
	return &schema.Resource{
		Description: "Manages a project (vault namespace) in TeamVault. Projects are the top-level container for organizing secrets.",

		CreateContext: resourceProjectCreate,
		ReadContext:   resourceProjectRead,
		UpdateContext: resourceProjectUpdate,
		DeleteContext: resourceProjectDelete,

		Importer: &schema.ResourceImporter{
			StateContext: resourceProjectImport,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The unique name of the project.",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "A human-readable description of the project.",
			},
			"created_by": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The user or service account that created the project.",
			},
			"created_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The timestamp when the project was created.",
			},
		},
	}
}

func resourceProjectCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*TeamVaultClient)

	name := d.Get("name").(string)
	description := d.Get("description").(string)

	req := &ProjectRequest{
		Name:        name,
		Description: description,
	}

	resp, err := client.CreateProject(req)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to create project %q: %w", name, err))
	}

	d.SetId(resp.ID)
	d.Set("name", resp.Name)
	d.Set("description", resp.Description)
	d.Set("created_by", resp.CreatedBy)
	d.Set("created_at", resp.CreatedAt)

	return nil
}

func resourceProjectRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*TeamVaultClient)

	name := d.Get("name").(string)
	if name == "" {
		// During import, name might not be set yet — use ID as fallback
		name = d.Id()
	}

	resp, err := client.GetProjectByName(name)
	if err != nil {
		if IsNotFound(err) {
			// Project was deleted outside Terraform
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read project %q: %w", name, err))
	}

	d.SetId(resp.ID)
	d.Set("name", resp.Name)
	d.Set("description", resp.Description)
	d.Set("created_by", resp.CreatedBy)
	d.Set("created_at", resp.CreatedAt)

	return nil
}

func resourceProjectUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// The TeamVault API currently does not support project updates (PUT/PATCH).
	// If the name changes, we must recreate; description changes are noted but
	// cannot be applied until the API supports it.
	if d.HasChange("name") {
		// Name change requires destroy + create (ForceNew would be better,
		// but we keep name mutable for future API compatibility).
		return diag.Diagnostics{
			{
				Severity: diag.Warning,
				Summary:  "Project rename not supported",
				Detail:   "The TeamVault API does not currently support renaming projects. To change the name, destroy and recreate the resource.",
			},
		}
	}

	if d.HasChange("description") {
		return diag.Diagnostics{
			{
				Severity: diag.Warning,
				Summary:  "Project description update not supported",
				Detail:   "The TeamVault API does not currently support updating project descriptions. The change has been noted but not applied.",
			},
		}
	}

	return resourceProjectRead(ctx, d, meta)
}

func resourceProjectDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// The TeamVault API currently does not expose a project delete endpoint.
	// We remove it from Terraform state. The project will remain in TeamVault
	// and can be re-imported.
	return diag.Diagnostics{
		{
			Severity: diag.Warning,
			Summary:  "Project deletion not fully supported",
			Detail:   "The TeamVault API does not expose a project delete endpoint. The project has been removed from Terraform state but still exists in TeamVault.",
		},
	}
}

// resourceProjectImport imports a project by name.
func resourceProjectImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	// Import ID is the project name
	name := d.Id()
	d.Set("name", name)

	return []*schema.ResourceData{d}, nil
}
