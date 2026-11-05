import { api } from "@/lib/api/client";

export type PrintFormat = "ticket" | "invoice";

export interface Settings {
  business: { name: string; taxId: string; address: string; phone: string; email: string; logoUrl: string | null };
  tax: { defaultPercent: number };
  pos: { discountLimitPercent: number; defaultPrintFormat: PrintFormat };
  invoice: { footerNote: string };
}

export interface SettingsInput {
  business: Omit<Settings["business"], "logoUrl">;
  tax: Settings["tax"];
  pos: Settings["pos"];
  invoice: Settings["invoice"];
}

export const settingsKeys = { all: ["settings"] as const, detail: ["settings", "detail"] as const };

export const settingsApi = {
  get: () => api<Settings>("GET", "/settings"),
  update: (input: SettingsInput) => api<Settings>("PUT", "/settings", { body: input }),
  uploadLogo: (file: File) => {
    const form = new FormData();
    form.append("logo", file);
    return api<Settings>("POST", "/settings/logo", { form });
  },
  removeLogo: () => api<Settings>("DELETE", "/settings/logo"),
};
