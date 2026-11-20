import { api } from "@/lib/api/client";

export type PrintFormat = "ticket" | "invoice";

export interface ScheduleDay {
  closed: boolean;
  open: string;
  close: string;
}

/** Usage hours; days go from Sunday to Saturday. */
export interface Schedule {
  enabled: boolean;
  toleranceMinutes: number;
  days: ScheduleDay[];
}

export interface Settings {
  business: { name: string; taxId: string; address: string; phone: string; email: string; logoUrl: string | null };
  tax: { defaultPercent: number };
  pos: { discountLimitPercent: number; defaultPrintFormat: PrintFormat };
  invoice: { footerNote: string };
  schedule: Schedule;
}

export interface SettingsInput {
  business: Omit<Settings["business"], "logoUrl">;
  tax: Settings["tax"];
  pos: Settings["pos"];
  invoice: Settings["invoice"];
  schedule: Schedule;
}

export interface ScheduleExtension {
  id: string;
  userId: string | null;
  userName: string | null;
  endsAt: string;
  createdByName: string;
  createdAt: string;
}

export const settingsKeys = {
  all: ["settings"] as const,
  detail: ["settings", "detail"] as const,
  extensions: ["settings", "extensions"] as const,
};

export const settingsApi = {
  get: () => api<Settings>("GET", "/settings"),
  update: (input: SettingsInput) => api<Settings>("PUT", "/settings", { body: input }),
  uploadLogo: (file: File) => {
    const form = new FormData();
    form.append("logo", file);
    return api<Settings>("POST", "/settings/logo", { form });
  },
  removeLogo: () => api<Settings>("DELETE", "/settings/logo"),
  extensions: () => api<{ data: ScheduleExtension[] }>("GET", "/schedule/extensions").then((r) => r.data),
  extend: (input: { userId: string | null; until: string }) => api<ScheduleExtension>("POST", "/schedule/extensions", { body: input }),
  endExtension: (id: string) => api<ScheduleExtension>("DELETE", `/schedule/extensions/${id}`),
};
