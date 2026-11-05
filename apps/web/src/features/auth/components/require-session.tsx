"use client";

import { LockKey } from "@phosphor-icons/react";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, type ReactNode } from "react";
import { can } from "../api";
import { CHANGE_PASSWORD_PATH, LOGIN_PATH } from "../navigation";
import { useSession } from "../session-store";

interface RequireSessionProps {
  children: ReactNode;
  /** Permission needed to view the content, e.g. "users:view". The API enforces it too. */
  permission?: string;
  /** Lets users with a temporary password through (only the change-password screen). */
  allowPasswordChange?: boolean;
}

/** Client guard: waits for the session, sends anonymous users to login and temporary-password users to change it. */
export function RequireSession({ children, permission, allowPasswordChange = false }: RequireSessionProps) {
  const router = useRouter();
  const pathname = usePathname();
  const status = useSession((s) => s.status);
  const user = useSession((s) => s.user);
  const mustChange = user?.mustChangePassword ?? false;

  useEffect(() => {
    if (status === "anonymous") router.replace(`${LOGIN_PATH}?next=${encodeURIComponent(pathname)}`);
    else if (status === "authenticated" && mustChange && !allowPasswordChange) router.replace(CHANGE_PASSWORD_PATH);
  }, [status, mustChange, allowPasswordChange, pathname, router]);

  if (status !== "authenticated" || (mustChange && !allowPasswordChange)) {
    return <div className="min-h-dvh bg-paper" aria-busy="true" />;
  }
  if (permission && !can(user, permission)) return <NoAccess />;
  return children;
}

export function NoAccess() {
  return (
    <div className="grid min-h-[60dvh] place-content-center justify-items-center gap-2 px-6 text-center">
      <LockKey size={32} className="text-ink-subtle" aria-hidden />
      <p className="text-title font-semibold">Sin acceso a esta sección</p>
      <p className="max-w-sm text-ink-muted">Tu rol no incluye este permiso. Pide al administrador que lo habilite.</p>
    </div>
  );
}
