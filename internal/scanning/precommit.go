package scanning

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
)

// PreCommitConfig is the template for .pre-commit-config.yaml.
const PreCommitConfig = `# TeamVault Secret Scanner — Pre-commit configuration
# See: https://pre-commit.com
repos:
  - repo: local
    hooks:
      - id: teamvault-scan
        name: TeamVault Secret Scanner
        entry: teamvault scan --path .
        language: system
        pass_filenames: false
        always_run: true
        stages: [pre-commit]
        description: |
          Scans staged files for accidentally committed secrets,
          API keys, tokens, and other sensitive data.
`

// GeneratePreCommitConfig writes a .pre-commit-config.yaml file.
func GeneratePreCommitConfig(dir string) error {
	path := filepath.Join(dir, ".pre-commit-config.yaml")

	// Don't overwrite existing config
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("pre-commit config already exists at %s", path)
	}

	if err := os.WriteFile(path, []byte(PreCommitConfig), 0644); err != nil {
		return fmt.Errorf("writing pre-commit config: %w", err)
	}

	return nil
}

// ScanResult holds the results of scanning a path.
type ScanResult struct {
	File     string    `json:"file"`
	Findings []Finding `json:"findings"`
}

// ScanOptions configures directory scanning behavior.
type ScanOptions struct {
	ExcludeDirs []string // Additional directory names to exclude
}

// defaultExcludeDirs are always skipped.
var defaultExcludeDirs = map[string]bool{
	".git":          true,
	".hg":           true,
	".svn":          true,
	"node_modules":  true,
	"vendor":        true,
	"__pycache__":   true,
	".venv":         true,
	".next":         true,
	".idea":         true,
	".vscode":       true,
	"dist":          true,
	"build":         true,
	".terraform":    true,
}

// shouldExcludeDir returns true if the directory name should be skipped.
func shouldExcludeDir(name string, extra []string) bool {
	if strings.HasPrefix(name, ".") && name != "." && name != ".." {
		// Skip all hidden dirs except .env files (handled at file level)
		if defaultExcludeDirs[name] {
			return true
		}
	}
	if defaultExcludeDirs[name] {
		return true
	}
	for _, e := range extra {
		if name == e {
			return true
		}
	}
	return false
}

// ScanPath scans files in a directory for secrets using default options.
func ScanPath(root string) ([]ScanResult, error) {
	return ScanPathWithOptions(root, ScanOptions{})
}

// ScanPathWithOptions scans files in a directory for secrets with custom options.
func ScanPathWithOptions(root string, opts ScanOptions) ([]ScanResult, error) {
	scanner := NewScanner()
	var results []ScanResult

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths
		}

		// Skip excluded directories
		if d.IsDir() {
			if shouldExcludeDir(d.Name(), opts.ExcludeDirs) {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip binary and non-text files
		if shouldSkipFile(path) {
			return nil
		}

		// Skip symlinks
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		// Read file — skip large files (>10MB)
		if info.Size() > 10*1024*1024 {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip unreadable files
		}

		findings := scanner.ScanText(string(content))
		if len(findings) > 0 {
			relPath, _ := filepath.Rel(root, path)
			if relPath == "" {
				relPath = path
			}
			results = append(results, ScanResult{
				File:     relPath,
				Findings: findings,
			})
		}

		return nil
	})

	return results, err
}

// ScanFiles scans specific files for secrets.
func ScanFiles(files []string) ([]ScanResult, error) {
	return ScanFilesWithOptions(files, ScanOptions{})
}

// ScanFilesWithOptions scans specific files for secrets with custom options.
func ScanFilesWithOptions(files []string, opts ScanOptions) ([]ScanResult, error) {
	scanner := NewScanner()
	var results []ScanResult

	for _, file := range files {
		if shouldSkipFile(file) {
			continue
		}

		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		findings := scanner.ScanText(string(content))
		if len(findings) > 0 {
			results = append(results, ScanResult{
				File:     file,
				Findings: findings,
			})
		}
	}

	return results, nil
}

// FormatResults formats scan results for CLI table output.
func FormatResults(results []ScanResult) string {
	if len(results) == 0 {
		return "✓ No secrets detected.\n"
	}

	totalFindings := 0
	for _, r := range results {
		totalFindings += len(r.Findings)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "⚠ Found %d potential secret(s) in %d file(s):\n\n", totalFindings, len(results))

	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FILE\tLINE\tPATTERN\tMATCH\tCONFIDENCE")
	fmt.Fprintln(w, "----\t----\t-------\t-----\t----------")
	for _, r := range results {
		for _, f := range r.Findings {
			confidence := fmt.Sprintf("%.0f%%", f.Confidence*100)
			fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\n", r.File, f.Line, f.PatternName, f.Match, confidence)
		}
	}
	w.Flush()
	b.WriteString("\n")

	return b.String()
}

// shouldSkipFile returns true if the file should be skipped based on extension.
func shouldSkipFile(path string) bool {
	skipExtensions := map[string]bool{
		".exe": true, ".bin": true, ".dll": true, ".so": true, ".dylib": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".ico": true,
		".svg": true, ".webp": true, ".bmp": true, ".tiff": true,
		".woff": true, ".woff2": true, ".ttf": true, ".eot": true, ".otf": true,
		".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".xz": true, ".7z": true, ".rar": true,
		".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
		".mp3": true, ".mp4": true, ".avi": true, ".mov": true, ".mkv": true, ".wav": true,
		".o": true, ".a": true, ".pyc": true, ".class": true,
		".wasm": true, ".sqlite": true, ".db": true,
	}

	ext := strings.ToLower(filepath.Ext(path))
	return skipExtensions[ext]
}
