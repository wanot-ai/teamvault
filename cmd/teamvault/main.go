// TeamVault CLI — manage secrets from the command line
//
// Usage:
//
//	teamvault login --server https://vault.example.com --email user@example.com
//	teamvault login --server https://vault.example.com --oidc
//	teamvault kv get myproject/api-keys/stripe
//	teamvault kv put myproject/api-keys/stripe --value sk_live_xxx
//	teamvault kv list myproject
//	teamvault run --project myproject --map "STRIPE_KEY=api-keys/stripe" -- node server.js
//	teamvault token create --project myproject --scopes read --ttl 1h
//	teamvault rotation set myproject/db/password --cron "0 0 * * MON" --connector random_password
//	teamvault rotate myproject/db/password
//	teamvault lease issue --type database --ttl 15m --project myproject
//	teamvault doctor --server https://vault.example.com
//	teamvault export --project myproject --format json > secrets.enc.json
//	teamvault import --project myproject --file secrets.enc.json
package main

import (
	"fmt"
	"os"

	"github.com/teamvault/teamvault/cmd/teamvault/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
