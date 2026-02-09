// Terraform Provider for TeamVault - Entry Point
//
// This is a stub provider that can be compiled as a Terraform plugin.
// Build: go build -o terraform-provider-teamvault
//
// Note: This requires the terraform-plugin-sdk dependency.
// Add to go.mod: github.com/hashicorp/terraform-plugin-sdk/v2

package main

import (
	"fmt"
	"os"
)

func main() {
	// This is a stub entry point. When the terraform-plugin-sdk is added as
	// a dependency, replace with:
	//
	//   plugin.Serve(&plugin.ServeOpts{
	//       ProviderFunc: provider.Provider,
	//   })
	//
	// For now, we print a helpful message.
	fmt.Println("TeamVault Terraform Provider v0.1.0 (stub)")
	fmt.Println("This provider requires terraform-plugin-sdk/v2 to be added to go.mod.")
	fmt.Println("See terraform/README.md for setup instructions.")
	os.Exit(0)
}
