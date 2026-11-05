import { api, type Page } from "@/lib/api/client";

export type MovementType = "purchase" | "sale" | "sale_void" | "customer_return" | "supplier_return" | "count_adjustment" | "shrinkage";

export interface Movement {
  id: string;
  product: { id: string; sku: string; name: string };
  type: MovementType;
  quantity: number;
  stockAfter: number;
  /** Present only for roles allowed to see costs. */
  unitCost?: string;
  reason: string;
  sourceType: string | null;
  sourceId: string | null;
  user: { id: string; name: string };
  createdAt: string;
}

export interface MovementFilters {
  productId: string;
  type: "all" | MovementType;
  from: string;
  to: string;
  page: number;
}

export interface StockAlert {
  product: { id: string; sku: string; name: string };
  category: string;
  stock: number;
  minStock: number;
  reorderPoint: number;
  level: "out" | "min" | "reorder";
  supplier: { id: string; name: string } | null;
  lastCost?: string | null;
}

export interface AdjustmentInput {
  productId: string;
  kind: "count" | "shrinkage";
  quantity: number;
  reason: string;
}

export const inventoryKeys = {
  all: ["inventory"] as const,
  movements: (f: MovementFilters) => ["inventory", "movements", f] as const,
  alerts: ["inventory", "alerts"] as const,
};

export const inventoryApi = {
  movements: (f: MovementFilters) => {
    const params = new URLSearchParams({ page: String(f.page), pageSize: "25" });
    if (f.productId) params.set("productId", f.productId);
    if (f.type !== "all") params.set("type", f.type);
    if (f.from) params.set("from", f.from);
    if (f.to) params.set("to", f.to);
    return api<Page<Movement>>("GET", `/inventory/movements?${params}`);
  },
  alerts: () => api<{ data: StockAlert[] }>("GET", "/inventory/alerts").then((r) => r.data),
  adjust: (input: AdjustmentInput) => api<{ stock: number }>("POST", "/inventory/adjustments", { body: input }),
};
