package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "teamvault",
	Short: "TeamVault — secret management for teams",
	Long: `TeamVault is a secret management platform for teams.
Manage secrets via CLI, inject them into processes at runtime,
and control access with fine-grained policies.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(kvCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(tokenCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(policyCmd)
	rootCmd.AddCommand(orgCmd)
	rootCmd.AddCommand(teamCmd)
}
