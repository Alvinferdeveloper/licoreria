"use client";

import { Key, PaperPlaneTilt, PencilSimple, Plus } from "@phosphor-icons/react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useDeferredValue, useState } from "react";
import { DataTable, RowActions, type Column } from "@/components/data/data-table";
import { FilterChips, NoticeBanner, Pagination, SearchInput, STATUS_OPTIONS, useNotice, type StatusFilter } from "@/components/data/toolbar";
import { PageHeader } from "@/components/shell/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { can } from "@/features/auth/api";
import { useSession } from "@/features/auth/session-store";
import { rolesApi, rolesKeys } from "@/features/roles/api";
import { formatDateTime, formatRelative } from "@/lib/format/date";
import { usersApi, usersKeys, type Access, type AccessChannel, type User, type UserFilters, type UserInput } from "../api";
import { usersCopy as copy } from "../copy";
import { AccessCodeSheet } from "./access-code-sheet";
import { UserSheet } from "./user-sheet";

export function UsersPage() {
  const queryClient = useQueryClient();
  const me = useSession((s) => s.user);
  const [search, setSearch] = useState("");
  const [active, setActive] = useState<StatusFilter>("all");
  const [page, setPage] = useState(1);
  const [sheet, setSheet] = useState<{ open: boolean; user: User | null }>({ open: false, user: null });
  const [grant, setGrant] = useState<{ user: User; access: Access } | null>(null);
  const { notice, success, failure } = useNotice();

  const filters: UserFilters = { search: useDeferredValue(search), active, page };
  const users = useQuery({ queryKey: usersKeys.list(filters), queryFn: () => usersApi.list(filters), placeholderData: keepPreviousData });
  const roles = useQuery({ queryKey: rolesKeys.all, queryFn: rolesApi.list });
  const refresh = () => queryClient.invalidateQueries({ queryKey: usersKeys.all });

  const statusMutation = useMutation({
    mutationFn: ({ user, value }: { user: User; value: boolean }) => usersApi.setActive(user.id, value),
    onSuccess: (user) => {
      success(user.active ? copy.notices.activated(user.fullName) : copy.notices.deactivated(user.fullName));
      void refresh();
    },
    onError: failure,
  });
  const showAccess = (user: User, access: Access, notice: (email: string) => string) => {
    if (access.channel === "code") setGrant({ user, access });
    else success(notice(user.email ?? ""));
  };
  const accessMutation = useMutation({
    mutationFn: ({ user, channel }: { user: User; channel: AccessChannel }) => usersApi.resendAccess(user.id, channel),
    onSuccess: (access, { user }) => showAccess(user, access, copy.notices.resent),
    onError: failure,
  });

  const saveUser = async (input: UserInput) => {
    const editing = sheet.user;
    if (editing) {
      await usersApi.update(editing.id, input);
      success(copy.notices.updated);
    } else {
      const { user, access } = await usersApi.create(input);
      showAccess(user, access, copy.notices.invited);
    }
    setSheet({ open: false, user: null });
    await refresh();
  };
  const accessPending = (user: User, channel: AccessChannel) =>
    accessMutation.isPending && accessMutation.variables?.user.id === user.id && accessMutation.variables.channel === channel;

  const canUpdate = can(me, "users:update");
  const canToggle = can(me, "users:delete");

  const columns: Column<User>[] = [
    {
      header: copy.columns.name,
      skeleton: 60,
      cell: (user) => (
        <>
          <p className="font-medium text-ink">
            {user.fullName}
            {user.id === me?.id && <span className="ml-2 text-caption font-normal text-ink-subtle">{copy.you}</span>}
          </p>
          <p className="text-caption text-ink-muted">{user.email ?? copy.noEmail}</p>
        </>
      ),
    },
    { header: copy.columns.username, skeleton: 40, cell: (user) => <span className="font-mono text-label">{user.username}</span> },
    { header: copy.columns.role, skeleton: 40, cell: (user) => <span className="text-label">{user.role.name}</span> },
    { header: copy.columns.status, skeleton: 30, cell: (user) => <StatusBadge user={user} /> },
    {
      header: copy.columns.lastLogin,
      skeleton: 40,
      cell: (user) =>
        user.lastLoginAt ? (
          <time className="text-label text-ink-muted" dateTime={user.lastLoginAt} title={formatDateTime(user.lastLoginAt)}>
            {formatRelative(user.lastLoginAt)}
          </time>
        ) : (
          <span className="text-label text-ink-muted">{copy.never}</span>
        ),
    },
    {
      header: <span className="sr-only">Acciones</span>,
      className: "w-px px-2",
      skeleton: 0,
      cell: (user) => (
        <RowActions>
          {canUpdate && (
            <Button variant="ghost" size="icon-sm" aria-label={`${copy.actions.edit} ${user.fullName}`} title={copy.actions.edit} onClick={() => setSheet({ open: true, user })}>
              <PencilSimple size={16} />
            </Button>
          )}
          {canUpdate && user.active && user.email && user.pendingActivation && (
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={`${copy.actions.resendInvite}: ${user.fullName}`}
              title={copy.actions.resendInvite}
              loading={accessPending(user, "email")}
              onClick={() => accessMutation.mutate({ user, channel: "email" })}
            >
              <PaperPlaneTilt size={16} />
            </Button>
          )}
          {canUpdate && user.active && user.id !== me?.id && (
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={`${copy.actions.code}: ${user.fullName}`}
              title={copy.actions.code}
              loading={accessPending(user, "code")}
              onClick={() => accessMutation.mutate({ user, channel: "code" })}
            >
              <Key size={16} />
            </Button>
          )}
          {canToggle && user.id !== me?.id && (user.active ? (
            <ConfirmButton size="sm" confirmLabel={copy.actions.confirm} onConfirm={() => statusMutation.mutate({ user, value: false })}>
              {copy.actions.deactivate}
            </ConfirmButton>
          ) : (
            <Button variant="ghost" size="sm" onClick={() => statusMutation.mutate({ user, value: true })}>
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
          can(me, "users:create") && (
            <Button variant="primary" onClick={() => setSheet({ open: true, user: null })}>
              <Plus size={16} weight="bold" aria-hidden />
              {copy.create}
            </Button>
          )
        }
      />
      <div className="space-y-4 px-8 pb-10">
        <div className="flex flex-wrap items-center gap-3">
          <SearchInput value={search} onChange={(v) => { setSearch(v); setPage(1); }} placeholder={copy.search} />
          <FilterChips label="Estado" value={active} options={STATUS_OPTIONS} onChange={(v) => { setActive(v); setPage(1); }} />
        </div>
        <NoticeBanner notice={notice} />
        <DataTable
          columns={columns}
          rows={users.data?.data}
          rowKey={(u) => u.id}
          loading={users.isPending}
          error={users.isError}
          onRetry={() => users.refetch()}
          empty={copy.empty}
          rowClassName={(u) => (u.active ? undefined : "text-ink-muted")}
        />
        <Pagination meta={users.data?.meta} onPage={setPage} />
      </div>
      <UserSheet open={sheet.open} user={sheet.user} roles={roles.data ?? []} onClose={() => setSheet({ open: false, user: null })} onSubmit={saveUser} />
      <AccessCodeSheet grant={grant} onClose={() => setGrant(null)} />
    </>
  );
}

function StatusBadge({ user }: { user: User }) {
  if (!user.active) return <Badge>{copy.status.inactive}</Badge>;
  if (user.pendingActivation) return <Badge tone="warning">{copy.status.pending}</Badge>;
  if (user.mustChangePassword) return <Badge tone="warning">{copy.status.temporary}</Badge>;
  return <Badge tone="success">{copy.status.active}</Badge>;
}
