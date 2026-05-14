import { router } from "@/router";
import { useCryptoSession } from "@/store/cryptoSession";

export type LockReason = "idle" | "hidden" | "expired";

export const LOCK_REASON_KEY = "lgv:lockReason";

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
