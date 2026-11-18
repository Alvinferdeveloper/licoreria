"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useEffect } from "react";
import { PinForm } from "@/features/sales/components/pin-form";
import { can } from "../api";
import { authCopy } from "../copy";
import { homePath, safeNext } from "../navigation";
import { useSession } from "../session-store";
import { ActivationForm } from "./activation-form";
import { AuthFrame } from "./auth-frame";
import { ChangePasswordForm } from "./change-password-form";
import { LoginForm } from "./login-form";
import { ResetPasswordForm } from "./recovery-forms";
import { RequireSession } from "./require-session";

/** Sends anyone with a session onward: right after logging in or when opening /login already signed in. */
export function LoginScreen() {
  const router = useRouter();
  const next = safeNext(useSearchParams().get("next"));
  const user = useSession((s) => s.user);
  const status = useSession((s) => s.status);

  useEffect(() => {
    if (status !== "authenticated" || !user) return;
    router.replace(user.mustChangePassword || !next ? homePath(user) : next);
  }, [status, user, next, router]);

  return (
    <AuthFrame title={authCopy.login.title}>
      <LoginForm />
    </AuthFrame>
  );
}

export function ResetPasswordScreen() {
  const token = useSearchParams().get("token");
  return (
    <AuthFrame title={authCopy.reset.title} intro={authCopy.reset.intro}>
      <ResetPasswordForm token={token} />
    </AuthFrame>
  );
}

export function ActivationScreen() {
  const token = useSearchParams().get("token");
  return (
    <AuthFrame title={authCopy.activate.title} intro={token ? authCopy.activate.introToken : authCopy.activate.introCode}>
      <ActivationForm token={token} />
    </AuthFrame>
  );
}

export function ChangePasswordScreen() {
  return (
    <RequireSession allowPasswordChange>
      <ChangePasswordContent />
    </RequireSession>
  );
}

function ChangePasswordContent() {
  const user = useSession((s) => s.user);
  const required = user?.mustChangePassword ?? false;
  const canAuthorize = can(user, "discounts:authorize") || can(user, "sales:void");
  const copy = authCopy.changePassword;
  return (
    <AuthFrame title={required ? copy.titleRequired : copy.title} intro={required ? copy.introRequired : copy.intro}>
      <ChangePasswordForm />
      {!required && canAuthorize && <PinForm />}
    </AuthFrame>
  );
}
