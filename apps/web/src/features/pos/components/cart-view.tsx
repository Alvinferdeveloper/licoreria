"use client";

import { ShoppingBagOpen, Trash, UserCircle } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Kbd } from "@/components/ui/kbd";
import { posCopy } from "../copy";
import { fitsInStock } from "../domain";
import { focusSearch } from "../hooks/search-focus";
import { DEMO_SESSION } from "../data/session";
import { POS_EXCHANGE_RATE, usePosStore } from "../store/pos-store";
import { useCartTotals } from "../store/selectors";
import { BillTotals } from "./bill-totals";
import { CartLineItem } from "./cart-line-item";

export function CartView() {
  const lines = usePosStore((state) => state.lines);
  const products = usePosStore((state) => state.products);
  const setNotice = usePosStore((state) => state.setNotice);
  const flash = usePosStore((state) => state.flash);
  const ageVerified = usePosStore((state) => state.ageVerified);
  const changeQuantity = usePosStore((state) => state.changeQuantity);
  const clearCart = usePosStore((state) => state.clearCart);
  const setAgeVerified = usePosStore((state) => state.setAgeVerified);
  const startCheckout = usePosStore((state) => state.startCheckout);
  const totals = useCartTotals();

  const isEmpty = lines.length === 0;

  return (
    <>
      <div className="flex items-center justify-between gap-3 border-b border-line px-5 py-3">
        <p className="flex items-center gap-2 text-label text-ink">
          <UserCircle size={20} className="text-ink-muted" aria-hidden />
          {posCopy.bill.customer}
        </p>
        {!isEmpty && (
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              clearCart();
              focusSearch();
            }}
          >
            <Trash size={16} aria-hidden />
            {posCopy.bill.clear}
          </Button>
        )}
      </div>

      {isEmpty ? (
        <div className="grid flex-1 place-content-center justify-items-center gap-2 px-8 text-center">
          <ShoppingBagOpen size={32} className="text-ink-subtle" aria-hidden />
          <p className="text-title font-semibold">{posCopy.bill.emptyTitle}</p>
          <p className="text-ink-muted">{posCopy.bill.emptyBody}</p>
        </div>
      ) : (
        <ul className="min-h-0 flex-1 divide-y divide-line overflow-y-auto" aria-label={posCopy.bill.title}>
          {lines.map((line) => (
            <CartLineItem
              key={line.key}
              line={line}
              canIncrease={!products[line.productId] || fitsInStock(products[line.productId], lines, line.unitsPerItem)}
              flashNonce={flash?.key === line.key ? flash.nonce : null}
              onQuantityChange={(quantity) => {
                if (!changeQuantity(line.key, quantity)) setNotice(posCopy.search.noMoreStock(line.name));
              }}
            />
          ))}
        </ul>
      )}

      <footer className="space-y-4 border-t border-line px-5 pt-4 pb-5">
        <BillTotals totals={totals} taxPercent={DEMO_SESSION.taxPercent} rate={POS_EXCHANGE_RATE} />
        <Checkbox
          label={posCopy.bill.ageCheck}
          checked={ageVerified}
          disabled={isEmpty}
          onChange={(event) => setAgeVerified(event.target.checked)}
        />
        <Button
          variant="primary"
          size="xl"
          className="w-full justify-between"
          disabled={isEmpty || !ageVerified}
          onClick={startCheckout}
          aria-describedby={!ageVerified && !isEmpty ? "age-hint" : undefined}
        >
          <span>{posCopy.bill.charge}</span>
          <span className="flex items-center gap-3 text-label font-medium opacity-80">
            {posCopy.bill.items(totals.itemCount)}
            <Kbd className="bg-on-accent/15 text-on-accent">{posCopy.shortcuts.charge}</Kbd>
          </span>
        </Button>
        {!ageVerified && !isEmpty && (
          <p id="age-hint" className="text-caption text-ink-muted">
            {posCopy.bill.ageRequired}
          </p>
        )}
      </footer>
    </>
  );
}

