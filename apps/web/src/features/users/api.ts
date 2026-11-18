import { api, type Page } from "@/lib/api/client";
import type { Role } from "@/features/auth/api";

export interface User {
  id: string;
  fullName: string;
  email: string | null;
  username: string;
  role: Role;
  active: boolean;
  mustChangePassword: boolean;
  pendingActivation: boolean;
  lastLoginAt: string | null;
  createdAt: string;
}

export interface UserInput {
  fullName: string;
  email: string;
  username: string;
  roleId: string;
}

export type AccessChannel = "email" | "code";

/** How the user gets in: an emailed link, or a code shown once to hand over in person. */
export interface Access {
  channel: AccessChannel;
  code?: string;
  expiresAt: string;
}

export interface UserFilters {
  search: string;
  active: "all" | "true" | "false";
  page: number;
}

export const usersKeys = {
  all: ["users"] as const,
  list: (filters: UserFilters) => ["users", "list", filters] as const,
};

export const usersApi = {
  list: ({ search, active, page }: UserFilters) => {
    const params = new URLSearchParams({ page: String(page), pageSize: "20" });
    if (search.trim()) params.set("search", search.trim());
    if (active !== "all") params.set("active", active);
    return api<Page<User>>("GET", `/users?${params}`);
  },
  create: (input: UserInput) => api<{ user: User; access: Access }>("POST", "/users", { body: input }),
  update: (id: string, input: UserInput) => api<User>("PUT", `/users/${id}`, { body: input }),
  setActive: (id: string, active: boolean) => api<User>("PATCH", `/users/${id}/status`, { body: { active } }),
  resendAccess: (id: string, channel: AccessChannel) =>
    api<Access>("POST", `/users/${id}/resend-access`, { body: { channel } }),
};
