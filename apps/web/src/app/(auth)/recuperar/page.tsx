import type { Metadata } from "next";
import { AuthFrame } from "@/features/auth/components/auth-frame";
import { ForgotPasswordForm } from "@/features/auth/components/recovery-forms";
import { authCopy } from "@/features/auth/copy";

export const metadata: Metadata = { title: "Recuperar acceso" };

export default function ForgotPasswordPage() {
  return (
    <AuthFrame title={authCopy.forgot.title} intro={authCopy.forgot.intro}>
      <ForgotPasswordForm />
    </AuthFrame>
  );
}
