"use client";

import { ArrowLeft, DownloadSimple, FileArrowUp, Plus } from "@phosphor-icons/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useRef, useState, type DragEvent } from "react";
import { DataTable, type Column } from "@/components/data/data-table";
import { FilterChips, NoticeBanner, useNotice } from "@/components/data/toolbar";
import { Money } from "@/components/money";
import { PageHeader } from "@/components/shell/page-header";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { saveBlob } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { amountToCents, catalogApi, catalogKeys, type ImportReport, type ImportRow } from "../api";
import { importCopy as copy } from "../copy";

type Step = { kind: "pick" } | { kind: "preview"; file: File; report: ImportReport } | { kind: "done"; report: ImportReport };
type RowFilter = (typeof copy.filters)[number]["value"];

const isAmount = (v: string) => /^\d{1,11}(\.\d{1,2})?$/.test(v);

export function ImportPage() {
  const queryClient = useQueryClient();
  const [step, setStep] = useState<Step>({ kind: "pick" });
  const [filter, setFilter] = useState<RowFilter>("all");
  const { notice, failure, clear } = useNotice();

  const preview = useMutation({
    mutationFn: (file: File) => catalogApi.importProducts(file, true).then((report) => ({ file, report })),
    onMutate: clear,
    onSuccess: ({ file, report }) => {
      setFilter(report.invalid > 0 ? "invalid" : "all");
      setStep({ kind: "preview", file, report });
    },
    onError: failure,
  });

  const commit = useMutation({
    mutationFn: (file: File) => catalogApi.importProducts(file, false),
    onMutate: clear,
    onSuccess: (report) => {
      setStep({ kind: "done", report });
      void queryClient.invalidateQueries({ queryKey: catalogKeys.products });
      void queryClient.invalidateQueries({ queryKey: catalogKeys.masters("brands") });
    },
    onError: failure,
  });

  const template = useMutation({
    mutationFn: catalogApi.importTemplate,
    onSuccess: (blob) => saveBlob(blob, "plantilla-productos.xlsx"),
    onError: failure,
  });

  return (
    <>
      <div className="px-8 pt-6">
        <Link href="/productos" className="inline-flex items-center gap-1.5 text-label text-ink-muted transition-colors hover:text-ink">
          <ArrowLeft size={14} weight="bold" aria-hidden />
          {copy.back}
        </Link>
      </div>
      <PageHeader
        title={copy.title}
        description={copy.description}
        actions={
          <Button loading={template.isPending} onClick={() => template.mutate()}>
            <DownloadSimple size={16} weight="bold" aria-hidden />
            {copy.template}
          </Button>
        }
      />
      <div className="space-y-4 px-8 pb-10">
        <NoticeBanner notice={notice} />
        {step.kind === "pick" && <PickStep busy={preview.isPending} onFile={(file) => preview.mutate(file)} />}
        {step.kind === "preview" && (
          <PreviewStep
            file={step.file}
            report={step.report}
            filter={filter}
            onFilter={setFilter}
            busy={commit.isPending}
            onChangeFile={() => setStep({ kind: "pick" })}
            onConfirm={() => commit.mutate(step.file)}
          />
        )}
        {step.kind === "done" && <DoneStep report={step.report} onAgain={() => setStep({ kind: "pick" })} />}
      </div>
    </>
  );
}

function PickStep({ busy, onFile }: { busy: boolean; onFile: (file: File) => void }) {
  const input = useRef<HTMLInputElement>(null);
  const [dragging, setDragging] = useState(false);

  const onDrop = (e: DragEvent) => {
    e.preventDefault();
    setDragging(false);
    const file = e.dataTransfer.files[0];
    if (file && !busy) onFile(file);
  };

  return (
    <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_20rem]">
      <div
        onDragOver={(e) => {
          e.preventDefault();
          setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={onDrop}
        className={cn(
          "grid min-h-64 place-items-center rounded-lg border-2 border-dashed px-6 py-10 text-center transition-colors duration-150",
          dragging ? "border-amber bg-amber-tint" : "border-line-strong bg-surface",
        )}
      >
        <div className="flex flex-col items-center gap-3">
          <FileArrowUp size={32} className="text-ink-subtle" aria-hidden />
          {busy ? (
            <p className="font-medium" aria-live="polite">{copy.drop.reading}</p>
          ) : (
            <>
              <p className="font-medium">{copy.drop.title}</p>
              <p className="text-label text-ink-muted">{copy.drop.or}</p>
            </>
          )}
          <input
            ref={input}
            type="file"
            accept=".xlsx,.csv,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
            className="sr-only"
            id="import-file"
            onChange={(e) => {
              const file = e.target.files?.[0];
              e.target.value = "";
              if (file) onFile(file);
            }}
          />
          <Button variant="primary" loading={busy} onClick={() => input.current?.click()}>
            {copy.drop.choose}
          </Button>
          <p className="text-caption text-ink-muted">{copy.drop.hint}</p>
        </div>
      </div>

      <aside className="space-y-3 rounded-lg border border-line bg-surface p-5">
        <h2 className="font-semibold">{copy.columns.title}</h2>
        <ColumnList label={copy.columns.required} names={copy.columns.requiredList} strong />
        <ColumnList label={copy.columns.optional} names={copy.columns.optionalList} />
        <p className="text-caption text-ink-muted">{copy.columns.note}</p>
      </aside>
    </div>
  );
}

function ColumnList({ label, names, strong }: { label: string; names: readonly string[]; strong?: boolean }) {
  return (
    <div className="space-y-1.5">
      <p className="text-caption font-medium text-ink-muted">{label}</p>
      <ul className="flex flex-wrap gap-1.5">
        {names.map((name) => (
          <li key={name} className={cn("rounded px-1.5 py-0.5 font-mono text-caption", strong ? "bg-amber-tint text-amber-ink" : "bg-sunken text-ink-muted")}>
            {name}
          </li>
        ))}
      </ul>
    </div>
  );
}

interface PreviewStepProps {
  file: File;
  report: ImportReport;
  filter: RowFilter;
  onFilter: (f: RowFilter) => void;
  busy: boolean;
  onChangeFile: () => void;
  onConfirm: () => void;
}

function PreviewStep({ file, report, filter, onFilter, busy, onChangeFile, onConfirm }: PreviewStepProps) {
  const rows = report.rows.filter((r) => filter === "all" || (filter === "invalid") === r.errors.length > 0);

  const columns: Column<ImportRow>[] = [
    { header: copy.table.line, className: "w-16 text-right", skeleton: 20, cell: (r) => <span className="tabular font-mono text-ink-muted">{r.line}</span> },
    { header: copy.table.sku, skeleton: 40, cell: (r) => <span className="font-mono tracking-wide">{r.sku || "—"}</span> },
    {
      header: copy.table.product,
      skeleton: 70,
      cell: (r) => (
        <div className="min-w-0">
          <p className="truncate font-medium">{r.name || "—"}</p>
          {r.brand && (
            <p className="truncate text-caption text-ink-muted">
              {r.brand}
              {r.newBrand && <span className="text-amber-ink"> · {copy.newBrand}</span>}
            </p>
          )}
        </div>
      ),
    },
    { header: copy.table.category, skeleton: 40, cell: (r) => <span className="text-label">{r.category || "—"}</span> },
    {
      header: copy.table.price,
      className: "text-right",
      skeleton: 40,
      cell: (r) => (isAmount(r.price) ? <Money cents={amountToCents(r.price)} className="font-mono" /> : <span className="font-mono text-ink-muted">{r.price || "—"}</span>),
    },
    {
      header: copy.table.result,
      className: "w-[34%]",
      skeleton: 50,
      cell: (r) =>
        r.errors.length === 0 ? (
          <Badge tone="success">{copy.ready}</Badge>
        ) : (
          <ul className="space-y-0.5 text-caption">
            {r.errors.map((e, i) => (
              <li key={i}>
                <span className="font-mono text-danger">{e.field}</span> <span className="text-ink">{e.message}</span>
              </li>
            ))}
          </ul>
        ),
    },
  ];

  return (
    <>
      <div className="flex flex-wrap items-center justify-between gap-4 rounded-lg border border-line bg-surface px-5 py-4">
        <div className="min-w-0 space-y-1">
          <p className="truncate font-medium">{file.name}</p>
          <p className="flex flex-wrap items-center gap-x-3 gap-y-1 text-label text-ink-muted">
            <span className="tabular">{copy.summary.rows(report.total)}</span>
            <span className="tabular text-success">{copy.summary.valid(report.valid)}</span>
            {report.invalid > 0 && <span className="tabular text-danger">{copy.summary.invalid(report.invalid)}</span>}
            {report.newBrands.length > 0 && <span>{copy.summary.newBrands(report.newBrands)}</span>}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="ghost" disabled={busy} onClick={onChangeFile}>
            {copy.changeFile}
          </Button>
          <Button variant="primary" loading={busy} disabled={report.valid === 0} onClick={onConfirm}>
            {copy.submit(report.valid)}
          </Button>
        </div>
      </div>
      {report.valid === 0 ? (
        <Alert>{copy.nothingValid}</Alert>
      ) : (
        report.invalid > 0 && <Alert tone="info">{copy.skipped(report.invalid)}</Alert>
      )}
      <FilterChips label={copy.table.result} value={filter} options={copy.filters} onChange={onFilter} />
      <DataTable
        columns={columns}
        rows={rows}
        rowKey={(r) => String(r.line)}
        loading={false}
        error={false}
        onRetry={() => undefined}
        empty={copy.empty}
        rowClassName={(r) => (r.errors.length > 0 ? "bg-danger-tint/40" : undefined)}
        minWidth={900}
      />
    </>
  );
}

function DoneStep({ report, onAgain }: { report: ImportReport; onAgain: () => void }) {
  return (
    <div className="space-y-4">
      <Alert tone="success">
        {copy.done(report.created)}
        {report.invalid > 0 && copy.doneSkipped(report.invalid)}
      </Alert>
      <div className="flex gap-2">
        <Link href="/productos" className={buttonVariants({ variant: "primary" })}>
          {copy.viewProducts}
        </Link>
        <Button onClick={onAgain}>
          <Plus size={16} weight="bold" aria-hidden />
          {copy.importAnother}
        </Button>
      </div>
    </div>
  );
}
