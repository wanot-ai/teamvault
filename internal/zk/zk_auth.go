package zk

import (
	"fmt"
	"sync"
)

// ZKAuthority manages ZK credential issuance and verification.
// It holds the BBS+ key pair and issues credentials after successful authentication.
type ZKAuthority struct {
	mu sync.RWMutex
	pk PublicKey
	sk PrivateKey
}

// NewZKAuthority creates a new ZK authority with a fresh BBS+ key pair.
func NewZKAuthority() (*ZKAuthority, error) {
	pk, sk, err := GenerateKeyPair(10)
	if err != nil {
		return nil, fmt.Errorf("generating BBS+ key pair: %w", err)
	}
	return &ZKAuthority{pk: pk, sk: sk}, nil
}

// NewZKAuthorityWithKeys creates a ZK authority with existing keys.
func NewZKAuthorityWithKeys(pk PublicKey, sk PrivateKey) *ZKAuthority {
	return &ZKAuthority{pk: pk, sk: sk}
}

// PublicKey returns the authority's public key (safe to share).
func (za *ZKAuthority) PublicKey() PublicKey {
	za.mu.RLock()
	defer za.mu.RUnlock()
	return za.pk
}

// IssueZKCredential creates a BBS+ credential for an authenticated user.
// This should only be called after successful JWT authentication.
// The credential contains claims about the user's identity and access attributes.
func (za *ZKAuthority) IssueZKCredential(userID, role, team, mfa string) (Credential, error) {
	za.mu.RLock()
	defer za.mu.RUnlock()

	claims := []Claim{
		{Name: "user_id", Value: userID},
		{Name: "role", Value: role},
		{Name: "team", Value: team},
		{Name: "mfa", Value: mfa},
	}

	cred, err := SignCredential(za.sk, claims)
	if err != nil {
		return Credential{}, fmt.Errorf("signing credential: %w", err)
	}

	return cred, nil
}

// VerifyZKToken verifies a selective disclosure proof and checks that
// all required claims are present in the disclosed set.
func (za *ZKAuthority) VerifyZKToken(proof Proof, requiredClaims []string) bool {
	za.mu.RLock()
	defer za.mu.RUnlock()

	// First, verify the cryptographic proof
	if !VerifyProof(za.pk, proof, proof.DisclosedClaims) {
		return false
	}

	// Then, check that all required claims are disclosed
	disclosedNames := make(map[string]bool, len(proof.DisclosedClaims))
	for _, c := range proof.DisclosedClaims {
		disclosedNames[c.Name] = true
	}

	for _, required := range requiredClaims {
		if !disclosedNames[required] {
			return false
		}
	}

	return true
}

// GetDisclosedClaim extracts a specific claim value from a verified proof.
// Returns the value and true if found, nil and false otherwise.
func GetDisclosedClaim(proof Proof, name string) (interface{}, bool) {
	for _, c := range proof.DisclosedClaims {
		if c.Name == name {
			return c.Value, true
		}
	}
	return nil, false
}

// VerifyZKTokenWithConstraints verifies a proof and checks claim value constraints.
// constraints maps claim names to their required values (nil means any value is accepted).
func (za *ZKAuthority) VerifyZKTokenWithConstraints(proof Proof, constraints map[string]interface{}) bool {
	za.mu.RLock()
	defer za.mu.RUnlock()

	// Cryptographic verification
	if !VerifyProof(za.pk, proof, proof.DisclosedClaims) {
		return false
	}

	// Build lookup of disclosed claims
	disclosed := make(map[string]interface{}, len(proof.DisclosedClaims))
	for _, c := range proof.DisclosedClaims {
		disclosed[c.Name] = c.Value
	}

	// Check constraints
	for name, requiredValue := range constraints {
		actualValue, ok := disclosed[name]
		if !ok {
			return false // Claim not disclosed
		}
		if requiredValue != nil {
			if fmt.Sprintf("%v", actualValue) != fmt.Sprintf("%v", requiredValue) {
				return false // Value doesn't match
			}
		}
	}

	return true
}
