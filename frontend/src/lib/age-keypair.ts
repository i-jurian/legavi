import { x25519 } from "@noble/curves/ed25519.js";
import { bech32 } from "@scure/base";

export type AgeKeys = {
  identity: string; // private key, format: AGE-SECRET-KEY-1...
  recipient: string; // public key, format: age1...
};

// Derives a deterministic age X25519 keypair from a 32-byte seed.
export function deriveAgeKeypair(seed: Uint8Array): AgeKeys {
  if (seed.length !== 32) {
    throw new Error(`seed must be 32 bytes, got ${seed.length}`);
  }

  // seed bytes are the private key; scalarMultBase derives its matching public key
  const publicKey = x25519.scalarMultBase(seed);

  // age uses bech32 to wrap both keys in human-readable strings,
  // identity is uppercased per the age spec
  return {
    identity: bech32
      .encode("AGE-SECRET-KEY-", bech32.toWords(seed), false)
      .toUpperCase(),
    recipient: bech32.encode("age", bech32.toWords(publicKey), false),
  };
}
