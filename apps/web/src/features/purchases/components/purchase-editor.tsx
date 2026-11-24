"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { ArrowLeft, Trash } from "@phosphor-icons/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { useFieldArray, useForm, useWatch, type Control } from "react-hook-form";
import { z } from "zod";
import { NoticeBanner, useNotice } from "@/components/data/toolbar";
import { Money } from "@/components/money";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { Field, Input, inputClassName } from "@/components/ui/field";
import { Sheet } from "@/components/ui/sheet";
import { can } from "@/features/auth/api";
import { useSession } from "@/features/auth/session-store";
import { amountToCents, catalogKeys, type Product } from "@/features/catalog/api";
import { ProductPicker } from "@/features/catalog/components/product-picker";
import { ApiError } from "@/lib/api/client";
import { applyApiError } from "@/lib/api/form-errors";
import { cn } from "@/lib/cn";
import { formatDate, formatDateTime, todayISO } from "@/lib/format/date";
import { formatAmount, parseAmount } from "@/lib/money";
import { purchaseKeys, purchasesApi, type Purchase, type PurchaseInput } from "../api";
import { purchasesCopy } from "../copy";
import { costChange, purchaseTotals } from "../domain";
import { PurchaseStatusBadge } from "./purchase-status-badge";

const { editor: copy, validation: v, notices } = purchasesCopy;
const moneyPattern = /^\d{1,11}(\.\d{1,2})?$/;

const schema = z.object({
  supplierId: z.string().min(1, v.supplier),
  supplierInvoiceNumber: z.string().trim().max(40),
  purchaseDate: z.string().min(1, v.date),
  notes: z.string().max(500),
  items: z
    .array(
      z.object({
        productId: z.string(),
        name: z.string(),
        sku: z.string(),
        taxPercent: z.string(),
        reference: z.string().nullable(),
        quantity: z.string().trim().regex(/^\d{1,6}$/, v.quantity).refine((s) => Number(s) > 0, v.quantity),
        unitCost: z.string().trim().regex(moneyPattern, v.cost),
        expiresOn: z.string(),
      }),
    )
    .min(1, v.lines),
});

type Values = z.infer<typeof schema>;
type Line = Values["items"][number];

const FIELDS = ["supplierId", "supplierInvoiceNumber", "purchaseDate", "notes", "items"] as const;
const costText = (amount: string) => String(Number(amount).toFixed(2));

function toValues(p: Purchase | undefined): Values {
  return {
    supplierId: p?.supplier.id ?? "",
    supplierInvoiceNumber: p?.supplierInvoiceNumber ?? "",
    purchaseDate: p?.purchaseDate ?? todayISO(),
    notes: p?.notes ?? "",
    items: (p?.items ?? []).map((i) => ({
      productId: i.product.id,
      name: i.product.name,
      sku: i.product.sku,
      taxPercent: i.taxPercent,
      reference: i.previousCost,
      quantity: String(i.quantity),
      unitCost: costText(i.unitCost),
      expiresOn: i.expiresOn ?? "",
    })),
  };
}

function toInput(values: Values): PurchaseInput {
  return {
    supplierId: values.supplierId,
    supplierInvoiceNumber: values.supplierInvoiceNumber.trim(),
    purchaseDate: values.purchaseDate,
    notes: values.notes.trim(),
    items: values.items.map((i) => ({ productId: i.productId, quantity: Number(i.quantity), unitCost: i.unitCost.trim(), expiresOn: i.expiresOn || null })),
  };
}

/** New purchase when id is null; otherwise loads it, editable only while it is a draft. */
export function PurchaseEditor({ id }: { id: string | null }) {
  const purchase = useQuery({ queryKey: purchaseKeys.detail(id ?? "new"), queryFn: () => purchasesApi.get(id!), enabled: id !== null });
  const suppliers = useQuery({ queryKey: purchaseKeys.suppliers, queryFn: purchasesApi.suppliers });

  // The supplier <select> is uncontrolled: it must have its options before the saved value lands.
  if ((id && purchase.isPending) || suppliers.isPending) return <p className="px-8 py-10 text-ink-muted">Cargando…</p>;
  if (id && purchase.isError) {
    return (
      <div className="px-8 py-10">
        <Alert>{purchase.error instanceof ApiError ? purchase.error.message : "No pudimos cargar la compra."}</Alert>
      </div>
    );
  }
  return <EditorForm key={purchase.data?.status ?? "new"} purchase={purchase.data} suppliers={suppliers.data ?? []} />;
}

function EditorForm({ purchase, suppliers }: { purchase: Purchase | undefined; suppliers: { id: string; name: string }[] }) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const me = useSession((s) => s.user);
  const { notice, success, failure } = useNotice();
  const [formError, setFormError] = useState<string | null>(null);
  const [voidOpen, setVoidOpen] = useState(false);
  const editable = !purchase || purchase.status === "draft";
  const canEdit = purchase ? can(me, "purchases:update") : can(me, "purchases:create");

  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: toValues(purchase) });
  const { register, control, handleSubmit, setError, formState } = form;
  const lines = useFieldArray({ control, name: "items" });

  const refresh = async (saved: Purchase) => {
    queryClient.setQueryData(purchaseKeys.detail(saved.id), saved);
    await queryClient.invalidateQueries({ queryKey: purchaseKeys.all });
  };

  const save = async (values: Values) => {
    const input = toInput(values);
    return purchase ? purchasesApi.update(purchase.id, input) : purchasesApi.create(input);
  };

  const onError = (error: unknown) => setFormError(applyApiError(error, setError, FIELDS));

  const saveDraft = handleSubmit(async (values) => {
    setFormError(null);
    try {
      const saved = await save(values);
      await refresh(saved);
      success(notices.saved);
      if (!purchase) router.replace(`/compras/${saved.id}`);
      else form.reset(toValues(saved));
    } catch (error) {
      onError(error);
    }
  });

  const receive = handleSubmit(async (values) => {
    setFormError(null);
    try {
      const saved = await save(values);
      const received = await purchasesApi.receive(saved.id);
      await refresh(received);
      void queryClient.invalidateQueries({ queryKey: catalogKeys.products });
      if (!purchase) router.replace(`/compras/${received.id}`);
      success(notices.received);
    } catch (error) {
      onError(error);
    }
  });

  const addProduct = (product: Product) => {
    const cost = product.averageCost && Number(product.averageCost) > 0 ? costText(product.averageCost) : "";
    lines.append({ productId: product.id, name: product.name, sku: product.sku, taxPercent: product.taxPercent, reference: null, quantity: "1", unitCost: cost, expiresOn: "" });
  };

  const title = purchase ? copy.title(purchase.supplier.name) : copy.newTitle;
  const busy = formState.isSubmitting;

  return (
    <>
      <div className="px-8 pt-6">
        <Link href="/compras" className="inline-flex items-center gap-1.5 text-label text-ink-muted transition-colors hover:text-ink">
          <ArrowLeft size={14} weight="bold" aria-hidden />
          {copy.back}
        </Link>
      </div>
      <header className="flex flex-wrap items-center gap-3 px-8 pt-3 pb-6">
        <h1 className="text-headline font-semibold tracking-headline">{title}</h1>
        {purchase && <PurchaseStatusBadge status={purchase.status} />}
      </header>

      <div className="px-8 [&>*:not(:empty)]:mb-4">
        <NoticeBanner notice={notice} />
        {formError && <Alert>{formError}</Alert>}
        {purchase?.status === "received" && purchase.receivedBy && purchase.receivedAt && (
          <Alert tone="success">{copy.receivedBy(purchase.receivedBy.name, formatDateTime(purchase.receivedAt))}</Alert>
        )}
        {purchase?.status === "voided" && purchase.voidedAt && <Alert tone="info">{copy.voided(formatDateTime(purchase.voidedAt), purchase.voidReason)}</Alert>}
      </div>

      <form onSubmit={saveDraft} className="grid gap-6 px-8 pb-10 xl:grid-cols-[minmax(0,1fr)_20rem]" noValidate>
        <div className="min-w-0 space-y-6">

          <section className="grid gap-4 rounded-lg border border-line bg-surface p-5 sm:grid-cols-2">
            <Field label={copy.supplier} error={formState.errors.supplierId?.message}>
              <select {...register("supplierId")} disabled={!editable} className={inputClassName}>
                <option value="">{copy.selectSupplier}</option>
                {suppliers.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                  </option>
                ))}
                {purchase && !suppliers.some((s) => s.id === purchase.supplier.id) && <option value={purchase.supplier.id}>{purchase.supplier.name}</option>}
              </select>
            </Field>
            <Field label={copy.date} error={formState.errors.purchaseDate?.message}>
              <Input type="date" max={todayISO()} {...register("purchaseDate")} disabled={!editable} />
            </Field>
            <Field label={copy.invoice} hint={copy.invoiceHint} error={formState.errors.supplierInvoiceNumber?.message}>
              <Input {...register("supplierInvoiceNumber")} disabled={!editable} className="font-mono" autoComplete="off" placeholder={copy.invoicePlaceholder} />
            </Field>
            <Field label={copy.notes} error={formState.errors.notes?.message}>
              <Input {...register("notes")} disabled={!editable} autoComplete="off" placeholder={copy.notesPlaceholder} />
            </Field>
          </section>

          <section className="space-y-3">
            <h2 className="font-semibold">{copy.lines}</h2>
            {editable && canEdit && <ProductPicker placeholder={copy.addProduct} onPick={addProduct} excludeIds={lines.fields.map((l) => l.productId)} autoFocus={!purchase} />}
            {formState.errors.items?.root?.message || formState.errors.items?.message ? (
              <p className="text-caption font-medium text-danger">{formState.errors.items?.root?.message ?? formState.errors.items?.message}</p>
            ) : null}
            <div className="overflow-x-auto rounded-lg border border-line bg-surface">
              <table className="w-full min-w-[720px] border-collapse text-left">
                <thead className="bg-chrome text-caption text-ink-muted">
                  <tr>
                    <th className="px-4 py-2.5 font-medium">{copy.columns.product}</th>
                    <th className="w-28 px-4 py-2.5 text-right font-medium">{copy.columns.quantity}</th>
                    <th className="w-40 px-4 py-2.5 text-right font-medium">{copy.columns.cost}</th>
                    <th className="w-44 px-4 py-2.5 font-medium" title={copy.expiresHint}>{copy.columns.expires}</th>
                    <th className="w-36 px-4 py-2.5 text-right font-medium">{copy.columns.subtotal}</th>
                    {editable && <th className="w-12" aria-label={copy.remove} />}
                  </tr>
                </thead>
                <tbody className="divide-y divide-line">
                  {lines.fields.map((line, index) => (
                    <LineRow key={line.id} index={index} control={control} editable={editable && canEdit} register={register} errors={formState.errors.items?.[index]} onRemove={() => lines.remove(index)} />
                  ))}
                </tbody>
              </table>
              {lines.fields.length === 0 && <p className="px-4 py-10 text-center text-ink-muted">{copy.noLines}</p>}
            </div>
          </section>
        </div>

        <aside className="space-y-4 xl:sticky xl:top-6 xl:self-start">
          <Totals control={control} />
          {editable && canEdit && (
            <div className="space-y-2">
              <ConfirmButton variant="primary" className="w-full" confirmLabel={copy.confirmReceive} disabled={busy} onConfirm={() => void receive()}>
                {copy.receive}
              </ConfirmButton>
              <p className="text-caption text-ink-muted">{copy.receiveHint}</p>
              <Button type="submit" className="w-full" loading={busy}>
                {copy.saveDraft}
              </Button>
            </div>
          )}
          {!editable && <p className="text-caption text-ink-muted">{copy.readOnly}</p>}
          {purchase && purchase.status !== "voided" && can(me, "purchases:void") && (
            <Button variant="ghost" className="w-full text-danger" onClick={() => setVoidOpen(true)}>
              {copy.void}
            </Button>
          )}
        </aside>
      </form>

      {purchase && (
        <VoidSheet
          open={voidOpen}
          onClose={() => setVoidOpen(false)}
          onVoid={async (reason) => {
            const voided = await purchasesApi.void(purchase.id, reason);
            await refresh(voided);
            void queryClient.invalidateQueries({ queryKey: catalogKeys.products });
            setVoidOpen(false);
            success(notices.voided);
          }}
          onFailure={failure}
        />
      )}
    </>
  );
}

interface LineRowProps {
  index: number;
  control: Control<Values>;
  editable: boolean;
  register: ReturnType<typeof useForm<Values>>["register"];
  errors: { quantity?: { message?: string }; unitCost?: { message?: string } } | undefined;
  onRemove: () => void;
}

function LineRow({ index, control, editable, register, errors, onRemove }: LineRowProps) {
  const line = useWatch({ control, name: `items.${index}` }) as Line;
  const cost = parseAmount(line.unitCost ?? "");
  const qty = Number(line.quantity);
  const reference = line.reference ? amountToCents(line.reference) : null;
  const change = cost !== null ? costChange(cost, reference) : null;
  const cell = "px-4 py-2.5 align-top";

  return (
    <tr>
      <td className={cell}>
        <p className="font-medium">{line.name}</p>
        <p className="font-mono text-caption tracking-wide text-ink-muted">{line.sku}</p>
      </td>
      <td className={cn(cell, "text-right")}>
        {editable ? (
          <>
            <Input {...register(`items.${index}.quantity`)} inputMode="numeric" placeholder={copy.quantityPlaceholder} aria-label={`Cantidad de ${line.name}`} aria-invalid={errors?.quantity ? true : undefined} className="h-9 text-right font-mono tabular" />
            {errors?.quantity && <p className="mt-1 text-caption text-danger">{errors.quantity.message}</p>}
          </>
        ) : (
          <span className="font-mono tabular">{line.quantity}</span>
        )}
      </td>
      <td className={cn(cell, "text-right")}>
        {editable ? (
          <>
            <Input {...register(`items.${index}.unitCost`)} inputMode="decimal" placeholder={copy.costPlaceholder} aria-label={`Costo unitario de ${line.name}`} aria-invalid={errors?.unitCost ? true : undefined} className="h-9 text-right font-mono tabular" />
            {errors?.unitCost && <p className="mt-1 text-caption text-danger">{errors.unitCost.message}</p>}
          </>
        ) : (
          <Money cents={cost ?? 0} className="font-mono" />
        )}
        {line.reference && (
          <p className="mt-1 text-caption text-ink-muted">
            {copy.previousCost(formatAmount(reference ?? 0))}
            {change !== null && Math.abs(change) >= 0.005 && (
              <span className={change > 0 ? "text-danger" : "text-success"}>
                {" "}
                {change > 0 ? "▲" : "▼"} {Math.abs(change * 100).toFixed(1)} %
              </span>
            )}
          </p>
        )}
      </td>
      <td className={cell}>
        {editable ? (
          <Input {...register(`items.${index}.expiresOn`)} type="date" aria-label={`${copy.columns.expires} ${line.name}`} className="h-9 font-mono tabular" />
        ) : (
          <span className="font-mono tabular text-label">{line.expiresOn ? formatDate(line.expiresOn) : <span className="text-ink-subtle">—</span>}</span>
        )}
      </td>
      <td className={cn(cell, "pt-4 text-right")}>{cost !== null && qty > 0 ? <Money cents={cost * qty} className="font-mono" /> : <span className="text-ink-subtle">—</span>}</td>
      {editable && (
        <td className={cn(cell, "px-2")}>
          <Button variant="ghost" size="icon-sm" aria-label={`${copy.remove} ${line.name}`} title={copy.remove} onClick={onRemove}>
            <Trash size={16} />
          </Button>
        </td>
      )}
    </tr>
  );
}

function Totals({ control }: { control: Control<Values> }) {
  const items = (useWatch({ control, name: "items" }) ?? []) as Line[];
  const totals = purchaseTotals(
    items.map((i) => ({ quantity: Number(i.quantity) || 0, unitCost: parseAmount(i.unitCost ?? "") ?? 0, taxPercent: Number(i.taxPercent) || 0 })),
  );
  const row = "flex items-baseline justify-between gap-4";
  return (
    <dl className="space-y-2 rounded-lg border border-line bg-surface p-5">
      <div className={row}>
        <dt className="text-ink-muted">{copy.subtotal}</dt>
        <dd><Money cents={totals.subtotal} className="font-mono" /></dd>
      </div>
      <div className={row}>
        <dt className="text-ink-muted">{copy.tax}</dt>
        <dd><Money cents={totals.tax} className="font-mono" /></dd>
      </div>
      <div className={cn(row, "border-t border-line pt-3")}>
        <dt className="font-semibold">{copy.total}</dt>
        <dd className="text-title font-semibold"><Money cents={totals.total} className="font-mono" /></dd>
      </div>
    </dl>
  );
}

function VoidSheet({ open, onClose, onVoid, onFailure }: { open: boolean; onClose: () => void; onVoid: (reason: string) => Promise<void>; onFailure: (e: unknown) => void }) {
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | null>(null);
  const mutation = useMutation({
    mutationFn: () => onVoid(reason.trim()),
    onSuccess: () => setReason(""),
    onError: (e) => {
      if (e instanceof ApiError && e.code === "insufficient_stock") setError(e.message);
      else onFailure(e);
    },
  });
  return (
    <Sheet open={open} onClose={onClose} title={copy.void} description={copy.voidHint}>
      <form
        className="space-y-4"
        onSubmit={(e) => {
          e.preventDefault();
          setError(null);
          if (reason.trim().length < 3) return setError(v.reason);
          mutation.mutate();
        }}
      >
        <Field label={copy.voidReason} error={error ?? undefined}>
          <Input value={reason} onChange={(e) => setReason(e.target.value)} autoFocus maxLength={300} placeholder={copy.voidReasonPlaceholder} />
        </Field>
        <div className="flex justify-end gap-2">
          <Button onClick={onClose}>{copy.cancel}</Button>
          <Button type="submit" variant="danger" loading={mutation.isPending}>
            {copy.confirmVoid}
          </Button>
        </div>
      </form>
    </Sheet>
  );
}
