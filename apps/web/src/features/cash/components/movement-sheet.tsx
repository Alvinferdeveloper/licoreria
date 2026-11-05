"use client";

import { useMutation } from "@tanstack/react-query";
import { useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/field";
import { SegmentedControl } from "@/components/ui/segmented-control";
import { Sheet } from "@/components/ui/sheet";
import { ApiError } from "@/lib/api/client";
import { parseAmount, toAmountString } from "@/lib/money";
import { cashApi, type CashMovement, type Currency, type Shift } from "../api";
import { cashCopy } from "../copy";

const { movement: copy, validation: v } = cashCopy;

export function MovementSheet({ open, shiftId, onClose, onDone }: { open: boolean; shiftId: string; onClose: () => void; onDone: (s: Shift) => void }) {
  return (
    <Sheet open={open} onClose={onClose} title={copy.title} description={copy.description}>
      {open && <MovementForm shiftId={shiftId} onClose={onClose} onDone={onDone} />}
    </Sheet>
  );
}

function MovementForm({ shiftId, onClose, onDone }: { shiftId: string; onClose: () => void; onDone: (s: Shift) => void }) {
  const [type, setType] = useState<CashMovement["type"]>("expense");
  const [currency, setCurrency] = useState<Currency>("NIO");
  const [amount, setAmount] = useState("");
  const [reason, setReason] = useState("");
  const [errors, setErrors] = useState<Partial<Record<"amount" | "reason" | "form", string>>>({});

  const mutation = useMutation({
    mutationFn: (body: Parameters<typeof cashApi.addMovement>[1]) => cashApi.addMovement(shiftId, body),
    onSuccess: onDone,
    onError: (e) => {
      if (e instanceof ApiError) setErrors({ amount: e.fieldMessage("amount"), reason: e.fieldMessage("reason"), form: e.fields.length ? undefined : e.message });
      else setErrors({ form: "Ocurrió un error inesperado." });
    },
  });

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    const cents = parseAmount(amount);
    const next: typeof errors = {};
    if (cents === null) next.amount = v.amount;
    else if (cents === 0) next.amount = v.positive;
    if (reason.trim().length < 3) next.reason = v.reason;
    setErrors(next);
    if (Object.keys(next).length || cents === null) return;
    mutation.mutate({ type, currency, amount: toAmountString(cents), reason: reason.trim() });
  };

  return (
    <form onSubmit={submit} className="space-y-5" noValidate>
      {errors.form && <Alert>{errors.form}</Alert>}
      <SegmentedControl label={copy.type} value={type} options={copy.types} onChange={setType} />
      <SegmentedControl label={copy.currency} value={currency} options={copy.currencies} onChange={setCurrency} />
      <Field label={copy.amount} error={errors.amount}>
        <Input value={amount} onChange={(e) => setAmount(e.target.value)} inputMode="decimal" className="font-mono tabular" autoFocus autoComplete="off" />
      </Field>
      <Field label={copy.reason} error={errors.reason}>
        <Input value={reason} onChange={(e) => setReason(e.target.value)} placeholder={copy.reasonPlaceholder} maxLength={300} />
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
