import { api, type Page } from "@/lib/api/client";

export interface AuditEntry {
  id: string;
  userName: string | null;
  action: string;
  entity: string;
  entityId: string | null;
  before: Record<string, unknown> | null;
  after: Record<string, unknown> | null;
  ip: string | null;
  userAgent: string | null;
  createdAt: string;
}

export interface AuditFilters {
  search: string;
  module: string;
  from: string;
  to: string;
  page: number;
}

export const auditKeys = {
  list: (f: AuditFilters) => ["audit-log", f] as const,
};

export const auditApi = {
  list: (f: AuditFilters) => {
    const params = new URLSearchParams({ page: String(f.page), pageSize: "25" });
    if (f.search) params.set("search", f.search);
    if (f.module) params.set("module", f.module);
    if (f.from) params.set("from", f.from);
    if (f.to) params.set("to", f.to);
    return api<Page<AuditEntry>>("GET", `/audit-log?${params}`);
  },
};
