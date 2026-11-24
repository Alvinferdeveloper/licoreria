"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { ImageSquare, Plus, Trash } from "@phosphor-icons/react";
import { useEffect, useRef, useState, type ComponentProps, type ReactNode } from "react";
import { useFieldArray, useForm, useWatch } from "react-hook-form";
import { z } from "zod";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Field, Input, inputClassName } from "@/components/ui/field";
import { applyApiError } from "@/lib/api/form-errors";
import { ApiError } from "@/lib/api/client";
import { catalogApi, type Master, type Product, type ProductInput } from "../api";
import { productsCopy } from "../copy";

const { form: copy, validation: v } = productsCopy;
const ph = copy.placeholders;

const money = z.string().trim().regex(/^\d{1,11}(\.\d{1,2})?$/, v.money);
const percent = z.string().trim().regex(/^\d{1,3}(\.\d{1,2})?$/, v.percent).refine((s) => Number(s) <= 100, v.percent);
const integer = z.string().trim().regex(/^\d{1,7}$/, v.integer);

const schema = z.object({
  name: z.string().trim().min(2, v.required),
  sku: z.string().trim().min(2, v.required),
  barcode: z.string().trim(),
  categoryId: z.string().min(1, v.required),
  brandId: z.string(),
  unitId: z.string().min(1, v.required),
  description: z.string(),
  price: money,
  taxPercent: percent,
  averageCost: z.string().trim().refine((s) => s === "" || /^\d{1,11}(\.\d{1,2})?$/.test(s), v.money),
  minStock: integer,
  reorderPoint: integer,
  allowNegativeStock: z.boolean(),
  alcoholPercent: z.string().trim().refine((s) => s === "" || percent.safeParse(s).success, v.percent),
  volumeMl: z.string().trim().refine((s) => s === "" || /^\d{1,6}$/.test(s), v.integer),
  presentations: z.array(
    z.object({
      id: z.string().optional(),
      name: z.string().trim().min(2, v.required),
      factor: z.string().trim().regex(/^\d+$/, v.factor).refine((s) => Number(s) > 1, v.factor),
      barcode: z.string().trim(),
      price: money,
    }),
  ),
});

type Values = z.infer<typeof schema>;

const FIELDS = ["name", "sku", "barcode", "categoryId", "brandId", "unitId", "price", "taxPercent", "averageCost", "minStock", "reorderPoint", "presentations"] as const;

function toValues(p: Product | null, defaultTax: number): Values {
  return {
    name: p?.name ?? "",
    sku: p?.sku ?? "",
    barcode: p?.barcode ?? "",
    categoryId: p?.category.id ?? "",
    brandId: p?.brand?.id ?? "",
    unitId: p?.unit.id ?? "",
    description: p?.description ?? "",
    price: p?.price ?? "",
    taxPercent: p ? String(Number(p.taxPercent)) : String(defaultTax),
    averageCost: "",
    minStock: String(p?.minStock ?? 0),
    reorderPoint: String(p?.reorderPoint ?? 0),
    allowNegativeStock: p?.allowNegativeStock ?? false,
    alcoholPercent: p?.alcoholPercent ? String(Number(p.alcoholPercent)) : "",
    volumeMl: p?.volumeMl ? String(p.volumeMl) : "",
    presentations: (p?.presentations ?? [])
      .filter((pr) => pr.active)
      .map((pr) => ({ id: pr.id, name: pr.name, factor: String(pr.factor), barcode: pr.barcode ?? "", price: pr.price })),
  };
}

function toInput(values: Values): ProductInput {
  const orNull = (s: string) => (s.trim() === "" ? null : s.trim());
  return {
    name: values.name,
    sku: values.sku,
    barcode: orNull(values.barcode),
    categoryId: values.categoryId,
    brandId: orNull(values.brandId),
    unitId: values.unitId,
    description: values.description,
    price: values.price,
    taxPercent: values.taxPercent,
    averageCost: values.averageCost || undefined,
    minStock: Number(values.minStock),
    reorderPoint: Number(values.reorderPoint),
    allowNegativeStock: values.allowNegativeStock,
    alcoholPercent: orNull(values.alcoholPercent),
    volumeMl: values.volumeMl ? Number(values.volumeMl) : null,
    presentations: values.presentations.map((p) => ({ id: p.id, name: p.name, factor: Number(p.factor), barcode: orNull(p.barcode), price: p.price })),
  };
}

interface ProductFormProps {
  product: Product | null;
  categories: Master[];
  brands: Master[];
  units: Master[];
  canEditPrices: boolean;
  onCancel: () => void;
  onSubmit: (input: ProductInput) => Promise<void>;
  defaultTax?: number;
  onUploadImage: (file: File) => Promise<Product>;
  onRemoveImage: () => Promise<Product>;
}

export function ProductForm({ product, categories, brands, units, canEditPrices, defaultTax = 15, onCancel, onSubmit, onUploadImage, onRemoveImage }: ProductFormProps) {
  const [formError, setFormError] = useState<string | null>(null);
  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: toValues(product, defaultTax) });
  const presentations = useFieldArray({ control: form.control, name: "presentations" });
  const { errors, isSubmitting } = form.formState;
  const priceLocked = !canEditPrices;
  const skuTyped = useRef(false);
  const skuField = form.register("sku");
  const [name, brandId] = useWatch({ control: form.control, name: ["name", "brandId"] });

  // New products get a SKU from brand + name + volume until the user types their own.
  useEffect(() => {
    if (product || skuTyped.current || name.trim().length < 3) return;
    const timer = setTimeout(() => {
      catalogApi.suggestSku(name.trim(), brandId).then(
        (sku) => !skuTyped.current && form.setValue("sku", sku, { shouldValidate: form.formState.isSubmitted }),
        () => undefined,
      );
    }, 350);
    return () => clearTimeout(timer);
  }, [product, name, brandId, form]);

  const submit = form.handleSubmit(async (values) => {
    setFormError(null);
    try {
      await onSubmit(toInput(values));
    } catch (error) {
      setFormError(applyApiError(error, form.setError, FIELDS));
    }
  });

  return (
    <form onSubmit={submit} noValidate className="space-y-8">
      {formError && <Alert>{formError}</Alert>}

      <ImageField product={product} onUpload={onUploadImage} onRemove={onRemoveImage} />

      <Section title={copy.sections.general}>
        <Field label={copy.name} error={errors.name?.message} className="sm:col-span-2">
          <Input autoFocus placeholder={ph.name} {...form.register("name")} />
        </Field>
        <Field label={copy.sku} hint={product ? copy.skuHint : copy.skuAutoHint} error={errors.sku?.message}>
          <Input
            className="font-mono uppercase"
            autoCapitalize="characters"
            spellCheck={false}
            placeholder={ph.sku}
            {...skuField}
            onChange={(event) => {
              skuTyped.current = event.target.value.trim() !== "";
              return skuField.onChange(event);
            }}
          />
        </Field>
        <Field label={copy.barcode} hint={copy.barcodeHint} error={errors.barcode?.message}>
          <Input className="font-mono" inputMode="numeric" spellCheck={false} placeholder={ph.barcode} {...form.register("barcode")} />
        </Field>
        <Field label={copy.category} error={errors.categoryId?.message}>
          <MasterSelect items={categories} placeholder={copy.select} {...form.register("categoryId")} />
        </Field>
        <Field label={copy.brand}>
          <MasterSelect items={brands} placeholder={copy.noBrand} {...form.register("brandId")} />
        </Field>
        <Field label={copy.unit} error={errors.unitId?.message}>
          <MasterSelect items={units} placeholder={copy.select} {...form.register("unitId")} />
        </Field>
      </Section>

      <Section title={copy.sections.price}>
        {priceLocked && <Alert tone="info" className="sm:col-span-2">{copy.priceLocked}</Alert>}
        <Field label={copy.price} error={errors.price?.message}>
          <MoneyInput disabled={priceLocked} {...form.register("price")} />
        </Field>
        <Field label={copy.tax} error={errors.taxPercent?.message}>
          <Input inputMode="decimal" className="tabular text-right font-mono" disabled={priceLocked} placeholder={ph.tax} {...form.register("taxPercent")} />
        </Field>
        {!product && canEditPrices && (
          <Field label={copy.cost} hint={copy.costHint} error={errors.averageCost?.message}>
            <MoneyInput {...form.register("averageCost")} />
          </Field>
        )}
        {product?.averageCost && <p className="self-end text-label text-ink-muted">{copy.currentCost(Number(product.averageCost).toFixed(2))}</p>}
      </Section>

      <Section title={copy.sections.stock}>
        {product && <p className="text-label text-ink-muted sm:col-span-2">{copy.currentStock(product.stock)}</p>}
        <Field label={copy.minStock} error={errors.minStock?.message}>
          <Input inputMode="numeric" className="tabular text-right font-mono" placeholder={ph.minStock} {...form.register("minStock")} />
        </Field>
        <Field label={copy.reorderPoint} error={errors.reorderPoint?.message}>
          <Input inputMode="numeric" className="tabular text-right font-mono" placeholder={ph.reorderPoint} {...form.register("reorderPoint")} />
        </Field>
        <Checkbox label={copy.allowNegative} className="sm:col-span-2" {...form.register("allowNegativeStock")} />
      </Section>

      <Section title={copy.sections.details}>
        <Field label={copy.alcohol} error={errors.alcoholPercent?.message}>
          <Input inputMode="decimal" className="tabular text-right font-mono" placeholder={ph.alcohol} {...form.register("alcoholPercent")} />
        </Field>
        <Field label={copy.volume} error={errors.volumeMl?.message}>
          <Input inputMode="numeric" className="tabular text-right font-mono" placeholder={ph.volume} {...form.register("volumeMl")} />
        </Field>
        <Field label={copy.description} className="sm:col-span-2">
          <textarea rows={2} className={`${inputClassName} h-auto py-2`} placeholder={ph.description} {...form.register("description")} />
        </Field>
      </Section>

      <section className="space-y-3">
        <div>
          <h3 className="text-label font-semibold">{copy.sections.presentations}</h3>
          <p className="text-caption text-ink-muted">{copy.presentationsHint}</p>
        </div>
        {presentations.fields.map((field, index) => {
          const e = errors.presentations?.[index];
          return (
            <div key={field.id} className="grid grid-cols-[minmax(0,1.4fr)_5rem_minmax(0,1.2fr)_minmax(0,1fr)_auto] items-start gap-2">
              <Field label={copy.presentationName} error={e?.name?.message}>
                <Input placeholder={ph.presentationName} {...form.register(`presentations.${index}.name`)} />
              </Field>
              <Field label={copy.factor} error={e?.factor?.message}>
                <Input inputMode="numeric" className="tabular text-right font-mono" placeholder={ph.factor} {...form.register(`presentations.${index}.factor`)} />
              </Field>
              <Field label={copy.barcode} error={e?.barcode?.message}>
                <Input className="font-mono" placeholder={ph.presentationBarcode} {...form.register(`presentations.${index}.barcode`)} />
              </Field>
              <Field label={copy.presentationPrice} error={e?.price?.message}>
                <MoneyInput disabled={priceLocked} {...form.register(`presentations.${index}.price`)} />
              </Field>
              <Button variant="ghost" size="icon" className="mt-6.5" aria-label={copy.remove} onClick={() => presentations.remove(index)}>
                <Trash size={16} />
              </Button>
            </div>
          );
        })}
        {errors.presentations?.message && <p className="text-caption font-medium text-danger">{errors.presentations.message}</p>}
        <Button size="sm" onClick={() => presentations.append({ name: "", factor: "6", barcode: "", price: "" })} disabled={priceLocked}>
          <Plus size={14} weight="bold" aria-hidden />
          {copy.addPresentation}
        </Button>
      </section>

      <div className="sticky -bottom-5 -mx-6 -mb-5 flex justify-end gap-2 border-t border-line bg-surface px-6 py-4">
        <Button variant="ghost" onClick={onCancel}>
          {copy.cancel}
        </Button>
        <Button type="submit" variant="primary" loading={isSubmitting}>
          {product ? copy.save : copy.createSubmit}
        </Button>
      </div>
    </form>
  );
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="space-y-3">
      <h3 className="text-label font-semibold">{title}</h3>
      <div className="grid gap-4 sm:grid-cols-2">{children}</div>
    </section>
  );
}

function MasterSelect({ items, placeholder, ...props }: ComponentProps<"select"> & { items: Master[]; placeholder: string }) {
  return (
    <select className={inputClassName} {...props}>
      <option value="">{placeholder}</option>
      {items
        .filter((item) => item.active)
        .map((item) => (
          <option key={item.id} value={item.id}>
            {item.name}
          </option>
        ))}
    </select>
  );
}

function MoneyInput(props: ComponentProps<"input">) {
  return (
    <div className="relative">
      <span className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 font-mono text-label text-ink-subtle">C$</span>
      <Input inputMode="decimal" className="tabular pl-10 text-right font-mono" placeholder={ph.money} {...props} />
    </div>
  );
}

function ImageField({ product, onUpload, onRemove }: { product: Product | null; onUpload: (file: File) => Promise<Product>; onRemove: () => Promise<Product> }) {
  const input = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!product) return <p className="rounded-md bg-sunken px-3 py-2.5 text-label text-ink-muted">{copy.imageAfterSave}</p>;

  const run = async (action: () => Promise<unknown>) => {
    setBusy(true);
    setError(null);
    try {
      await action();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "No se pudo actualizar la imagen.");
    } finally {
      setBusy(false);
      if (input.current) input.current.value = "";
    }
  };

  return (
    <div className="flex items-center gap-4">
      <div className="grid size-20 shrink-0 place-items-center overflow-hidden rounded-lg border border-line bg-sunken">
        {product.imageUrl ? (
          // eslint-disable-next-line @next/next/no-img-element -- remote Supabase URL, sized by the container
          <img src={product.imageUrl} alt="" className="size-full object-cover" />
        ) : (
          <ImageSquare size={28} className="text-ink-subtle" aria-hidden />
        )}
      </div>
      <div className="min-w-0 space-y-1.5">
        <div className="flex flex-wrap gap-2">
          <input ref={input} type="file" accept="image/jpeg,image/png,image/webp" className="sr-only" id="product-image"
            onChange={(e) => { const file = e.target.files?.[0]; if (file) void run(() => onUpload(file)); }} />
          <Button size="sm" loading={busy} onClick={() => input.current?.click()}>
            {product.imageUrl ? copy.replaceImage : copy.uploadImage}
          </Button>
          {product.imageUrl && (
            <Button size="sm" variant="ghost" disabled={busy} onClick={() => void run(onRemove)}>
              {copy.removeImage}
            </Button>
          )}
        </div>
        {error ? <p className="text-caption font-medium text-danger">{error}</p> : <p className="text-caption text-ink-muted">{copy.imageHint}</p>}
      </div>
    </div>
  );
}
