"use client";

import { PencilSimple, Plus } from "@phosphor-icons/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { DataTable, RowActions, type Column } from "@/components/data/data-table";
import { FilterChips, NoticeBanner, useNotice } from "@/components/data/toolbar";
import { PageHeader } from "@/components/shell/page-header";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { Field, Input, inputClassName } from "@/components/ui/field";
import { Sheet } from "@/components/ui/sheet";
import { can } from "@/features/auth/api";
import { useSession } from "@/features/auth/session-store";
import { applyApiError } from "@/lib/api/form-errors";
import { catalogApi, catalogKeys, type Master, type MasterKind } from "../api";

const KINDS = {
  categories: { label: "Categorías", singular: "categoría", hasParent: true, hasAbbrev: false, hasActive: true },
  brands: { label: "Marcas", singular: "marca", hasParent: false, hasAbbrev: false, hasActive: true },
  units: { label: "Unidades", singular: "unidad", hasParent: false, hasAbbrev: true, hasActive: false },
} as const;

const TABS = (Object.keys(KINDS) as MasterKind[]).map((value) => ({ value, label: KINDS[value].label }));

export function MastersPage() {
  const queryClient = useQueryClient();
  const me = useSession((s) => s.user);
  const [kind, setKind] = useState<MasterKind>("categories");
  const [sheet, setSheet] = useState<{ open: boolean; item: Master | null }>({ open: false, item: null });
  const { notice, success, failure } = useNotice();
  const config = KINDS[kind];

  const items = useQuery({ queryKey: catalogKeys.masters(kind), queryFn: () => catalogApi.masters(kind) });
  const refresh = () => queryClient.invalidateQueries({ queryKey: catalogKeys.masters(kind) });

  const statusMutation = useMutation({
    mutationFn: ({ item, value }: { item: Master; value: boolean }) => catalogApi.setMasterActive(kind, item.id, value),
    onSuccess: (item) => {
      success(`${item.name} ${item.active ? "se activó" : "se desactivó"}.`);
      void refresh();
    },
    onError: failure,
  });

  const parentName = (id?: string) => items.data?.find((c) => c.id === id)?.name;

  const columns: Column<Master>[] = [
    {
      header: "Nombre",
      skeleton: 50,
      cell: (item) => (
        <div>
          <p className="font-medium text-ink">{item.name}</p>
          {item.parentId && <p className="text-caption text-ink-muted">Dentro de {parentName(item.parentId)}</p>}
        </div>
      ),
    },
    ...(config.hasAbbrev ? [{ header: "Abreviatura", skeleton: 20, cell: (item: Master) => <span className="font-mono text-label">{item.abbreviation}</span> }] : []),
    { header: "Productos", className: "text-right", skeleton: 15, cell: (item) => <span className="tabular font-mono text-label">{item.productCount}</span> },
    ...(config.hasActive
      ? [{ header: "Estado", skeleton: 20, cell: (item: Master) => (item.active ? <Badge tone="success">Activa</Badge> : <Badge>Inactiva</Badge>) }]
      : []),
    {
      header: <span className="sr-only">Acciones</span>,
      className: "w-px px-2",
      skeleton: 0,
      cell: (item) => (
        <RowActions>
          {can(me, "categories:update") && (
            <Button variant="ghost" size="icon-sm" aria-label={`Editar ${item.name}`} onClick={() => setSheet({ open: true, item })}>
              <PencilSimple size={16} />
            </Button>
          )}
          {config.hasActive && can(me, "categories:delete") && (item.active ? (
            <ConfirmButton size="sm" confirmLabel="¿Confirmar?" onConfirm={() => statusMutation.mutate({ item, value: false })}>
              Desactivar
            </ConfirmButton>
          ) : (
            <Button variant="ghost" size="sm" onClick={() => statusMutation.mutate({ item, value: true })}>
              Activar
            </Button>
          ))}
        </RowActions>
      ),
    },
  ];

  return (
    <>
      <PageHeader
        title="Categorías, marcas y unidades"
        description="Clasificación del catálogo. Desactivar una categoría o marca la oculta al crear productos, sin afectar los existentes."
        actions={
          can(me, "categories:create") && (
            <Button variant="primary" onClick={() => setSheet({ open: true, item: null })}>
              <Plus size={16} weight="bold" aria-hidden />
              Nueva {config.singular}
            </Button>
          )
        }
      />
      <div className="space-y-4 px-8 pb-10">
        <FilterChips label="Tipo" value={kind} options={TABS} onChange={setKind} />
        <NoticeBanner notice={notice} />
        <DataTable
          columns={columns}
          rows={items.data}
          rowKey={(i) => i.id}
          loading={items.isPending}
          error={items.isError}
          onRetry={() => items.refetch()}
          empty={{ title: "Sin registros", body: `Agrega la primera ${config.singular}.` }}
          minWidth={560}
        />
      </div>

      <Sheet open={sheet.open} onClose={() => setSheet({ open: false, item: null })} title={sheet.item ? `Editar ${config.singular}` : `Nueva ${config.singular}`}>
        <MasterForm
          key={`${kind}-${sheet.item?.id ?? "new"}`}
          kind={kind}
          item={sheet.item}
          categories={kind === "categories" ? (items.data ?? []) : []}
          onCancel={() => setSheet({ open: false, item: null })}
          onSaved={(saved) => {
            setSheet({ open: false, item: null });
            success(`${saved.name} se guardó.`);
            void refresh();
          }}
        />
      </Sheet>
    </>
  );
}

interface MasterFormProps {
  kind: MasterKind;
  item: Master | null;
  categories: Master[];
  onCancel: () => void;
  onSaved: (item: Master) => void;
}

function MasterForm({ kind, item, categories, onCancel, onSaved }: MasterFormProps) {
  const config = KINDS[kind];
  const [formError, setFormError] = useState<string | null>(null);
  const form = useForm({ defaultValues: { name: item?.name ?? "", abbreviation: item?.abbreviation ?? "", parentId: item?.parentId ?? "" } });
  const { errors, isSubmitting } = form.formState;

  const submit = form.handleSubmit(async (values) => {
    setFormError(null);
    const body = {
      name: values.name,
      ...(config.hasAbbrev && { abbreviation: values.abbreviation }),
      ...(config.hasParent && { parentId: values.parentId || null }),
    };
    try {
      onSaved(item ? await catalogApi.updateMaster(kind, item.id, body) : await catalogApi.createMaster(kind, body));
    } catch (error) {
      setFormError(applyApiError(error, form.setError, ["name", "abbreviation", "parentId"]));
    }
  });

  return (
    <form onSubmit={submit} noValidate className="space-y-5">
      {formError && <Alert>{formError}</Alert>}
      <Field label="Nombre" error={errors.name?.message}>
        <Input autoFocus {...form.register("name", { required: "Este campo es obligatorio.", minLength: { value: 2, message: "Escribe al menos 2 caracteres." } })} />
      </Field>
      {config.hasAbbrev && (
        <Field label="Abreviatura" error={errors.abbreviation?.message}>
          <Input className="font-mono" {...form.register("abbreviation", { required: "Este campo es obligatorio." })} />
        </Field>
      )}
      {config.hasParent && (
        <Field label="Categoría superior" hint="Opcional: para agrupar, por ejemplo Vino dentro de Bebidas." error={errors.parentId?.message}>
          <select className={inputClassName} {...form.register("parentId")}>
            <option value="">Ninguna</option>
            {categories.filter((c) => c.id !== item?.id && c.active).map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
        </Field>
      )}
      <div className="flex justify-end gap-2 border-t border-line pt-5">
        <Button variant="ghost" onClick={onCancel}>
          Cancelar
        </Button>
        <Button type="submit" variant="primary" loading={isSubmitting}>
          Guardar
        </Button>
      </div>
    </form>
  );
}
