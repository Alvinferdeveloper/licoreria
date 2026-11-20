"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { Field, Input, inputClassName } from "@/components/ui/field";
import { usersApi } from "@/features/users/api";
import { ApiError } from "@/lib/api/client";
import { settingsApi, settingsKeys } from "../api";
import { settingsCopy } from "../copy";

const copy = settingsCopy.extensions;

const clock = new Intl.DateTimeFormat("es-NI", { hour: "numeric", minute: "2-digit", timeZone: "America/Managua" });

export function ScheduleExtensions({ editable, onMessage }: { editable: boolean; onMessage: (message: string) => void }) {
  const queryClient = useQueryClient();
  const extensions = useQuery({ queryKey: settingsKeys.extensions, queryFn: settingsApi.extensions });
  const users = useQuery({
    queryKey: ["users", "list", { search: "", active: "true", page: 1 }],
    queryFn: () => usersApi.list({ search: "", active: "true", page: 1 }),
    enabled: editable,
  });
  const [userId, setUserId] = useState("");
  const [until, setUntil] = useState("");
  const [error, setError] = useState<string | null>(null);

  const refresh = () => queryClient.invalidateQueries({ queryKey: settingsKeys.extensions });

  const extend = useMutation({
    mutationFn: settingsApi.extend,
    onSuccess: () => {
      setUntil("");
      void refresh();
      onMessage(copy.created);
    },
    onError: (e) => setError(e instanceof ApiError ? (e.fieldMessage("until") ?? e.message) : String(e)),
  });

  const end = useMutation({
    mutationFn: settingsApi.endExtension,
    onSuccess: () => {
      void refresh();
      onMessage(copy.ended);
    },
  });

  const submit = () => {
    setError(null);
    if (!until) return setError(settingsCopy.validation.until);
    extend.mutate({ userId: userId || null, until });
  };

  return (
    <>
      {extensions.data?.length ? (
        <ul className="divide-y divide-line">
          {extensions.data.map((x) => (
            <li key={x.id} className="flex items-center justify-between gap-4 py-2.5">
              <div className="min-w-0">
                <p>{copy.row(x.userName ?? copy.everyone, clock.format(new Date(x.endsAt)))}</p>
                <p className="text-caption text-ink-muted">{copy.by(x.createdByName)}</p>
              </div>
              {editable && (
                <ConfirmButton size="sm" confirmLabel={copy.confirmEnd} onConfirm={() => end.mutate(x.id)} disabled={end.isPending}>
                  {copy.end}
                </ConfirmButton>
              )}
            </li>
          ))}
        </ul>
      ) : (
        <p className="text-ink-muted">{copy.empty}</p>
      )}

      {editable && (
        <div className="grid gap-4 sm:grid-cols-[minmax(0,1fr)_9rem_auto] sm:items-end">
          <Field label={copy.who}>
            <select className={inputClassName} value={userId} onChange={(e) => setUserId(e.target.value)}>
              <option value="">{copy.everyone}</option>
              {users.data?.data.map((u) => (
                <option key={u.id} value={u.id}>
                  {u.fullName}
                </option>
              ))}
            </select>
          </Field>
          <Field label={copy.until} error={error ?? undefined}>
            <Input type="time" value={until} onChange={(e) => setUntil(e.target.value)} className="tabular font-mono" />
          </Field>
          <Button onClick={submit} loading={extend.isPending}>
            {copy.submit}
          </Button>
        </div>
      )}
    </>
  );
}
