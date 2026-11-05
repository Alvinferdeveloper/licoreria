"use client";

import { Money } from "@/components/money";
import { lineSubtotal, type CartLine } from "../domain";
import { QuantityStepper } from "./quantity-stepper";

interface CartLineItemProps {
  line: CartLine;
  canIncrease: boolean;
  flashNonce: number | null;
  onQuantityChange: (quantity: number) => void;
}

export function CartLineItem({ line, canIncrease, flashNonce, onQuantityChange }: CartLineItemProps) {
  return (
    <li className="relative isolate grid grid-cols-[minmax(0,1fr)_auto] gap-x-3 gap-y-2 px-5 py-3">
      {flashNonce !== null && (
        <span key={flashNonce} aria-hidden className="animate-line-flash absolute inset-0 -z-10 bg-amber-tint" />
      )}
      <div className="min-w-0">
        <p className="truncate font-medium">{line.name}</p>
        <p className="text-caption text-ink-muted">
          {line.detail} · <Money cents={line.unitPrice} className="font-mono" />
        </p>
      </div>
      <Money cents={lineSubtotal(line)} className="self-start text-right font-mono font-medium" />
      <div className="justify-self-start">
        <QuantityStepper value={line.quantity} canIncrease={canIncrease} onChange={onQuantityChange} />
      </div>
    </li>
  );
}
