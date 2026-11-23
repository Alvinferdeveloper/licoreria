"use client";

import { CheckCircle, WarningCircle } from "@phosphor-icons/react";
import type { DecodeHintType } from "@zxing/library";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/field";
import { Sheet } from "@/components/ui/sheet";
import { cn } from "@/lib/cn";
import { posCopy } from "../copy";

const copy = posCopy.camera;
/** The same code is read many times per second; a repeat inside this window is the same scan. */
const REPEAT_MS = 2000;

export type ScanOutcome = { added: string } | { failed: string };

interface Entry extends Partial<{ added: string; failed: string }> {
  id: number;
  code: string;
}

interface CameraScannerProps {
  open: boolean;
  onClose: () => void;
  onCode: (code: string) => Promise<ScanOutcome>;
}

/** Reads barcodes with the device camera (rear one on phones) and hands each new code to the POS. */
export function CameraScanner({ open, onClose, onCode }: CameraScannerProps) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const onCodeRef = useRef(onCode);
  const [status, setStatus] = useState<"starting" | "live" | "error">("starting");
  const [error, setError] = useState("");
  const [entries, setEntries] = useState<Entry[]>([]);
  const [resolution, setResolution] = useState("");
  const [typed, setTyped] = useState("");

  useEffect(() => {
    onCodeRef.current = onCode;
  }, [onCode]);

  const submitTyped = async (event: FormEvent) => {
    event.preventDefault();
    const code = typed.trim();
    if (!code) return;
    setTyped("");
    const outcome = await onCode(code);
    beep("added" in outcome);
    setEntries((list) => [{ id: Date.now(), code, ...outcome }, ...list].slice(0, 6));
  };

  useEffect(() => {
    if (!open) return;
    const video = videoRef.current;
    let stream: MediaStream | undefined;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let cancelled = false;
    let last = { code: "", at: 0 };
    let busy = false;

    const fail = (message: string) => {
      setStatus("error");
      setError(message);
    };

    const start = async () => {
      setStatus("starting");
      setEntries([]);
      if (!window.isSecureContext || !navigator.mediaDevices?.getUserMedia) return fail(copy.errors.insecure);
      const [{ BrowserMultiFormatOneDReader }, { BarcodeFormat, DecodeHintType }] = await Promise.all([import("@zxing/browser"), import("@zxing/library")]);
      // Only linear barcodes: faster per frame, and these readers don't flood the console on blurry frames.
      const hints = new Map<DecodeHintType, unknown>([
        [DecodeHintType.POSSIBLE_FORMATS, [BarcodeFormat.EAN_13, BarcodeFormat.EAN_8, BarcodeFormat.UPC_A, BarcodeFormat.UPC_E, BarcodeFormat.CODE_128]],
        // Laptop webcams give small, soft images; trying harder is what makes them readable.
        [DecodeHintType.TRY_HARDER, true],
      ]);
      const reader = new BrowserMultiFormatOneDReader(hints);

      const found = (code: string) => {
        const now = Date.now();
        if (code === last.code && now - last.at < REPEAT_MS) return;
        last = { code, at: now };
        busy = true;
        void onCodeRef.current(code).then((outcome) => {
          busy = false;
          beep("added" in outcome);
          setEntries((list) => [{ id: now, code, ...outcome }, ...list].slice(0, 6));
        });
      };

      try {
        stream = await navigator.mediaDevices.getUserMedia({
          audio: false,
          video: { facingMode: { ideal: "environment" }, width: { ideal: 1920 }, height: { ideal: 1080 } },
        });
        if (cancelled || !video) return stream.getTracks().forEach((t) => t.stop());
        video.srcObject = stream;
        await video.play();

        const track = stream.getVideoTracks()[0];
        track.onended = () => !cancelled && fail(copy.errors.ended);
        const settings = track.getSettings();
        setResolution(settings.width && settings.height ? `${settings.width}×${settings.height}` : "");
        await track.applyConstraints({ advanced: [{ focusMode: "continuous" } as MediaTrackConstraintSet] }).catch(() => undefined);
        if (cancelled) return;
        setStatus("live");

        // Our own loop: a frame the reader can't handle is skipped, it never stops the camera.
        const canvas = document.createElement("canvas");
        const context = canvas.getContext("2d", { willReadFrequently: true });
        const tick = () => {
          if (cancelled) return;
          if (!busy && context && video.readyState >= video.HAVE_CURRENT_DATA && video.videoWidth > 0) {
            canvas.width = video.videoWidth;
            canvas.height = video.videoHeight;
            context.drawImage(video, 0, 0);
            try {
              found(reader.decodeFromCanvas(canvas).getText());
            } catch {
              // No readable barcode in this frame.
            }
          }
          timer = setTimeout(tick, 100);
        };
        tick();
      } catch (e) {
        if (cancelled) return;
        const name = e instanceof DOMException ? e.name : "";
        fail(
          name === "NotAllowedError" || name === "SecurityError"
            ? copy.errors.denied
            : name === "NotFoundError" || name === "OverconstrainedError"
              ? copy.errors.missing
              : name === "NotReadableError"
                ? copy.errors.busy
                : copy.errors.other,
        );
      }
    };

    void start();
    return () => {
      cancelled = true;
      clearTimeout(timer);
      stream?.getTracks().forEach((t) => t.stop());
      if (video) video.srcObject = null;
    };
  }, [open]);

  return (
    <Sheet open={open} onClose={onClose} title={copy.title} description={copy.description} footer={<Button variant="primary" onClick={onClose}>{copy.done}</Button>}>
      <div className="space-y-5">
        <div className="relative aspect-[4/3] overflow-hidden rounded-lg bg-[oklch(18.5%_0.01_60)]">
          <video ref={videoRef} muted playsInline className={cn("size-full object-cover", status !== "live" && "invisible")} />
          {status === "live" && (
            <>
              <div aria-hidden className="pointer-events-none absolute inset-x-[12%] top-1/2 h-[38%] -translate-y-1/2 rounded-md border-2 border-on-accent/80 shadow-[0_0_0_100vmax_oklch(18.5%_0.01_60/0.35)]" />
              <p className="absolute inset-x-0 bottom-3 text-center text-caption text-on-accent">{copy.aim}</p>
            </>
          )}
          {status !== "live" && (
            <p role={status === "error" ? "alert" : undefined} className="absolute inset-0 grid place-content-center px-8 text-center text-label text-[oklch(94.5%_0.008_78)]">
              {status === "error" ? error : copy.starting}
            </p>
          )}
        </div>
        {status === "live" && (
          <p className="-mt-3 text-caption text-ink-muted">
            {copy.tip}
            {resolution && <span className="text-ink-subtle"> · {copy.resolution(resolution)}</span>}
          </p>
        )}

        <form onSubmit={submitTyped} className="flex items-end gap-2">
          <Field label={copy.manual} className="flex-1">
            <Input
              value={typed}
              onChange={(event) => setTyped(event.target.value)}
              inputMode="numeric"
              autoComplete="off"
              placeholder={copy.manualPlaceholder}
              className="font-mono tabular"
            />
          </Field>
          <Button type="submit" variant="secondary">
            {copy.manualSubmit}
          </Button>
        </form>

        <ul aria-live="polite" className="space-y-1.5">
          {entries.length === 0 && <li className="text-label text-ink-muted">{copy.empty}</li>}
          {entries.map((entry) => (
            <li key={entry.id} className="flex items-start gap-2 text-label">
              {entry.added ? (
                <CheckCircle size={18} weight="fill" className="mt-px shrink-0 text-success" aria-hidden />
              ) : (
                <WarningCircle size={18} weight="fill" className="mt-px shrink-0 text-danger" aria-hidden />
              )}
              <span className="min-w-0">
                <span className={cn("block", entry.failed && "text-danger")}>{entry.added ? `${copy.added}: ${entry.added}` : entry.failed}</span>
                <span className="block font-mono text-caption text-ink-subtle">{entry.code}</span>
              </span>
            </li>
          ))}
        </ul>
      </div>
    </Sheet>
  );
}

let audio: AudioContext | undefined;

/** Short high beep when a product is added, a low one when the code wasn't usable. */
function beep(ok: boolean) {
  try {
    audio ??= new AudioContext();
    const osc = audio.createOscillator();
    const gain = audio.createGain();
    osc.frequency.value = ok ? 1760 : 330;
    gain.gain.setValueAtTime(0.08, audio.currentTime);
    gain.gain.exponentialRampToValueAtTime(0.0001, audio.currentTime + (ok ? 0.09 : 0.2));
    osc.connect(gain).connect(audio.destination);
    osc.start();
    osc.stop(audio.currentTime + (ok ? 0.09 : 0.2));
  } catch {
    // Sound is a nicety; scanning works without it.
  }
}
