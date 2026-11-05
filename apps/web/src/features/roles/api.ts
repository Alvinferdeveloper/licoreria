import { api } from "@/lib/api/client";

export interface RoleDetail {
  id: string;
  code: string | null;
  name: string;
  description: string;
  isSystem: boolean;
  userCount: number;
  permissions: string[];
}

export interface PermissionDef {
  key: string;
  module: string;
  action: string;
  description: string;
}

export const rolesKeys = {
  all: ["roles"] as const,
  permissions: ["permissions"] as const,
};

export const rolesApi = {
  list: () => api<{ data: RoleDetail[] }>("GET", "/roles").then((r) => r.data),
  permissions: () => api<{ data: PermissionDef[] }>("GET", "/permissions").then((r) => r.data),
  setPermissions: (id: string, permissions: string[]) =>
    api<RoleDetail>("PUT", `/roles/${id}/permissions`, { body: { permissions } }),
};

export const isAdminRole = (role: Pick<RoleDetail, "code">) => role.code === "admin";
