package api

import (
	"net/http"
	"strings"

	"github.com/teamvault/teamvault/internal/audit"
	"github.com/teamvault/teamvault/internal/db"
)

type createTeamRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type addMemberRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

type removeMemberRequest struct {
	UserID string `json:"user_id"`
}

func (s *Server) handleCreateTeam(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "user authentication required")
		return
	}

	orgID := r.PathValue("id")
	if orgID == "" {
		writeError(w, http.StatusBadRequest, "org id is required")
		return
	}

	// Verify the org exists
	_, err := s.db.GetOrgByID(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusNotFound, "organization not found")
		return
	}

	var req createTeamRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	team, err := s.db.CreateTeam(r.Context(), orgID, req.Name, req.Description)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "team name already exists in this organization")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create team")
		return
	}

	s.audit.Log(r.Context(), audit.Event{
		ActorType: "user",
		ActorID:   claims.UserID,
		Action:    "team.create",
		Resource:  "team:" + team.ID,
		Outcome:   "success",
		IP:        getClientIP(r.Context()),
	})

	writeJSON(w, http.StatusCreated, team)
}

func (s *Server) handleListTeams(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if orgID == "" {
		writeError(w, http.StatusBadRequest, "org id is required")
		return
	}

	teams, err := s.db.ListTeamsByOrg(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list teams")
		return
	}

	if teams == nil {
		teams = []db.Team{}
	}

	writeJSON(w, http.StatusOK, teams)
}

func (s *Server) handleAddTeamMember(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "user authentication required")
		return
	}

	teamID := r.PathValue("id")
	if teamID == "" {
		writeError(w, http.StatusBadRequest, "team id is required")
		return
	}

	// Verify the team exists
	_, err := s.db.GetTeamByID(r.Context(), teamID)
	if err != nil {
		writeError(w, http.StatusNotFound, "team not found")
		return
	}

	var req addMemberRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	if !isValidUUID(req.UserID) {
		writeError(w, http.StatusBadRequest, "user_id must be a valid UUID")
		return
	}

	if req.Role == "" {
		req.Role = "member"
	}

	member, err := s.db.AddTeamMember(r.Context(), teamID, req.UserID, req.Role)
	if err != nil {
		if isDBForeignKeyError(err) || isDBInvalidInputError(err) {
			writeError(w, http.StatusBadRequest, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to add team member")
		return
	}

	s.audit.Log(r.Context(), audit.Event{
		ActorType: "user",
		ActorID:   claims.UserID,
		Action:    "team.member.add",
		Resource:  "team:" + teamID + "/member:" + req.UserID,
		Outcome:   "success",
		IP:        getClientIP(r.Context()),
	})

	writeJSON(w, http.StatusCreated, member)
}

func (s *Server) handleRemoveTeamMember(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "user authentication required")
		return
	}

	teamID := r.PathValue("id")
	if teamID == "" {
		writeError(w, http.StatusBadRequest, "team id is required")
		return
	}

	var req removeMemberRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	if !isValidUUID(req.UserID) {
		writeError(w, http.StatusBadRequest, "user_id must be a valid UUID")
		return
	}

	if err := s.db.RemoveTeamMember(r.Context(), teamID, req.UserID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "team member not found")
			return
		}
		if isDBInvalidInputError(err) {
			writeError(w, http.StatusNotFound, "team member not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to remove team member")
		return
	}

	s.audit.Log(r.Context(), audit.Event{
		ActorType: "user",
		ActorID:   claims.UserID,
		Action:    "team.member.remove",
		Resource:  "team:" + teamID + "/member:" + req.UserID,
		Outcome:   "success",
		IP:        getClientIP(r.Context()),
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// handleListAllTeams lists all teams (admin) or the current user's teams.
func (s *Server) handleListAllTeams(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())

	if claims != nil && claims.Role == "admin" {
		// Admin: return all teams
		teams, err := s.db.ListAllTeams(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list teams")
			return
		}
		if teams == nil {
			teams = []db.Team{}
		}
		writeJSON(w, http.StatusOK, teams)
		return
	}

	// Non-admin: return teams the user belongs to
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "user authentication required")
		return
	}

	memberships, err := s.db.GetUserTeams(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list user teams")
		return
	}

	// Resolve team details for each membership
	var teams []db.Team
	for _, m := range memberships {
		team, err := s.db.GetTeamByID(r.Context(), m.TeamID)
		if err != nil {
			continue // skip teams that can't be resolved
		}
		teams = append(teams, *team)
	}
	if teams == nil {
		teams = []db.Team{}
	}
	writeJSON(w, http.StatusOK, teams)
}

func (s *Server) handleListTeamMembers(w http.ResponseWriter, r *http.Request) {
	teamID := r.PathValue("id")
	if teamID == "" {
		writeError(w, http.StatusBadRequest, "team id is required")
		return
	}

	if !isValidUUID(teamID) {
		writeError(w, http.StatusBadRequest, "team id must be a valid UUID")
		return
	}

	members, err := s.db.ListTeamMembers(r.Context(), teamID)
	if err != nil {
		if isDBInvalidInputError(err) {
			writeError(w, http.StatusBadRequest, "invalid team id")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to list team members")
		return
	}

	if members == nil {
		members = []db.TeamMember{}
	}

	writeJSON(w, http.StatusOK, members)
}
