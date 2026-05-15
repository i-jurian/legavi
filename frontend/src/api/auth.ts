import type {
  AuthenticationResponseJSON,
  PublicKeyCredentialCreationOptionsJSON,
  PublicKeyCredentialRequestOptionsJSON,
  RegistrationResponseJSON,
} from "@simplewebauthn/browser";
import { queryOptions } from "@tanstack/react-query";
import { sessionFetch, lockAndLogout } from "@/lib/session";
import { useCryptoSession } from "@/store/cryptoSession";

type RegisterStartBody = {
  email: string;
  displayName: string;
};

type RegisterVerifyBody = {
  ageRecipient: string;
  nickname: string;
  response: RegistrationResponseJSON;
};

type LoginStartBody = {
  email: string;
};

type LoginVerifyBody = {
  response: AuthenticationResponseJSON;
};

type CredentialCreationResponse = {
  publicKey: PublicKeyCredentialCreationOptionsJSON;
};

type CredentialRequestResponse = {
  publicKey: PublicKeyCredentialRequestOptionsJSON;
};

export type Me = {
  id: string;
  email: string;
  displayName: string;
};

const BASE = "/api/v1/auth";

export async function registerStart(
  body: RegisterStartBody,
): Promise<CredentialCreationResponse> {
  const res = await fetch(`${BASE}/register/start`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    throw new Error(`register/start failed: ${res.status} ${await res.text()}`);
  }
  return res.json();
}

export async function registerVerify(body: RegisterVerifyBody): Promise<void> {
  const res = await fetch(`${BASE}/register/verify`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    throw new Error(
      `register/verify failed: ${res.status} ${await res.text()}`,
    );
  }
}

export async function loginStart(
  body: LoginStartBody,
): Promise<CredentialRequestResponse> {
  const res = await fetch(`${BASE}/login/start`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    throw new Error(`login/start failed: ${res.status} ${await res.text()}`);
  }
  return res.json();
}

export async function loginVerify(body: LoginVerifyBody): Promise<void> {
  const res = await fetch(`${BASE}/login/verify`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    throw new Error(`login/verify failed: ${res.status} ${await res.text()}`);
  }
}

export async function unlockStart(): Promise<CredentialRequestResponse> {
  const res = await sessionFetch(`${BASE}/unlock/start`, { method: "POST" });
  if (!res.ok) {
    throw new Error(`unlock/start failed: ${res.status} ${await res.text()}`);
  }
  return res.json();
}

export async function unlockVerify(body: LoginVerifyBody): Promise<void> {
  const res = await sessionFetch(`${BASE}/unlock/verify`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    throw new Error(`unlock/verify failed: ${res.status} ${await res.text()}`);
  }
}

export async function logout(): Promise<void> {
  const res = await fetch(`${BASE}/logout`, {
    method: "POST",
    credentials: "include",
  });
  if (!res.ok && res.status !== 401) {
    throw new Error(`logout failed: ${res.status} ${await res.text()}`);
  }
}

export async function me(): Promise<Me> {
  const res = await fetch(`${BASE}/me`, {
    method: "GET",
    credentials: "include",
  });
  if (res.status === 401) {
    if (useCryptoSession.getState().state === "UNLOCKED") {
      await lockAndLogout("expired");
    }
    throw new Error("unauthorized");
  }
  if (!res.ok) {
    throw new Error(`me failed: ${res.status}`);
  }
  return res.json();
}

export const meQuery = queryOptions({
  queryKey: ["me"] as const,
  queryFn: me,
  staleTime: 5 * 60 * 1000,
});
