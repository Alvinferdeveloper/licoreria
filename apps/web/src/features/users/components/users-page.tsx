"use client";

import { MagnifyingGlass, PaperPlaneTilt, PencilSimple, Plus } from "@phosphor-icons/react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useDeferredValue, useEffect, useState } from "react";
import { PageHeader } from "@/components/shell/page-header";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { inputClassName } from "@/components/ui/field";
import { can } from "@/features/auth/api";
import { useSession } from "@/features/auth/session-store";
import { rolesApi, rolesKeys } from "@/features/roles/api";
import { ApiError } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { formatDateTime, formatRelative } from "@/lib/format/date";
import { usersApi, usersKeys, type User, type UserFilters, type UserInput } from "../api";
import { usersCopy as copy } from "../copy";
import { UserSheet } from "./user-sheet";

type Notice = { tone: "success" | "danger"; text: string };

export function UsersPage() {
  const queryClient = useQueryClient();
  const me = useSession((s) => s.user);
  const [search, setSearch] = useState("");
  const [active, setActive] = useState<UserFilters["active"]>("all");
  const [page, setPage] = useState(1);
  const [sheet, setSheet] = useState<{ open: boolean; user: User | null }>({ open: false, user: null });
  const [notice, setNotice] = useState<Notice | null>(null);

  const filters: UserFilters = { search: useDeferredValue(search), active, page };
  const users = useQuery({ queryKey: usersKeys.list(filters), queryFn: () => usersApi.list(filters), placeholderData: keepPreviousData });
  const roles = useQuery({ queryKey: rolesKeys.all, queryFn: rolesApi.list });

  useEffect(() => {
    if (!notice) return;
    const timer = setTimeout(() => setNotice(null), 5000);
    return () => clearTimeout(timer);
  }, [notice]);

  const refresh = () => queryClient.invalidateQueries({ queryKey: usersKeys.all });
  const report = (error: unknown) =>
    setNotice({ tone: "danger", text: error instanceof ApiError ? error.message : "Ocurrió un error inesperado." });

  const statusMutation = useMutation({
    mutationFn: ({ user, value }: { user: User; value: boolean }) => usersApi.setActive(user.id, value),
    onSuccess: (user) => {
      setNotice({ tone: "success", text: user.active ? copy.notices.activated(user.fullName) : copy.notices.deactivated(user.fullName) });
      void refresh();
    },
    onError: report,
  });

  const resendMutation = useMutation({
    mutationFn: (user: User) => usersApi.resendAccess(user.id),
    onSuccess: (_, user) => setNotice({ tone: "success", text: copy.notices.resent(user.email) }),
    onError: report,
  });

  const saveUser = async (input: UserInput) => {
    const editing = sheet.user;
    const saved = editing ? await usersApi.update(editing.id, input) : await usersApi.create(input);
    setSheet({ open: false, user: null });
    setNotice({ tone: "success", text: editing ? copy.notices.updated : copy.notices.created(saved.email) });
    await refresh();
  };

  const canCreate = can(me, "users:create");
  const canUpdate = can(me, "users:update");
  const canToggle = can(me, "users:delete");
  const meta = users.data?.meta;

  return (
    <>
      <PageHeader
        title={copy.title}
        description={copy.description}
        actions={
          canCreate && (
            <Button variant="primary" onClick={() => setSheet({ open: true, user: null })}>
              <Plus size={16} weight="bold" aria-hidden />
              {copy.create}
            </Button>
          )
        }
      />

      <div className="space-y-4 px-8 pb-10">
        <div className="flex flex-wrap items-center gap-3">
          <div className="relative w-full max-w-xs">
            <MagnifyingGlass size={16} className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-ink-subtle" aria-hidden />
            <input
              type="search"
              value={search}
              onChange={(e) => {
                setSearch(e.target.value);
                setPage(1);
              }}
              placeholder={copy.search}
              aria-label={copy.search}
              className={cn(inputClassName, "h-9 pl-9")}
            />
          </div>
          <div className="flex gap-1.5" role="group" aria-label="Estado">
            {(["all", "true", "false"] as const).map((value) => (
              <button
                key={value}
                type="button"
                aria-pressed={active === value}
                onClick={() => {
                  setActive(value);
                  setPage(1);
                }}
                className={cn(
                  "press h-7 rounded-full px-3 text-caption font-medium transition-colors duration-150",
                  active === value ? "bg-amber-tint text-amber-ink" : "bg-sunken text-ink-muted hover:text-ink",
                )}
              >
                {copy.filters[value]}
              </button>
            ))}
          </div>
        </div>

        <div aria-live="polite">{notice && <Alert tone={notice.tone}>{notice.text}</Alert>}</div>

        <div className="overflow-x-auto rounded-lg border border-line bg-surface">
          <table className="w-full min-w-[760px] border-collapse text-left">
            <thead className="bg-chrome text-caption text-ink-muted">
              <tr>
                <th className="px-4 py-2.5 font-medium">{copy.columns.name}</th>
                <th className="px-4 py-2.5 font-medium">{copy.columns.username}</th>
                <th className="px-4 py-2.5 font-medium">{copy.columns.role}</th>
                <th className="px-4 py-2.5 font-medium">{copy.columns.status}</th>
                <th className="px-4 py-2.5 font-medium">{copy.columns.lastLogin}</th>
                <th className="w-px px-4 py-2.5">
                  <span className="sr-only">Acciones</span>
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {users.isPending && <SkeletonRows />}
              {users.data?.data.map((user) => {
                const isMe = user.id === me?.id;
                return (
                  <tr key={user.id} className={cn("group h-14 transition-colors hover:bg-chrome/60", !user.active && "text-ink-muted")}>
                    <td className="px-4">
                      <p className="font-medium text-ink">
                        {user.fullName}
                        {isMe && <span className="ml-2 text-caption font-normal text-ink-subtle">{copy.you}</span>}
                      </p>
                      <p className="text-caption text-ink-muted">{user.email}</p>
                    </td>
                    <td className="px-4 font-mono text-label">{user.username}</td>
                    <td className="px-4 text-label">{user.role.name}</td>
                    <td className="px-4">
                      <StatusBadge user={user} />
                    </td>
                    <td className="px-4 text-label text-ink-muted">
                      {user.lastLoginAt ? <time dateTime={user.lastLoginAt} title={formatDateTime(user.lastLoginAt)}>{formatRelative(user.lastLoginAt)}</time> : copy.never}
                    </td>
                    <td className="px-2">
                      <div className="flex justify-end gap-1 opacity-100 transition-opacity md:opacity-0 md:group-hover:opacity-100 md:group-focus-within:opacity-100">
                        {canUpdate && (
                          <Button variant="ghost" size="icon-sm" aria-label={`${copy.actions.edit} ${user.fullName}`} title={copy.actions.edit} onClick={() => setSheet({ open: true, user })}>
                            <PencilSimple size={16} />
                          </Button>
                        )}
                        {canUpdate && user.active && (
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            aria-label={`${copy.actions.resend} a ${user.fullName}`}
                            title={copy.actions.resend}
                            loading={resendMutation.isPending && resendMutation.variables?.id === user.id}
                            onClick={() => resendMutation.mutate(user)}
                          >
                            <PaperPlaneTilt size={16} />
                          </Button>
                        )}
                        {canToggle && !isMe &&
                          (user.active ? (
                            <ConfirmButton size="sm" confirmLabel={copy.actions.confirm} onConfirm={() => statusMutation.mutate({ user, value: false })}>
                              {copy.actions.deactivate}
                            </ConfirmButton>
                          ) : (
                            <Button variant="ghost" size="sm" onClick={() => statusMutation.mutate({ user, value: true })}>
                              {copy.actions.activate}
                            </Button>
                          ))}
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>

          {users.isError && (
            <div className="flex items-center justify-between gap-4 px-4 py-6">
              <p className="text-ink-muted">{copy.error}</p>
              <Button size="sm" onClick={() => users.refetch()}>
                {copy.retry}
              </Button>
            </div>
          )}
          {users.data && users.data.data.length === 0 && (
            <div className="px-4 py-12 text-center">
              <p className="font-semibold">{copy.empty.title}</p>
              <p className="text-ink-muted">{copy.empty.body}</p>
            </div>
          )}
        </div>

        {meta && meta.total > meta.pageSize && (
          <div className="flex items-center justify-end gap-3 text-label text-ink-muted">
            <span className="tabular">
              {copy.pagination((meta.page - 1) * meta.pageSize + 1, Math.min(meta.page * meta.pageSize, meta.total), meta.total)}
            </span>
            <Button size="sm" disabled={meta.page <= 1} onClick={() => setPage((p) => p - 1)}>
              {copy.previous}
            </Button>
            <Button size="sm" disabled={meta.page * meta.pageSize >= meta.total} onClick={() => setPage((p) => p + 1)}>
              {copy.next}
            </Button>
          </div>
        )}
      </div>

      <UserSheet
        open={sheet.open}
        user={sheet.user}
        roles={roles.data ?? []}
        onClose={() => setSheet({ open: false, user: null })}
        onSubmit={saveUser}
      />
    </>
  );
}

function StatusBadge({ user }: { user: User }) {
  if (!user.active) return <Badge>{copy.status.inactive}</Badge>;
  if (user.mustChangePassword) return <Badge tone="warning">{copy.status.temporary}</Badge>;
  return <Badge tone="success">{copy.status.active}</Badge>;
}

function SkeletonRows() {
  return Array.from({ length: 4 }, (_, i) => (
    <tr key={i} className="h-14" aria-hidden>
      {[48, 20, 24, 16, 20, 0].map((width, j) => (
        <td key={j} className="px-4">
          {width > 0 && <span className="block h-3 animate-pulse rounded-xs bg-sunken" style={{ width: `${width * 2}%` }} />}
        </td>
      ))}
    </tr>
  ));
}
