import type {
  AuthenticationResponseJSON,
  RegistrationResponseJSON,
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
