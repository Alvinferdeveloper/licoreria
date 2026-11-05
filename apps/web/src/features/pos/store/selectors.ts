"use client";

import { useMemo } from "react";
import { computeTotals, summarizeTender } from "../domain";
import { POS_EXCHANGE_RATE, usePosStore } from "./pos-store";

export function useCartTotals() {
  const lines = usePosStore((state) => state.lines);
  return useMemo(() => computeTotals(lines), [lines]);
}

export function useTenderSummary() {
  const { total } = useCartTotals();
  const payments = usePosStore((state) => state.payments);
  return useMemo(() => summarizeTender(total, payments, POS_EXCHANGE_RATE), [total, payments]);
}
