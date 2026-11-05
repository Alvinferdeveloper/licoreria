"use client";

import { Minus, Plus } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { posCopy } from "../copy";

interface QuantityStepperProps {
  value: number;
  max: number;
  onChange: (value: number) => void;
}

export function QuantityStepper({ value, max, onChange }: QuantityStepperProps) {
  return (
    <div className="flex items-center rounded-md border border-line bg-surface">
      <Button variant="ghost" size="icon-sm" aria-label={posCopy.bill.decrease} onClick={() => onChange(value - 1)}>
        <Minus size={14} weight="bold" />
      </Button>
      <span className="tabular w-7 text-center font-mono text-label font-medium" aria-live="polite">
        {value}
      </span>
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label={posCopy.bill.increase}
        disabled={value >= max}
        onClick={() => onChange(value + 1)}
      >
        <Plus size={14} weight="bold" />
      </Button>
    </div>
  );
}
