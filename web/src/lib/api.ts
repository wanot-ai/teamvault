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

// ─── Policies ────────────────────────────────────────────────────────────────

export const policies = {
  list: () => apiFetch<Policy[]>("/policies"),

  create: (data: CreatePolicyRequest) =>
    apiFetch<Policy>("/policies", {
      method: "POST",
      body: JSON.stringify(data),
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
