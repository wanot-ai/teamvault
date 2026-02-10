// Package scanning implements secret detection for finding leaked credentials
// in text, files, and git repositories. It uses pattern matching and entropy
// analysis to identify common secret formats.
package scanning

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

// Finding represents a detected secret in scanned text.
type Finding struct {
	PatternName string  `json:"pattern_name"`
	Match       string  `json:"match"`
	Line        int     `json:"line"`
	Confidence  float64 `json:"confidence"` // 0.0 to 1.0
}

// Pattern defines a secret detection pattern.
type Pattern struct {
	Name       string
	Regex      *regexp.Regexp
	Confidence float64
}

// defaultPatterns are the built-in secret detection patterns.
var defaultPatterns = []Pattern{
	// ─── AWS ─────────────────────────────────────────────────────────
	{
		Name:       "AWS Access Key ID",
		Regex:      regexp.MustCompile(`(?:^|[^A-Za-z0-9/+=])(AKIA[0-9A-Z]{16})(?:[^A-Za-z0-9/+=]|$)`),
		Confidence: 0.95,
	},
	{
		Name:       "AWS Secret Access Key",
		Regex:      regexp.MustCompile(`(?i)(?:aws[_\-]?secret[_\-]?(?:access[_\-]?)?key)\s*[=:]\s*['"]?([A-Za-z0-9/+=]{40})['"]?`),
		Confidence: 0.90,
	},

	// ─── GitHub ──────────────────────────────────────────────────────
	{
		Name:       "GitHub Personal Access Token",
		Regex:      regexp.MustCompile(`(?:^|[^A-Za-z0-9_])ghp_[A-Za-z0-9_]{36,255}(?:[^A-Za-z0-9_]|$)`),
		Confidence: 0.95,
	},
	{
		Name:       "GitHub OAuth Token",
		Regex:      regexp.MustCompile(`(?:^|[^A-Za-z0-9_])gho_[A-Za-z0-9_]{36,255}(?:[^A-Za-z0-9_]|$)`),
		Confidence: 0.95,
	},
	{
		Name:       "GitHub App Token",
		Regex:      regexp.MustCompile(`(?:^|[^A-Za-z0-9_])(?:ghu|ghs|ghr)_[A-Za-z0-9_]{36,255}(?:[^A-Za-z0-9_]|$)`),
		Confidence: 0.95,
	},
	{
		Name:       "GitHub Fine-Grained PAT",
		Regex:      regexp.MustCompile(`(?:^|[^A-Za-z0-9_])github_pat_[A-Za-z0-9_]{22,255}(?:[^A-Za-z0-9_]|$)`),
		Confidence: 0.95,
	},

	// ─── Slack ───────────────────────────────────────────────────────
	{
		Name:       "Slack Bot Token",
		Regex:      regexp.MustCompile(`(?:^|[^A-Za-z0-9\-])xoxb-[0-9]{10,13}-[0-9]{10,13}-[a-zA-Z0-9]{24,34}(?:[^A-Za-z0-9\-]|$)`),
		Confidence: 0.95,
	},
	{
		Name:       "Slack User Token",
		Regex:      regexp.MustCompile(`(?:^|[^A-Za-z0-9\-])xoxp-[0-9]{10,13}-[0-9]{10,13}-[0-9]{10,13}-[a-f0-9]{32}(?:[^A-Za-z0-9\-]|$)`),
		Confidence: 0.95,
	},
	{
		Name:       "Slack App-Level Token",
		Regex:      regexp.MustCompile(`(?:^|[^A-Za-z0-9\-])xoxs-[0-9A-Za-z\-\.]{50,}(?:[^A-Za-z0-9\-]|$)`),
		Confidence: 0.85,
	},
	{
		Name:       "Slack Webhook URL",
		Regex:      regexp.MustCompile(`https://hooks\.slack\.com/services/T[A-Z0-9]{8,}/B[A-Z0-9]{8,}/[a-zA-Z0-9]{24}`),
		Confidence: 0.90,
	},

	// ─── Google ──────────────────────────────────────────────────────
	{
		Name:       "Google API Key",
		Regex:      regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`),
		Confidence: 0.85,
	},

	// ─── Stripe ──────────────────────────────────────────────────────
	{
		Name:       "Stripe Secret Key",
		Regex:      regexp.MustCompile(`sk_live_[0-9a-zA-Z]{24,}`),
		Confidence: 0.95,
	},
	{
		Name:       "Stripe Publishable Key",
		Regex:      regexp.MustCompile(`pk_live_[0-9a-zA-Z]{24,}`),
		Confidence: 0.80,
	},

	// ─── Private Keys ───────────────────────────────────────────────
	{
		Name:       "Private Key (PEM)",
		Regex:      regexp.MustCompile(`-----BEGIN\s+(?:RSA\s+|EC\s+|OPENSSH\s+|DSA\s+|PGP\s+)?PRIVATE\s+KEY-----`),
		Confidence: 0.99,
	},

	// ─── JWT ─────────────────────────────────────────────────────────
	{
		Name:       "JWT Token",
		Regex:      regexp.MustCompile(`(?:^|[^A-Za-z0-9_\-.])eyJ[A-Za-z0-9_-]{20,}\.eyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_\-]{20,}(?:[^A-Za-z0-9_\-.]|$)`),
		Confidence: 0.85,
	},

	// ─── Database URLs ──────────────────────────────────────────────
	{
		Name:       "Database URL (PostgreSQL)",
		Regex:      regexp.MustCompile(`postgres(?:ql)?://[^\s'"]{10,}`),
		Confidence: 0.90,
	},
	{
		Name:       "Database URL (MySQL)",
		Regex:      regexp.MustCompile(`mysql://[^\s'"]{10,}`),
		Confidence: 0.90,
	},
	{
		Name:       "Database URL (MongoDB)",
		Regex:      regexp.MustCompile(`mongodb(?:\+srv)?://[^\s'"]{10,}`),
		Confidence: 0.90,
	},
	{
		Name:       "Database URL (Redis)",
		Regex:      regexp.MustCompile(`redis(?:s)?://[^\s'"]*:[^\s'"]+@[^\s'"]+`),
		Confidence: 0.90,
	},

	// ─── Heroku ──────────────────────────────────────────────────────
	{
		Name:       "Heroku API Key",
		Regex:      regexp.MustCompile(`(?i)heroku.*[=:]\s*[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
		Confidence: 0.85,
	},

	// ─── Generic Patterns ───────────────────────────────────────────
	{
		Name:       "Generic API Key Assignment",
		Regex:      regexp.MustCompile(`(?i)(?:api_key|apikey|api_secret|api_token)\s*[=:]\s*['"]?([a-zA-Z0-9_\-]{20,})['"]?`),
		Confidence: 0.70,
	},
	{
		Name:       "Generic Secret Assignment",
		Regex:      regexp.MustCompile(`(?i)(?:secret|password|passwd|token|credential)\s*[=:]\s*['"]([^'"]{8,})['"]`),
		Confidence: 0.60,
	},
}

// Scanner detects secrets in text using pattern matching and entropy analysis.
type Scanner struct {
	patterns         []Pattern
	entropyThreshold float64
	minEntropyLength int
}

// NewScanner creates a scanner with default patterns.
func NewScanner() *Scanner {
	return &Scanner{
		patterns:         defaultPatterns,
		entropyThreshold: 4.5, // Shannon entropy threshold for "high entropy"
		minEntropyLength: 20,  // Minimum string length for entropy checking
	}
}

// NewScannerWithPatterns creates a scanner with custom patterns in addition to defaults.
func NewScannerWithPatterns(extra []Pattern) *Scanner {
	s := NewScanner()
	s.patterns = append(s.patterns, extra...)
	return s
}

// ScanText scans text for secret patterns and returns all findings.
func (s *Scanner) ScanText(text string) []Finding {
	var findings []Finding
	lines := strings.Split(text, "\n")

	for lineNum, line := range lines {
		// Pattern-based detection
		for _, p := range s.patterns {
			matches := p.Regex.FindAllString(line, -1)
			for _, match := range matches {
				findings = append(findings, Finding{
					PatternName: p.Name,
					Match:       redactMatch(strings.TrimSpace(match)),
					Line:        lineNum + 1,
					Confidence:  p.Confidence,
				})
			}
		}

		// Entropy-based detection for unquoted high-entropy strings
		entropyFindings := s.scanLineEntropy(line, lineNum+1)
		findings = append(findings, entropyFindings...)
	}

	// Deduplicate findings on same line with same pattern
	findings = deduplicateFindings(findings)

	return findings
}

// ScanTextSummary scans text and returns a summary string.
func (s *Scanner) ScanTextSummary(text string) string {
	findings := s.ScanText(text)
	if len(findings) == 0 {
		return "No secrets detected."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Found %d potential secret(s):\n", len(findings))
	for _, f := range findings {
		fmt.Fprintf(&b, "  Line %d: %s (confidence: %.0f%%) — %s\n",
			f.Line, f.PatternName, f.Confidence*100, f.Match)
	}
	return b.String()
}

// scanLineEntropy checks for high-entropy strings that might be secrets.
func (s *Scanner) scanLineEntropy(line string, lineNum int) []Finding {
	var findings []Finding

	// Look for assignment patterns with high-entropy values
	assignmentRe := regexp.MustCompile(`(?i)(?:key|token|secret|password|credential|auth)\s*[=:]\s*['"]?([a-zA-Z0-9+/=_\-]{20,})['"]?`)
	matches := assignmentRe.FindAllStringSubmatch(line, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		value := match[1]
		if len(value) < s.minEntropyLength {
			continue
		}

		entropy := shannonEntropy(value)
		if entropy >= s.entropyThreshold {
			findings = append(findings, Finding{
				PatternName: "High-Entropy String",
				Match:       redactMatch(value),
				Line:        lineNum,
				Confidence:  normalizeEntropy(entropy),
			})
		}
	}

	return findings
}

// shannonEntropy calculates the Shannon entropy of a string.
func shannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}

	freq := make(map[rune]float64)
	for _, c := range s {
		freq[c]++
	}

	length := float64(len(s))
	entropy := 0.0
	for _, count := range freq {
		p := count / length
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}
	return entropy
}

// normalizeEntropy converts Shannon entropy to a 0-1 confidence score.
func normalizeEntropy(entropy float64) float64 {
	// Entropy range for secrets is typically 4.5-6.0
	// Map to 0.5-0.85 confidence
	if entropy < 4.5 {
		return 0.3
	}
	if entropy > 6.0 {
		return 0.85
	}
	return 0.5 + (entropy-4.5)/1.5*0.35
}

// redactMatch partially redacts a match for safe display.
func redactMatch(s string) string {
	if len(s) <= 8 {
		return s[:2] + "***"
	}
	if len(s) <= 16 {
		return s[:3] + strings.Repeat("*", len(s)-6) + s[len(s)-3:]
	}
	// Show first 5 and last 5 characters
	return s[:5] + "..." + s[len(s)-5:]
}

// deduplicateFindings removes duplicate findings on the same line.
func deduplicateFindings(findings []Finding) []Finding {
	type key struct {
		line    int
		pattern string
	}
	seen := make(map[key]bool)
	var result []Finding
	for _, f := range findings {
		k := key{line: f.Line, pattern: f.PatternName}
		if !seen[k] {
			seen[k] = true
			result = append(result, f)
		}
	}
	return result
}
