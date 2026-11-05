import { api, apiBlob, type Page } from "@/lib/api/client";

export interface SaleItem {
  id: string;
  productId: string;
  presentationId: string | null;
  name: string;
  sku: string;
  quantity: number;
  unitsPerItem: number;
  unitPrice: string;
  discount: string;
  taxPercent: string;
  tax: string;
  subtotal: string;
  total: string;
}

export interface SalePayment {
  method: "cash" | "card" | "transfer";
  currency: "NIO" | "USD";
  amount: string;
  exchangeRate: string;
  amountNio: string;
  reference: string | null;
}

export interface Sale {
  id: string;
  invoice: string;
  series: string;
  number: number;
  status: "completed" | "voided";
  register: { id: string; name: string };
  cashier: { id: string; name: string };
  customer: { id: string; name: string; taxId: string | null; email: string | null };
  shiftId: string;
  subtotal: string;
  discount: string;
  tax: string;
  total: string;
  exchangeRate: string;
  changeNio: string;
  itemCount: number;
  notes: string;
  voidReason: string | null;
  voidedAt: string | null;
  createdAt: string;
  items?: SaleItem[];
  payments?: SalePayment[];
}

export interface Authorization {
  username: string;
  pin: string;
}

export interface SaleInput {
  customerId: string | null;
  ageVerified: boolean;
  notes: string;
  discount: { kind: "percent" | "amount"; value: string } | null;
  authorization: Authorization | null;
  lines: { productId: string; presentationId: string | null; quantity: number }[];
  payments: { method: SalePayment["method"]; currency: SalePayment["currency"]; amount: string; reference: string | null }[];
}

export interface SaleFilters {
  search: string;
  status: "all" | Sale["status"];
  from: string;
  to: string;
  page: number;
}

export interface StoreSettings {
  business: { name: string; taxId: string; address: string; phone: string; email: string; logoUrl: string | null };
  pos: { discountLimitPercent: number; allowNegativeStock: boolean; defaultPrintFormat: string };
}

export const salesKeys = {
  all: ["sales"] as const,
  list: (f: SaleFilters) => ["sales", "list", f] as const,
  detail: (id: string) => ["sales", "detail", id] as const,
  store: ["settings", "store"] as const,
};

export const salesApi = {
  /** The key makes retries safe: the API returns the first sale instead of charging twice. */
  create: (key: string, input: SaleInput) => api<Sale>("POST", "/sales", { body: input, headers: { "Idempotency-Key": key } }),
  list: (f: SaleFilters) => {
    const params = new URLSearchParams({ page: String(f.page), pageSize: "20" });
    if (f.search.trim()) params.set("search", f.search.trim());
    if (f.status !== "all") params.set("status", f.status);
    if (f.from) params.set("from", f.from);
    if (f.to) params.set("to", f.to);
    return api<Page<Sale>>("GET", `/sales?${params}`);
  },
  get: (id: string) => api<Sale>("GET", `/sales/${id}`),
  void: (id: string, reason: string, authorization: Authorization | null) => api<Sale>("POST", `/sales/${id}/void`, { body: { reason, authorization } }),
  invoicePdf: (id: string) => apiBlob(`/sales/${id}/invoice.pdf`),
  email: (id: string, email: string) => api<void>("POST", `/sales/${id}/email`, { body: { email } }),
  store: () => api<StoreSettings>("GET", "/settings/store"),
  setPin: (currentPassword: string, pin: string) => api<void>("POST", "/auth/pin", { body: { currentPassword, pin } }),
};
