// @simplewebauthn/browser ships its own AuthenticationExtensionsClientOutputs without prf.
// This augments it, remove once they add prf upstream.

export {};

declare module "@simplewebauthn/browser" {
  interface AuthenticationExtensionsClientOutputs {
    prf?: AuthenticationExtensionsPRFOutputs;
  }
}
