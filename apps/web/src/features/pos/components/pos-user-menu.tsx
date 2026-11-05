"use client";

import { GearSix, SignOut } from "@phosphor-icons/react";
import { useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Button, buttonVariants } from "@/components/ui/button";
import { can } from "@/features/auth/api";
import { logout, useSession } from "@/features/auth/session-store";

/** Signed-in cashier, link to administration when allowed, and sign out. */
export function PosUserMenu() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const user = useSession((s) => s.user);
  if (!user) return null;

  return (
    <div className="flex items-center gap-1">
      <span className="mr-2 hidden text-label text-ink-muted md:inline">{user.fullName}</span>
      {can(user, "users:view") && (
        <Link href="/usuarios" className={buttonVariants({ variant: "ghost", size: "icon-sm" })} aria-label="Administración" title="Administración">
          <GearSix size={18} />
        </Link>
      )}
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label="Cerrar sesión"
        title="Cerrar sesión"
        onClick={async () => {
          await logout();
          queryClient.clear();
          router.replace("/login");
        }}
      >
        <SignOut size={18} />
      </Button>
    </div>
  );
}
