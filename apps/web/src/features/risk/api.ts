import { api } from "@/lib/api/client";

export type Period = "7" | "30" | "90";
export type Flag = "voids" | "discounts" | "shortages" | "pin" | "outside_hours";
export type Level = "high" | "medium" | "low";

export interface CashierRisk {
  user: { id: string; name: string };
  sales: number;
  total: string;
  voids: number;
  voidedAmount: string;
  discounts: number;
  discountAmount: string;
  nearLimit: number;
  overLimit: number;
  shifts: number;
  shortShifts: number;
  shortage: string;
  overage: string;
  expenses: number;
  pinFailures: number;
  outsideHours: number;
  flags: Flag[];
  level: Level;
}

export interface RiskEvent {
  kind: "void" | "shortage" | "discount" | "pin" | "outside_hours";
  at: string;
  user: string;
  amount: string | null;
  detail: string;
  saleId: string | null;
}

export interface RiskReport {
  days: number;
  discountLimitPercent: number;
  store: { void: number; discount: number; short: number };
  cashiers: CashierRisk[];
  events: RiskEvent[];
}

export const riskKeys = { report: (period: Period) => ["risk", period] as const };

export const riskApi = {
  report: (period: Period) => api<RiskReport>("GET", `/cash-risk?days=${period}`),
};
