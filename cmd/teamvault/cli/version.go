package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags.
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the TeamVault CLI version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("teamvault version %s\n", Version)
	},
}
