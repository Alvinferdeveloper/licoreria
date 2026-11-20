"use client";

import { CheckCircle, CloudSlash, Printer } from "@phosphor-icons/react";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { salesApi, salesKeys } from "@/features/sales/api";
import { InvoiceActions } from "@/features/sales/components/invoice-actions";
import { Money } from "@/components/money";
import { Button } from "@/components/ui/button";
import { Kbd } from "@/components/ui/kbd";
import { posCopy } from "../copy";
import type { CompletedSale } from "../store/pos-store";
import { ProvisionalTicket } from "./provisional-ticket";

interface SaleCompleteViewProps {
  sale: CompletedSale;
  onNewSale: () => void;
}

const tone = (t: "success" | "danger") => (t === "success" ? "text-caption text-success" : "text-caption text-danger");

export function SaleCompleteView({ sale, onNewSale }: SaleCompleteViewProps) {
  const store = useQuery({ queryKey: salesKeys.store, queryFn: salesApi.store, staleTime: 5 * 60_000 });
  const [message, setMessage] = useState<{ text: string; tone: "success" | "danger" } | null>(null);
  const offline = sale.offline;
  return (
    <>
      <div className="flex flex-1 flex-col justify-center gap-8 px-6" aria-live="assertive">
        <div className="space-y-1">
          {offline ? (
            <p className="flex items-center gap-2 text-title font-semibold text-amber-ink">
              <CloudSlash size={24} weight="fill" aria-hidden />
              {posCopy.offline.complete.title}
            </p>
          ) : (
            <p className="flex items-center gap-2 text-title font-semibold text-success">
              <CheckCircle size={24} weight="fill" aria-hidden />
              {posCopy.complete.title}
            </p>
          )}
          <p className="text-label text-ink-muted">
            {offline ? posCopy.offline.complete.receipt : posCopy.complete.invoice}{" "}
            <span className="font-mono tracking-wide text-ink">{sale.invoice}</span>
            {" · "}
            {posCopy.bill.items(sale.itemCount)}
          </p>
          {offline && <p className="text-caption text-ink-muted">{posCopy.offline.complete.body}</p>}
        </div>

        <div>
          <p className="text-label text-ink-muted">
            {sale.change > 0 ? posCopy.complete.changeToGive : posCopy.complete.noChange}
          </p>
          <Money cents={sale.change} className="block text-total font-semibold tracking-display" />
        </div>

        <div className="space-y-2">
          {offline ? (
            <>
              <Button variant="primary" onClick={() => window.print()}>
                <Printer size={16} aria-hidden />
                {posCopy.offline.complete.print}
              </Button>
              <ProvisionalTicket receipt={offline} store={store.data} />
            </>
          ) : (
            <InvoiceActions
              saleId={sale.id}
              defaultFormat={store.data?.pos.defaultPrintFormat}
              customerEmail={sale.customerEmail}
              onMessage={(text, tone) => setMessage({ text, tone })}
            />
          )}
          {message && (
            <p role="status" className={tone(message.tone)}>
              {message.text}
            </p>
          )}
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
