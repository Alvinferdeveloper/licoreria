import { CURRENCIES, type CurrencyCode } from "./currency";

/** Integer amount in the currency's minor unit (centavos / cents). */
export type Cents = number;

export const toCents = (amount: number): Cents => Math.round(amount * 100);

export const fromCents = (cents: Cents): number => cents / 100;

export const sumCents = (values: readonly Cents[]): Cents =>
  values.reduce((total, value) => total + value, 0);

/** Applies a percentage (e.g. 15 for 15%) and rounds half away from zero. */
export const percentOf = (cents: Cents, percent: number): Cents =>
  Math.round((cents * percent) / 100);

/** Converts USD cents to NIO cents using a NIO-per-USD rate. */
export const usdToNio = (usdCents: Cents, rate: number): Cents =>
  Math.round(usdCents * rate);

/** Converts NIO cents to USD cents, rounding to the nearest cent. */
export const nioToUsd = (nioCents: Cents, rate: number): Cents =>
  Math.round(nioCents / rate);

export const toBaseCents = (cents: Cents, currency: CurrencyCode, rate: number): Cents =>
  currency === "USD" ? usdToNio(cents, rate) : cents;

const numberFormat = new Intl.NumberFormat("es-NI", {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

export function formatAmount(cents: Cents): string {
  return numberFormat.format(fromCents(cents));
}

export function formatMoney(cents: Cents, currency: CurrencyCode = "NIO"): string {
  const sign = cents < 0 ? "-" : "";
  return `${sign}${CURRENCIES[currency].symbol} ${formatAmount(Math.abs(cents))}`;
}

export function formatRate(rate: number): string {
  return new Intl.NumberFormat("es-NI", {
    minimumFractionDigits: 2,
    maximumFractionDigits: 4,
  }).format(rate);
}

/** Parses user input like "1,250.50" or "1250.5" into cents; null when invalid. */
export function parseAmount(input: string): Cents | null {
  const normalized = input.replace(/[\s,]/g, "");
  if (!/^\d+(\.\d{0,2})?$/.test(normalized)) return null;
  return toCents(Number(normalized));
}
