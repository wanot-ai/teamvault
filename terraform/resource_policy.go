package main

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourcePolicy() *schema.Resource {
	return &schema.Resource{
		Description: "Manages an IAM policy in TeamVault. Policies define access control rules using RBAC, ABAC, or PBAC models.",

		CreateContext: resourcePolicyCreate,
		ReadContext:   resourcePolicyRead,
		UpdateContext: resourcePolicyUpdate,
		DeleteContext: resourcePolicyDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"org_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The organization ID this policy belongs to.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The unique name of the policy within the organization.",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "A human-readable description of what this policy does.",
			},
			"policy_type": {
				Type:             schema.TypeString,
				Required:         true,
				Description:      "The type of policy: 'rbac' (role-based), 'abac' (attribute-based), or 'pbac' (policy-based).",
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringInSlice([]string{"rbac", "abac", "pbac"}, false)),
			},
			"hcl_source": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The policy definition in HCL format. If provided, the server parses it into the policy document.",
			},
			"policy_doc": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The compiled policy document (JSON). Computed from hcl_source.",
			},
			"created_by": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The user who created this policy.",
			},
			"created_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The timestamp when the policy was created.",
			},
			"updated_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The timestamp when the policy was last updated.",
			},
		},
	}
}

func resourcePolicyCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*TeamVaultClient)

	req := &IAMPolicyRequest{
		OrgID:       d.Get("org_id").(string),
		Name:        d.Get("name").(string),
		Description: d.Get("description").(string),
		PolicyType:  d.Get("policy_type").(string),
		HCLSource:   d.Get("hcl_source").(string),
	}

	resp, err := client.CreateIAMPolicy(req)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to create policy %q: %w", req.Name, err))
	}

	d.SetId(resp.ID)
	setPolicyState(d, resp)

	return nil
}

func resourcePolicyRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*TeamVaultClient)

	id := d.Id()
	resp, err := client.GetIAMPolicy(id)
	if err != nil {
		if IsNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read policy %q: %w", id, err))
	}

	setPolicyState(d, resp)

	return nil
}

func resourcePolicyUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*TeamVaultClient)

	id := d.Id()

	if d.HasChanges("name", "description", "policy_type", "hcl_source") {
		req := &IAMPolicyRequest{
			OrgID:       d.Get("org_id").(string),
			Name:        d.Get("name").(string),
			Description: d.Get("description").(string),
			PolicyType:  d.Get("policy_type").(string),
			HCLSource:   d.Get("hcl_source").(string),
		}

		resp, err := client.UpdateIAMPolicy(id, req)
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to update policy %q: %w", id, err))
		}

		setPolicyState(d, resp)
	}

	return nil
}

func resourcePolicyDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*TeamVaultClient)

	id := d.Id()
	if err := client.DeleteIAMPolicy(id); err != nil {
		if IsNotFound(err) {
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to delete policy %q: %w", id, err))
	}

	return nil
}

// setPolicyState updates the Terraform resource data from an API response.
func setPolicyState(d *schema.ResourceData, resp *IAMPolicyResponse) {
	d.Set("org_id", resp.OrgID)
	d.Set("name", resp.Name)
	d.Set("description", resp.Description)
	d.Set("policy_type", resp.PolicyType)
	d.Set("hcl_source", resp.HCLSource)
	d.Set("policy_doc", string(resp.PolicyDoc))
	d.Set("created_by", resp.CreatedBy)
	d.Set("created_at", resp.CreatedAt)
	d.Set("updated_at", resp.UpdatedAt)
}
