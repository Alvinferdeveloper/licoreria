"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/field";
import { SegmentedControl } from "@/components/ui/segmented-control";
import { Sheet } from "@/components/ui/sheet";
import { parseAmount, type Cents } from "@/lib/money";
import { posCopy } from "../copy";
import { validateDiscount, type Discount } from "../domain";

const copy = posCopy.discount;

interface DiscountSheetProps {
  open: boolean;
  current: Discount | null;
  subtotal: Cents;
  onApply: (discount: Discount | null) => void;
  onClose: () => void;
}

export function DiscountSheet({ open, current, subtotal, onApply, onClose }: DiscountSheetProps) {
  return (
    <Sheet open={open} onClose={onClose} title={copy.title} description={copy.description}>
      {open && <DiscountForm current={current} subtotal={subtotal} onApply={onApply} onClose={onClose} />}
    </Sheet>
  );
}

function DiscountForm({ current, subtotal, onApply, onClose }: Omit<DiscountSheetProps, "open">) {
  const [kind, setKind] = useState<Discount["kind"]>(current?.kind ?? "percent");
  const [value, setValue] = useState(current ? (current.kind === "percent" ? String(current.value) : (current.value / 100).toFixed(2)) : "");
  const [error, setError] = useState<string | null>(null);

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    const parsed = kind === "percent" ? Number(value.replace(",", ".")) : parseAmount(value);
    if (parsed === null || !Number.isFinite(parsed)) return setError(copy.invalid);
    const discount: Discount = kind === "percent" ? { kind, value: Math.round(parsed * 100) / 100, authorizedBy: null } : { kind, value: parsed, authorizedBy: null };
    const problem = validateDiscount(discount, subtotal);
    if (problem) return setError(problem === "exceeds-subtotal" ? copy.tooLarge : copy.invalid);
    onApply(discount);
  };

  return (
    <form onSubmit={submit} className="space-y-5" noValidate>
      <SegmentedControl label={copy.kind} value={kind} options={copy.kinds} onChange={setKind} />
      <Field label={copy.value} error={error ?? undefined}>
        <Input value={value} onChange={(e) => setValue(e.target.value)} inputMode="decimal" className="font-mono tabular" autoFocus autoComplete="off" />
      </Field>
      <div className="flex justify-between gap-2">
        {current ? (
          <Button variant="ghost" onClick={() => onApply(null)}>
            {copy.remove}
          </Button>
        ) : (
          <span />
        )}
        <div className="flex gap-2">
          <Button onClick={onClose}>{copy.cancel}</Button>
          <Button type="submit" variant="primary">
            {copy.apply}
          </Button>
        </div>
      </div>
    </form>
  );
}
