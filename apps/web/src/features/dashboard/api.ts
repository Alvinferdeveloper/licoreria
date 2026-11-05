import { api } from "@/lib/api/client";

export interface Dashboard {
  scope: "all" | "self";
  kpis: {
    salesToday: string;
    salesYesterday: string;
    salesCount: number;
    averageTicket: string;
    profitToday?: string;
    lowStock: number;
  };
  daily: { date: string; total: string; previous: string }[];
  top: { name: string; units: number; revenue: string }[];
  categories: { name: string; total: string }[];
  methods: { date: string; cashNio: string; cashUsd: string; card: string; transfer: string }[];
  heatmap: { weekday: number; hour: number; sales: number; total: string }[];
}

export const dashboardKeys = { all: ["dashboard"] as const };

export const dashboardApi = { get: () => api<Dashboard>("GET", "/dashboard") };
