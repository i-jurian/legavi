import { useState } from "react";
import { startAuthentication } from "@simplewebauthn/browser";
import { unlockStart, unlockVerify } from "@/api/auth";
import { decodePRFInput, readPRFFirst } from "@/lib/prf";
import { router } from "@/router";
import { useCryptoSession } from "@/store/cryptoSession";

export type LockReason = "idle" | "hidden" | "expired";

const LOCK_REASON_KEY = "lgv:lockReason";
const LOGOUT_URL = "/api/v1/auth/logout";

export async function lockAndLogout(reason: LockReason): Promise<void> {
  if (
    reason !== "expired" &&
    useCryptoSession.getState().state !== "UNLOCKED"
  ) {
    return;
  }
  sessionStorage.setItem(LOCK_REASON_KEY, reason);
  if (reason !== "expired") {
    await fetch(LOGOUT_URL, {
      method: "POST",
      credentials: "include",
    }).catch(() => {});
  }
  useCryptoSession.getState().lock();
  await router.navigate({ to: "/login" });
}

export async function sessionFetch(
  input: RequestInfo,
  init?: RequestInit,
): Promise<Response> {
  const res = await fetch(input, { credentials: "include", ...init });
  if (res.status === 401) {
    await lockAndLogout("expired");
    throw new Error("session expired");
  }
  return res;
}

function isLockReason(v: unknown): v is LockReason {
  return v === "idle" || v === "hidden" || v === "expired";
}

export function consumeLockReason(
  messages: Record<LockReason, string>,
): string | null {
  const raw = sessionStorage.getItem(LOCK_REASON_KEY);
  if (raw !== null) sessionStorage.removeItem(LOCK_REASON_KEY);
  if (raw && isLockReason(raw)) return messages[raw];
  return null;
}

export function useUnlock() {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function unlock() {
    setBusy(true);
    setError(null);
    try {
      const { publicKey } = await unlockStart();
      decodePRFInput(publicKey);
      const response = await startAuthentication({ optionsJSON: publicKey });

      const prfBytes = readPRFFirst(response);
      if (!prfBytes) {
        throw new Error("This passkey doesn't support PRF.");
      }

      await unlockVerify({ response });
      useCryptoSession.getState().unlock(prfBytes);
    } catch (err) {
      if (err instanceof Error && err.name === "NotAllowedError") {
        setError("Cancelled. Try again.");
      } else {
        setError(err instanceof Error ? err.message : "unlock failed");
      }
    } finally {
      setBusy(false);
    }
  }

  return { unlock, busy, error };
}
