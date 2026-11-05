"use client";

import { CheckCircle } from "@phosphor-icons/react";
import { Money } from "@/components/money";
import { Button } from "@/components/ui/button";
import { Kbd } from "@/components/ui/kbd";
import { posCopy } from "../copy";
import type { CompletedSale } from "../store/pos-store";

interface SaleCompleteViewProps {
  sale: CompletedSale;
  onNewSale: () => void;
}

export function SaleCompleteView({ sale, onNewSale }: SaleCompleteViewProps) {
  return (
    <>
      <div className="flex flex-1 flex-col justify-center gap-8 px-6" aria-live="assertive">
        <div className="space-y-1">
          <p className="flex items-center gap-2 text-title font-semibold text-success">
            <CheckCircle size={24} weight="fill" aria-hidden />
            {posCopy.complete.title}
          </p>
          <p className="text-label text-ink-muted">
            {posCopy.complete.invoice} <span className="font-mono tracking-wide text-ink">{sale.invoice}</span>
            {" · "}
            {posCopy.bill.items(sale.itemCount)}
          </p>
        </div>

        <div>
          <p className="text-label text-ink-muted">
            {sale.change > 0 ? posCopy.complete.changeToGive : posCopy.complete.noChange}
          </p>
          <Money cents={sale.change} className="block text-total font-semibold tracking-display" />
        </div>

        <dl className="space-y-1.5 border-t border-line pt-4 text-ink-muted">
          <div className="flex justify-between">
            <dt>{posCopy.bill.total}</dt>
            <dd><Money cents={sale.total} className="font-mono" /></dd>
          </div>
          <div className="flex justify-between">
            <dt>{posCopy.tender.payments}</dt>
            <dd><Money cents={sale.paid} className="font-mono" /></dd>
          </div>
        </dl>
      </div>

      <footer className="border-t border-line px-5 pt-4 pb-5">
        <Button variant="primary" size="xl" className="w-full justify-between" onClick={onNewSale} autoFocus>
          {posCopy.complete.newSale}
          <Kbd className="bg-on-accent/15 text-on-accent">{posCopy.shortcuts.confirm}</Kbd>
        </Button>
      </footer>
    </>
  );
}
