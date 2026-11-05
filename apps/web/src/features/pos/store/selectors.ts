"use client";

import { useMemo } from "react";
import { computeTotals, summarizeTender } from "../domain";
import { usePosStore } from "./pos-store";

export function useCartTotals() {
  const lines = usePosStore((state) => state.lines);
  const discount = usePosStore((state) => state.discount);
  return useMemo(() => computeTotals(lines, discount), [lines, discount]);
}

export function useTenderSummary() {
  const { total } = useCartTotals();
  const payments = usePosStore((state) => state.payments);
  const rate = usePosStore((state) => state.rate);
  return useMemo(() => summarizeTender(total, payments, rate), [total, payments, rate]);
}
