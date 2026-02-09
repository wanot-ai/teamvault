// TeamVault API Client — typed fetch wrapper for all endpoints

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

// ─── Types ───────────────────────────────────────────────────────────────────

export interface User {
  id: string;
  email: string;
  name: string;
  role: "admin" | "member";
  created_at: string;
}

export interface Project {
  id: string;
  name: string;
  description: string;
  created_by: string;
  created_at: string;
}

export interface Secret {
  id: string;
  project_id: string;
  path: string;
  description: string;
  created_by: string;
  created_at: string;
  deleted_at: string | null;
  current_version?: number;
}

export interface SecretValue {
  id: string;
  path: string;
  value: string;
  version: number;
  created_by: string;
  created_at: string;
}

export interface SecretVersion {
  id: string;
  secret_id: string;
  version: number;
  created_by: string;
  created_at: string;
}

export interface ServiceAccount {
  id: string;
  name: string;
  project_id: string;
  scopes: string[];
  created_by: string;
  created_at: string;
  expires_at: string | null;
  token?: string; // Only returned on creation
}

export interface Policy {
  id: string;
  name: string;
  effect: "allow" | "deny";
  actions: string[];
  resource_pattern: string;
  subject_type: "user" | "service_account";
  subject_id: string | null;
  conditions: Record<string, unknown> | null;
  created_at: string;
}

export interface AuditEvent {
  id: string;
  timestamp: string;
  actor_type: "user" | "service_account";
  actor_id: string;
  action: string;
  resource: string;
  outcome: "success" | "denied" | "error";
  ip: string;
  metadata: Record<string, unknown> | null;
}

// ─── Organization / Team / Agent Types ───────────────────────────────────────

export interface Organization {
  id: string;
  name: string;
  slug: string;
  description: string;
  owner_id: string;
  created_at: string;
  updated_at: string;
  member_count?: number;
  team_count?: number;
}

export interface Team {
  id: string;
  org_id: string;
  name: string;
  slug: string;
  description: string;
  created_by: string;
  created_at: string;
  updated_at: string;
  member_count?: number;
  agent_count?: number;
}

export interface TeamMember {
  id: string;
  team_id: string;
  user_id: string;
  role: "owner" | "admin" | "member" | "viewer";
  user: User;
  added_by: string;
  added_at: string;
}

export interface Agent {
  id: string;
  team_id: string;
  name: string;
  description: string;
  scopes: string[];
  token_status: "active" | "expired" | "revoked";
  created_by: string;
  created_at: string;
  expires_at: string | null;
  last_used_at: string | null;
  token?: string; // Only returned on creation
}

// ─── IAM Policy Types ────────────────────────────────────────────────────────

export type IAMPolicyType = "rbac" | "abac" | "pbac";

export interface IAMPolicy {
  id: string;
  name: string;
  description: string;
  type: IAMPolicyType;
  effect: "allow" | "deny";
  hcl_source: string;
  rules: IAMPolicyRule[];
  conditions: Record<string, unknown> | null;
  created_by: string;
  created_at: string;
  updated_at: string;
  enabled: boolean;
}

export interface IAMPolicyRule {
  id: string;
  policy_id: string;
  actions: string[];
  resources: string[];
  subjects: string[];
  conditions: Record<string, unknown> | null;
}

// ─── Request Types ───────────────────────────────────────────────────────────

export interface LoginRequest {
  email: string;
  password: string;
}

export interface LoginResponse {
  token: string;
  user: User;
}

export interface RegisterRequest {
  email: string;
  password: string;
  name: string;
}

export interface CreateProjectRequest {
  name: string;
  description?: string;
}

export interface PutSecretRequest {
  value: string;
  description?: string;
}

export interface CreateServiceAccountRequest {
  name: string;
  project_id: string;
  scopes: string[];
  expires_at?: string;
}

export interface CreatePolicyRequest {
  name: string;
  effect: "allow" | "deny";
  actions: string[];
  resource_pattern: string;
  subject_type: "user" | "service_account";
  subject_id?: string;
  conditions?: Record<string, unknown>;
}

export interface CreateOrganizationRequest {
  name: string;
  slug?: string;
  description?: string;
}

export interface UpdateOrganizationRequest {
  name?: string;
  description?: string;
}

export interface CreateTeamRequest {
  name: string;
  slug?: string;
  description?: string;
}

export interface UpdateTeamRequest {
  name?: string;
  description?: string;
}

export interface AddTeamMemberRequest {
  user_id: string;
  role: "admin" | "member" | "viewer";
}

export interface CreateAgentRequest {
  name: string;
  description?: string;
  scopes: string[];
  ttl_hours?: number;
}

export interface CreateIAMPolicyRequest {
  name: string;
  description?: string;
  type: IAMPolicyType;
  effect: "allow" | "deny";
  hcl_source?: string;
  rules?: Omit<IAMPolicyRule, "id" | "policy_id">[];
  conditions?: Record<string, unknown>;
  enabled?: boolean;
}

export interface UpdateIAMPolicyRequest {
  name?: string;
  description?: string;
  effect?: "allow" | "deny";
  hcl_source?: string;
  rules?: Omit<IAMPolicyRule, "id" | "policy_id">[];
  conditions?: Record<string, unknown>;
  enabled?: boolean;
}

export interface AuditFilters {
  action?: string;
  actor_id?: string;
  resource?: string;
  outcome?: string;
  from?: string;
  to?: string;
  limit?: number;
  offset?: number;
}

// ─── API Error ───────────────────────────────────────────────────────────────

export class ApiError extends Error {
  constructor(
    public status: number,
    public statusText: string,
    public body?: string
  ) {
    super(`API Error ${status}: ${statusText}`);
    this.name = "ApiError";
  }
}

// ─── Fetch wrapper ───────────────────────────────────────────────────────────

function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem("teamvault_token");
}

async function apiFetch<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const token = getToken();
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string>),
  };

  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  });

  if (res.status === 401) {
    // Clear token and redirect to login
    if (typeof window !== "undefined") {
      localStorage.removeItem("teamvault_token");
      localStorage.removeItem("teamvault_user");
      window.location.href = "/login";
    }
    throw new ApiError(res.status, res.statusText);
  }

  if (!res.ok) {
    const body = await res.text().catch(() => "");
    throw new ApiError(res.status, res.statusText, body);
  }

  // Handle 204 No Content
  if (res.status === 204) {
    return undefined as T;
  }

  return res.json();
}

// ─── Auth ────────────────────────────────────────────────────────────────────

export const auth = {
  login: (data: LoginRequest) =>
    apiFetch<LoginResponse>("/auth/login", {
      method: "POST",
      body: JSON.stringify(data),
    }),

  register: (data: RegisterRequest) =>
    apiFetch<LoginResponse>("/auth/register", {
      method: "POST",
      body: JSON.stringify(data),
    }),

  me: () => apiFetch<User>("/auth/me"),
};

// ─── Projects ────────────────────────────────────────────────────────────────

export const projects = {
  list: () => apiFetch<Project[]>("/projects"),

  create: (data: CreateProjectRequest) =>
    apiFetch<Project>("/projects", {
      method: "POST",
      body: JSON.stringify(data),
    }),
};

// ─── Secrets ─────────────────────────────────────────────────────────────────

export const secrets = {
  list: (projectId: string) =>
    apiFetch<Secret[]>(`/secrets/${projectId}`),

  get: (projectId: string, path: string) =>
    apiFetch<SecretValue>(`/secrets/${projectId}/${encodeURIComponent(path)}`),

  put: (projectId: string, path: string, data: PutSecretRequest) =>
    apiFetch<SecretValue>(`/secrets/${projectId}/${encodeURIComponent(path)}`, {
      method: "PUT",
      body: JSON.stringify(data),
    }),

  delete: (projectId: string, path: string) =>
    apiFetch<void>(`/secrets/${projectId}/${encodeURIComponent(path)}`, {
      method: "DELETE",
    }),

  versions: (projectId: string, path: string) =>
    apiFetch<SecretVersion[]>(
      `/secrets/${projectId}/${encodeURIComponent(path)}/versions`
    ),
};

// ─── Service Accounts ────────────────────────────────────────────────────────

export const serviceAccounts = {
  list: () => apiFetch<ServiceAccount[]>("/service-accounts"),

  create: (data: CreateServiceAccountRequest) =>
    apiFetch<ServiceAccount>("/service-accounts", {
      method: "POST",
      body: JSON.stringify(data),
    }),
};

// ─── Policies (legacy) ──────────────────────────────────────────────────────

export const policies = {
  list: () => apiFetch<Policy[]>("/policies"),

  create: (data: CreatePolicyRequest) =>
    apiFetch<Policy>("/policies", {
      method: "POST",
      body: JSON.stringify(data),
    }),
};

// ─── Organizations ───────────────────────────────────────────────────────────

export const orgs = {
  list: () => apiFetch<Organization[]>("/orgs"),

  get: (orgId: string) => apiFetch<Organization>(`/orgs/${orgId}`),

  create: (data: CreateOrganizationRequest) =>
    apiFetch<Organization>("/orgs", {
      method: "POST",
      body: JSON.stringify(data),
    }),

  update: (orgId: string, data: UpdateOrganizationRequest) =>
    apiFetch<Organization>(`/orgs/${orgId}`, {
      method: "PATCH",
      body: JSON.stringify(data),
    }),

  delete: (orgId: string) =>
    apiFetch<void>(`/orgs/${orgId}`, {
      method: "DELETE",
    }),
};

// ─── Teams ───────────────────────────────────────────────────────────────────

export const teams = {
  list: (orgId: string) => apiFetch<Team[]>(`/orgs/${orgId}/teams`),

  get: (orgId: string, teamId: string) =>
    apiFetch<Team>(`/orgs/${orgId}/teams/${teamId}`),

  create: (orgId: string, data: CreateTeamRequest) =>
    apiFetch<Team>(`/orgs/${orgId}/teams`, {
      method: "POST",
      body: JSON.stringify(data),
    }),

  update: (orgId: string, teamId: string, data: UpdateTeamRequest) =>
    apiFetch<Team>(`/orgs/${orgId}/teams/${teamId}`, {
      method: "PATCH",
      body: JSON.stringify(data),
    }),

  delete: (orgId: string, teamId: string) =>
    apiFetch<void>(`/orgs/${orgId}/teams/${teamId}`, {
      method: "DELETE",
    }),
};

// ─── Team Members ────────────────────────────────────────────────────────────

export const teamMembers = {
  list: (orgId: string, teamId: string) =>
    apiFetch<TeamMember[]>(`/orgs/${orgId}/teams/${teamId}/members`),

  add: (orgId: string, teamId: string, data: AddTeamMemberRequest) =>
    apiFetch<TeamMember>(`/orgs/${orgId}/teams/${teamId}/members`, {
      method: "POST",
      body: JSON.stringify(data),
    }),

  remove: (orgId: string, teamId: string, userId: string) =>
    apiFetch<void>(`/orgs/${orgId}/teams/${teamId}/members/${userId}`, {
      method: "DELETE",
    }),

  updateRole: (orgId: string, teamId: string, userId: string, role: string) =>
    apiFetch<TeamMember>(
      `/orgs/${orgId}/teams/${teamId}/members/${userId}`,
      {
        method: "PATCH",
        body: JSON.stringify({ role }),
      }
    ),
};

// ─── Agents ──────────────────────────────────────────────────────────────────

export const agents = {
  list: (orgId: string, teamId: string) =>
    apiFetch<Agent[]>(`/orgs/${orgId}/teams/${teamId}/agents`),

  get: (orgId: string, teamId: string, agentId: string) =>
    apiFetch<Agent>(`/orgs/${orgId}/teams/${teamId}/agents/${agentId}`),

  create: (orgId: string, teamId: string, data: CreateAgentRequest) =>
    apiFetch<Agent>(`/orgs/${orgId}/teams/${teamId}/agents`, {
      method: "POST",
      body: JSON.stringify(data),
    }),

  revoke: (orgId: string, teamId: string, agentId: string) =>
    apiFetch<Agent>(`/orgs/${orgId}/teams/${teamId}/agents/${agentId}/revoke`, {
      method: "POST",
    }),

  delete: (orgId: string, teamId: string, agentId: string) =>
    apiFetch<void>(`/orgs/${orgId}/teams/${teamId}/agents/${agentId}`, {
      method: "DELETE",
    }),
};

// ─── IAM Policies ────────────────────────────────────────────────────────────

export const iamPolicies = {
  list: (type?: IAMPolicyType) => {
    const params = type ? `?type=${type}` : "";
    return apiFetch<IAMPolicy[]>(`/iam/policies${params}`);
  },

  get: (policyId: string) =>
    apiFetch<IAMPolicy>(`/iam/policies/${policyId}`),

  create: (data: CreateIAMPolicyRequest) =>
    apiFetch<IAMPolicy>("/iam/policies", {
      method: "POST",
      body: JSON.stringify(data),
    }),

  update: (policyId: string, data: UpdateIAMPolicyRequest) =>
    apiFetch<IAMPolicy>(`/iam/policies/${policyId}`, {
      method: "PATCH",
      body: JSON.stringify(data),
    }),

  delete: (policyId: string) =>
    apiFetch<void>(`/iam/policies/${policyId}`, {
      method: "DELETE",
    }),

  toggle: (policyId: string, enabled: boolean) =>
    apiFetch<IAMPolicy>(`/iam/policies/${policyId}`, {
      method: "PATCH",
      body: JSON.stringify({ enabled }),
    }),
};

// ─── Audit ───────────────────────────────────────────────────────────────────

export const audit = {
  list: (filters?: AuditFilters) => {
    const params = new URLSearchParams();
    if (filters) {
      Object.entries(filters).forEach(([key, value]) => {
        if (value !== undefined && value !== "") {
          params.set(key, String(value));
        }
      });
    }
    const query = params.toString();
    return apiFetch<AuditEvent[]>(`/audit${query ? `?${query}` : ""}`);
  },
};
