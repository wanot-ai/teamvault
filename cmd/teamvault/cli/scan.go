package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/teamvault/teamvault/internal/scanning"
)

var (
	scanPath      string
	scanFormat    string
	scanPreCommit bool
	scanExclude   []string
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan files for leaked secrets",
	Long: `Scan directories for accidentally committed secrets, API keys, tokens,
and other sensitive data.

Detects:
  • AWS Access Keys and Secret Keys
  • GitHub tokens (ghp_, gho_, ghs_, github_pat_)
  • Slack tokens (xoxb-, xoxp-, xoxs-)
  • Private keys (RSA, EC, OpenSSH)
  • JWT tokens
  • Database connection URLs
  • Generic high-entropy API keys

Examples:
  teamvault scan --path .
  teamvault scan --path ./src --format json
  teamvault scan --path . --pre-commit
  teamvault scan --path . --exclude vendor --exclude node_modules`,
	RunE: runScan,
}

func init() {
	scanCmd.Flags().StringVar(&scanPath, "path", ".", "Directory or file to scan")
	scanCmd.Flags().StringVar(&scanFormat, "format", "table", "Output format: table, json")
	scanCmd.Flags().BoolVar(&scanPreCommit, "pre-commit", false, "Generate .pre-commit-config.yaml")
	scanCmd.Flags().StringSliceVar(&scanExclude, "exclude", nil, "Directory names to exclude (repeatable)")
}

func runScan(cmd *cobra.Command, args []string) error {
	// Handle --pre-commit flag
	if scanPreCommit {
		dir := scanPath
		if dir == "" {
			dir = "."
		}
		if err := scanning.GeneratePreCommitConfig(dir); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "✓ Generated .pre-commit-config.yaml in %s\n", dir)
		fmt.Fprintf(os.Stderr, "  Run: pre-commit install\n")
		return nil
	}

	// Handle paths from args as well
	paths := args
	if scanPath != "" && scanPath != "." {
		paths = append([]string{scanPath}, paths...)
	}

	var results []scanning.ScanResult
	var err error

	opts := scanning.ScanOptions{
		ExcludeDirs: scanExclude,
	}

	// If specific files are provided, scan those
	if len(paths) > 0 && paths[0] != "." {
		results, err = scanning.ScanFilesWithOptions(paths, opts)
	} else {
		results, err = scanning.ScanPathWithOptions(scanPath, opts)
	}
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Output results based on format
	switch strings.ToLower(scanFormat) {
	case "json":
		return outputScanJSON(results)
	default:
		return outputScanTable(results)
	}
}

func outputScanJSON(results []scanning.ScanResult) error {
	// Flatten into findings-per-file format
	type jsonFinding struct {
		File       string  `json:"file"`
		Line       int     `json:"line"`
		Pattern    string  `json:"pattern"`
		Match      string  `json:"match"`
		Confidence float64 `json:"confidence"`
	}

	totalFindings := 0
	var findings []jsonFinding
	for _, r := range results {
		for _, f := range r.Findings {
			findings = append(findings, jsonFinding{
				File:       r.File,
				Line:       f.Line,
				Pattern:    f.PatternName,
				Match:      f.Match,
				Confidence: f.Confidence,
			})
			totalFindings++
		}
	}

	output := struct {
		Total    int           `json:"total"`
		Files    int           `json:"files"`
		Findings []jsonFinding `json:"findings"`
	}{
		Total:    totalFindings,
		Files:    len(results),
		Findings: findings,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(output); err != nil {
		return err
	}

	if totalFindings > 0 {
		os.Exit(1)
	}
	return nil
}

func outputScanTable(results []scanning.ScanResult) error {
	output := scanning.FormatResults(results)
	fmt.Print(output)

	totalFindings := 0
	for _, r := range results {
		totalFindings += len(r.Findings)
	}
	if totalFindings > 0 {
		os.Exit(1)
	}
	return nil
}
