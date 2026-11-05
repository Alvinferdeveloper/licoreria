import { api, ApiError, type Page } from "@/lib/api/client";

export interface Ref {
  id: string;
  name: string;
}

export interface Register {
  id: string;
  name: string;
  invoiceSeries: string;
  nextInvoiceNumber: number;
  active: boolean;
  openShift: { id: string; user: Ref; openedAt: string } | null;
}

export interface ShiftSummary {
  salesCount: number;
  cashSalesNio: string;
  cashSalesUsd: string;
  changeNio: string;
  changeUsd: string;
  cardNio: string;
  transferNio: string;
  incomeNio: string;
  incomeUsd: string;
  expenseNio: string;
  expenseUsd: string;
  expectedNio: string;
  expectedUsd: string;
}

export type Currency = "NIO" | "USD";

export interface CashMovement {
  id: string;
  type: "income" | "expense";
  currency: Currency;
  amount: string;
  reason: string;
  user: Ref;
  createdAt: string;
}

export interface Shift {
  id: string;
  register: Ref;
  user: Ref;
  status: "open" | "closed";
  openedAt: string;
  closedAt: string | null;
  openingNio: string;
  openingUsd: string;
  expectedNio: string | null;
  expectedUsd: string | null;
  countedNio: string | null;
  countedUsd: string | null;
  differenceNio: string | null;
  differenceUsd: string | null;
  notes: string;
  summary?: ShiftSummary;
  movements?: CashMovement[];
}

export interface Rate {
  date: string;
  rate: string;
  createdBy: Ref;
  createdAt: string;
}

export interface RegisterInput {
  name: string;
  invoiceSeries: string;
  active: boolean;
}

export const cashKeys = {
  current: ["cash", "current"] as const,
  registers: ["cash", "registers"] as const,
  shifts: (page: number) => ["cash", "shifts", page] as const,
  shift: (id: string) => ["cash", "shift", id] as const,
  rateToday: ["rates", "today"] as const,
  rates: (page: number) => ["rates", "list", page] as const,
};

export const cashApi = {
  /** Resolves to null when the user has no open shift. */
  current: () => api<Shift>("GET", "/cash-shifts/current").catch(nullOn("no_open_shift")),
  registers: () => api<{ data: Register[] }>("GET", "/registers").then((r) => r.data),
  createRegister: (input: RegisterInput) => api<{ data: Register[] }>("POST", "/registers", { body: input }),
  updateRegister: (id: string, input: RegisterInput) => api<{ data: Register[] }>("PUT", `/registers/${id}`, { body: input }),
  shifts: (page: number) => api<Page<Shift>>("GET", `/cash-shifts?page=${page}&pageSize=20`),
  shift: (id: string) => api<Shift>("GET", `/cash-shifts/${id}`),
  open: (body: { registerId: string; openingNio: string; openingUsd: string }) => api<Shift>("POST", "/cash-shifts", { body }),
  addMovement: (id: string, body: { type: CashMovement["type"]; currency: Currency; amount: string; reason: string }) =>
    api<Shift>("POST", `/cash-shifts/${id}/movements`, { body }),
  close: (id: string, body: { countedNio: string; countedUsd: string; notes: string }) => api<Shift>("POST", `/cash-shifts/${id}/close`, { body }),

  /** Resolves to null when today's rate is not registered yet. */
  rateToday: () => api<Rate>("GET", "/exchange-rates/today").catch(nullOn("exchange_rate_missing")),
  rates: (page: number) => api<Page<Rate>>("GET", `/exchange-rates?page=${page}&pageSize=15`),
  setRate: (body: { rate: string; date?: string }) => api<Rate>("POST", "/exchange-rates", { body }),
};

function nullOn(code: string) {
  return (error: unknown) => {
    if (error instanceof ApiError && error.code === code) return null;
    throw error;
  };
}
