"use client";

import { useEffect } from "react";
import { useHotkeys } from "@/hooks/use-hotkeys";
import { DEMO_SESSION } from "../data/session";
import { focusSearch } from "../hooks/search-focus";
import { usePosStore } from "../store/pos-store";
import { useTenderSummary } from "../store/selectors";
import { BillPanel } from "./bill-panel";
import { PosHeader } from "./pos-header";
import { ProductSearch } from "./product-search";

export function PosScreen() {
  const mode = usePosStore((state) => state.mode);
  const startCheckout = usePosStore((state) => state.startCheckout);
  const backToCart = usePosStore((state) => state.backToCart);
  const confirmSale = usePosStore((state) => state.confirmSale);
  const startNewSale = usePosStore((state) => state.startNewSale);
  const { isCovered } = useTenderSummary();

  useEffect(() => {
    usePosStore.persist.rehydrate();
  }, []);

  useHotkeys({
    F2: () => {
      if (mode === "cart") focusSearch();
    },
    F4: () => {
      if (mode === "cart") startCheckout();
    },
    Escape: () => {
      if (mode !== "tender") return;
      backToCart();
      requestAnimationFrame(focusSearch);
    },
    ...(mode === "tender" && isCovered && { Enter: confirmSale }),
    ...(mode === "complete" && {
      Enter: () => {
        startNewSale();
        requestAnimationFrame(focusSearch);
      },
    }),
  });

  return (
    <div className="grid h-dvh grid-rows-[auto_minmax(0,1fr)]">
      <PosHeader session={DEMO_SESSION} />
      <main className="grid min-h-0 grid-cols-1 lg:grid-cols-[minmax(0,1fr)_400px]">
        <ProductSearch locked={mode !== "cart"} />
        <BillPanel />
      </main>
    </div>
  );
}
