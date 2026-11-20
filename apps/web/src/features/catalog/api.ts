import { api, apiBlob, type FieldError, type Page } from "@/lib/api/client";

export interface Ref {
  id: string;
  name: string;
}

export interface Presentation {
  id: string;
  name: string;
  factor: number;
  barcode: string | null;
  price: string;
  active: boolean;
}

export interface Product {
  id: string;
  sku: string;
  barcode: string | null;
  name: string;
  description: string;
  category: Ref;
  brand: Ref | null;
  unit: Ref & { abbreviation: string };
  alcoholPercent: string | null;
  volumeMl: number | null;
  imageUrl: string | null;
  price: string;
  /** Present only for roles allowed to see costs. */
  averageCost?: string;
  taxPercent: string;
  stock: number;
  minStock: number;
  reorderPoint: number;
  allowNegativeStock: boolean;
  active: boolean;
  presentations: Presentation[];
}

export interface PresentationInput {
  id?: string;
  name: string;
  factor: number;
  barcode: string | null;
  price: string;
}

export interface ProductInput {
  sku: string;
  barcode: string | null;
  name: string;
  description: string;
  categoryId: string;
  brandId: string | null;
  unitId: string;
  alcoholPercent: string | null;
  volumeMl: number | null;
  price: string;
  averageCost?: string;
  taxPercent: string;
  minStock: number;
  reorderPoint: number;
  allowNegativeStock: boolean;
  presentations: PresentationInput[];
}

export interface ProductFilters {
  search: string;
  categoryId: string;
  active: "all" | "true" | "false";
  lowStock: boolean;
  page: number;
  pageSize?: number;
}

export interface Master {
  id: string;
  name: string;
  abbreviation?: string;
  parentId?: string;
  active: boolean;
  productCount: number;
}

export interface ImportRow {
  line: number;
  sku: string;
  name: string;
  category: string;
  brand: string;
  price: string;
  newBrand: boolean;
  /** Empty when the row can be imported; `field` is the file's column name. */
  errors: FieldError[];
}

export interface ImportReport {
  total: number;
  valid: number;
  invalid: number;
  created: number;
  newBrands: string[];
  rows: ImportRow[];
}

export type MasterKind ="categories" | "brands" | "units";

export const catalogKeys = {
  products: ["products"] as const,
  productList: (f: ProductFilters) => ["products", "list", f] as const,
  masters: (kind: MasterKind) => ["masters", kind] as const,
};

export const catalogApi = {
  products: (f: ProductFilters) => {
    const params = new URLSearchParams({ page: String(f.page), pageSize: String(f.pageSize ?? 20) });
    if (f.search.trim()) params.set("search", f.search.trim());
    if (f.categoryId) params.set("categoryId", f.categoryId);
    if (f.active !== "all") params.set("active", f.active);
    if (f.lowStock) params.set("lowStock", "true");
    return api<Page<Product>>("GET", `/products?${params}`, { timeoutMs: 8_000 });
  },
  lookup: (code: string) =>
    api<{ product: Product; presentationId: string | null }>("GET", `/products/lookup?code=${encodeURIComponent(code)}`, { timeoutMs: 8_000 }),
  createProduct: (input: ProductInput) => api<Product>("POST", "/products", { body: input }),
  updateProduct: (id: string, input: ProductInput) => api<Product>("PUT", `/products/${id}`, { body: input }),
  setProductActive: (id: string, active: boolean) => api<Product>("PATCH", `/products/${id}/status`, { body: { active } }),
  uploadImage: (id: string, file: File) => {
    const form = new FormData();
    form.set("image", file);
    return api<Product>("POST", `/products/${id}/image`, { form });
  },
  removeImage: (id: string) => api<Product>("DELETE", `/products/${id}/image`),
  importProducts: (file: File, dryRun: boolean) => {
    const form = new FormData();
    form.set("file", file);
    return api<ImportReport>("POST", `/products/import${dryRun ? "?dryRun=true" : ""}`, { form });
  },
  importTemplate: () => apiBlob("/products/import/template"),

  masters: (kind: MasterKind) => api<{ data: Master[] }>("GET", `/${kind}`).then((r) => r.data),
  createMaster: (kind: MasterKind, body: { name: string; abbreviation?: string; parentId?: string | null }) =>
    api<Master>("POST", `/${kind}`, { body }),
  updateMaster: (kind: MasterKind, id: string, body: { name: string; abbreviation?: string; parentId?: string | null }) =>
    api<Master>("PUT", `/${kind}/${id}`, { body }),
  setMasterActive: (kind: MasterKind, id: string, active: boolean) =>
    api<Master>("PATCH", `/${kind}/${id}/status`, { body: { active } }),
};

export { amountToCents } from "@/lib/money";
