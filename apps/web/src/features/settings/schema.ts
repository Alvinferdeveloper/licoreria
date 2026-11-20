import { z } from "zod";
import type { PrintFormat, Settings, SettingsInput } from "./api";
import { settingsCopy } from "./copy";

const v = settingsCopy.validation;

const percent = z.string().trim().regex(/^\d{1,3}(\.\d{1,2})?$/, v.percent).refine((s) => Number(s) <= 100, v.percent);

const day = z
  .object({ closed: z.boolean(), open: z.string(), close: z.string() })
  .refine((d) => d.closed || (d.open !== "" && d.close !== "" && d.open !== d.close), { message: v.day, path: ["close"] });

const schedule = z
  .object({
    enabled: z.boolean(),
    tolerance: z.string().trim().regex(/^\d{1,3}$/, v.tolerance).refine((s) => Number(s) <= 120, v.tolerance),
    days: z.array(day).length(7),
  })
  .refine((s) => !s.enabled || s.days.some((d) => !d.closed), { message: v.noOpenDay, path: ["days"] });

export const settingsSchema = z.object({
  name: z.string().trim().min(1, v.name).max(120),
  taxId: z.string().trim().max(30),
  address: z.string().trim().max(200),
  phone: z.string().trim().max(40),
  email: z.string().trim().max(120).refine((s) => s === "" || z.email().safeParse(s).success, v.email),
  taxPercent: percent,
  discountLimitPercent: percent,
  defaultPrintFormat: z.enum(["ticket", "invoice"]),
  footerNote: z.string().trim().max(160),
  schedule,
});

export type SettingsValues = z.infer<typeof settingsSchema>;

export function toValues(s: Settings): SettingsValues {
  return {
    name: s.business.name,
    taxId: s.business.taxId,
    address: s.business.address,
    phone: s.business.phone,
    email: s.business.email,
    taxPercent: String(s.tax.defaultPercent),
    discountLimitPercent: String(s.pos.discountLimitPercent),
    defaultPrintFormat: s.pos.defaultPrintFormat === "invoice" ? "invoice" : "ticket",
    footerNote: s.invoice.footerNote,
    schedule: {
      enabled: s.schedule.enabled,
      tolerance: String(s.schedule.toleranceMinutes),
      days: s.schedule.days.map((d) => ({ closed: d.closed, open: d.open, close: d.close })),
    },
  };
}

export function toInput(v: SettingsValues): SettingsInput {
  return {
    business: { name: v.name, taxId: v.taxId, address: v.address, phone: v.phone, email: v.email },
    tax: { defaultPercent: Number(v.taxPercent) },
    pos: { discountLimitPercent: Number(v.discountLimitPercent), defaultPrintFormat: v.defaultPrintFormat as PrintFormat },
    invoice: { footerNote: v.footerNote },
    schedule: { enabled: v.schedule.enabled, toleranceMinutes: Number(v.schedule.tolerance), days: v.schedule.days },
  };
}
