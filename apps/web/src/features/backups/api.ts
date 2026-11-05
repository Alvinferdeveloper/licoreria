import { api, apiBlob, type Page } from "@/lib/api/client";

export interface Verification {
  ok: boolean;
  schemaVersion: number;
  tables: { table: string; rows: number }[] | null;
}

export interface Backup {
  id: string;
  fileName: string;
  sizeBytes: number | null;
  checksum: string | null;
  kind: "manual" | "scheduled" | "pre_restore";
  status: "running" | "completed" | "failed";
  createdBy: { id: string; name: string } | null;
  createdAt: string;
  completedAt: string | null;
  offsite: boolean;
  available: boolean;
  verifiedAt: string | null;
  verification: Verification | null;
}

export interface BackupStatus {
  offsite: boolean;
  scheduleHour: number;
  dailyRetention: number;
  weeklyRetention: number;
}

export interface RestoreResult {
  restored: Backup;
  safetyCopy: Backup;
}

export const CONFIRMATION_PHRASE = "RESTAURAR";

export const backupKeys = {
  all: ["backups"] as const,
  list: (page: number) => ["backups", "list", page] as const,
  status: ["backups", "status"] as const,
};

export const backupsApi = {
  list: (page: number) => api<Page<Backup>>("GET", `/backups?page=${page}&pageSize=15`),
  status: () => api<BackupStatus>("GET", "/backups/status"),
  create: () => api<Backup>("POST", "/backups"),
  verify: (id: string) => api<Backup>("POST", `/backups/${id}/verify`),
  download: (id: string) => apiBlob(`/backups/${id}/file`),
  restore: (id: string, body: { currentPassword: string; confirmation: string }) =>
    api<RestoreResult>("POST", `/backups/${id}/restore`, { body }),
};
