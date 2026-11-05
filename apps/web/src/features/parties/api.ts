import { api, type Page } from "@/lib/api/client";

export type PartyKind = "customers" | "suppliers";

export interface Party {
  id: string;
  active: boolean;
  isWalkIn?: boolean;
  [field: string]: string | number | boolean | null | undefined;
}

export interface PartyFilters {
  search: string;
  active: "all" | "true" | "false";
  page: number;
}

export const partiesKeys = {
  all: (kind: PartyKind) => [kind] as const,
  list: (kind: PartyKind, f: PartyFilters) => [kind, "list", f] as const,
};

export const partiesApi = {
  list: (kind: PartyKind, f: PartyFilters) => {
    const params = new URLSearchParams({ page: String(f.page), pageSize: "20" });
    if (f.search.trim()) params.set("search", f.search.trim());
    if (f.active !== "all") params.set("active", f.active);
    return api<Page<Party>>("GET", `/${kind}?${params}`);
  },
  create: (kind: PartyKind, body: Record<string, unknown>) => api<Party>("POST", `/${kind}`, { body }),
  update: (kind: PartyKind, id: string, body: Record<string, unknown>) => api<Party>("PUT", `/${kind}/${id}`, { body }),
  setActive: (kind: PartyKind, id: string, active: boolean) => api<Party>("PATCH", `/${kind}/${id}/status`, { body: { active } }),
};
