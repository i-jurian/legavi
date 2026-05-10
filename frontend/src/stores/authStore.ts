import { create } from "zustand";

type AuthState = {
  accessToken: string | null;
  userId: string | null;
  setSession: (accessToken: string, userId: string) => void;
  clearSession: () => void;
};

/** Tracks who the current user is and their active login. */
export const useAuthStore = create<AuthState>((set) => ({
  accessToken: null,
  userId: null,
  setSession: (accessToken, userId) => set({ accessToken, userId }),
  clearSession: () => set({ accessToken: null, userId: null }),
}));
