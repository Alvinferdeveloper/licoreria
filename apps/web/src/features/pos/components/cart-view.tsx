"use client";

import { ClockCounterClockwise, Pause, ShoppingBagOpen, Tag, Trash, UserCircle } from "@phosphor-icons/react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Kbd } from "@/components/ui/kbd";
import { formatRelative } from "@/lib/format/date";
import { formatMoney } from "@/lib/money";
import { posCopy } from "../copy";
import { fitsInStock } from "../domain";
import { focusSearch } from "../hooks/search-focus";
import { usePosStore } from "../store/pos-store";
import { useCartTotals } from "../store/selectors";
import { BillTotals } from "./bill-totals";
import { CartLineItem } from "./cart-line-item";
import { CustomerSheet } from "./customer-sheet";
import { DiscountSheet } from "./discount-sheet";

export function CartView() {
  const lines = usePosStore((state) => state.lines);
  const products = usePosStore((state) => state.products);
  const discount = usePosStore((state) => state.discount);
  const held = usePosStore((state) => state.held);
  const rate = usePosStore((state) => state.rate);
  const setNotice = usePosStore((state) => state.setNotice);
  const flash = usePosStore((state) => state.flash);
  const ageVerified = usePosStore((state) => state.ageVerified);
  const changeQuantity = usePosStore((state) => state.changeQuantity);
  const clearCart = usePosStore((state) => state.clearCart);
  const setDiscount = usePosStore((state) => state.setDiscount);
  const setAgeVerified = usePosStore((state) => state.setAgeVerified);
  const startCheckout = usePosStore((state) => state.startCheckout);
  const holdSale = usePosStore((state) => state.holdSale);
  const resumeSale = usePosStore((state) => state.resumeSale);
  const discardHeld = usePosStore((state) => state.discardHeld);
  const totals = useCartTotals();
  const [discountOpen, setDiscountOpen] = useState(false);
  const [heldOpen, setHeldOpen] = useState(false);
  const [customerOpen, setCustomerOpen] = useState(false);
  const customer = usePosStore((state) => state.customer);
  const setCustomer = usePosStore((state) => state.setCustomer);

  const isEmpty = lines.length === 0;
  const discountLabel = discount ? posCopy.discount.summary(discount.kind === "percent" ? `${discount.value} %` : formatMoney(discount.value)) : undefined;

  return (
    <>
      <div className="flex items-center justify-between gap-2 border-b border-line px-5 py-3">
        <button
          type="button"
          className="flex min-w-0 items-center gap-2 rounded-md px-1.5 py-1 text-left text-label text-ink hover:bg-sunken"
          title={posCopy.customer.change}
          onClick={() => setCustomerOpen(true)}
        >
          <UserCircle size={20} className="shrink-0 text-ink-muted" aria-hidden />
          <span className="min-w-0">
            <span className="block truncate">{customer?.name ?? posCopy.bill.customer}</span>
            {customer?.taxId && <span className="block font-mono text-caption text-ink-muted">{customer.taxId}</span>}
          </span>
        </button>
        <div className="flex items-center gap-1">
          {held.length > 0 && (
            <Button variant="ghost" size="sm" aria-expanded={heldOpen} onClick={() => setHeldOpen(!heldOpen)}>
              <ClockCounterClockwise size={16} aria-hidden />
              {posCopy.bill.held(held.length)}
            </Button>
          )}
          {!isEmpty && (
            <>
              <Button variant="ghost" size="sm" onClick={() => {
                  if (!holdSale()) return;
                  setNotice(posCopy.bill.heldNotice);
                  focusSearch();
                }} title={`${posCopy.bill.hold} (${posCopy.shortcuts.hold})`}>
                <Pause size={16} aria-hidden />
                <Kbd>{posCopy.shortcuts.hold}</Kbd>
              </Button>
              <Button variant="ghost" size="sm" onClick={() => { clearCart(); focusSearch(); }}>
                <Trash size={16} aria-hidden />
                {posCopy.bill.clear}
              </Button>
            </>
          )}
        </div>
      </div>

      {heldOpen && held.length > 0 && (
        <section className="space-y-1 border-b border-line bg-sunken px-5 py-3" aria-label={posCopy.bill.heldTitle}>
          <p className="text-caption font-medium text-ink-muted">{posCopy.bill.heldTitle}</p>
          <ul className="space-y-1">
            {held.map((h) => (
              <li key={h.id} className="flex items-center justify-between gap-2">
                <span className="text-label">{posCopy.bill.heldLine(h.lines.reduce((n, l) => n + l.quantity, 0), formatRelative(h.heldAt))}</span>
                <span className="flex gap-1">
                  <Button size="sm" onClick={() => { resumeSale(h.id); setHeldOpen(false); focusSearch(); }}>
                    {posCopy.bill.resume}
                  </Button>
                  <Button size="sm" variant="ghost" onClick={() => discardHeld(h.id)}>
                    {posCopy.bill.discard}
                  </Button>
                </span>
              </li>
            ))}
          </ul>
        </section>
      )}

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
        {!isEmpty && (
          <Button variant="ghost" size="sm" className="-ml-2" onClick={() => setDiscountOpen(true)}>
            <Tag size={16} aria-hidden />
            {discountLabel ?? posCopy.bill.addDiscount}
          </Button>
        )}
        <BillTotals totals={totals} rate={rate} discountLabel={discountLabel} />
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

      <CustomerSheet
        open={customerOpen}
        onClose={() => setCustomerOpen(false)}
        onPick={(c) => {
          setCustomer(c);
          setCustomerOpen(false);
          focusSearch();
        }}
      />
      <DiscountSheet
        open={discountOpen}
        current={discount}
        subtotal={totals.subtotal}
        onClose={() => setDiscountOpen(false)}
        onApply={(d) => {
          setDiscount(d);
          setDiscountOpen(false);
        }}
      />
    </>
  );
}
