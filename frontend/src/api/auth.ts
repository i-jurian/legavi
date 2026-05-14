import type {
  AuthenticationResponseJSON,
  PublicKeyCredentialCreationOptionsJSON,
  PublicKeyCredentialRequestOptionsJSON,
  RegistrationResponseJSON,
} from "@simplewebauthn/browser";
import { lockAndLogout } from "@/lib/session";

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

type Me = {
  id: string;
  email: string;
  displayName: string;
};

const BASE = "/api/v1/auth";

async function authFetch(
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
  const res = await authFetch(`${BASE}/me`, { method: "GET" });
  if (!res.ok) {
    throw new Error(`me failed: ${res.status}`);
  }
  return res.json();
}
