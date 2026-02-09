-- IAM: Organizations, Teams, Agents, and IAM Policies
CREATE TABLE orgs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT UNIQUE NOT NULL,
    description TEXT,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE teams (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID REFERENCES orgs(id) NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(org_id, name)
);

CREATE TABLE team_members (
    team_id UUID REFERENCES teams(id) NOT NULL,
    user_id UUID REFERENCES users(id) NOT NULL,
    role TEXT NOT NULL DEFAULT 'member',
    joined_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY(team_id, user_id)
);

CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id UUID REFERENCES teams(id) NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    token_hash TEXT NOT NULL,
    scopes TEXT[],
    metadata JSONB,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT now(),
    expires_at TIMESTAMPTZ,
    UNIQUE(team_id, name)
);

-- Add org_id to projects
ALTER TABLE projects ADD COLUMN org_id UUID REFERENCES orgs(id);

-- IAM policies table (replaces simple policies)
CREATE TABLE iam_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID REFERENCES orgs(id) NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    policy_type TEXT NOT NULL, -- 'rbac', 'abac', 'pbac'
    policy_doc JSONB NOT NULL,
    hcl_source TEXT, -- original HCL source
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(org_id, name)
);

-- Indexes
CREATE INDEX idx_teams_org_id ON teams(org_id);
CREATE INDEX idx_team_members_user_id ON team_members(user_id);
CREATE INDEX idx_agents_team_id ON agents(team_id);
CREATE INDEX idx_projects_org_id ON projects(org_id);
CREATE INDEX idx_iam_policies_org_id ON iam_policies(org_id);
CREATE INDEX idx_iam_policies_type ON iam_policies(policy_type);
