import type {
  AuthenticationResponseJSON,
  PublicKeyCredentialCreationOptionsJSON,
  PublicKeyCredentialRequestOptionsJSON,
  RegistrationResponseJSON,
} from "@simplewebauthn/browser";

const BASE = "/api/v1/auth";

type RegisterStartBody = {
  email: string;
  displayName: string;
};

type RegisterVerifyBody = {
  ageRecipient: string;
  nickname?: string;
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
