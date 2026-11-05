import { can, type SessionUser } from "./api";

export const CHANGE_PASSWORD_PATH = "/cambiar-contrasena";
export const LOGIN_PATH = "/login";

/** Destinations in priority order; the first one the user may open is their home. */
const HOME_CANDIDATES: readonly { path: string; permission: string }[] = [
  { path: "/pos", permission: "pos:view" },
  { path: "/usuarios", permission: "users:view" },
  { path: "/roles", permission: "roles:view" },
];

export function homePath(user: SessionUser): string {
  if (user.mustChangePassword) return CHANGE_PASSWORD_PATH;
  return HOME_CANDIDATES.find((c) => can(user, c.permission))?.path ?? "/sin-acceso";
}

/** Accepts only same-site relative paths, so ?next= can't redirect off the app. */
export function safeNext(next: string | null): string | null {
  if (!next || !next.startsWith("/") || next.startsWith("//")) return null;
  return next;
}
