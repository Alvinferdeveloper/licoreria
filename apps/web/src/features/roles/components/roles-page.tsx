"use client";

import { LockSimple } from "@phosphor-icons/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { PageHeader } from "@/components/shell/page-header";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { can } from "@/features/auth/api";
import { useSession } from "@/features/auth/session-store";
import { ApiError } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { isAdminRole, rolesApi, rolesKeys, type PermissionDef, type RoleDetail } from "../api";
import { rolesCopy as copy } from "../copy";

const ACTIONS = ["view", "create", "update", "delete", "void", "authorize"] as const;

export function RolesPage() {
  const queryClient = useQueryClient();
  const me = useSession((s) => s.user);
  const roles = useQuery({ queryKey: rolesKeys.all, queryFn: rolesApi.list });
  const permissions = useQuery({ queryKey: rolesKeys.permissions, queryFn: rolesApi.permissions });
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [draft, setDraft] = useState<Set<string> | null>(null);
  const [notice, setNotice] = useState<{ tone: "success" | "danger"; text: string } | null>(null);

  const selected = roles.data?.find((r) => r.id === selectedId) ?? roles.data?.[0];
  const current = useMemo(() => new Set(selected?.permissions ?? []), [selected]);
  const working = draft ?? current;
  const readOnly = !selected || isAdminRole(selected) || !can(me, "roles:update");
  const changes = symmetricDifference(working, current);

  const matrix = useMemo(() => groupByModule(permissions.data ?? []), [permissions.data]);

  const save = useMutation({
    mutationFn: () => rolesApi.setPermissions(selected!.id, [...working]),
    onSuccess: (role) => {
      queryClient.setQueryData<RoleDetail[]>(rolesKeys.all, (list) => list?.map((r) => (r.id === role.id ? role : r)));
      setDraft(null);
      setNotice({ tone: "success", text: copy.saved(role.name) });
    },
    onError: (error) => setNotice({ tone: "danger", text: error instanceof ApiError ? error.message : "Ocurrió un error inesperado." }),
  });

  const selectRole = (role: RoleDetail) => {
    setSelectedId(role.id);
    setDraft(null);
    setNotice(null);
  };

  const toggle = (key: string) => {
    const next = new Set(working);
    if (next.has(key)) next.delete(key);
    else next.add(key);
    setDraft(next);
  };

  return (
    <>
      <PageHeader title={copy.title} description={copy.description} />

      {(roles.isError || permissions.isError) && (
        <div className="px-8">
          <Alert>{copy.error}</Alert>
        </div>
      )}

      <div className="grid gap-6 px-8 pb-10 lg:grid-cols-[240px_minmax(0,1fr)]">
        <nav aria-label="Roles" className="space-y-1">
          {roles.data?.map((role) => {
            const active = role.id === selected?.id;
            return (
              <button
                key={role.id}
                type="button"
                aria-current={active ? "true" : undefined}
                onClick={() => selectRole(role)}
                className={cn(
                  "press w-full rounded-md px-3 py-2.5 text-left transition-colors duration-150",
                  active ? "bg-surface shadow-(--shadow-float-low)" : "hover:bg-sunken",
                )}
              >
                <span className="flex items-center gap-2 font-medium">
                  {role.name}
                  {isAdminRole(role) && <LockSimple size={14} className="text-ink-subtle" aria-label="Bloqueado" />}
                </span>
                <span className="text-caption text-ink-muted">{copy.users(role.userCount)}</span>
              </button>
            );
          })}
        </nav>

        {selected && (
          <section aria-labelledby="role-title" className="min-w-0 space-y-4">
            <div>
              <h2 id="role-title" className="text-title font-semibold tracking-headline">
                {selected.name}
              </h2>
              <p className="text-ink-muted">{selected.description}</p>
            </div>

            {isAdminRole(selected) && <Alert tone="info">{copy.adminLocked}</Alert>}
            <div aria-live="polite">{notice && <Alert tone={notice.tone}>{notice.text}</Alert>}</div>

            <div className="overflow-x-auto rounded-lg border border-line bg-surface">
              <table className="w-full min-w-[640px] border-collapse">
                <thead className="bg-chrome text-caption text-ink-muted">
                  <tr>
                    <th className="px-4 py-2.5 text-left font-medium">{copy.module}</th>
                    {ACTIONS.map((action) => (
                      <th key={action} className="w-24 px-2 py-2.5 text-center font-medium">
                        {copy.actions[action]}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody className="divide-y divide-line">
                  {matrix.map(([module, defs]) => (
                    <tr key={module} className="h-11">
                      <th scope="row" className="px-4 text-left text-label font-medium">
                        {copy.modules[module] ?? module}
                      </th>
                      {ACTIONS.map((action) => {
                        const def = defs.get(action);
                        if (!def) return <td key={action} />;
                        return (
                          <td key={action} className="text-center">
                            <input
                              type="checkbox"
                              checked={working.has(def.key)}
                              disabled={readOnly}
                              onChange={() => toggle(def.key)}
                              title={def.description}
                              aria-label={`${copy.modules[module] ?? module}: ${def.description}`}
                              className="size-4.5 cursor-pointer accent-[var(--amber)] disabled:cursor-not-allowed"
                            />
                          </td>
                        );
                      })}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {!readOnly && changes > 0 && (
              <div className="sticky bottom-4 flex items-center justify-between gap-4 rounded-lg border border-line bg-surface px-4 py-3 shadow-(--shadow-float-high)">
                <span className="text-label text-ink-muted">{copy.pending(changes)}</span>
                <div className="flex gap-2">
                  <Button variant="ghost" onClick={() => setDraft(null)}>
                    {copy.discard}
                  </Button>
                  <Button variant="primary" loading={save.isPending} onClick={() => save.mutate()}>
                    {copy.save}
                  </Button>
                </div>
              </div>
            )}
          </section>
        )}
      </div>
    </>
  );
}

function groupByModule(defs: PermissionDef[]): [string, Map<string, PermissionDef>][] {
  const modules = new Map<string, Map<string, PermissionDef>>();
  for (const def of defs) {
    if (!modules.has(def.module)) modules.set(def.module, new Map());
    modules.get(def.module)!.set(def.action, def);
  }
  const order = Object.keys(copy.modules);
  return [...modules].sort(([a], [b]) => order.indexOf(a) - order.indexOf(b));
}

function symmetricDifference(a: Set<string>, b: Set<string>) {
  let count = 0;
  for (const key of a) if (!b.has(key)) count++;
  for (const key of b) if (!a.has(key)) count++;
  return count;
}
