"use client";

import { Plus } from "@phosphor-icons/react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useDeferredValue, useState } from "react";
import { DataTable, type Column } from "@/components/data/data-table";
import { FilterChips, Pagination, SearchInput } from "@/components/data/toolbar";
import { Money } from "@/components/money";
import { PageHeader } from "@/components/shell/page-header";
import { buttonVariants } from "@/components/ui/button";
import { can } from "@/features/auth/api";
import { useSession } from "@/features/auth/session-store";
import { amountToCents } from "@/features/catalog/api";
import { formatDate } from "@/lib/format/date";
import { purchaseKeys, purchasesApi, type Purchase, type PurchaseFilters } from "../api";
import { purchasesCopy as copy } from "../copy";
import { PurchaseStatusBadge } from "./purchase-status-badge";

export function PurchasesPage() {
  const router = useRouter();
  const me = useSession((s) => s.user);
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState<PurchaseFilters["status"]>("all");
  const [page, setPage] = useState(1);
  const filters: PurchaseFilters = { search: useDeferredValue(search), status, page };
  const purchases = useQuery({ queryKey: purchaseKeys.list(filters), queryFn: () => purchasesApi.list(filters), placeholderData: keepPreviousData });
  const filtered = search.trim() !== "" || status !== "all";

  const columns: Column<Purchase>[] = [
    { header: copy.columns.date, skeleton: 30, cell: (p) => <span className="font-mono tabular text-label">{formatDate(p.purchaseDate)}</span> },
    {
      header: copy.columns.supplier,
      skeleton: 60,
      cell: (p) => (
        <Link href={`/compras/${p.id}`} className="font-medium text-ink hover:underline" onClick={(e) => e.stopPropagation()}>
          {p.supplier.name}
        </Link>
      ),
    },
    {
      header: copy.columns.invoice,
      skeleton: 30,
      cell: (p) => (p.supplierInvoiceNumber ? <span className="font-mono text-label">{p.supplierInvoiceNumber}</span> : <span className="text-label text-ink-subtle">{copy.noInvoice}</span>),
    },
    { header: copy.columns.items, className: "text-right", skeleton: 20, cell: (p) => <span className="font-mono tabular text-label">{p.itemCount}</span> },
    { header: copy.columns.total, className: "text-right", skeleton: 40, cell: (p) => <Money cents={amountToCents(p.total)} className="font-mono" /> },
    { header: copy.columns.status, skeleton: 30, cell: (p) => <PurchaseStatusBadge status={p.status} /> },
  ];

  return (
    <>
      <PageHeader
        title={copy.title}
        description={copy.description}
        actions={
          can(me, "purchases:create") && (
            <Link href="/compras/nueva" className={buttonVariants({ variant: "primary" })}>
              <Plus size={16} weight="bold" aria-hidden />
              {copy.create}
            </Link>
          )
        }
      />
      <div className="space-y-4 px-8 pb-10">
        <div className="flex flex-wrap items-center gap-3">
          <SearchInput value={search} onChange={(v) => { setSearch(v); setPage(1); }} placeholder={copy.search} />
          <FilterChips label={copy.statusLabel} value={status} options={copy.statusFilters} onChange={(v) => { setStatus(v); setPage(1); }} />
        </div>
        <DataTable
          columns={columns}
          rows={purchases.data?.data}
          rowKey={(p) => p.id}
          loading={purchases.isPending}
          error={purchases.isError}
          onRetry={() => purchases.refetch()}
          empty={filtered ? copy.emptyFiltered : copy.empty}
          rowClassName={(p) => (p.status === "voided" ? "text-ink-muted" : undefined)}
          onRowClick={(p) => router.push(`/compras/${p.id}`)}
        />
        <Pagination meta={purchases.data?.meta} onPage={setPage} />
      </div>
    </>
  );
}
