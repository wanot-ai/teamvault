// Package policy implements the policy evaluation engine.
// It supports three policy types:
//   - RBAC: Role-Based Access Control (check user/agent role against path)
//   - ABAC: Attribute-Based Access Control (check attributes like environment, mfa, ip, team)
//   - PBAC: Policy-Based Access Control (full policy documents with rules, effects, conditions)
package policy

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"strings"

	"github.com/teamvault/teamvault/internal/db"
)

// Engine evaluates access policies (both legacy and IAM).
type Engine struct {
	database *db.DB
}

// NewEngine creates a new policy evaluation engine.
func NewEngine(database *db.DB) *Engine {
	return &Engine{database: database}
}

// Request represents a policy evaluation request.
type Request struct {
	SubjectType string // "user", "service_account", or "agent"
	SubjectID   string
	Action      string // "read", "write", "delete", "list"
	Resource    string // "project/path" format
	IsAdmin     bool   // Admin users bypass policy checks

	// IAM-specific fields
	OrgID string // Organization context for IAM policy lookup

	// Attributes for ABAC evaluation
	Attributes *RequestAttributes
}

// RequestAttributes holds contextual attributes for ABAC evaluation.
type RequestAttributes struct {
	Environment string `json:"environment,omitempty"` // "production", "staging", "dev"
	MFA         bool   `json:"mfa,omitempty"`         // Whether MFA was used
	IP          string `json:"ip,omitempty"`           // Client IP address
	Team        string `json:"team,omitempty"`         // Team name
	Role        string `json:"role,omitempty"`         // User/agent role
	AgentName   string `json:"agent_name,omitempty"`   // Agent name (for PBAC subject matching)
}

// Result represents the outcome of a policy evaluation.
type Result struct {
	Allowed bool
	Reason  string
}

// PolicyDocument represents the JSON structure of an IAM policy document.
type PolicyDocument struct {
	Name    string          `json:"name"`
	Type    string          `json:"type"` // "rbac", "abac", "pbac"
	Subject *PolicySubject  `json:"subject,omitempty"`
	Rules   []PolicyRule    `json:"rules"`
}

// PolicySubject identifies who this policy applies to.
type PolicySubject struct {
	Type string `json:"type"` // "user", "agent", "team"
	Name string `json:"name,omitempty"`
	Team string `json:"team,omitempty"`
	Role string `json:"role,omitempty"`
}

// PolicyRule represents a single rule within a policy document.
type PolicyRule struct {
	Effect       string            `json:"effect"`       // "allow" or "deny"
	Path         string            `json:"path"`         // Resource path pattern
	Capabilities []string          `json:"capabilities"` // "read", "write", "delete", "list", "*"
	Conditions   []PolicyCondition `json:"conditions"`   // Conditions that must be met
}

// PolicyCondition represents a condition that must be satisfied.
type PolicyCondition struct {
	Attribute string `json:"attribute"` // "environment", "mfa", "ip_cidr", "time_after", "time_before"
	Operator  string `json:"operator"`  // "eq", "neq", "in", "not_in", "cidr_match"
	Value     string `json:"value"`     // Expected value
}

// Evaluate checks whether the request is allowed.
// It evaluates both legacy policies and IAM policies.
// Logic:
//   - Admin users always pass
//   - Evaluate legacy policies first (for backward compatibility)
//   - Then evaluate IAM policies (RBAC, ABAC, PBAC)
//   - If any "deny" matches, deny
//   - If any "allow" matches and no "deny" matches, allow
//   - Default: deny
func (e *Engine) Evaluate(ctx context.Context, req Request) (*Result, error) {
	// Admins bypass all policy checks
	if req.IsAdmin {
		return &Result{Allowed: true, Reason: "admin bypass"}, nil
	}

	// Phase 1: Evaluate legacy policies (backward compatibility)
	legacyResult, err := e.evaluateLegacy(ctx, req)
	if err != nil {
		return nil, err
	}

	// Phase 2: Evaluate IAM policies if org context is available
	if req.OrgID != "" {
		iamResult, err := e.evaluateIAM(ctx, req)
		if err != nil {
			return nil, err
		}

		// Deny from either system takes precedence
		if iamResult != nil && !iamResult.Allowed && strings.HasPrefix(iamResult.Reason, "denied") {
			return iamResult, nil
		}

		// If IAM explicitly allows, use that
		if iamResult != nil && iamResult.Allowed {
			return iamResult, nil
		}
	}

	// Fall back to legacy result
	if legacyResult != nil {
		return legacyResult, nil
	}

	return &Result{Allowed: false, Reason: "no matching allow policy (default deny)"}, nil
}

// evaluateLegacy evaluates legacy (v1) policies.
func (e *Engine) evaluateLegacy(ctx context.Context, req Request) (*Result, error) {
	policies, err := e.database.GetPoliciesForSubject(ctx, req.SubjectType, req.SubjectID)
	if err != nil {
		return nil, err
	}

	hasAllow := false

	for _, pol := range policies {
		if !matchAction(pol.Actions, req.Action) {
			continue
		}
		if !matchResource(pol.ResourcePattern, req.Resource) {
			continue
		}
		if pol.Effect == "deny" {
			return &Result{
				Allowed: false,
				Reason:  "denied by legacy policy: " + pol.Name,
			}, nil
		}
		if pol.Effect == "allow" {
			hasAllow = true
		}
	}

	if hasAllow {
		return &Result{Allowed: true, Reason: "allowed by legacy policy"}, nil
	}

	return &Result{Allowed: false, Reason: "no matching allow policy (default deny)"}, nil
}

// evaluateIAM evaluates IAM policies (RBAC, ABAC, PBAC) for the given org.
func (e *Engine) evaluateIAM(ctx context.Context, req Request) (*Result, error) {
	iamPolicies, err := e.database.ListIAMPolicies(ctx, req.OrgID)
	if err != nil {
		return nil, err
	}

	if len(iamPolicies) == 0 {
		return nil, nil // No IAM policies, defer to legacy
	}

	hasAllow := false
	var allowReason string

	for _, iamPol := range iamPolicies {
		var doc PolicyDocument
		if err := json.Unmarshal(iamPol.PolicyDoc, &doc); err != nil {
			continue // Skip malformed policies
		}

		switch iamPol.PolicyType {
		case "rbac":
			result := evaluateRBAC(doc, req)
			if result != nil {
				if !result.Allowed {
					return result, nil // Deny takes priority
				}
				hasAllow = true
				allowReason = result.Reason
			}
		case "abac":
			result := evaluateABAC(doc, req)
			if result != nil {
				if !result.Allowed {
					return result, nil
				}
				hasAllow = true
				allowReason = result.Reason
			}
		case "pbac":
			result := evaluatePBAC(doc, req)
			if result != nil {
				if !result.Allowed {
					return result, nil
				}
				hasAllow = true
				allowReason = result.Reason
			}
		}
	}

	if hasAllow {
		return &Result{Allowed: true, Reason: allowReason}, nil
	}

	return nil, nil // No matching IAM policies
}

// evaluateRBAC checks RBAC policies: role-based access against paths.
func evaluateRBAC(doc PolicyDocument, req Request) *Result {
	// Check if the subject matches
	if doc.Subject != nil {
		if !matchSubject(doc.Subject, req) {
			return nil // Policy doesn't apply to this subject
		}
	}

	for _, rule := range doc.Rules {
		if !matchAction(rule.Capabilities, req.Action) {
			continue
		}
		if !matchResource(rule.Path, req.Resource) {
			continue
		}

		// For RBAC, the main check is role matching (done via subject)
		if rule.Effect == "deny" {
			return &Result{
				Allowed: false,
				Reason:  "denied by RBAC policy: " + doc.Name,
			}
		}
		if rule.Effect == "allow" {
			return &Result{
				Allowed: true,
				Reason:  "allowed by RBAC policy: " + doc.Name,
			}
		}
	}

	return nil
}

// evaluateABAC checks ABAC policies: attribute-based conditions.
func evaluateABAC(doc PolicyDocument, req Request) *Result {
	if doc.Subject != nil {
		if !matchSubject(doc.Subject, req) {
			return nil
		}
	}

	for _, rule := range doc.Rules {
		if !matchAction(rule.Capabilities, req.Action) {
			continue
		}
		if !matchResource(rule.Path, req.Resource) {
			continue
		}

		// Check all conditions
		if !evaluateConditions(rule.Conditions, req) {
			continue
		}

		if rule.Effect == "deny" {
			return &Result{
				Allowed: false,
				Reason:  "denied by ABAC policy: " + doc.Name,
			}
		}
		if rule.Effect == "allow" {
			return &Result{
				Allowed: true,
				Reason:  "allowed by ABAC policy: " + doc.Name,
			}
		}
	}

	return nil
}

// evaluatePBAC checks PBAC policies: full policy document evaluation.
func evaluatePBAC(doc PolicyDocument, req Request) *Result {
	if doc.Subject != nil {
		if !matchSubject(doc.Subject, req) {
			return nil
		}
	}

	var denyResult *Result

	for _, rule := range doc.Rules {
		if !matchAction(rule.Capabilities, req.Action) {
			continue
		}
		if !matchResource(rule.Path, req.Resource) {
			continue
		}
		if !evaluateConditions(rule.Conditions, req) {
			continue
		}

		if rule.Effect == "deny" {
			denyResult = &Result{
				Allowed: false,
				Reason:  "denied by PBAC policy: " + doc.Name,
			}
		}
		if rule.Effect == "allow" && denyResult == nil {
			return &Result{
				Allowed: true,
				Reason:  "allowed by PBAC policy: " + doc.Name,
			}
		}
	}

	if denyResult != nil {
		return denyResult
	}

	return nil
}

// matchSubject checks if the request subject matches the policy's subject specification.
func matchSubject(subject *PolicySubject, req Request) bool {
	// Match subject type
	if subject.Type != "" && subject.Type != req.SubjectType {
		return false
	}

	if req.Attributes == nil {
		// If no attributes, can only match on type
		return subject.Name == "" && subject.Team == "" && subject.Role == ""
	}

	// Match agent name
	if subject.Name != "" && subject.Name != req.Attributes.AgentName {
		return false
	}

	// Match team
	if subject.Team != "" && subject.Team != req.Attributes.Team {
		return false
	}

	// Match role
	if subject.Role != "" && subject.Role != req.Attributes.Role {
		return false
	}

	return true
}

// evaluateConditions checks all conditions against request attributes.
func evaluateConditions(conditions []PolicyCondition, req Request) bool {
	if len(conditions) == 0 {
		return true // No conditions = always match
	}

	if req.Attributes == nil {
		return false // Conditions exist but no attributes to evaluate
	}

	for _, cond := range conditions {
		if !evaluateCondition(cond, req.Attributes) {
			return false // All conditions must match (AND logic)
		}
	}

	return true
}

// evaluateCondition checks a single condition against attributes.
func evaluateCondition(cond PolicyCondition, attrs *RequestAttributes) bool {
	var attrValue string

	switch cond.Attribute {
	case "environment":
		attrValue = attrs.Environment
	case "mfa":
		if attrs.MFA {
			attrValue = "true"
		} else {
			attrValue = "false"
		}
	case "ip_cidr":
		return matchCIDR(attrs.IP, cond.Value)
	case "team":
		attrValue = attrs.Team
	case "role":
		attrValue = attrs.Role
	default:
		return false // Unknown attribute
	}

	switch cond.Operator {
	case "eq", "":
		return attrValue == cond.Value
	case "neq":
		return attrValue != cond.Value
	case "in":
		values := strings.Split(cond.Value, ",")
		for _, v := range values {
			if strings.TrimSpace(v) == attrValue {
				return true
			}
		}
		return false
	case "not_in":
		values := strings.Split(cond.Value, ",")
		for _, v := range values {
			if strings.TrimSpace(v) == attrValue {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// matchCIDR checks if an IP address falls within a CIDR range.
func matchCIDR(ip, cidr string) bool {
	if ip == "" || cidr == "" {
		return false
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		// Try treating it as a single IP
		singleIP := net.ParseIP(cidr)
		if singleIP == nil {
			return false
		}
		return parsedIP.Equal(singleIP)
	}

	return ipNet.Contains(parsedIP)
}

// matchAction checks if the requested action matches any of the policy's actions/capabilities.
func matchAction(policyActions []string, requestedAction string) bool {
	for _, a := range policyActions {
		if a == "*" || a == requestedAction {
			return true
		}
	}
	return false
}

// matchResource checks if the requested resource matches the policy's resource pattern.
// Supports glob patterns like "myproject/*", "services/*/staging/*", etc.
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

	// Handle ** for deep paths
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

	// Handle multi-segment wildcards like "services/*/staging/*"
	// Split pattern and resource by "/" and match segment by segment
	patternParts := strings.Split(pattern, "/")
	resourceParts := strings.Split(resource, "/")

	return matchSegments(patternParts, resourceParts)
}

// matchSegments matches path segments supporting * wildcards in individual segments.
func matchSegments(pattern, resource []string) bool {
	if len(pattern) == 0 && len(resource) == 0 {
		return true
	}
	if len(pattern) == 0 || len(resource) == 0 {
		// Pattern ends with * — match remaining
		if len(pattern) == 1 && pattern[0] == "*" {
			return true
		}
		return false
	}

	if pattern[0] == "*" || pattern[0] == resource[0] {
		return matchSegments(pattern[1:], resource[1:])
	}

	return false
}
