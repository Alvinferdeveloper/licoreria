"use client";

import { ArrowLeft, FileXls, FilePdf, X } from "@phosphor-icons/react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { Fragment, useDeferredValue, useState } from "react";
import { PageHeader } from "@/components/shell/page-header";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input, inputClassName } from "@/components/ui/field";
import { ProductPicker } from "@/features/catalog/components/product-picker";
import { ApiError, saveBlob } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { formatDate, formatDateTime, todayISO } from "@/lib/format/date";
import { amountToCents, formatAmount } from "@/lib/money";
import { reportKeys, reportsApi, type ColumnKind, type ReportCatalog, type ReportColumn, type ReportDefinition, type ReportFilters, type ReportResult } from "../api";
import { reportsCopy as copy } from "../copy";

const optionList: Partial<Record<string, keyof ReportCatalog["options"]>> = {
  user: "users",
  category: "categories",
  brand: "brands",
  supplier: "suppliers",
  register: "registers",
};

/** First day of the current month and today, for reports with a date range. */
function defaults(def: ReportDefinition): ReportFilters {
  const today = todayISO();
  const values: ReportFilters = {};
  for (const p of def.params) {
    if (p.key === "from") values.from = `${today.slice(0, 8)}01`;
    if (p.key === "to") values.to = today;
  }
  return values;
}

export function ReportView({ reportKey }: { reportKey: string }) {
  const catalog = useQuery({ queryKey: reportKeys.catalog, queryFn: reportsApi.catalog });
  const def = catalog.data?.reports.find((r) => r.key === reportKey);

  if (catalog.isPending) return <p className="px-8 py-10 text-ink-muted">Cargando…</p>;
  if (!def || !catalog.data) return <Alert className="m-8">Ese reporte no existe o no tienes acceso.</Alert>;
  return <ReportScreen key={def.key} def={def} catalog={catalog.data} />;
}

function ReportScreen({ def, catalog }: { def: ReportDefinition; catalog: ReportCatalog }) {
  const [filters, setFilters] = useState<ReportFilters>(() => defaults(def));
  const [productName, setProductName] = useState("");
  const [exporting, setExporting] = useState<"xlsx" | "pdf" | null>(null);
  const [exportError, setExportError] = useState<string | null>(null);
  const applied = useDeferredValue(filters);
  const missing = def.params.find((p) => p.required && !applied[p.key]);
  const result = useQuery({
    queryKey: reportKeys.run(def.key, applied),
    queryFn: () => reportsApi.run(def.key, applied),
    enabled: !missing,
    placeholderData: keepPreviousData,
  });
  const set = (key: string, value: string) => setFilters((f) => ({ ...f, [key]: value }));

  const download = async (format: "xlsx" | "pdf") => {
    setExporting(format);
    setExportError(null);
    try {
      saveBlob(await reportsApi.export(def.key, applied, format), `${def.key}-${todayISO()}.${format}`);
    } catch {
      setExportError(copy.exportError);
    } finally {
      setExporting(null);
    }
  };

  return (
    <>
      <div className="px-8 pt-6">
        <Link href="/reportes" className="inline-flex items-center gap-1.5 text-label text-ink-muted transition-colors hover:text-ink">
          <ArrowLeft size={14} weight="bold" aria-hidden />
          {copy.back}
        </Link>
      </div>
      <PageHeader
        title={def.title}
        description={def.description}
        actions={
          <>
            <Button loading={exporting === "xlsx"} disabled={!!missing} onClick={() => void download("xlsx")}>
              <FileXls size={16} aria-hidden />
              {copy.excel}
            </Button>
            <Button loading={exporting === "pdf"} disabled={!!missing} onClick={() => void download("pdf")}>
              <FilePdf size={16} aria-hidden />
              {copy.pdf}
            </Button>
          </>
        }
      />
      <div className="space-y-4 px-8 pb-10">
        <div className="flex flex-wrap items-end gap-3">
          {def.params.map((p) => {
            const value = filters[p.key] ?? "";
            const list = optionList[p.kind];
            return (
              <label key={p.key} className="flex flex-col gap-1 text-caption font-medium text-ink-muted">
                {p.label}
                {p.kind === "date" ? (
                  <Input type="date" value={value} onChange={(e) => set(p.key, e.target.value)} className="h-9 w-auto" />
                ) : list ? (
                  <select value={value} onChange={(e) => set(p.key, e.target.value)} className={cn(inputClassName, "h-9 w-auto min-w-40")}>
                    <option value="">{copy.all}</option>
                    {(catalog.options[list] ?? []).map((o) => (
                      <option key={o.id} value={o.id}>
                        {o.name}
                      </option>
                    ))}
                  </select>
                ) : p.kind === "choice" ? (
                  <select value={value} onChange={(e) => set(p.key, e.target.value)} className={cn(inputClassName, "h-9 w-auto min-w-36")}>
                    <option value="">{copy.all}</option>
                    {p.choices?.map((c) => (
                      <option key={c.value} value={c.value}>
                        {c.label}
                      </option>
                    ))}
                  </select>
                ) : p.kind === "bool" ? (
                  <span className="flex h-9 items-center">
                    <Checkbox label={p.label} checked={value === "true"} onChange={(e) => set(p.key, e.target.checked ? "true" : "")} />
                  </span>
                ) : p.kind === "product" ? (
                  value ? (
                    <span className="flex h-9 items-center gap-2 rounded-md border border-line bg-surface pr-1 pl-3 text-label font-normal text-ink">
                      {productName}
                      <Button variant="ghost" size="icon-sm" aria-label={copy.clearProduct} onClick={() => set(p.key, "")}>
                        <X size={14} />
                      </Button>
                    </span>
                  ) : (
                    <ProductPicker placeholder={copy.pickProduct} className="w-72 font-normal" onPick={(prod) => { setProductName(prod.name); set(p.key, prod.id); }} />
                  )
                ) : (
                  <Input value={value} onChange={(e) => set(p.key, e.target.value)} placeholder={copy.textFilter} className="h-9 w-48" />
                )}
              </label>
            );
          })}
        </div>

        {exportError && <Alert>{exportError}</Alert>}
        {missing ? (
          <Alert tone="info">{copy.required(missing.label)}</Alert>
        ) : result.isError ? (
          <Alert>{result.error instanceof ApiError ? result.error.message : "No pudimos generar el reporte."}</Alert>
        ) : (
          <ReportTable def={def} result={result.data} loading={result.isPending} stale={result.isFetching} />
        )}
      </div>
    </>
  );
}

function cell(value: string | number | null | undefined, kind: ColumnKind): string {
  if (value === null || value === undefined || value === "") return "—";
  switch (kind) {
    case "money":
      return formatAmount(amountToCents(String(value)));
    case "datetime":
      return formatDateTime(String(value));
    case "date":
      return formatDate(String(value).slice(0, 10));
    case "percent":
      return `${value} %`;
    case "number":
      return Number(value).toLocaleString("es-NI");
    default:
      return String(value);
  }
}

const numeric = (c: ReportColumn) => c.kind === "money" || c.kind === "number" || c.kind === "percent";

function ReportTable({ def, result, loading, stale }: { def: ReportDefinition; result: ReportResult | undefined; loading: boolean; stale: boolean }) {
  const columns = def.columns.filter((c) => c.key !== def.groupBy);
  if (loading || !result) return <div className="h-64 animate-pulse rounded-lg bg-sunken" aria-hidden />;

  return (
    <div className={cn("space-y-2 transition-opacity", stale && "opacity-60")}>
      <p className="text-caption text-ink-muted">{copy.rows(result.rows.length)}</p>
      <div className="overflow-x-auto rounded-lg border border-line bg-surface">
        <table className="w-full min-w-[760px] border-collapse text-left text-label">
          <thead className="bg-chrome text-caption text-ink-muted">
            <tr>
              {columns.map((c) => (
                <th key={c.key} className={cn("px-3 py-2.5 font-medium", numeric(c) && "text-right")}>
                  {c.label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {result.rows.map((row, i) => {
              const group = def.groupBy ? row[def.groupBy] : undefined;
              const header = def.groupBy && (i === 0 || result.rows[i - 1][def.groupBy] !== group);
              return (
                <Fragment key={i}>
                  {header && (
                    <tr className="bg-paper">
                      <th colSpan={columns.length} className="border-t border-line px-3 py-2 text-left font-semibold">
                        {String(group ?? "—")}
                      </th>
                    </tr>
                  )}
                  <tr className="border-t border-line">
                    {columns.map((c) => (
                      <td key={c.key} className={cn("px-3 py-2", numeric(c) && "text-right tabular", (c.kind === "mono" || numeric(c)) && "font-mono", c.kind === "money" && String(row[c.key]).startsWith("-") && "text-danger")}>
                        {cell(row[c.key], c.kind)}
                      </td>
                    ))}
                  </tr>
                </Fragment>
              );
            })}
          </tbody>
          {result.totals && result.rows.length > 0 && (
            <tfoot className="border-t-2 border-line-strong font-semibold">
              <tr>
                {columns.map((c, i) => (
                  <td key={c.key} className={cn("px-3 py-2.5", numeric(c) && "text-right font-mono tabular")}>
                    {i === 0 ? copy.total : c.key in result.totals! ? cell(result.totals![c.key], c.kind) : ""}
                  </td>
                ))}
              </tr>
            </tfoot>
          )}
        </table>
        {result.rows.length === 0 && (
          <div className="px-4 py-12 text-center">
            <p className="font-semibold">{copy.empty.title}</p>
            <p className="text-ink-muted">{copy.empty.body}</p>
          </div>
        )}
      </div>
      {result.truncated && <p className="text-caption text-ink-muted">{copy.truncated(result.rows.length)}</p>}
    </div>
  );
}
