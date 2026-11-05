"use client";

import { EnvelopeSimple, FilePdf, Receipt } from "@phosphor-icons/react";
import { useMutation } from "@tanstack/react-query";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/field";
import { ApiError } from "@/lib/api/client";
import { salesApi } from "../api";
import { invoiceCopy as copy } from "../copy";

interface InvoiceActionsProps {
  saleId: string;
  /** "ticket" or "invoice"; that action is shown as the primary one. */
  defaultFormat?: string;
  customerEmail?: string | null;
  onMessage?: (message: string, tone: "success" | "danger") => void;
}

/** Print the ticket, open the detailed PDF or email the invoice for a recorded sale. */
export function InvoiceActions({ saleId, defaultFormat = "ticket", customerEmail, onMessage }: InvoiceActionsProps) {
  const [emailing, setEmailing] = useState(false);
  const [email, setEmail] = useState(customerEmail ?? "");

  const openTicket = () => window.open(`/ticket/${saleId}?print=1`, "_blank", "popup,width=420,height=760");

  const openPdf = async () => {
    // Opened before the request so the browser treats it as a direct click, not a popup.
    const tab = window.open("", "_blank");
    try {
      const blob = await salesApi.invoicePdf(saleId);
      const url = URL.createObjectURL(blob);
      if (tab) tab.location.href = url;
      else window.location.href = url;
    } catch {
      tab?.close();
      onMessage?.(copy.pdfError, "danger");
    }
  };

  const send = useMutation({
    mutationFn: () => salesApi.email(saleId, email.trim()),
    onSuccess: () => {
      setEmailing(false);
      onMessage?.(copy.sent(email.trim()), "success");
    },
    onError: (e) => onMessage?.(e instanceof ApiError ? (e.fieldMessage("email") ?? e.message) : copy.invalidEmail, "danger"),
  });

  const ticketFirst = defaultFormat !== "invoice";
  return (
    <div className="space-y-2">
      <div className="grid grid-cols-3 gap-2">
        <Button variant={ticketFirst ? "primary" : "secondary"} onClick={openTicket}>
          <Receipt size={16} aria-hidden />
          {copy.ticket}
        </Button>
        <Button variant={ticketFirst ? "secondary" : "primary"} onClick={() => void openPdf()}>
          <FilePdf size={16} aria-hidden />
          {copy.pdf}
        </Button>
        <Button aria-expanded={emailing} onClick={() => setEmailing(!emailing)}>
          <EnvelopeSimple size={16} aria-hidden />
          {copy.email}
        </Button>
      </div>
      {emailing && (
        <form
          className="flex gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            if (!/^\S+@\S+\.\S+$/.test(email.trim())) return onMessage?.(copy.invalidEmail, "danger");
            send.mutate();
          }}
        >
          <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder={copy.emailLabel} aria-label={copy.emailLabel} className="h-9" autoFocus />
          <Button type="submit" loading={send.isPending}>
            {copy.send}
          </Button>
        </form>
      )}
    </div>
  );
}
