import type { Metadata } from "next";
import { ChangePasswordScreen } from "@/features/auth/components/auth-screens";

export const metadata: Metadata = { title: "Cambiar contraseña" };

export default function ChangePasswordPage() {
  return <ChangePasswordScreen />;
}
