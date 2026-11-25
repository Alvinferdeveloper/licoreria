import { api } from "@/lib/api/client";

export interface Role {
  id: string;
  code: string | null;
  name: string;
}

export interface SessionUser {
  id: string;
  fullName: string;
  email: string;
  username: string;
  role: Role;
  permissions: string[];
  mustChangePassword: boolean;
  /** Minutes without activity before the session closes; 0 means no limit. */
  idleTimeoutMinutes: number;
}

export interface SessionResponse {
  accessToken: string;
  accessTokenExpiresAt: string;
  user: SessionUser;
}

export const authApi = {
  login: (identifier: string, password: string) =>
    api<SessionResponse>("POST", "/auth/login", { body: { identifier, password }, skipRefresh: true }),
  refresh: () => api<SessionResponse>("POST", "/auth/refresh", { skipRefresh: true }),
  logout: () => api<void>("POST", "/auth/logout", { skipRefresh: true }),
  me: () => api<SessionUser>("GET", "/auth/me"),
  changePassword: (currentPassword: string, newPassword: string) =>
    api<void>("POST", "/auth/change-password", { body: { currentPassword, newPassword } }),
  forgotPassword: (email: string) =>
    api<{ message: string }>("POST", "/auth/forgot-password", { body: { email }, skipRefresh: true }),
  resetPassword: (token: string, newPassword: string) =>
    api<void>("POST", "/auth/reset-password", { body: { token, newPassword }, skipRefresh: true }),
  activate: (input: { token: string } | { username: string; code: string }, newPassword: string) =>
    api<void>("POST", "/auth/activate", { body: { ...input, newPassword }, skipRefresh: true }),
};

export const can = (user: SessionUser | null, permission: string) =>
  user?.permissions.includes(permission) ?? false;
