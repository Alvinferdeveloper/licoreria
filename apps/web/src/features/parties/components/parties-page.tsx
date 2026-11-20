"use client";

import { PencilSimple, Plus } from "@phosphor-icons/react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useDeferredValue, useState } from "react";
import { useForm } from "react-hook-form";
import { DataTable, RowActions, type Column } from "@/components/data/data-table";
import { FilterChips, NoticeBanner, Pagination, SearchInput, STATUS_OPTIONS, useNotice, type StatusFilter } from "@/components/data/toolbar";
import { PageHeader } from "@/components/shell/page-header";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { Field, Input } from "@/components/ui/field";
import { Sheet } from "@/components/ui/sheet";
import { can } from "@/features/auth/api";
import { useSession } from "@/features/auth/session-store";
import { applyApiError } from "@/lib/api/form-errors";
import { cn } from "@/lib/cn";
import { partiesApi, partiesKeys, type Party, type PartyFilters, type PartyKind } from "../api";
import { partyConfigs, type PartyConfig } from "../config";
import { partiesCopy as copy } from "../copy";

/** Takes only the kind so server pages can render it; the config holds functions that stay client-side. */
export function PartiesPage({ kind: partyKind }: { kind: PartyKind }) {
  const config = partyConfigs[partyKind];
  const queryClient = useQueryClient();
  const me = useSession((s) => s.user);
  const [search, setSearch] = useState("");
  const [active, setActive] = useState<StatusFilter>("all");
  const [page, setPage] = useState(1);
  const [sheet, setSheet] = useState<{ open: boolean; item: Party | null }>({ open: false, item: null });
  const { notice, success, failure } = useNotice();
  const { kind, permission } = config;

  const filters: PartyFilters = { search: useDeferredValue(search), active, page };
  const list = useQuery({ queryKey: partiesKeys.list(kind, filters), queryFn: () => partiesApi.list(kind, filters), placeholderData: keepPreviousData });
  const refresh = () => queryClient.invalidateQueries({ queryKey: partiesKeys.all(kind) });
  const nameOf = (row: Party) => String(row[config.nameField] ?? "");

  const statusMutation = useMutation({
    mutationFn: ({ item, value }: { item: Party; value: boolean }) => partiesApi.setActive(kind, item.id, value),
    onSuccess: (item) => {
      success(copy.notices.toggled(nameOf(item), item.active));
      void refresh();
    },
    onError: failure,
  });

  const columns: Column<Party>[] = [
    {
      header: copy.columns.name,
      skeleton: 60,
      cell: (row) => (
        <div>
          <p className="font-medium text-ink">{nameOf(row)}</p>
          <p className="text-caption text-ink-muted">{config.detail(row) || copy.noContact}</p>
        </div>
      ),
    },
    { header: copy.columns.email, skeleton: 40, cell: (row) => <span className="text-label text-ink-muted">{String(row.email ?? "")}</span> },
    {
      header: copy.columns.status,
      skeleton: 25,
      cell: (row) =>
        row.isWalkIn ? <Badge tone="amber">{copy.status.walkIn}</Badge> : row.active ? <Badge tone="success">{copy.status.active}</Badge> : <Badge>{copy.status.inactive}</Badge>,
    },
    {
      header: <span className="sr-only">{copy.columns.actions}</span>,
      className: "w-px px-2",
      skeleton: 0,
      cell: (row) =>
        row.isWalkIn ? null : (
          <RowActions>
            {can(me, `${permission}:update`) && (
              <Button variant="ghost" size="icon-sm" aria-label={copy.actions.edit(nameOf(row))} onClick={() => setSheet({ open: true, item: row })}>
                <PencilSimple size={16} />
              </Button>
            )}
            {can(me, `${permission}:delete`) && (row.active ? (
              <ConfirmButton size="sm" confirmLabel={copy.actions.confirm} onConfirm={() => statusMutation.mutate({ item: row, value: false })}>
                {copy.actions.deactivate}
              </ConfirmButton>
            ) : (
              <Button variant="ghost" size="sm" onClick={() => statusMutation.mutate({ item: row, value: true })}>
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
        title={config.title}
        description={config.description}
        actions={
          can(me, `${permission}:create`) && (
            <Button variant="primary" onClick={() => setSheet({ open: true, item: null })}>
              <Plus size={16} weight="bold" aria-hidden />
              {config.create}
            </Button>
          )
        }
      />
      <div className="space-y-4 px-8 pb-10">
        <div className="flex flex-wrap items-center gap-3">
          <SearchInput value={search} onChange={(v) => { setSearch(v); setPage(1); }} placeholder={config.search} />
          <FilterChips label={copy.statusFilter} value={active} options={STATUS_OPTIONS} onChange={(v) => { setActive(v); setPage(1); }} />
        </div>
        <NoticeBanner notice={notice} />
        <DataTable
          columns={columns}
          rows={list.data?.data}
          rowKey={(r) => r.id}
          loading={list.isPending}
          error={list.isError}
          onRetry={() => list.refetch()}
          empty={copy.empty(config.singular)}
          rowClassName={(r) => (r.active ? undefined : "text-ink-muted")}
          minWidth={640}
        />
        <Pagination meta={list.data?.meta} onPage={setPage} />
      </div>

      <Sheet open={sheet.open} onClose={() => setSheet({ open: false, item: null })} title={sheet.item ? copy.editTitle(config.singular) : config.create}>
        <PartyForm
          key={sheet.item?.id ?? "new"}
          config={config}
          item={sheet.item}
          onCancel={() => setSheet({ open: false, item: null })}
          onSaved={(saved) => {
            setSheet({ open: false, item: null });
            success(copy.notices.saved(nameOf(saved)));
            void refresh();
          }}
        />
      </Sheet>
    </>
  );
}

function PartyForm({ config, item, onCancel, onSaved }: { config: PartyConfig; item: Party | null; onCancel: () => void; onSaved: (p: Party) => void }) {
  const [formError, setFormError] = useState<string | null>(null);
  const defaults = Object.fromEntries(config.fields.map((f) => [f.name, item?.[f.name] == null ? "" : String(item[f.name])]));
  const form = useForm<Record<string, string>>({ defaultValues: defaults });
  const { errors, isSubmitting } = form.formState;

  const submit = form.handleSubmit(async (values) => {
    setFormError(null);
    const body = Object.fromEntries(
      config.fields.map((f) => {
        const value = values[f.name]?.trim() ?? "";
        if (f.type === "number") return [f.name, Number(value || 0)];
        return [f.name, f.nullable && value === "" ? null : value];
      }),
    );
    try {
      onSaved(item ? await partiesApi.update(config.kind, item.id, body) : await partiesApi.create(config.kind, body));
    } catch (error) {
      setFormError(applyApiError(error, form.setError, config.fields.map((f) => f.name)));
    }
  });

  return (
    <form onSubmit={submit} noValidate className="space-y-5">
      {formError && <Alert>{formError}</Alert>}
      <div className="grid gap-4 sm:grid-cols-2">
        {config.fields.map((f, i) => (
          <Field key={f.name} label={f.label} hint={f.hint} error={errors[f.name]?.message} className={cn(f.wide && "sm:col-span-2")}>
            <Input
              type={f.type === "number" ? "text" : (f.type ?? "text")}
              inputMode={f.type === "number" ? "numeric" : undefined}
              autoFocus={i === 0}
              placeholder={f.placeholder}
              className={cn(f.mono && "font-mono")}
              {...form.register(f.name, f.required ? { required: copy.form.required } : {})}
            />
          </Field>
        ))}
      </div>
      <div className="flex justify-end gap-2 border-t border-line pt-5">
        <Button variant="ghost" onClick={onCancel}>
          {copy.form.cancel}
        </Button>
        <Button type="submit" variant="primary" loading={isSubmitting}>
          {copy.form.save}
        </Button>
      </div>
    </form>
  );
}
