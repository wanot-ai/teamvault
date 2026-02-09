package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var kvCmd = &cobra.Command{
	Use:   "kv",
	Short: "Manage key-value secrets",
	Long: `Manage key-value secrets in TeamVault projects.

Examples:
  teamvault kv get myproject/api-keys/stripe
  teamvault kv put myproject/api-keys/stripe --value sk_live_xxx
  teamvault kv list myproject`,
}

var kvGetCmd = &cobra.Command{
	Use:   "get PROJECT/PATH",
	Short: "Fetch and print a secret value",
	Long: `Fetch a secret from TeamVault and print its value to stdout.

The argument should be in the format PROJECT/PATH where PATH can contain
multiple segments separated by slashes.

Examples:
  teamvault kv get myproject/api-keys/stripe
  teamvault kv get myproject/db/postgres-url`,
	Args: cobra.ExactArgs(1),
	RunE: runKVGet,
}

var (
	kvPutValue string
)

var kvPutCmd = &cobra.Command{
	Use:   "put PROJECT/PATH --value VALUE",
	Short: "Create or update a secret",
	Long: `Create or update a secret in TeamVault. If the secret already exists,
a new version is created.

Examples:
  teamvault kv put myproject/api-keys/stripe --value sk_live_xxx
  teamvault kv put myproject/db/postgres-url --value "postgres://user:pass@host/db"`,
	Args: cobra.ExactArgs(1),
	RunE: runKVPut,
}

var kvListCmd = &cobra.Command{
	Use:   "list PROJECT",
	Short: "List secrets in a project",
	Long: `List all secrets in a TeamVault project. Only paths are shown,
not secret values.

Examples:
  teamvault kv list myproject`,
	Args: cobra.ExactArgs(1),
	RunE: runKVList,
}

var kvTreeCmd = &cobra.Command{
	Use:   "tree PROJECT",
	Short: "Display secrets as a folder tree",
	Long: `Fetch all secrets in a project and display their paths as a folder tree.

Examples:
  teamvault kv tree myproject`,
	Args: cobra.ExactArgs(1),
	RunE: runKVTree,
}

func init() {
	kvPutCmd.Flags().StringVar(&kvPutValue, "value", "", "Secret value to store")
	kvPutCmd.MarkFlagRequired("value")

	kvCmd.AddCommand(kvGetCmd)
	kvCmd.AddCommand(kvPutCmd)
	kvCmd.AddCommand(kvListCmd)
	kvCmd.AddCommand(kvTreeCmd)
}

// parseProjectPath splits "project/path/to/secret" into project and path.
func parseProjectPath(arg string) (string, string, error) {
	idx := strings.Index(arg, "/")
	if idx < 0 {
		return "", "", fmt.Errorf("invalid format: expected PROJECT/PATH (e.g. myproject/api-keys/stripe)")
	}

	project := arg[:idx]
	path := arg[idx+1:]

	if project == "" {
		return "", "", fmt.Errorf("project name cannot be empty")
	}
	if path == "" {
		return "", "", fmt.Errorf("secret path cannot be empty")
	}

	return project, path, nil
}

func runKVGet(cmd *cobra.Command, args []string) error {
	project, path, err := parseProjectPath(args[0])
	if err != nil {
		return err
	}

	client, err := NewClient()
	if err != nil {
		return err
	}

	secret, err := client.GetSecret(project, path)
	if err != nil {
		return fmt.Errorf("failed to get secret %s/%s: %w", project, path, err)
	}

	// Print only the value to stdout (no trailing newline for piping)
	fmt.Print(secret.Value)
	return nil
}

func runKVPut(cmd *cobra.Command, args []string) error {
	project, path, err := parseProjectPath(args[0])
	if err != nil {
		return err
	}

	if kvPutValue == "" {
		return fmt.Errorf("--value cannot be empty")
	}

	client, err := NewClient()
	if err != nil {
		return err
	}

	if err := client.PutSecret(project, path, kvPutValue); err != nil {
		return fmt.Errorf("failed to put secret %s/%s: %w", project, path, err)
	}

	fmt.Fprintf(os.Stderr, "✓ Secret %s/%s saved\n", project, path)
	return nil
}

func runKVList(cmd *cobra.Command, args []string) error {
	project := args[0]
	if project == "" {
		return fmt.Errorf("project name cannot be empty")
	}

	client, err := NewClient()
	if err != nil {
		return err
	}

	secrets, err := client.ListSecrets(project)
	if err != nil {
		return fmt.Errorf("failed to list secrets in %s: %w", project, err)
	}

	if len(secrets) == 0 {
		fmt.Fprintf(os.Stderr, "No secrets found in project %q\n", project)
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PATH\tVERSION\tCREATED")
	for _, s := range secrets {
		created := s.CreatedAt
		if len(created) > 19 {
			created = created[:19] // Trim to readable datetime
		}
		fmt.Fprintf(w, "%s\t%d\t%s\n", s.Path, s.Version, created)
	}
	w.Flush()

	return nil
}

// --- kv tree ---

// treeNode represents a node in the folder tree.
type treeNode struct {
	name     string
	children map[string]*treeNode
	isLeaf   bool
}

func newTreeNode(name string) *treeNode {
	return &treeNode{
		name:     name,
		children: make(map[string]*treeNode),
	}
}

// insert adds a path (split by /) into the tree.
func (n *treeNode) insert(parts []string) {
	if len(parts) == 0 {
		return
	}

	child, exists := n.children[parts[0]]
	if !exists {
		child = newTreeNode(parts[0])
		n.children[parts[0]] = child
	}

	if len(parts) == 1 {
		// Leaf node (secret key)
		child.isLeaf = true
	} else {
		child.insert(parts[1:])
	}
}

// sortedChildNames returns child names in sorted order.
func (n *treeNode) sortedChildNames() []string {
	names := make([]string, 0, len(n.children))
	for name := range n.children {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// printTree renders the tree with box-drawing characters.
func printTree(node *treeNode, prefix string, isLast bool, isRoot bool) {
	if !isRoot {
		connector := "├── "
		if isLast {
			connector = "└── "
		}
		displayName := node.name
		if !node.isLeaf || len(node.children) > 0 {
			// It's a directory (has children or is not a leaf)
			if len(node.children) > 0 {
				displayName += "/"
			}
		}
		fmt.Printf("%s%s%s\n", prefix, connector, displayName)
	}

	childPrefix := prefix
	if !isRoot {
		if isLast {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}
	}

	names := node.sortedChildNames()
	for i, name := range names {
		child := node.children[name]
		isLastChild := i == len(names)-1
		printTree(child, childPrefix, isLastChild, false)
	}
}

func runKVTree(cmd *cobra.Command, args []string) error {
	project := args[0]
	if project == "" {
		return fmt.Errorf("project name cannot be empty")
	}

	client, err := NewClient()
	if err != nil {
		return err
	}

	secrets, err := client.ListSecrets(project)
	if err != nil {
		return fmt.Errorf("failed to list secrets in %s: %w", project, err)
	}

	if len(secrets) == 0 {
		fmt.Fprintf(os.Stderr, "No secrets found in project %q\n", project)
		return nil
	}

	// Build tree
	root := newTreeNode(project)
	for _, s := range secrets {
		parts := strings.Split(s.Path, "/")
		root.insert(parts)
	}

	// Print the project root name
	fmt.Printf("%s/\n", project)

	// Print children
	names := root.sortedChildNames()
	for i, name := range names {
		child := root.children[name]
		isLast := i == len(names)-1
		printTree(child, "", isLast, false)
	}

	return nil
}
