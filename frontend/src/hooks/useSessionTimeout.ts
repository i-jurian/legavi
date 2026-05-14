import { useEffect } from "react";
import { lockAndLogout } from "@/lib/session";

const IDLE_TIMEOUT_MS = 5 * 60 * 1000;
const VISIBILITY_TIMEOUT_MS = 60 * 1000;

/** Auto-logout on idle or hidden-tab timeout. */
export function useSessionTimeout() {
  useEffect(() => {
    let idleTimer: ReturnType<typeof setTimeout>;
    let visibilityTimer: ReturnType<typeof setTimeout> | null = null;

    const resetIdle = () => {
      clearTimeout(idleTimer);
      idleTimer = setTimeout(() => void lockAndLogout("idle"), IDLE_TIMEOUT_MS);
    };

    const onVisibilityChange = () => {
      if (document.hidden) {
        visibilityTimer = setTimeout(
          () => void lockAndLogout("hidden"),
          VISIBILITY_TIMEOUT_MS,
        );
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
  }, []);
}
