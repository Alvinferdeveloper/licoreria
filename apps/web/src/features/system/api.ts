import { api } from "@/lib/api/client";

export type CheckStatus = "ok" | "warn" | "fail" | "unknown";

export interface HourStats {
  hour: string;
  requests: number;
  clientErrors: number;
  serverErrors: number;
  avgMs: number;
  maxMs: number;
}

export interface FailedRequest {
  at: string;
  method: string;
  path: string;
  status: number;
  requestId: string;
}

export interface SystemStatus {
  status: CheckStatus;
  checkedAt: string;
  api: { status: CheckStatus; startedAt: string; goVersion: string; revision: string; goroutines: number; memoryMb: number };
  database: { status: CheckStatus; latencyMs: number; sizeBytes: number; inUse: number; idle: number; maxConns: number; version: string };
  backups: { status: CheckStatus; lastAt: string | null; lastFailedAt: string | null; offsite: boolean; scheduleHour: number };
  disk: { status: CheckStatus; totalBytes: number; freeBytes: number };
  email: { status: CheckStatus; sender: "resend" | "smtp"; pending: number; oldestPendingAt: string | null; sent24h: number; failed24h: number };
  storage: { status: CheckStatus; enabled: boolean };
  exchangeRate: { status: CheckStatus; today: string | null };
  activity: { activeSessions: number; activeUsers: number; openShifts: number; offlineSynced24h: number };
  traffic: { status: CheckStatus; hours: HourStats[]; failed: FailedRequest[] };
}

export const systemKeys = { status: ["system", "status"] as const };

export const systemApi = {
  status: () => api<SystemStatus>("GET", "/system/status"),
};
