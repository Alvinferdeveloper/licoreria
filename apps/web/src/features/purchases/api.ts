import { api, type Page } from "@/lib/api/client";

export type PurchaseStatus = "draft" | "received" | "voided";

export interface Ref {
  id: string;
  name: string;
}

export interface PurchaseItem {
  id: string;
  product: { id: string; sku: string; name: string };
  quantity: number;
  unitCost: string;
  subtotal: string;
  taxPercent: string;
  /** Unit cost of the product's previous received purchase, to compare prices. */
  previousCost: string | null;
  expiresOn: string | null;
}

export interface Purchase {
  id: string;
  supplier: Ref;
  supplierInvoiceNumber: string;
  status: PurchaseStatus;
  purchaseDate: string;
  subtotal: string;
  tax: string;
  total: string;
  notes: string;
  itemCount: number;
  createdBy: Ref;
  receivedBy: Ref | null;
  receivedAt: string | null;
  voidReason: string;
  voidedAt: string | null;
  createdAt: string;
  items?: PurchaseItem[];
}

export interface PurchaseInput {
  supplierId: string;
  supplierInvoiceNumber: string;
  purchaseDate: string;
  notes: string;
  items: { productId: string; quantity: number; unitCost: string; expiresOn?: string | null }[];
}

export interface PurchaseFilters {
  search: string;
  status: "all" | PurchaseStatus;
  page: number;
}

export const purchaseKeys = {
  all: ["purchases"] as const,
  list: (f: PurchaseFilters) => ["purchases", "list", f] as const,
  detail: (id: string) => ["purchases", "detail", id] as const,
  suppliers: ["suppliers", "options"] as const,
};

export const purchasesApi = {
  list: (f: PurchaseFilters) => {
    const params = new URLSearchParams({ page: String(f.page), pageSize: "20" });
    if (f.search.trim()) params.set("search", f.search.trim());
    if (f.status !== "all") params.set("status", f.status);
    return api<Page<Purchase>>("GET", `/purchases?${params}`);
  },
  get: (id: string) => api<Purchase>("GET", `/purchases/${id}`),
  create: (input: PurchaseInput) => api<Purchase>("POST", "/purchases", { body: input }),
  update: (id: string, input: PurchaseInput) => api<Purchase>("PUT", `/purchases/${id}`, { body: input }),
  receive: (id: string) => api<Purchase>("POST", `/purchases/${id}/receive`),
  void: (id: string, reason: string) => api<Purchase>("POST", `/purchases/${id}/void`, { body: { reason } }),
  suppliers: () => api<Page<Ref>>("GET", "/suppliers?active=true&pageSize=100").then((r) => r.data),
};
