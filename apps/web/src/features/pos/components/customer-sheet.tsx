"use client";

import { Plus, UserCircle } from "@phosphor-icons/react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useDeferredValue, useState } from "react";
import { SearchInput } from "@/components/data/toolbar";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/field";
import { Sheet } from "@/components/ui/sheet";
import { can } from "@/features/auth/api";
import { useSession } from "@/features/auth/session-store";
import { partiesApi, partiesKeys, type Party } from "@/features/parties/api";
import { ApiError } from "@/lib/api/client";
import { posCopy } from "../copy";
import type { Customer } from "../domain";

const copy = posCopy.customer;

const toCustomer = (p: Party): Customer => ({
  id: p.id,
  name: String(p.fullName ?? ""),
  taxId: (p.taxId as string | null) ?? null,
  phone: (p.phone as string | null) ?? null,
});

interface CustomerSheetProps {
  open: boolean;
  onPick: (customer: Customer | null) => void;
  onClose: () => void;
}

export function CustomerSheet({ open, onPick, onClose }: CustomerSheetProps) {
  return (
    <Sheet open={open} onClose={onClose} title={copy.title} description={copy.description}>
      {open && <CustomerPicker onPick={onPick} />}
    </Sheet>
  );
}

function CustomerPicker({ onPick }: { onPick: (customer: Customer | null) => void }) {
  const me = useSession((s) => s.user);
  const [search, setSearch] = useState("");
  const [creating, setCreating] = useState(false);
  const filters = { search: useDeferredValue(search), active: "true", page: 1 } as const;
  const results = useQuery({ queryKey: partiesKeys.list("customers", filters), queryFn: () => partiesApi.list("customers", filters) });
  const customers = (results.data?.data ?? []).filter((p) => !p.isWalkIn);

  if (creating) return <QuickCreate initialName={search} onCreated={onPick} onCancel={() => setCreating(false)} />;

  return (
    <div className="space-y-4">
      <SearchInput value={search} onChange={setSearch} placeholder={copy.search} className="max-w-none" />
      <ul className="divide-y divide-line rounded-lg border border-line">
        <li>
          <button type="button" className="flex w-full items-center gap-3 px-4 py-3 text-left hover:bg-sunken" onClick={() => onPick(null)}>
            <UserCircle size={20} className="text-ink-subtle" aria-hidden />
            <span className="font-medium">{copy.walkIn}</span>
          </button>
        </li>
        {customers.map((p) => (
          <li key={p.id}>
            <button type="button" className="w-full px-4 py-3 text-left hover:bg-sunken" onClick={() => onPick(toCustomer(p))}>
              <span className="block font-medium">{String(p.fullName)}</span>
              {p.taxId && <span className="block font-mono text-caption text-ink-muted">{String(p.taxId)}</span>}
            </button>
          </li>
        ))}
        {results.data && customers.length === 0 && search.trim() && <li className="px-4 py-3 text-label text-ink-muted">{copy.none}</li>}
      </ul>
      {can(me, "customers:create") && (
        <Button onClick={() => setCreating(true)}>
          <Plus size={16} weight="bold" aria-hidden />
          {copy.create}
        </Button>
      )}
    </div>
  );
}

function QuickCreate({ initialName, onCreated, onCancel }: { initialName: string; onCreated: (c: Customer) => void; onCancel: () => void }) {
  const [fullName, setFullName] = useState(initialName);
  const [taxId, setTaxId] = useState("");
  const [email, setEmail] = useState("");
  const [error, setError] = useState<ApiError | null>(null);
  const mutation = useMutation({
    mutationFn: () =>
      partiesApi.create("customers", { fullName: fullName.trim(), taxId: taxId.trim() || null, email: email.trim() || null, phone: null, birthDate: null, address: "" }),
    onSuccess: (p) => onCreated(toCustomer(p)),
    onError: (e) => setError(e instanceof ApiError ? e : new ApiError(0, "unknown", "Ocurrió un error inesperado.")),
  });

  return (
    <form
      className="space-y-4"
      onSubmit={(e) => {
        e.preventDefault();
        mutation.mutate();
      }}
    >
      {error && !error.fields.length && <Alert>{error.message}</Alert>}
      <Field label={copy.name} error={error?.fieldMessage("fullName")}>
        <Input value={fullName} onChange={(e) => setFullName(e.target.value)} autoFocus maxLength={120} />
      </Field>
      <Field label={copy.taxId} error={error?.fieldMessage("taxId")}>
        <Input value={taxId} onChange={(e) => setTaxId(e.target.value)} className="font-mono" maxLength={20} />
      </Field>
      <Field label={copy.email} error={error?.fieldMessage("email")}>
        <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} maxLength={254} />
      </Field>
      <div className="flex justify-end gap-2">
        <Button onClick={onCancel}>{copy.cancel}</Button>
        <Button type="submit" variant="primary" loading={mutation.isPending}>
          {copy.save}
        </Button>
      </div>
    </form>
  );
}
