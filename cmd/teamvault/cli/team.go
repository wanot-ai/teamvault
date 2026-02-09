package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// --- Team API types ---

// TeamResponse represents a team returned by the API.
type TeamResponse struct {
	ID          string       `json:"id"`
	OrgID       string       `json:"org_id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	CreatedAt   string       `json:"created_at"`
	MemberCount int          `json:"member_count"`
	Members     []TeamMember `json:"members,omitempty"`
}

// TeamMember represents a member within a team.
type TeamMember struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

// --- API client methods ---

// CreateTeam creates a new team within an organization.
func (c *APIClient) CreateTeam(orgID, name, description string) (*TeamResponse, error) {
	var resp TeamResponse
	body := map[string]string{
		"org_id": orgID,
		"name":   name,
	}
	if description != "" {
		body["description"] = description
	}
	err := c.do("POST", "/api/v1/teams", body, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListTeams returns all teams, optionally filtered by org.
func (c *APIClient) ListTeams(orgID string) ([]TeamResponse, error) {
	var resp struct {
		Teams []TeamResponse `json:"teams"`
	}
	path := "/api/v1/teams"
	if orgID != "" {
		path = fmt.Sprintf("/api/v1/teams?org_id=%s", orgID)
	}
	err := c.do("GET", path, nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Teams, nil
}

// AddTeamMember adds a user to a team.
func (c *APIClient) AddTeamMember(teamID, email, role string) error {
	body := map[string]string{
		"email": email,
	}
	if role != "" {
		body["role"] = role
	}
	return c.do("POST", fmt.Sprintf("/api/v1/teams/%s/members", teamID), body, nil)
}

// AddTeamAgent adds a service account / agent to a team.
func (c *APIClient) AddTeamAgent(teamID, agentID, role string) error {
	body := map[string]string{
		"agent_id": agentID,
	}
	if role != "" {
		body["role"] = role
	}
	return c.do("POST", fmt.Sprintf("/api/v1/teams/%s/agents", teamID), body, nil)
}

// --- Cobra commands ---

var teamCmd = &cobra.Command{
	Use:   "team",
	Short: "Manage teams",
	Long: `Create and manage teams, add members and agents.

Examples:
  teamvault team create --org my-org --name backend-team
  teamvault team list --org my-org
  teamvault team add-member --team TEAM_ID --email user@example.com --role admin
  teamvault team add-agent --team TEAM_ID --agent-id sa_xxx --role read`,
}

var (
	teamCreateOrg         string
	teamCreateName        string
	teamCreateDescription string

	teamListOrg string

	teamAddMemberTeam  string
	teamAddMemberEmail string
	teamAddMemberRole  string

	teamAddAgentTeam    string
	teamAddAgentAgentID string
	teamAddAgentRole    string
)

var teamCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new team",
	Long: `Create a new team within an organization.

Examples:
  teamvault team create --org my-org --name backend-team
  teamvault team create --org my-org --name frontend-team --description "Frontend engineers"`,
	RunE: runTeamCreate,
}

var teamListCmd = &cobra.Command{
	Use:   "list",
	Short: "List teams",
	Long: `List all teams, optionally filtered by organization.

Examples:
  teamvault team list
  teamvault team list --org my-org`,
	RunE: runTeamList,
}

var teamAddMemberCmd = &cobra.Command{
	Use:   "add-member",
	Short: "Add a user to a team",
	Long: `Add a user to a team by email address.

Examples:
  teamvault team add-member --team TEAM_ID --email user@example.com
  teamvault team add-member --team TEAM_ID --email user@example.com --role admin`,
	RunE: runTeamAddMember,
}

var teamAddAgentCmd = &cobra.Command{
	Use:   "add-agent",
	Short: "Add a service account (agent) to a team",
	Long: `Add a service account or agent to a team.

Examples:
  teamvault team add-agent --team TEAM_ID --agent-id sa_xxx
  teamvault team add-agent --team TEAM_ID --agent-id sa_xxx --role read`,
	RunE: runTeamAddAgent,
}

func init() {
	teamCreateCmd.Flags().StringVar(&teamCreateOrg, "org", "", "Organization ID or name (required)")
	teamCreateCmd.Flags().StringVar(&teamCreateName, "name", "", "Team name (required)")
	teamCreateCmd.Flags().StringVar(&teamCreateDescription, "description", "", "Team description")
	teamCreateCmd.MarkFlagRequired("org")
	teamCreateCmd.MarkFlagRequired("name")

	teamListCmd.Flags().StringVar(&teamListOrg, "org", "", "Filter by organization ID or name")

	teamAddMemberCmd.Flags().StringVar(&teamAddMemberTeam, "team", "", "Team ID (required)")
	teamAddMemberCmd.Flags().StringVar(&teamAddMemberEmail, "email", "", "User email to add (required)")
	teamAddMemberCmd.Flags().StringVar(&teamAddMemberRole, "role", "", "Role for the member (e.g. admin, member, read)")
	teamAddMemberCmd.MarkFlagRequired("team")
	teamAddMemberCmd.MarkFlagRequired("email")

	teamAddAgentCmd.Flags().StringVar(&teamAddAgentTeam, "team", "", "Team ID (required)")
	teamAddAgentCmd.Flags().StringVar(&teamAddAgentAgentID, "agent-id", "", "Agent/service account ID (required)")
	teamAddAgentCmd.Flags().StringVar(&teamAddAgentRole, "role", "", "Role for the agent (e.g. read, write)")
	teamAddAgentCmd.MarkFlagRequired("team")
	teamAddAgentCmd.MarkFlagRequired("agent-id")

	teamCmd.AddCommand(teamCreateCmd)
	teamCmd.AddCommand(teamListCmd)
	teamCmd.AddCommand(teamAddMemberCmd)
	teamCmd.AddCommand(teamAddAgentCmd)
}

func runTeamCreate(cmd *cobra.Command, args []string) error {
	client, err := NewClient()
	if err != nil {
		return err
	}

	team, err := client.CreateTeam(teamCreateOrg, teamCreateName, teamCreateDescription)
	if err != nil {
		return fmt.Errorf("failed to create team: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Team created\n")
	fmt.Fprintf(os.Stderr, "  ID:   %s\n", team.ID)
	fmt.Fprintf(os.Stderr, "  Name: %s\n", team.Name)
	fmt.Fprintf(os.Stderr, "  Org:  %s\n", team.OrgID)

	out, _ := json.MarshalIndent(team, "", "  ")
	fmt.Println(string(out))

	return nil
}

func runTeamList(cmd *cobra.Command, args []string) error {
	client, err := NewClient()
	if err != nil {
		return err
	}

	teams, err := client.ListTeams(teamListOrg)
	if err != nil {
		return fmt.Errorf("failed to list teams: %w", err)
	}

	if len(teams) == 0 {
		fmt.Fprintf(os.Stderr, "No teams found\n")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tORG\tMEMBERS\tDESCRIPTION\tCREATED")
	for _, t := range teams {
		created := t.CreatedAt
		if len(created) > 19 {
			created = created[:19]
		}
		desc := t.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		if desc == "" {
			desc = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n", t.Name, t.OrgID, t.MemberCount, desc, created)
	}
	w.Flush()

	return nil
}

func runTeamAddMember(cmd *cobra.Command, args []string) error {
	client, err := NewClient()
	if err != nil {
		return err
	}

	if err := client.AddTeamMember(teamAddMemberTeam, teamAddMemberEmail, teamAddMemberRole); err != nil {
		return fmt.Errorf("failed to add member: %w", err)
	}

	role := teamAddMemberRole
	if role == "" {
		role = "member"
	}
	fmt.Fprintf(os.Stderr, "✓ Added %s to team %s (role: %s)\n", teamAddMemberEmail, teamAddMemberTeam, role)
	return nil
}

func runTeamAddAgent(cmd *cobra.Command, args []string) error {
	client, err := NewClient()
	if err != nil {
		return err
	}

	if err := client.AddTeamAgent(teamAddAgentTeam, teamAddAgentAgentID, teamAddAgentRole); err != nil {
		return fmt.Errorf("failed to add agent: %w", err)
	}

	role := teamAddAgentRole
	if role == "" {
		role = "read"
	}
	fmt.Fprintf(os.Stderr, "✓ Added agent %s to team %s (role: %s)\n", teamAddAgentAgentID, teamAddAgentTeam, role)
	return nil
}
