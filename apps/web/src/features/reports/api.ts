import { api, apiBlob } from "@/lib/api/client";

export type ParamKind = "date" | "user" | "category" | "brand" | "supplier" | "register" | "product" | "choice" | "bool" | "text";
export type ColumnKind = "text" | "mono" | "money" | "number" | "percent" | "datetime" | "date";

export interface ReportParam {
  key: string;
  label: string;
  kind: ParamKind;
  required?: boolean;
  choices?: { value: string; label: string }[];
}

export interface ReportColumn {
  key: string;
  label: string;
  kind: ColumnKind;
  total?: boolean;
}

export interface ReportDefinition {
  key: string;
  title: string;
  description: string;
  group: string;
  params: ReportParam[];
  columns: ReportColumn[];
  groupBy?: string;
}

export interface Option {
  id: string;
  name: string;
}

export interface ReportCatalog {
  reports: ReportDefinition[];
  options: Partial<Record<"users" | "categories" | "brands" | "suppliers" | "registers", Option[]>>;
}

export type ReportRow = Record<string, string | number | null>;

export interface ReportResult {
  key: string;
  title: string;
  columns: ReportColumn[];
  rows: ReportRow[];
  totals?: ReportRow;
  truncated: boolean;
  filters: string[];
}

export type ReportFilters = Record<string, string>;

const query = (filters: ReportFilters, format?: string) => {
  const params = new URLSearchParams(Object.entries(filters).filter(([, v]) => v !== ""));
  if (format) params.set("format", format);
  return params.toString();
};

export const reportKeys = {
  catalog: ["reports", "catalog"] as const,
  run: (key: string, filters: ReportFilters) => ["reports", "run", key, filters] as const,
};

export const reportsApi = {
  catalog: () => api<ReportCatalog>("GET", "/reports"),
  run: (key: string, filters: ReportFilters) => api<ReportResult>("GET", `/reports/${key}?${query(filters)}`),
  export: (key: string, filters: ReportFilters, format: "xlsx" | "pdf") => apiBlob(`/reports/${key}?${query(filters, format)}`),
};
