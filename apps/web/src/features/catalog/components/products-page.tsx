"use client";

import { FileArrowUp, ImageSquare, PencilSimple, Plus, Warning, XCircle } from "@phosphor-icons/react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useDeferredValue, useState } from "react";
import { DataTable, RowActions, type Column } from "@/components/data/data-table";
import { FilterChips, NoticeBanner, Pagination, SearchInput, STATUS_OPTIONS, useNotice, type StatusFilter } from "@/components/data/toolbar";
import { Money } from "@/components/money";
import { PageHeader } from "@/components/shell/page-header";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { inputClassName } from "@/components/ui/field";
import { Sheet } from "@/components/ui/sheet";
import { can } from "@/features/auth/api";
import { useSession } from "@/features/auth/session-store";
import { salesApi, salesKeys } from "@/features/sales/api";
import { cn } from "@/lib/cn";
import { amountToCents, catalogApi, catalogKeys, type Product, type ProductFilters, type ProductInput } from "../api";
import { productsCopy as copy } from "../copy";
import { ProductForm } from "./product-form";

export function ProductsPage() {
  const queryClient = useQueryClient();
  const me = useSession((s) => s.user);
  const [search, setSearch] = useState("");
  const [categoryId, setCategoryId] = useState("");
  const [active, setActive] = useState<StatusFilter>("all");
  const [lowStock, setLowStock] = useState(false);
  const [page, setPage] = useState(1);
  const [sheet, setSheet] = useState<{ open: boolean; product: Product | null }>({ open: false, product: null });
  const { notice, success, failure } = useNotice();

  const filters: ProductFilters = { search: useDeferredValue(search), categoryId, active, lowStock, page };
  const products = useQuery({ queryKey: catalogKeys.productList(filters), queryFn: () => catalogApi.products(filters), placeholderData: keepPreviousData });
  const categories = useQuery({ queryKey: catalogKeys.masters("categories"), queryFn: () => catalogApi.masters("categories") });
  const brands = useQuery({ queryKey: catalogKeys.masters("brands"), queryFn: () => catalogApi.masters("brands") });
  const units = useQuery({ queryKey: catalogKeys.masters("units"), queryFn: () => catalogApi.masters("units") });
  const store = useQuery({ queryKey: salesKeys.store, queryFn: salesApi.store });
  const refresh = () => queryClient.invalidateQueries({ queryKey: catalogKeys.products });
  const resetPage = <T,>(set: (v: T) => void) => (v: T) => { set(v); setPage(1); };

  const statusMutation = useMutation({
    mutationFn: ({ product, value }: { product: Product; value: boolean }) => catalogApi.setProductActive(product.id, value),
    onSuccess: (p) => {
      success(p.active ? copy.notices.activated(p.name) : copy.notices.deactivated(p.name));
      void refresh();
    },
    onError: failure,
  });

  const save = async (input: ProductInput) => {
    const editing = sheet.product;
    const saved = editing ? await catalogApi.updateProduct(editing.id, input) : await catalogApi.createProduct(input);
    success(editing ? copy.notices.updated : copy.notices.created(saved.name));
    // A new product stays open so its image can be added right away.
    setSheet(editing ? { open: false, product: null } : { open: true, product: saved });
    await refresh();
  };

  const updateImage = async (action: () => Promise<Product>, message: string) => {
    const updated = await action();
    setSheet({ open: true, product: updated });
    success(message);
    await refresh();
    return updated;
  };

  const canUpdate = can(me, "products:update");
  const canToggle = can(me, "products:delete");

  const columns: Column<Product>[] = [
    {
      header: copy.columns.product,
      skeleton: 70,
      cell: (p) => (
        <div className="flex items-center gap-3">
          <div className="grid size-10 shrink-0 place-items-center overflow-hidden rounded-md border border-line bg-sunken">
            {/* eslint-disable-next-line @next/next/no-img-element -- remote Supabase thumbnails */}
            {p.imageUrl ? <img src={p.imageUrl} alt="" className="size-full object-cover" loading="lazy" /> : <ImageSquare size={18} className="text-ink-subtle" aria-hidden />}
          </div>
          <div className="min-w-0">
            <p className="truncate font-medium text-ink">{p.name}</p>
            <p className="truncate text-caption text-ink-muted">
              <span className="font-mono tracking-wide">{p.sku}</span>
              {p.presentations.some((pr) => pr.active) && ` · ${copy.presentations(p.presentations.filter((pr) => pr.active).length)}`}
            </p>
          </div>
        </div>
      ),
    },
    { header: copy.columns.category, skeleton: 40, cell: (p) => <span className="text-label">{p.category.name}</span> },
    { header: copy.columns.price, className: "text-right", skeleton: 50, cell: (p) => <Money cents={amountToCents(p.price)} className="font-mono" /> },
    { header: copy.columns.stock, className: "text-right", skeleton: 30, cell: (p) => <StockCell product={p} /> },
    {
      header: copy.columns.status,
      skeleton: 30,
      cell: (p) => (p.active ? <Badge tone="success">{copy.status.active}</Badge> : <Badge>{copy.status.inactive}</Badge>),
    },
    {
      header: <span className="sr-only">Acciones</span>,
      className: "w-px px-2",
      skeleton: 0,
      cell: (p) => (
        <RowActions>
          {canUpdate && (
            <Button variant="ghost" size="icon-sm" aria-label={`${copy.actions.edit} ${p.name}`} title={copy.actions.edit} onClick={() => setSheet({ open: true, product: p })}>
              <PencilSimple size={16} />
            </Button>
          )}
          {canToggle && (p.active ? (
            <ConfirmButton size="sm" confirmLabel={copy.actions.confirm} onConfirm={() => statusMutation.mutate({ product: p, value: false })}>
              {copy.actions.deactivate}
            </ConfirmButton>
          ) : (
            <Button variant="ghost" size="sm" onClick={() => statusMutation.mutate({ product: p, value: true })}>
              {copy.actions.activate}
            </Button>
          ))}
        </RowActions>
      ),
    },
  ];

  return (
    <>
      <PageHeader
        title={copy.title}
        description={copy.description}
        actions={
          can(me, "products:create") && (
            <>
              {can(me, "products:authorize") && (
                <Link href="/productos/importar" className={buttonVariants()}>
                  <FileArrowUp size={16} weight="bold" aria-hidden />
                  {copy.import}
                </Link>
              )}
              <Button variant="primary" onClick={() => setSheet({ open: true, product: null })}>
                <Plus size={16} weight="bold" aria-hidden />
                {copy.create}
              </Button>
            </>
          )
        }
      />
      <div className="space-y-4 px-8 pb-10">
        <div className="flex flex-wrap items-center gap-3">
          <SearchInput value={search} onChange={resetPage(setSearch)} placeholder={copy.search} />
          <select value={categoryId} onChange={(e) => resetPage(setCategoryId)(e.target.value)} aria-label={copy.columns.category} className={cn(inputClassName, "h-9 w-auto")}>
            <option value="">{copy.allCategories}</option>
            {categories.data?.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
          <FilterChips label="Estado" value={active} options={STATUS_OPTIONS} onChange={resetPage(setActive)} />
          <button
            type="button"
            aria-pressed={lowStock}
            onClick={() => resetPage(setLowStock)(!lowStock)}
            className={cn("press flex h-7 items-center gap-1.5 rounded-full px-3 text-caption font-medium transition-colors", lowStock ? "bg-warning-tint text-warning" : "bg-sunken text-ink-muted hover:text-ink")}
          >
            <Warning size={12} weight="bold" aria-hidden />
            {copy.lowStock}
          </button>
        </div>
        <NoticeBanner notice={notice} />
        <DataTable
          columns={columns}
          rows={products.data?.data}
          rowKey={(p) => p.id}
          loading={products.isPending}
          error={products.isError}
          onRetry={() => products.refetch()}
          empty={copy.empty}
          rowClassName={(p) => (p.active ? undefined : "text-ink-muted")}
          minWidth={860}
        />
        <Pagination meta={products.data?.meta} onPage={setPage} />
      </div>

      <Sheet open={sheet.open} onClose={() => setSheet({ open: false, product: null })} title={sheet.product ? copy.form.editTitle : copy.form.createTitle} size="lg">
        <ProductForm
          key={sheet.product?.id ?? `new-${store.data?.tax.defaultPercent}`}
          product={sheet.product}
          categories={categories.data ?? []}
          brands={brands.data ?? []}
          units={units.data ?? []}
          canEditPrices={can(me, "products:authorize")}
          defaultTax={store.data?.tax.defaultPercent}
          onCancel={() => setSheet({ open: false, product: null })}
          onSubmit={save}
          onUploadImage={(file) => updateImage(() => catalogApi.uploadImage(sheet.product!.id, file), copy.notices.imageSaved)}
          onRemoveImage={() => updateImage(() => catalogApi.removeImage(sheet.product!.id), copy.notices.imageRemoved)}
        />
      </Sheet>
    </>
  );
}

function StockCell({ product }: { product: Product }) {
  if (product.stock <= 0) {
    return (
      <Badge tone="danger">
        <XCircle size={12} weight="bold" aria-hidden />
        {copy.stock.out}
      </Badge>
    );
  }
  if (product.stock <= product.minStock) {
    return (
      <Badge tone="warning">
        <Warning size={12} weight="bold" aria-hidden />
        <span className="tabular">{product.stock}</span>
      </Badge>
    );
  }
  return <span className="tabular font-mono text-label">{product.stock}</span>;
}
