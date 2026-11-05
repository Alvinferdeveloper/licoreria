"use client";

import { ArrowLeft, CheckCircle, X } from "@phosphor-icons/react";
import { useState, type FormEvent, type ReactNode } from "react";
import { Money } from "@/components/money";
import { Button } from "@/components/ui/button";
import { Kbd } from "@/components/ui/kbd";
import { SegmentedControl } from "@/components/ui/segmented-control";
import { cn } from "@/lib/cn";
import { CURRENCIES, formatRate, nioToUsd, parseAmount, toCents, type CurrencyCode } from "@/lib/money";
import { posCopy } from "../copy";
import { amountDue, BILL_SHORTCUTS, paymentInBase, type PaymentMethod } from "../domain";
import { POS_EXCHANGE_RATE, usePosStore } from "../store/pos-store";
import { useTenderSummary } from "../store/selectors";

const CURRENCY_OPTIONS = [
  { value: "NIO", label: "C$ Córdobas" },
  { value: "USD", label: "US$ Dólares" },
] as const;

const METHOD_OPTIONS = (Object.keys(posCopy.tender.methods) as PaymentMethod[]).map((method) => ({
  value: method,
  label: posCopy.tender.methods[method],
}));

const TENDER_INPUT_ID = "tender-amount";

export function TenderView() {
  const payments = usePosStore((state) => state.payments);
  const addPayment = usePosStore((state) => state.addPayment);
  const removePayment = usePosStore((state) => state.removePayment);
  const backToCart = usePosStore((state) => state.backToCart);
  const confirmSale = usePosStore((state) => state.confirmSale);
  const tender = useTenderSummary();

  const [currency, setCurrency] = useState<CurrencyCode>("NIO");
  const [method, setMethod] = useState<PaymentMethod>("cash");
  const [amountInput, setAmountInput] = useState("");

  const due = amountDue(tender.remaining, currency, POS_EXCHANGE_RATE);
  const symbol = CURRENCIES[currency].symbol;

  const pay = (amount: number) => {
    addPayment({ method, currency, amount });
    setAmountInput("");
    document.getElementById(TENDER_INPUT_ID)?.focus();
  };

  const onSubmit = (event: FormEvent) => {
    event.preventDefault();
    const amount = parseAmount(amountInput);
    if (amount) return pay(amount);
    if (!amountInput.trim() && tender.isCovered) confirmSale();
  };

  return (
    <>
      <div className="flex items-center gap-2 border-b border-line px-3 py-2">
        <Button variant="ghost" size="sm" onClick={backToCart}>
          <ArrowLeft size={16} aria-hidden />
          {posCopy.tender.back}
          <Kbd>{posCopy.shortcuts.back}</Kbd>
        </Button>
      </div>

      <div className="min-h-0 flex-1 space-y-5 overflow-y-auto px-5 py-4">
        <div aria-live="polite">
          {tender.isCovered ? (
            <>
              <p className="flex items-center gap-1.5 text-label text-success">
                <CheckCircle size={16} weight="fill" aria-hidden />
                {posCopy.tender.change}
              </p>
              <Money cents={tender.change} className="block text-total font-semibold tracking-display" />
            </>
          ) : (
            <>
              <p className="text-label text-ink-muted">
                {payments.length ? posCopy.tender.missing : posCopy.tender.due}
              </p>
              <Money
                cents={tender.remaining}
                className={cn("block text-total font-semibold tracking-display", payments.length > 0 && "text-danger")}
              />
              <Money
                cents={nioToUsd(tender.remaining, POS_EXCHANGE_RATE)}
                currency="USD"
                className="mt-1 block text-title text-ink-muted"
              />
            </>
          )}
          <p className="mt-2 text-caption text-ink-subtle">
            {posCopy.tender.rateUsed(formatRate(POS_EXCHANGE_RATE))}
          </p>
        </div>

        {!tender.isCovered && (
          <form onSubmit={onSubmit} className="space-y-3">
            <SegmentedControl label={posCopy.tender.currency} value={currency} options={CURRENCY_OPTIONS} onChange={setCurrency} />
            <SegmentedControl label={posCopy.tender.method} value={method} options={METHOD_OPTIONS} onChange={setMethod} />

            <div className="space-y-1.5">
              <label htmlFor={TENDER_INPUT_ID} className="text-label font-medium">
                {posCopy.tender.amountLabel}
              </label>
              <div className="relative">
                <span className="pointer-events-none absolute top-1/2 left-4 -translate-y-1/2 font-mono text-ink-subtle">
                  {symbol}
                </span>
                <input
                  id={TENDER_INPUT_ID}
                  inputMode="decimal"
                  autoComplete="off"
                  autoFocus
                  value={amountInput}
                  onChange={(event) => setAmountInput(event.target.value)}
                  placeholder={(due / 100).toFixed(2)}
                  className="tabular h-13 w-full rounded-lg border border-field bg-surface pr-4 pl-14 text-right font-mono text-title text-ink placeholder:text-ink-subtle focus:border-amber focus:shadow-[0_0_0_3px_color-mix(in_oklch,var(--amber)_25%,transparent)] focus:outline-none"
                />
              </div>
              {method !== "cash" && <p className="text-caption text-ink-muted">{posCopy.tender.nonCashCapped}</p>}
            </div>

            <div className="grid grid-cols-3 gap-2">
              {method === "cash" &&
                BILL_SHORTCUTS[currency].map((bill) => (
                  <TenderKey key={bill} onClick={() => pay(toCents(bill))}>
                    {symbol} {bill.toLocaleString("es-NI")}
                  </TenderKey>
                ))}
              <TenderKey emphasis onClick={() => pay(due)}>
                {posCopy.tender.exact}
              </TenderKey>
            </div>
          </form>
        )}

        {payments.length > 0 && (
          <section className="space-y-2">
            <h3 className="text-caption font-medium text-ink-muted">{posCopy.tender.payments}</h3>
            <ul className="divide-y divide-line rounded-lg border border-line">
              {payments.map((payment) => (
                <li key={payment.id} className="flex items-center gap-3 py-2 pr-2 pl-4">
                  <span className="flex-1 text-label">
                    {posCopy.tender.methods[payment.method]} · {CURRENCIES[payment.currency].symbol}
                  </span>
                  <span className="text-right">
                    <Money cents={payment.amount} currency={payment.currency} className="block font-mono font-medium" />
                    {payment.currency === "USD" && (
                      <Money cents={paymentInBase(payment, POS_EXCHANGE_RATE)} className="block font-mono text-caption text-ink-muted" />
                    )}
                  </span>
                  <Button variant="ghost" size="icon-sm" aria-label={posCopy.tender.remove} onClick={() => removePayment(payment.id)}>
                    <X size={14} />
                  </Button>
                </li>
              ))}
            </ul>
          </section>
        )}
      </div>

      <footer className="border-t border-line px-5 pt-4 pb-5">
        <Button variant="primary" size="xl" className="w-full justify-between" disabled={!tender.isCovered} onClick={confirmSale}>
          {posCopy.tender.confirm}
          <Kbd className="bg-on-accent/15 text-on-accent">{posCopy.shortcuts.confirm}</Kbd>
        </Button>
      </footer>
    </>
  );
}

function TenderKey({ emphasis, ...props }: { emphasis?: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      className={cn(
        "press h-14 rounded-lg border font-mono text-body font-medium shadow-(--shadow-edge) transition-colors duration-150",
        emphasis
          ? "border-amber/40 bg-amber-tint text-amber-ink hover:border-amber"
          : "border-line-strong bg-surface text-ink hover:bg-chrome",
      )}
      {...props}
    />
  );
}
