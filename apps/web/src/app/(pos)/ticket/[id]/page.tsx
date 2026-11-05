import type { Metadata } from "next";
import { RequireSession } from "@/features/auth/components/require-session";
import { TicketView } from "@/features/sales/components/ticket-view";

export const metadata: Metadata = { title: "Ticket" };

/** Depends on the browser session, like the (app) routes. */
export const instant = false;

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return (
    <RequireSession permission="sales:view">
      <TicketView id={id} />
    </RequireSession>
  );
}
