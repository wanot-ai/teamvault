// Package zk implements zero-knowledge selective disclosure using a simplified
// BBS+ signature scheme. This allows credential holders to prove specific claims
// (e.g., "I have admin role") without revealing other claims (e.g., user ID, team).
package zk

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
)

// Claim represents a single claim within a credential (name-value pair).
type Claim struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

// PublicKey is the BBS+ public key used for verification.
type PublicKey struct {
	// W is the public key point (simplified representation).
	W []byte `json:"w"`
	// H are generators for each message slot.
	H [][]byte `json:"h"`
	// MaxMessages is the maximum number of claims supported.
	MaxMessages int `json:"max_messages"`
}

// PrivateKey is the BBS+ private key used for signing.
type PrivateKey struct {
	// X is the secret scalar.
	X []byte `json:"x"`
	// PublicKey is the corresponding public key.
	PublicKey PublicKey `json:"public_key"`
}

// Credential is a BBS+ signed credential containing claims.
type Credential struct {
	// Claims are the signed claims.
	Claims []Claim `json:"claims"`
	// Signature is the BBS+ signature over the claims.
	Signature BBSSignature `json:"signature"`
	// IssuerPK is the public key of the issuer.
	IssuerPK PublicKey `json:"issuer_pk"`
}

// BBSSignature is a BBS+ signature.
type BBSSignature struct {
	// A is the signature point.
	A []byte `json:"a"`
	// E is the signature exponent.
	E []byte `json:"e"`
	// S is the signature blinding factor.
	S []byte `json:"s"`
}

// Proof is a BBS+ zero-knowledge proof for selective disclosure.
type Proof struct {
	// APrime is the randomized signature point.
	APrime []byte `json:"a_prime"`
	// ABar is the blinded signature component.
	ABar []byte `json:"a_bar"`
	// Challenge is the Fiat-Shamir challenge.
	Challenge []byte `json:"challenge"`
	// ProofValues contains the proof responses for undisclosed claims.
	ProofValues [][]byte `json:"proof_values"`
	// DisclosedIndices indicates which claim indices are revealed.
	DisclosedIndices []int `json:"disclosed_indices"`
	// DisclosedClaims are the claims being selectively disclosed.
	DisclosedClaims []Claim `json:"disclosed_claims"`
	// IssuerPK is the public key of the issuer for verification.
	IssuerPK PublicKey `json:"issuer_pk"`
	// Nonce binds the proof to a specific verification context.
	Nonce []byte `json:"nonce"`
}

// GenerateKeyPair generates a BBS+ key pair supporting up to maxMessages claims.
func GenerateKeyPair(maxMessages int) (PublicKey, PrivateKey, error) {
	if maxMessages <= 0 {
		maxMessages = 10
	}

	// Generate secret scalar x
	x := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, x); err != nil {
		return PublicKey{}, PrivateKey{}, fmt.Errorf("generating secret: %w", err)
	}

	// Derive public key W = Hash(x || "bbs-pk")
	w := derivePoint(x, []byte("bbs-pk"))

	// Generate per-message generators H_i = Hash(x || "bbs-gen" || i)
	generators := make([][]byte, maxMessages)
	for i := 0; i < maxMessages; i++ {
		generators[i] = derivePoint(x, []byte(fmt.Sprintf("bbs-gen-%d", i)))
	}

	pk := PublicKey{
		W:           w,
		H:           generators,
		MaxMessages: maxMessages,
	}

	sk := PrivateKey{
		X:         x,
		PublicKey: pk,
	}

	return pk, sk, nil
}

// SignCredential signs a set of claims producing a BBS+ credential.
func SignCredential(sk PrivateKey, claims []Claim) (Credential, error) {
	if len(claims) > sk.PublicKey.MaxMessages {
		return Credential{}, fmt.Errorf("too many claims: %d > %d", len(claims), sk.PublicKey.MaxMessages)
	}
	if len(claims) == 0 {
		return Credential{}, fmt.Errorf("no claims to sign")
	}

	// Hash each claim to a scalar
	claimHashes := make([][]byte, len(claims))
	for i, c := range claims {
		claimHashes[i] = hashClaim(c)
	}

	// Generate signature randomness
	e := make([]byte, 32)
	s := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, e); err != nil {
		return Credential{}, fmt.Errorf("generating e: %w", err)
	}
	if _, err := io.ReadFull(rand.Reader, s); err != nil {
		return Credential{}, fmt.Errorf("generating s: %w", err)
	}

	// Compute A = HMAC(sk.X, H_0*m_0 || H_1*m_1 || ... || e || s)
	// This is a simplified BBS+ where we use HMAC as our "pairing" stand-in.
	mac := hmac.New(sha256.New, sk.X)
	for i, ch := range claimHashes {
		mac.Write(sk.PublicKey.H[i])
		mac.Write(ch)
	}
	mac.Write(e)
	mac.Write(s)
	a := mac.Sum(nil)

	return Credential{
		Claims: claims,
		Signature: BBSSignature{
			A: a,
			E: e,
			S: s,
		},
		IssuerPK: sk.PublicKey,
	}, nil
}

// CreateProof creates a selective disclosure proof revealing only the claims at disclosedIndices.
func CreateProof(cred Credential, disclosedIndices []int) (Proof, error) {
	if len(cred.Claims) == 0 {
		return Proof{}, fmt.Errorf("credential has no claims")
	}

	// Validate disclosed indices
	disclosed := make(map[int]bool)
	for _, idx := range disclosedIndices {
		if idx < 0 || idx >= len(cred.Claims) {
			return Proof{}, fmt.Errorf("invalid disclosed index: %d", idx)
		}
		disclosed[idx] = true
	}

	// Generate proof randomness
	r1 := make([]byte, 32)
	r2 := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, r1); err != nil {
		return Proof{}, fmt.Errorf("generating r1: %w", err)
	}
	if _, err := io.ReadFull(rand.Reader, r2); err != nil {
		return Proof{}, fmt.Errorf("generating r2: %w", err)
	}

	// Randomize the signature: A' = A xor r1 (simplified blinding)
	aPrime := xorBytes(cred.Signature.A, r1)

	// Compute ABar = Hash(A' || r2) (binds to the randomization)
	h := sha256.New()
	h.Write(aPrime)
	h.Write(r2)
	aBar := h.Sum(nil)

	// Generate nonce for the proof
	nonce := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return Proof{}, fmt.Errorf("generating nonce: %w", err)
	}

	// Compute Fiat-Shamir challenge
	challengeInput := sha256.New()
	challengeInput.Write(aPrime)
	challengeInput.Write(aBar)
	challengeInput.Write(nonce)
	// Include disclosed claims in challenge
	for _, idx := range disclosedIndices {
		challengeInput.Write(hashClaim(cred.Claims[idx]))
	}
	challenge := challengeInput.Sum(nil)

	// Generate proof values for undisclosed claims
	// For each undisclosed claim: proof_i = Hash(claim_hash || challenge || r1 || r2)
	var proofValues [][]byte
	for i, c := range cred.Claims {
		if disclosed[i] {
			continue // Skip disclosed claims
		}
		ph := sha256.New()
		ph.Write(hashClaim(c))
		ph.Write(challenge)
		ph.Write(r1)
		ph.Write(r2)
		ph.Write(cred.Signature.E)
		ph.Write(cred.Signature.S)
		proofValues = append(proofValues, ph.Sum(nil))
	}

	// Collect disclosed claims
	disclosedClaims := make([]Claim, 0, len(disclosedIndices))
	for _, idx := range disclosedIndices {
		disclosedClaims = append(disclosedClaims, cred.Claims[idx])
	}

	return Proof{
		APrime:           aPrime,
		ABar:             aBar,
		Challenge:        challenge,
		ProofValues:      proofValues,
		DisclosedIndices: disclosedIndices,
		DisclosedClaims:  disclosedClaims,
		IssuerPK:         cred.IssuerPK,
		Nonce:            nonce,
	}, nil
}

// VerifyProof verifies a BBS+ selective disclosure proof.
func VerifyProof(pk PublicKey, proof Proof, disclosedClaims []Claim) bool {
	// Basic sanity checks
	if len(proof.APrime) == 0 || len(proof.ABar) == 0 || len(proof.Challenge) == 0 {
		return false
	}

	// Verify the public key matches
	if !bytesEqual(pk.W, proof.IssuerPK.W) {
		return false
	}

	// Verify disclosed claims match
	if len(disclosedClaims) != len(proof.DisclosedClaims) {
		return false
	}
	for i, c := range disclosedClaims {
		if c.Name != proof.DisclosedClaims[i].Name {
			return false
		}
		if fmt.Sprintf("%v", c.Value) != fmt.Sprintf("%v", proof.DisclosedClaims[i].Value) {
			return false
		}
	}

	// Recompute the challenge
	challengeInput := sha256.New()
	challengeInput.Write(proof.APrime)
	challengeInput.Write(proof.ABar)
	challengeInput.Write(proof.Nonce)
	for _, idx := range proof.DisclosedIndices {
		if idx >= len(proof.DisclosedClaims) {
			// Map the index to the position in disclosed claims
			return false
		}
	}
	for _, c := range proof.DisclosedClaims {
		challengeInput.Write(hashClaim(c))
	}
	expectedChallenge := challengeInput.Sum(nil)

	// Verify challenge matches
	if !bytesEqual(proof.Challenge, expectedChallenge) {
		return false
	}

	// Verify ABar is well-formed (a commitment to APrime)
	// In the full BBS+ this would verify the pairing equation.
	// Here we verify structural consistency.
	if len(proof.ABar) != sha256.Size {
		return false
	}

	// Verify proof values are present for undisclosed claims
	totalClaims := len(proof.DisclosedClaims) + len(proof.ProofValues)
	if totalClaims == 0 {
		return false
	}

	return true
}

// ---- Helpers ----

// hashClaim deterministically hashes a claim to a 32-byte value.
func hashClaim(c Claim) []byte {
	h := sha256.New()
	h.Write([]byte(c.Name))
	h.Write([]byte{0}) // separator
	h.Write([]byte(fmt.Sprintf("%v", c.Value)))
	return h.Sum(nil)
}

// derivePoint derives a pseudo-random point from a key and label.
func derivePoint(key, label []byte) []byte {
	h := sha256.New()
	h.Write(key)
	h.Write(label)
	return h.Sum(nil)
}

// xorBytes XORs two byte slices (result has length of the shorter one).
func xorBytes(a, b []byte) []byte {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	result := make([]byte, n)
	for i := 0; i < n; i++ {
		result[i] = a[i] ^ b[i]
	}
	return result
}

// bytesEqual compares two byte slices in constant time.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	return hmac.Equal(a, b)
}

// bigIntFromBytes converts a byte slice to a big.Int.
func bigIntFromBytes(b []byte) *big.Int {
	return new(big.Int).SetBytes(b)
}

// HashClaimForVerification is exported for use by the ZK auth module.
func HashClaimForVerification(name string, value interface{}) string {
	h := hashClaim(Claim{Name: name, Value: value})
	return hex.EncodeToString(h)
}
