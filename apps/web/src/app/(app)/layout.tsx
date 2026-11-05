import { AppShell } from "@/components/shell/app-shell";
import { RequireSession } from "@/features/auth/components/require-session";

/** Content depends on the browser session, so these routes opt out of instant-navigation prerendering. */
export const instant = false;

export default function AppLayout({ children }: LayoutProps<"/">) {
  return (
    <RequireSession>
      <AppShell>{children}</AppShell>
    </RequireSession>
  );
}
