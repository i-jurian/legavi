import {
  base64URLStringToBuffer,
  type AuthenticationResponseJSON,
  type RegistrationResponseJSON,
} from "@simplewebauthn/browser";

export function readPRFFirst(
  response: RegistrationResponseJSON | AuthenticationResponseJSON,
): Uint8Array | undefined {
  const first = response.clientExtensionResults.prf?.results?.first;
  if (!first) return undefined;
  if (first instanceof Uint8Array) return first;
  if (first instanceof ArrayBuffer) return new Uint8Array(first);
  return new Uint8Array(first.buffer, first.byteOffset, first.byteLength);
}

export function decodePRFInput(options: { extensions?: unknown }): void {
  const ev = (
    options.extensions as { prf?: { eval?: { first?: unknown } } } | undefined
  )?.prf?.eval;
  if (ev && typeof ev.first === "string") {
    ev.first = base64URLStringToBuffer(ev.first);
  }
}
