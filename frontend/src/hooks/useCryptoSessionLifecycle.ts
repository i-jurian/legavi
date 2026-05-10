import { useEffect } from "react";
import { useCryptoSession } from "../stores/cryptoSession";

const IDLE_TIMEOUT_MS = 5 * 60 * 1000;
const VISIBILITY_TIMEOUT_MS = 60 * 1000;

/** Locks the vault automatically when the user is idle or switches tabs. */
export function useCryptoSessionLifecycle() {
  const lock = useCryptoSession((s) => s.lock);

  useEffect(() => {
    let idleTimer: ReturnType<typeof setTimeout>;
    let visibilityTimer: ReturnType<typeof setTimeout> | null = null;

    const resetIdle = () => {
      clearTimeout(idleTimer);
      idleTimer = setTimeout(lock, IDLE_TIMEOUT_MS);
    };

    const onVisibilityChange = () => {
      if (document.hidden) {
        visibilityTimer = setTimeout(lock, VISIBILITY_TIMEOUT_MS);
      } else if (visibilityTimer) {
        clearTimeout(visibilityTimer);
        visibilityTimer = null;
      }
    };

    resetIdle();
    ["mousedown", "keydown", "touchstart"].forEach((evt) =>
      document.addEventListener(evt, resetIdle),
    );
    document.addEventListener("visibilitychange", onVisibilityChange);

    return () => {
      clearTimeout(idleTimer);
      if (visibilityTimer) clearTimeout(visibilityTimer);
      ["mousedown", "keydown", "touchstart"].forEach((evt) =>
        document.removeEventListener(evt, resetIdle),
      );
      document.removeEventListener("visibilitychange", onVisibilityChange);
    };
  }, [lock]);
}
