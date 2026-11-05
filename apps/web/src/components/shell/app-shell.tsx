"use client";

import { AddressBook, CashRegister, Package, ShieldCheck, SignOut, Stack, Tag, Truck, UsersThree, Wine, type Icon } from "@phosphor-icons/react";
import { useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { can } from "@/features/auth/api";
import { logout, useSession } from "@/features/auth/session-store";
import { ThemeToggle } from "@/features/theme";
import { cn } from "@/lib/cn";

interface NavItem {
  href: string;
  label: string;
  icon: Icon;
  permission: string;
}

const NAV: readonly { label: string; items: readonly NavItem[] }[] = [
  { label: "Vender", items: [{ href: "/pos", label: "Punto de venta", icon: CashRegister, permission: "pos:view" }] },
  {
    label: "Inventario",
    items: [
      { href: "/productos", label: "Productos", icon: Wine, permission: "products:view" },
      { href: "/categorias", label: "Categorías y marcas", icon: Tag, permission: "categories:view" },
      { href: "/inventario", label: "Existencias y kardex", icon: Stack, permission: "inventory:view" },
      { href: "/compras", label: "Compras", icon: Package, permission: "purchases:view" },
    ],
  },
  {
    label: "Personas",
    items: [
      { href: "/clientes", label: "Clientes", icon: AddressBook, permission: "customers:view" },
      { href: "/proveedores", label: "Proveedores", icon: Truck, permission: "suppliers:view" },
      { href: "/usuarios", label: "Usuarios", icon: UsersThree, permission: "users:view" },
    ],
  },
  { label: "Sistema", items: [{ href: "/roles", label: "Roles y permisos", icon: ShieldCheck, permission: "roles:view" }] },
];

export function AppShell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const queryClient = useQueryClient();
  const user = useSession((s) => s.user);

  const groups = NAV.map((group) => ({ ...group, items: group.items.filter((item) => can(user, item.permission)) })).filter(
    (group) => group.items.length > 0,
  );

  const signOut = async () => {
    await logout();
    queryClient.clear();
    router.replace("/login");
  };

  return (
    <div className="grid min-h-dvh grid-cols-1 md:grid-cols-[232px_minmax(0,1fr)]">
      <aside className="flex flex-col border-r border-line bg-chrome md:sticky md:top-0 md:h-dvh">
        <div className="flex h-13 items-center justify-between px-4">
          <span className="text-title font-semibold tracking-headline">Añejo</span>
          <ThemeToggle lightLabel="Usar tema claro" darkLabel="Usar tema oscuro" />
        </div>

        <nav className="flex-1 space-y-5 overflow-y-auto px-2 py-3" aria-label="Principal">
          {groups.map((group) => (
            <div key={group.label}>
              <p className="px-2.5 pb-1.5 text-caption font-medium text-ink-subtle">{group.label}</p>
              <ul className="space-y-0.5">
                {group.items.map((item) => {
                  const active = pathname === item.href || pathname.startsWith(`${item.href}/`);
                  return (
                    <li key={item.href}>
                      <Link
                        href={item.href}
                        aria-current={active ? "page" : undefined}
                        className={cn(
                          "relative flex h-8 items-center gap-2.5 rounded-sm px-2.5 text-label font-medium transition-colors duration-150",
                          active ? "bg-surface text-ink" : "text-ink-muted hover:bg-sunken hover:text-ink",
                        )}
                      >
                        <item.icon size={16} aria-hidden />
                        {item.label}
                        {active && <span className="ml-auto size-1.5 rounded-full bg-amber" aria-hidden />}
                      </Link>
                    </li>
                  );
                })}
              </ul>
            </div>
          ))}
        </nav>

        {user && (
          <div className="border-t border-line p-3">
            <div className="flex items-center gap-2.5">
              <span className="grid size-8 shrink-0 place-items-center rounded-full bg-sunken text-caption font-semibold text-ink-muted">
                {initials(user.fullName)}
              </span>
              <div className="min-w-0 flex-1">
                <p className="truncate text-label font-medium">{user.fullName}</p>
                <p className="truncate text-caption text-ink-muted">{user.role.name}</p>
              </div>
              <Button variant="ghost" size="icon-sm" onClick={signOut} aria-label="Cerrar sesión" title="Cerrar sesión">
                <SignOut size={16} />
              </Button>
            </div>
            <Link href="/cambiar-contrasena" className="mt-2 block px-1 text-caption text-ink-muted hover:text-ink">
              Cambiar contraseña
            </Link>
          </div>
        )}
      </aside>

      <main className="min-w-0 bg-paper">{children}</main>
    </div>
  );
}

function initials(name: string) {
  return name
    .split(/\s+/)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? "")
    .join("");
}
