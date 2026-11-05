"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useEffect, useState, type ReactNode } from "react";
import { ApiError, configureApiClient } from "@/lib/api/client";
import { refreshSession, useSession } from "@/features/auth/session-store";

configureApiClient({
  getAccessToken: () => useSession.getState().accessToken,
  refreshSession,
});

function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        refetchOnWindowFocus: false,
        // Client errors (validation, permissions, not found) won't fix themselves on retry.
        retry: (count, error) => !(error instanceof ApiError && error.status < 500) && count < 2,
      },
    },
  });
}

export function Providers({ children }: { children: ReactNode }) {
  const [queryClient] = useState(createQueryClient);

  useEffect(() => {
    if (useSession.getState().status === "loading") void refreshSession();
  }, []);

  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}
