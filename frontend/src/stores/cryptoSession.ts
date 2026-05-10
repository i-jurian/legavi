import { create } from "zustand";

type SessionState = "LOCKED" | "UNLOCKING" | "UNLOCKED" | "LOCKING";

type CryptoSession = {
  state: SessionState;
  ageIdentity: Uint8Array | null;
  unlock: (identity: Uint8Array) => void;
  lock: () => void;
};

/** Holds the unlocked vault key while the user is signed in. */
export const useCryptoSession = create<CryptoSession>((set, get) => ({
  state: "LOCKED",
  ageIdentity: null,
  unlock: (identity) => set({ state: "UNLOCKED", ageIdentity: identity }),
  lock: () => {
    const { ageIdentity } = get();
    if (ageIdentity) ageIdentity.fill(0);
    set({ state: "LOCKED", ageIdentity: null });
  },
}));
