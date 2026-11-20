"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/field";
import { SegmentedControl } from "@/components/ui/segmented-control";
import { Sheet } from "@/components/ui/sheet";
import { catalogKeys, type Product } from "@/features/catalog/api";
import { ProductPicker } from "@/features/catalog/components/product-picker";
import { ApiError } from "@/lib/api/client";
import { inventoryApi, inventoryKeys, type AdjustmentInput } from "../api";
import { inventoryCopy } from "../copy";

const { adjustment: copy, validation: v } = inventoryCopy;
type Kind = AdjustmentInput["kind"];

export function AdjustmentSheet({ open, onClose, onDone }: { open: boolean; onClose: () => void; onDone: (name: string, stock: number) => void }) {
  return (
    <Sheet open={open} onClose={onClose} title={copy.title} description={copy.description}>
      {open && <AdjustmentForm onClose={onClose} onDone={onDone} />}
    </Sheet>
  );
}

function AdjustmentForm({ onClose, onDone }: { onClose: () => void; onDone: (name: string, stock: number) => void }) {
  const queryClient = useQueryClient();
  const [product, setProduct] = useState<Product | null>(null);
  const [kind, setKind] = useState<Kind>("count");
  const [quantity, setQuantity] = useState("");
  const [reason, setReason] = useState("");
  const [errors, setErrors] = useState<Partial<Record<"product" | "quantity" | "reason" | "form", string>>>({});

  const mutation = useMutation({
    mutationFn: (input: AdjustmentInput) => inventoryApi.adjust(input),
    onSuccess: ({ stock }) => {
      void queryClient.invalidateQueries({ queryKey: inventoryKeys.all });
      void queryClient.invalidateQueries({ queryKey: catalogKeys.products });
      onDone(product!.name, stock);
      onClose();
    },
    onError: (error) => {
      if (error instanceof ApiError) {
        setErrors({ quantity: error.fieldMessage("quantity"), reason: error.fieldMessage("reason"), form: error.fields.length ? undefined : error.message });
      } else setErrors({ form: "Ocurrió un error inesperado. Intenta de nuevo." });
    },
  });

  const counted = /^\d{1,6}$/.test(quantity.trim()) ? Number(quantity) : null;
  const diff = product && counted !== null && kind === "count" ? counted - product.stock : null;

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    const next: typeof errors = {};
    if (!product) next.product = v.product;
    if (counted === null) next.quantity = kind === "count" ? v.quantity : v.lost;
    else if (kind === "shrinkage" && counted === 0) next.quantity = v.lost;
    if (reason.trim().length < 3) next.reason = v.reason;
    setErrors(next);
    if (Object.keys(next).length > 0 || !product || counted === null) return;
    mutation.mutate({ productId: product.id, kind, quantity: counted, reason: reason.trim() });
  };

  return (
    <form onSubmit={submit} className="space-y-5" noValidate>
      {errors.form && <Alert>{errors.form}</Alert>}
      <div className="space-y-1.5">
        <p className="text-label font-medium">{copy.product}</p>
        {product ? (
          <div className="flex items-center justify-between gap-3 rounded-md border border-line bg-sunken px-3 py-2">
            <div className="min-w-0">
              <p className="truncate font-medium">{product.name}</p>
              <p className="text-caption text-ink-muted">
                <span className="font-mono">{product.sku}</span> · {copy.currentStock(product.stock)}
              </p>
            </div>
            <Button size="sm" variant="ghost" onClick={() => setProduct(null)}>
              {copy.change}
            </Button>
          </div>
        ) : (
          <ProductPicker placeholder={copy.search} onPick={setProduct} autoFocus />
        )}
        {errors.product && <p className="text-caption font-medium text-danger">{errors.product}</p>}
      </div>

      <SegmentedControl label={copy.kind} value={kind} options={copy.kinds} onChange={setKind} />

      <Field
        label={kind === "count" ? copy.counted : copy.lost}
        error={errors.quantity}
        hint={diff !== null ? <span className={diff < 0 ? "text-danger" : undefined}>{copy.countedHint(diff)}</span> : undefined}
      >
        <Input
          value={quantity}
          onChange={(e) => setQuantity(e.target.value)}
          inputMode="numeric"
          placeholder={copy.quantityPlaceholders[kind]}
          className="font-mono tabular"
          autoComplete="off"
        />
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
