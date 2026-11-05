export const CURRENCIES = {
  NIO: { code: "NIO", symbol: "C$", name: "Córdobas" },
  USD: { code: "USD", symbol: "US$", name: "Dólares" },
} as const;

export type CurrencyCode = keyof typeof CURRENCIES;

export const BASE_CURRENCY: CurrencyCode = "NIO";
