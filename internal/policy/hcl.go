package policy

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsimple"
)

// HCLPolicy represents the top-level HCL policy structure.
type HCLPolicy struct {
	Name    string          `hcl:"name,label"`
	Type    string          `hcl:"type"`
	Subject *HCLSubject     `hcl:"subject,block"`
	Rules   []HCLRule       `hcl:"rule,block"`
}

// HCLSubject represents the subject block in HCL.
type HCLSubject struct {
	Type string `hcl:"type"`
	Name string `hcl:"name,optional"`
	Team string `hcl:"team,optional"`
	Role string `hcl:"role,optional"`
}

// HCLRule represents a rule block in HCL.
type HCLRule struct {
	Effect       string         `hcl:"effect"`
	Path         string         `hcl:"path"`
	Capabilities []string       `hcl:"capabilities"`
	Conditions   []HCLCondition `hcl:"condition,block"`
}

// HCLCondition represents a condition block in HCL.
type HCLCondition struct {
	Attribute string `hcl:"attribute"`
	Operator  string `hcl:"operator,optional"`
	Value     string `hcl:"value"`
}

// HCLFile wraps the top-level file to parse multiple policies.
type HCLFile struct {
	Policies []HCLPolicy `hcl:"policy,block"`
}

// ParseHCL parses an HCL policy file and returns a PolicyDocument.
// The HCL format is:
//
//	policy "ci-agent-deploy" {
//	  type = "pbac"
//
//	  subject {
//	    type = "agent"
//	    name = "ci-bot"
//	    team = "platform"
//	  }
//
//	  rule {
//	    effect       = "allow"
//	    path         = "services/*/staging/*"
//	    capabilities = ["read", "list"]
//
//	    condition {
//	      attribute = "environment"
//	      operator  = "eq"
//	      value     = "staging"
//	    }
//	  }
//	}
func ParseHCL(src []byte, filename string) (*PolicyDocument, error) {
	var file HCLFile
	err := hclsimple.Decode(filename, src, nil, &file)
	if err != nil {
		// Try to provide helpful error messages
		if diags, ok := err.(hcl.Diagnostics); ok {
			for _, diag := range diags {
				if diag.Severity == hcl.DiagError {
					return nil, fmt.Errorf("HCL parse error at %s: %s", diag.Subject, diag.Detail)
				}
			}
		}
		return nil, fmt.Errorf("parsing HCL: %w", err)
	}

	if len(file.Policies) == 0 {
		return nil, fmt.Errorf("no policy block found in HCL")
	}

	// Use the first policy (for single-policy documents)
	hclPol := file.Policies[0]
	return convertHCLToDocument(hclPol)
}

// ParseHCLMulti parses an HCL file containing multiple policy blocks.
func ParseHCLMulti(src []byte, filename string) ([]PolicyDocument, error) {
	var file HCLFile
	err := hclsimple.Decode(filename, src, nil, &file)
	if err != nil {
		return nil, fmt.Errorf("parsing HCL: %w", err)
	}

	var docs []PolicyDocument
	for _, hclPol := range file.Policies {
		doc, err := convertHCLToDocument(hclPol)
		if err != nil {
			return nil, fmt.Errorf("converting policy %q: %w", hclPol.Name, err)
		}
		docs = append(docs, *doc)
	}

	return docs, nil
}

// convertHCLToDocument converts a parsed HCL policy into a PolicyDocument.
func convertHCLToDocument(hclPol HCLPolicy) (*PolicyDocument, error) {
	doc := &PolicyDocument{
		Name: hclPol.Name,
		Type: hclPol.Type,
	}

	if doc.Type == "" {
		doc.Type = "pbac" // Default to PBAC
	}

	// Convert subject
	if hclPol.Subject != nil {
		doc.Subject = &PolicySubject{
			Type: hclPol.Subject.Type,
			Name: hclPol.Subject.Name,
			Team: hclPol.Subject.Team,
			Role: hclPol.Subject.Role,
		}
	}

	// Convert rules
	for _, hclRule := range hclPol.Rules {
		rule := PolicyRule{
			Effect:       hclRule.Effect,
			Path:         hclRule.Path,
			Capabilities: hclRule.Capabilities,
		}

		if rule.Effect == "" {
			rule.Effect = "allow" // Default effect
		}

		// Convert conditions
		for _, hclCond := range hclRule.Conditions {
			cond := PolicyCondition{
				Attribute: hclCond.Attribute,
				Operator:  hclCond.Operator,
				Value:     hclCond.Value,
			}
			if cond.Operator == "" {
				cond.Operator = "eq" // Default operator
			}
			rule.Conditions = append(rule.Conditions, cond)
		}

		doc.Rules = append(doc.Rules, rule)
	}

	return doc, nil
}
