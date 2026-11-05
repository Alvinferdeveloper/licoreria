import { describe, expect, it } from "vitest";
import { formatMoney, parseAmount, usdToNio } from "./money";

describe("money", () => {
  it("formats amounts with currency symbol and es-NI separators", () => {
    expect(formatMoney(125050)).toBe("C$ 1,250.50");
    expect(formatMoney(3415, "USD")).toBe("US$ 34.15");
    expect(formatMoney(-500)).toBe("-C$ 5.00");
  });

  it("parses cashier input", () => {
    expect(parseAmount("1,250.50")).toBe(125050);
    expect(parseAmount(" 500 ")).toBe(50000);
    expect(parseAmount("12.5")).toBe(1250);
    expect(parseAmount("12.555")).toBeNull();
    expect(parseAmount("abc")).toBeNull();
    expect(parseAmount("")).toBeNull();
  });

  it("converts dollars to córdobas with the day's rate", () => {
    expect(usdToNio(3000, 36.6)).toBe(109800);
  });
});
