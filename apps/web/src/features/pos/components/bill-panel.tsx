"use client";

import { focusSearch } from "../hooks/search-focus";
import { usePosStore } from "../store/pos-store";
import { CartView } from "./cart-view";
import { SaleCompleteView } from "./sale-complete-view";
import { TenderView } from "./tender-view";

export function BillPanel() {
  const mode = usePosStore((state) => state.mode);
  const lastSale = usePosStore((state) => state.lastSale);
  const startNewSale = usePosStore((state) => state.startNewSale);

  return (
    <aside className="flex min-h-0 flex-col border-l border-line bg-surface" aria-label="La cuenta">
      {mode === "cart" && <CartView />}
      {mode === "tender" && <TenderView />}
      {mode === "complete" && lastSale && (
        <SaleCompleteView
          sale={lastSale}
          onNewSale={() => {
            startNewSale();
            requestAnimationFrame(focusSearch);
          }}
        />
      )}
    </aside>
  );
}
