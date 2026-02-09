// Package policy implements the policy evaluation engine.
// It uses simple path-based matching with allow/deny rules.
package policy

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/teamvault/teamvault/internal/db"
)

// Engine evaluates access policies.
type Engine struct {
	database *db.DB
}

// NewEngine creates a new policy evaluation engine.
func NewEngine(database *db.DB) *Engine {
	return &Engine{database: database}
}

// Request represents a policy evaluation request.
type Request struct {
	SubjectType string // "user" or "service_account"
	SubjectID   string
	Action      string // "read", "write", "delete", "list"
	Resource    string // "project/path" format
	IsAdmin     bool   // Admin users bypass policy checks
}

// Result represents the outcome of a policy evaluation.
type Result struct {
	Allowed bool
	Reason  string
}

// Evaluate checks whether the request is allowed based on policies.
// Logic:
//   - Admin users always pass
//   - For each request, find matching policies
//   - If any "deny" matches, deny
//   - If any "allow" matches and no "deny" matches, allow
//   - Default: deny
func (e *Engine) Evaluate(ctx context.Context, req Request) (*Result, error) {
	// Admins bypass policy checks
	if req.IsAdmin {
		return &Result{Allowed: true, Reason: "admin bypass"}, nil
	}

	// Get all policies for this subject
	policies, err := e.database.GetPoliciesForSubject(ctx, req.SubjectType, req.SubjectID)
	if err != nil {
		return nil, err
	}

	hasAllow := false

	for _, policy := range policies {
		// Check if the action matches
		if !matchAction(policy.Actions, req.Action) {
			continue
		}

		// Check if the resource pattern matches
		if !matchResource(policy.ResourcePattern, req.Resource) {
			continue
		}

		// If this is a deny policy, immediately deny
		if policy.Effect == "deny" {
			return &Result{
				Allowed: false,
				Reason:  "denied by policy: " + policy.Name,
			}, nil
		}

		// If this is an allow policy, record it
		if policy.Effect == "allow" {
			hasAllow = true
		}
	}

	if hasAllow {
		return &Result{Allowed: true, Reason: "allowed by policy"}, nil
	}

	return &Result{Allowed: false, Reason: "no matching allow policy (default deny)"}, nil
}

// matchAction checks if the requested action matches any of the policy's actions.
func matchAction(policyActions []string, requestedAction string) bool {
	for _, a := range policyActions {
		if a == "*" || a == requestedAction {
			return true
		}
	}
	return false
}

// matchResource checks if the requested resource matches the policy's resource pattern.
// Supports glob patterns like "myproject/*", "myproject/api-keys/*", etc.
func matchResource(pattern, resource string) bool {
	// Normalize
	pattern = strings.TrimPrefix(pattern, "/")
	resource = strings.TrimPrefix(resource, "/")

	matched, err := filepath.Match(pattern, resource)
	if err != nil {
		return false
	}
	if matched {
		return true
	}

	// Also try matching with ** for deep paths
	// filepath.Match doesn't support **, so handle it manually
	if strings.Contains(pattern, "**") {
		prefix := strings.SplitN(pattern, "**", 2)[0]
		return strings.HasPrefix(resource, prefix)
	}

	// Handle trailing wildcard for directory matching
	// e.g., "myproject/*" should match "myproject/api-keys/stripe"
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(resource, prefix+"/")
	}

	return false
}
