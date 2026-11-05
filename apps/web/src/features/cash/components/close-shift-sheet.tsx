"use client";

import { useMutation } from "@tanstack/react-query";
import { useState } from "react";
import { Money } from "@/components/money";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/field";
import { Sheet } from "@/components/ui/sheet";
import { ApiError } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { amountToCents, formatMoney, parseAmount, toAmountString, type CurrencyCode } from "@/lib/money";
import { cashApi, type Shift, type ShiftSummary } from "../api";
import { cashCopy } from "../copy";

const { closing: copy, validation: v } = cashCopy;

interface CloseShiftSheetProps {
  open: boolean;
  shift: Shift;
  summary: ShiftSummary;
  onClose: () => void;
  onDone: (s: Shift) => void;
}

export function CloseShiftSheet({ open, shift, summary, onClose, onDone }: CloseShiftSheetProps) {
  return (
    <Sheet open={open} onClose={onClose} title={copy.title} description={copy.description}>
      {open && <CloseForm shift={shift} summary={summary} onClose={onClose} onDone={onDone} />}
    </Sheet>
  );
}

function CloseForm({ shift, summary, onClose, onDone }: Omit<CloseShiftSheetProps, "open">) {
  const [nio, setNio] = useState("");
  const [usd, setUsd] = useState("");
  const [notes, setNotes] = useState("");
  const [errors, setErrors] = useState<Partial<Record<"nio" | "usd" | "form", string>>>({});

  const mutation = useMutation({
    mutationFn: (body: Parameters<typeof cashApi.close>[1]) => cashApi.close(shift.id, body),
    onSuccess: onDone,
    onError: (e) => setErrors({ form: e instanceof ApiError ? e.message : "Ocurrió un error inesperado." }),
  });

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    const nioCents = parseAmount(nio);
    const usdCents = parseAmount(usd);
    const next: typeof errors = {};
    if (nioCents === null) next.nio = v.amount;
    if (usdCents === null) next.usd = v.amount;
    setErrors(next);
    if (Object.keys(next).length || nioCents === null || usdCents === null) return;
    mutation.mutate({ countedNio: toAmountString(nioCents), countedUsd: toAmountString(usdCents), notes: notes.trim() });
  };

  return (
    <form onSubmit={submit} className="space-y-5" noValidate>
      {errors.form && <Alert>{errors.form}</Alert>}
      <CountField label={copy.countedNio} currency="NIO" expected={summary.expectedNio} value={nio} onChange={setNio} error={errors.nio} autoFocus />
      <CountField label={copy.countedUsd} currency="USD" expected={summary.expectedUsd} value={usd} onChange={setUsd} error={errors.usd} />
      <Field label={copy.notes} hint={copy.notesHint}>
        <Input value={notes} onChange={(e) => setNotes(e.target.value)} maxLength={500} />
      </Field>
      <div className="flex justify-end gap-2">
        <Button onClick={onClose}>{copy.cancel}</Button>
        <Button type="submit" variant="primary" loading={mutation.isPending}>
          {copy.submit}
        </Button>
      </div>
    </form>
  );
}

interface CountFieldProps {
  label: string;
  currency: CurrencyCode;
  expected: string;
  value: string;
  onChange: (v: string) => void;
  error?: string;
  autoFocus?: boolean;
}

/** Counted amount with the live difference against what the shift expects. */
function CountField({ label, currency, expected, value, onChange, error, autoFocus }: CountFieldProps) {
  const counted = parseAmount(value);
  const diff = counted === null ? null : counted - amountToCents(expected);
  return (
    <div className="space-y-2">
      <Field label={label} error={error} hint={copy.expected(formatMoney(amountToCents(expected), currency))}>
        <Input value={value} onChange={(e) => onChange(e.target.value)} inputMode="decimal" className="h-12 text-body-lg font-mono tabular" autoFocus={autoFocus} autoComplete="off" />
      </Field>
      {diff !== null && value.trim() !== "" && (
        <p className={cn("flex justify-between rounded-md px-3 py-2 text-label", diff === 0 ? "bg-success-tint text-success" : "bg-danger-tint text-danger")}>
          <span>{copy.difference}</span>
          <span className="font-mono tabular font-medium">{diff === 0 ? copy.balanced : <Money cents={diff} currency={currency} />}</span>
        </p>
      )}
    </div>
  );
}
